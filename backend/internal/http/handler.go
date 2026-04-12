package http

import (
	"encoding/json"
	"io"
	"log/slog"
	stdhttp "net/http"
	"strings"
	"sync"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/ejabberd"
	"corp-messenger/backend/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Handler struct {
	logger          *slog.Logger
	storage         storage.Storage
	ejabberd        ejabberd.Client
	jwt             *auth.JWTService
	corsOrigins     map[string]bool
	sessionDuration time.Duration
}

// Rate limiting
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

var (
	rateLimits          = make(map[string]*rateLimitEntry)
	rateLimitMux        sync.RWMutex
	maxRateLimitEntries = 10000 // Hard limit to prevent memory exhaustion
)

// getClientIP extracts client IP with proxy-aware validation
func getClientIP(r *stdhttp.Request) string {
	// Prefer X-Forwarded-For but only take first IP (closest to client)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take only the first IP in the chain
		if idx := strings.Index(forwarded, ","); idx != -1 {
			forwarded = forwarded[:idx]
		}
		forwarded = strings.TrimSpace(forwarded)
		// Basic validation: must look like IP
		if strings.Contains(forwarded, ".") || strings.Contains(forwarded, ":") {
			return forwarded
		}
	}
	// Fall back to RemoteAddr, stripping port if present
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// Simple rate limiter: 5 requests per minute per IP
func rateLimitMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		ip := getClientIP(r)
		now := time.Now()

		rateLimitMux.Lock()

		// Clean up expired entry if exists
		entry, exists := rateLimits[ip]
		if exists && now.After(entry.resetTime) {
			delete(rateLimits, ip)
			exists = false
		}

		// Create new entry if needed, with size limit enforcement
		if !exists {
			// Evict oldest entries if at capacity (simple eviction: clear expired entries first)
			if len(rateLimits) >= maxRateLimitEntries {
				for k, v := range rateLimits {
					if now.After(v.resetTime) {
						delete(rateLimits, k)
					}
				}
				// If still at capacity, refuse new entries (defensive)
				if len(rateLimits) >= maxRateLimitEntries {
					rateLimitMux.Unlock()
					WriteError(w, stdhttp.StatusTooManyRequests, "rate_limited", "Server overloaded, please try again later")
					return
				}
			}

			rateLimits[ip] = &rateLimitEntry{
				count:     1,
				resetTime: now.Add(time.Minute),
			}
			rateLimitMux.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		// Increment count atomically under lock
		entry.count++
		count := entry.count
		rateLimitMux.Unlock()

		if count > 5 {
			WriteError(w, stdhttp.StatusTooManyRequests, "rate_limited", "Too many requests, please try again later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func NewHandler(logger *slog.Logger, storage storage.Storage, ejabberd ejabberd.Client, jwtSecret string, corsOrigins []string, sessionDuration time.Duration) stdhttp.Handler {
	// Build CORS origins map (lowercase for case-insensitive comparison)
	originsMap := make(map[string]bool)
	for _, origin := range corsOrigins {
		origin = strings.ToLower(origin)
		// Validate origin format: must start with http:// or https:// and not contain path
		if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
			// Strip any path component - only keep scheme://host:port
			if idx := strings.Index(origin[8:], "/"); idx != -1 {
				origin = origin[:idx+8]
			}
			originsMap[origin] = true
		}
	}

	h := &Handler{
		logger:          logger,
		storage:         storage,
		ejabberd:        ejabberd,
		jwt:             auth.NewJWTService(jwtSecret),
		corsOrigins:     originsMap,
		sessionDuration: sessionDuration,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(h.corsMiddleware)
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))
	r.Use(middleware.StripSlashes)

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints with rate limiting
		r.With(rateLimitMiddleware).Post("/auth/register", h.register)
		r.With(rateLimitMiddleware).Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(h.jwt, h.storage))

			r.Post("/auth/logout", h.logout)
			r.Get("/auth/me", h.getCurrentUser)

			r.Get("/users", h.getUsers)
			r.Get("/users/{id}", h.getUserByID)
			r.Put("/users/me", h.updateCurrentUser)

			r.Post("/chats", h.createChat)
			r.Get("/chats", h.getUserChats)
			r.Get("/chats/{id}", h.getChatByID)
			r.Post("/chats/{id}/members", h.addChatMember)

			r.Post("/chats/{id}/messages", h.sendMessage)
			r.Get("/chats/{id}/messages", h.getChatMessages)
		})
	})

	r.NotFound(h.notFound)

	return r
}

func (h *Handler) corsMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		origin := strings.ToLower(r.Header.Get("Origin"))

		// Validate origin format - must start with http:// or https:// and not contain path
		if origin != "" && (!strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") ||
			strings.Contains(origin[7:], "/") || // path after http://
			strings.Contains(origin[8:], "/")) { // path after https://
			// Invalid origin format - treat as no origin
			origin = ""
		}

		// Only set CORS headers for whitelisted origins (case-insensitive)
		if h.corsOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours cache for preflight
		}
		// For requests without origin or unknown origins, no CORS headers are set
		// This is more secure than wildcard

		if r.Method == "OPTIONS" {
			w.WriteHeader(stdhttp.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func decodeJSON(r io.Reader, v interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}
