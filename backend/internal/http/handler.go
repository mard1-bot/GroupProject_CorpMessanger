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
	rateLimits   = make(map[string]*rateLimitEntry)
	rateLimitMux sync.RWMutex
)

// Simple rate limiter: 5 requests per minute per IP
func rateLimitMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}

		rateLimitMux.Lock()
		entry, exists := rateLimits[ip]
		now := time.Now()

		if !exists || now.After(entry.resetTime) {
			rateLimits[ip] = &rateLimitEntry{
				count:     1,
				resetTime: now.Add(time.Minute),
			}
			rateLimitMux.Unlock()
			next.ServeHTTP(w, r)
			return
		}

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
		originsMap[strings.ToLower(origin)] = true
	}

	h := &Handler{
		logger:          logger,
		storage:         storage,
		ejabberd:        ejabberd,
		jwt:             auth.NewJWTService(jwtSecret),
		corsOrigins:     originsMap,
		sessionDuration: sessionDuration,
	}

	// Clean up old rate limit entries periodically
	go func() {
		for {
			time.Sleep(10 * time.Minute)
			now := time.Now()
			rateLimitMux.Lock()
			for ip, entry := range rateLimits {
				if now.After(entry.resetTime) {
					delete(rateLimits, ip)
				}
			}
			rateLimitMux.Unlock()
		}
	}()

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
