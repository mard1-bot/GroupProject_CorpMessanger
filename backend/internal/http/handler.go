package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	stdhttp "net/http"
	"strings"
	"sync"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/ejabberd"
	"corp-messenger/backend/internal/notifications"
	"corp-messenger/backend/internal/storage"
	"corp-messenger/backend/internal/websocket"

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
	hub             *websocket.Hub
	notificationSvc *notifications.Service
	baseURL         string
}

// Rate limiting
type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

var (
	rateLimits           = make(map[string]*rateLimitEntry)
	rateLimitMux         sync.RWMutex
	maxRateLimitEntries  = 10000 // Hard limit to prevent memory exhaustion (default, can be configured)
	rateLimitStopChan    = make(chan struct{})
	rateLimitCleanupOnce sync.Once
	rateLimitRequests    = 20 // Default: 20 requests per window
	rateLimitWindow      = 60 // Default: 60 seconds
)

// StartRateLimitCleanup starts a background goroutine that periodically cleans up expired rate limit entries
func StartRateLimitCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rateLimitMux.Lock()
				now := time.Now()
				for k, v := range rateLimits {
					if now.After(v.resetTime) {
						delete(rateLimits, k)
					}
				}
				rateLimitMux.Unlock()
			case <-rateLimitStopChan:
				return
			}
		}
	}()
}

// StopRateLimitCleanup stops the rate limit cleanup goroutine
func StopRateLimitCleanup() {
	rateLimitCleanupOnce.Do(func() {
		close(rateLimitStopChan)
	})
}

// getClientIP extracts client IP with proxy-aware validation
func getClientIP(r *stdhttp.Request) string {
	// Prefer X-Forwarded-For but only take first IP (closest to client)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take only the first IP in the chain
		if idx := strings.Index(forwarded, ","); idx != -1 {
			forwarded = forwarded[:idx]
		}
		forwarded = strings.TrimSpace(forwarded)
		// Basic validation: must look like a valid IP
		if isValidIP(forwarded) {
			return forwarded
		}
	}
	// Fall back to RemoteAddr, stripping port if present
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	// Validate the extracted IP
	if isValidIP(ip) {
		return ip
	}
	// If all else fails, use a sentinel value to still rate limit invalid IPs
	// This prevents bypassing rate limiting with malformed IP addresses
	return "invalid_ip"
}

// isValidIP performs basic validation that a string looks like an IP address
func isValidIP(ip string) bool {
	if ip == "" {
		return false
	}
	// Handle IPv6 zone IDs (e.g., fe80::1%eth0)
	zoneIdx := strings.Index(ip, "%")
	if zoneIdx != -1 {
		ip = ip[:zoneIdx]
	}
	// Handle IPv6 addresses in brackets (e.g., [::1])
	if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") {
		ip = ip[1 : len(ip)-1]
	}
	// Basic check: IPv4 has dots, IPv6 has colons
	hasDots := strings.Contains(ip, ".")
	hasColons := strings.Contains(ip, ":")
	if !hasDots && !hasColons {
		return false
	}
	// Should not have both (mixed format is invalid)
	if hasDots && hasColons {
		return false
	}
	// Basic length checks (after stripping zone ID and brackets)
	if len(ip) < 7 || len(ip) > 45 { // Min IPv4: 1.1.1.1 (7), Max IPv6: 45 chars
		return false
	}
	// Should only contain valid IP characters
	for _, c := range ip {
		valid := (c >= '0' && c <= '9') || c == '.' || c == ':' || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !valid {
			return false
		}
	}
	return true
}

// Simple rate limiter: configurable requests per window per IP
func rateLimitMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		ip := getClientIP(r)
		now := time.Now()

		rateLimitMux.Lock()
		entry, exists := rateLimits[ip]
		if !exists || now.After(entry.resetTime) {
			rateLimits[ip] = &rateLimitEntry{
				count:     1,
				resetTime: now.Add(time.Duration(rateLimitWindow) * time.Second),
			}
			rateLimitMux.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		entry.count++
		rateLimits[ip] = entry

		// Prevent memory exhaustion by limiting number of tracked IPs
		if len(rateLimits) > maxRateLimitEntries {
			// First, remove expired entries
			now := time.Now()
			for k, v := range rateLimits {
				if now.After(v.resetTime) {
					delete(rateLimits, k)
				}
			}
			// If still over capacity, remove oldest entries by reset time
			if len(rateLimits) > maxRateLimitEntries {
				type ipTime struct {
					ip        string
					resetTime time.Time
				}
				entries := make([]ipTime, 0, len(rateLimits))
				for k, v := range rateLimits {
					entries = append(entries, ipTime{ip: k, resetTime: v.resetTime})
				}
				// Sort by reset time (oldest first)
				for i := 0; i < len(entries); i++ {
					for j := i + 1; j < len(entries); j++ {
						if entries[i].resetTime.After(entries[j].resetTime) {
							entries[i], entries[j] = entries[j], entries[i]
						}
					}
				}
				// Remove oldest 25% of entries
				toRemove := len(entries) / 4
				if toRemove < 1 {
					toRemove = 1
				}
				for i := 0; i < toRemove; i++ {
					delete(rateLimits, entries[i].ip)
				}
			}
		}

		count := entry.count
		rateLimitMux.Unlock()

		if count > rateLimitRequests {
			WriteError(w, stdhttp.StatusTooManyRequests, "rate_limited", "Too many requests, please try again later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func NewHandler(logger *slog.Logger, storage storage.Storage, ejabberd ejabberd.Client, jwtSecret string, corsOrigins []string, sessionDuration time.Duration, hub *websocket.Hub, notificationSvc *notifications.Service, baseURL string, requests, window, entries int) stdhttp.Handler {
	// Set configurable rate limit values
	if requests > 0 {
		rateLimitRequests = requests
	}
	if window > 0 {
		rateLimitWindow = window
	}
	if entries > 0 {
		maxRateLimitEntries = entries
	}
	// Build CORS origins map (lowercase for case-insensitive comparison)
	originsMap := make(map[string]bool)
	for _, origin := range corsOrigins {
		origin = strings.ToLower(origin)
		// Validate origin format: must start with http:// or https:// and not contain path
		if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
			// Strip any path component - only keep scheme://host:port
			// Handle both http:// (7 chars) and https:// (8 chars)
			schemeLen := 7
			if strings.HasPrefix(origin, "https://") {
				schemeLen = 8
			}
			if idx := strings.Index(origin[schemeLen:], "/"); idx != -1 {
				origin = origin[:idx+schemeLen]
			}
			originsMap[origin] = true
		}
	}

	// Set allowed origins for WebSocket origin validation
	websocket.AllowedOrigins = originsMap

	h := &Handler{
		logger:          logger,
		storage:         storage,
		ejabberd:        ejabberd,
		jwt:             auth.NewJWTService(jwtSecret),
		corsOrigins:     originsMap,
		sessionDuration: sessionDuration,
		hub:             hub,
		notificationSvc: notificationSvc,
		baseURL:         baseURL,
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(h.corsMiddleware)
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoverMiddleware(logger))
	r.Use(middleware.StripSlashes)
	r.Use(AuditMiddleware(storage, logger))

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	// WebSocket endpoint - auth handled in handler (supports token in query param)
	r.Get("/ws", h.handleWebSocket)

	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints with rate limiting
		r.With(rateLimitMiddleware).Post("/auth/register", h.register)
		r.With(rateLimitMiddleware).Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(h.jwt, h.storage))

			r.Post("/auth/logout", h.logout)
			r.Get("/auth/me", h.getCurrentUser)

			r.Get("/users", h.getUsers)
			r.Put("/users/me/status", h.updateUserStatus)
			r.Post("/users/me/avatar", h.uploadAvatar)
			r.Get("/users/me/mentions", h.getUserMentions)

			r.Post("/chats", h.createChat)
			r.Get("/chats", h.getUserChats)
			r.Get("/chats/{id}", h.getChatByID)
			r.Delete("/chats/{id}", h.deleteChat)
			r.Post("/chats/{id}/members", h.addChatMember)
			r.Delete("/chats/{id}/members/{userID}", h.removeChatMember)
			r.Post("/chats/{id}/mute", h.muteChat)
			r.Post("/chats/{id}/unmute", h.unmuteChat)
			r.Post("/chats/{id}/pin", h.pinChat)
			r.Post("/chats/{id}/unpin", h.unpinChat)
			r.Post("/chats/{id}/archive", h.archiveChat)
			r.Post("/chats/{id}/unarchive", h.unarchiveChat)
			r.Delete("/chats/{id}/me", h.softDeleteChat)
			r.Delete("/chats/{id}/history", h.clearChatHistory)
			r.Get("/chats/{id}/export", h.exportChat)
			r.Post("/chats/{id}/avatar", h.uploadChatAvatar)

			r.Post("/messages/{id}/reply", h.replyMessage)
			r.Post("/messages/{id}/forward", h.forwardMessage)
			r.Get("/messages/{id}/reactions", h.getMessageReactions)
			r.Post("/reactions", h.addReaction)
			r.Delete("/reactions", h.removeReaction)

			r.Post("/typing", h.sendTypingIndicator)

			r.Post("/chats/{id}/messages", h.sendMessage)
			r.Get("/chats/{id}/messages", h.getChatMessages)
			r.Get("/chats/{id}/messages/search", h.searchMessages)
			r.Get("/messages/search", h.searchAllMessages)
			r.Put("/chats/{id}/messages/{msgID}", h.editMessage)
			r.Delete("/chats/{id}/messages/{msgID}", h.deleteMessage)
			r.Post("/chats/{id}/messages/{msgID}/files", h.uploadFile)

			// Notifications
			r.Post("/devices/register", h.registerDevice)
			r.Get("/notifications/settings", h.getNotificationSettings)
			r.Put("/notifications/settings", h.updateNotificationSettings)
			r.Get("/notifications/unread", h.getUnreadCount)

			// E2E Encryption
			r.Post("/encryption/keys", h.registerPublicKey)
			r.Get("/encryption/keys/me", h.getMyPublicKey)
			r.Get("/encryption/keys/user", h.getUserPublicKey)
			r.Get("/encryption/keys/chat", h.getChatPublicKeys)

			// Bookmarks
			r.Post("/bookmarks", h.addBookmark)
			r.Delete("/bookmarks/{id}", h.removeBookmark)
			r.Get("/bookmarks", h.getBookmarks)

			// Blocking
			r.Post("/users/block", h.blockUser)
			r.Delete("/users/block/{id}", h.unblockUser)
			r.Get("/users/blocked", h.getBlockedUsers)

			// Audit logs (admin only)
			r.With(AdminOnly).Get("/audit/logs", h.getAuditLogs)
			r.With(AdminOnly).Get("/audit/logs/all", h.getAllAuditLogs)
		})
	})

	// Static file server for uploads
	fileServer := http.FileServer(http.Dir("./uploads"))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", fileServer))

	r.NotFound(h.notFound)

	return r
}

func (h *Handler) corsMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		origin := strings.ToLower(r.Header.Get("Origin"))

		// Validate origin format - must start with http:// or https:// and not contain path
		if origin != "" {
			hasPath := false
			if strings.HasPrefix(origin, "http://") {
				hasPath = strings.Contains(origin[7:], "/")
			} else if strings.HasPrefix(origin, "https://") {
				hasPath = strings.Contains(origin[8:], "/")
			} else {
				// Invalid scheme
				origin = ""
			}
			if hasPath {
				// Invalid origin format - treat as no origin
				origin = ""
			}
		}

		// Only set CORS headers for whitelisted origins (case-insensitive)
		if h.corsOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
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

// handleWebSocket handles WebSocket upgrade requests.
func (h *Handler) handleWebSocket(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	// Handle CORS for WebSocket preflight
	origin := strings.ToLower(r.Header.Get("Origin"))
	if h.corsOrigins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	}

	// Handle preflight request
	if r.Method == "OPTIONS" {
		w.WriteHeader(stdhttp.StatusOK)
		return
	}

	// Get claims from context (set by AuthMiddleware) or from query parameter
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		// Try to get token from query parameter (for WebSocket connections)
		token := r.URL.Query().Get("token")
		if token == "" {
			WriteError(w, stdhttp.StatusUnauthorized, "unauthorized", "Authentication required for WebSocket")
			return
		}
		var err error
		claims, err = h.jwt.ParseToken(token)
		if err != nil {
			WriteError(w, stdhttp.StatusUnauthorized, "invalid_token", "Invalid or expired token")
			return
		}
	}

	if err := websocket.ServeWs(h.hub, w, r, claims.UserID); err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		WriteError(w, stdhttp.StatusInternalServerError, "websocket_error", "Failed to upgrade connection")
		return
	}
}
