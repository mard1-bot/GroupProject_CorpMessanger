package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/crypto"
	"corp-messenger/backend/internal/ejabberd"
	"corp-messenger/backend/internal/livekit"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/notifications"
	"corp-messenger/backend/internal/services"
	"corp-messenger/backend/internal/storage"
	"corp-messenger/backend/internal/websocket"
	"corp-messenger/backend/internal/xmppsync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
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
	syncService     *xmppsync.SyncService
	turnServerURI   string
	turnUsername    string
	turnPassword    string
	liveKit         *livekit.Service
	rateLimiter     *RedisRateLimiter

	messageService services.MessageService
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

	maxFailedAttempts = 5                // Lock account after 5 failed attempts
	lockoutDuration   = 15 * time.Minute // Lock for 15 minutes
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

// sanitizeInput sanitizes user input to prevent XSS and injection attacks
func sanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	// Limit length to prevent DoS
	if len(input) > 10000 {
		input = input[:10000]
	}

	return input
}

// trackFailedLoginAttempt tracks failed login attempts for account lockout using database
func (h *Handler) trackFailedLoginAttempt(ctx context.Context, email, ip string) error {
	return h.storage.RecordLoginAttempt(ctx, email, ip, nil, false)
}

// isAccountLocked checks if an account is currently locked using database
func (h *Handler) isAccountLocked(ctx context.Context, email string) (bool, error) {
	// Check failed attempts in the last 15 minutes
	since := time.Now().Add(-lockoutDuration)
	count, err := h.storage.GetFailedLoginAttempts(ctx, email, since)
	if err != nil {
		return false, err
	}

	return count >= maxFailedAttempts, nil
}

// logAudit creates an audit log entry
func (h *Handler) logAudit(ctx context.Context, userID uuid.UUID, action, resource string, details map[string]interface{}) {
	// Serialize details to JSON
	detailsJSON := ""
	if details != nil {
		if b, err := json.Marshal(details); err == nil {
			detailsJSON = string(b)
		}
	}

	log := &models.AuditLog{
		UserID:   userID,
		Action:   action,
		Resource: resource,
		Details:  detailsJSON,
	}

	// Log asynchronously to not block the request
	go func() {
		if err := h.storage.CreateAuditLog(context.Background(), log); err != nil {
			h.logger.Error("failed to create audit log", "error", err, "user_id", userID, "action", action)
		}
	}()
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
	// Use net.ParseIP for proper IP validation
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil
}

// Redis-based rate limiter: configurable requests per window per IP
func (h *Handler) rateLimitMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		ip := getClientIP(r)

		// Check rate limit using Redis
		allowed, remaining, resetTime, err := h.rateLimiter.CheckRateLimit(r.Context(), ip, rateLimitRequests, rateLimitWindow)
		if err != nil {
			// If Redis is down, allow the request but log error
			h.logger.Error("Rate limiter error", "error", err, "ip", ip)
			next.ServeHTTP(w, r)
			return
		}

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rateLimitRequests))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if !allowed {
			WriteErrorCode(w, stdhttp.StatusTooManyRequests, "rate_limited", "Too many requests, please try again later")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func NewHandler(logger *slog.Logger, storage storage.Storage, ejabberd ejabberd.Client, jwtSecret string, corsOrigins []string, sessionDuration time.Duration, hub *websocket.Hub, notificationSvc *notifications.Service, baseURL string, requests, window, entries int, syncService *xmppsync.SyncService, turnServerURI, turnUsername, turnPassword string, liveKit *livekit.Service, redisURL string) stdhttp.Handler {
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
		// Validate origin format using URL parsing
		parsedURL, err := url.Parse(origin)
		if err != nil {
			continue // Skip invalid origins
		}
		// Only allow http and https schemes
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			continue
		}
		// Strip path and query components - only keep scheme://host:port
		normalizedOrigin := parsedURL.Scheme + "://" + parsedURL.Host
		originsMap[normalizedOrigin] = true
	}

	// Set allowed origins for WebSocket origin validation
	websocket.AllowedOrigins = originsMap

	// Initialize Redis rate limiter
	if redisURL == "" {
		redisURL = "redis:6379" // Default for local development
	}
	rateLimiter := NewRedisRateLimiter(redisURL)

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
		syncService:     syncService,
		turnServerURI:   turnServerURI,
		turnUsername:    turnUsername,
		turnPassword:    turnPassword,
		liveKit:         liveKit,
		rateLimiter:     rateLimiter,
		messageService:  services.NewMessageService(logger, storage, hub, notificationSvc, syncService),
	}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(SecurityHeadersMiddleware)
	r.Use(CSRFProtectionMiddleware)
	r.Use(h.corsMiddleware)
	r.Use(ValidationMiddleware)
	r.Use(AccessLogMiddleware(logger))
	r.Use(RecoveryMiddleware(logger))
	r.Use(middleware.StripSlashes)

	r.Get("/health", h.health)
	r.Get("/ready", h.ready)

	// WebSocket endpoint - auth handled in handler (supports token in query param)
	r.Get("/ws", h.handleWebSocket)

	// Serve uploaded files — decrypt on-the-fly if encryption is enabled
	r.Get("/uploads/{filename}", h.serveUploadedFile)

	r.Route("/api/v1", func(r chi.Router) {
		// Auth endpoints with rate limiting
		r.With(h.rateLimitMiddleware).Post("/auth/register", h.register)
		r.With(h.rateLimitMiddleware).Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(h.jwt, h.storage))
			r.Use(AuditMiddleware(storage, logger))

			r.Post("/auth/logout", h.logout)
			r.Get("/auth/me", h.getCurrentUser)
			r.Post("/auth/change-password", h.changePassword)
			r.Delete("/auth/me", h.deleteUser)
			r.Get("/webrtc/turn", h.getTurnConfig)

			r.Get("/users", h.getUsers)
			r.Get("/users/{id}", h.getUserByID)
			r.Put("/users/me/status", h.updateUserStatus)
			r.Post("/users/me/avatar", h.uploadAvatar)
			r.Get("/users/me/mentions", h.getUserMentions)

			// 2FA routes (admin only)
			r.Route("/2fa", func(r chi.Router) {
				r.Use(AdminOnly)
				r.Post("/setup", h.setupTwoFactor)
				r.Post("/enable", h.enableTwoFactor)
				r.Post("/disable", h.disableTwoFactor)
				r.Post("/verify", h.verifyTwoFactor)
				r.Get("/status", h.getTwoFactorStatus)
				r.Post("/backup-codes/regenerate", h.regenerateBackupCodes)
			})

			// Admin routes (admin only)
			r.Route("/admin", func(r chi.Router) {
				r.Use(AdminOnly)
				r.Get("/stats", h.adminGetStats)
				r.Get("/users", h.adminGetUsers)
				r.Get("/users/{id}", h.adminGetUserByID)
				r.Post("/users", h.adminCreateUser)
				r.Put("/users/{id}", h.adminUpdateUser)
				r.Put("/users/{id}/role", h.adminChangeRole)
				r.Post("/users/{id}/block", h.adminBlockUser)
				r.Post("/users/{id}/unblock", h.adminUnblockUser)
				r.Delete("/users/{id}", h.adminDeleteUser)
				r.Post("/users/{id}/reset-password", h.adminResetPassword)
				r.Get("/audit-logs", h.adminGetAuditLogs)
				r.Delete("/audit-logs", h.deleteAuditLogs)
			})

			r.Post("/chats", h.createChat)
			r.Get("/chats", h.getUserChats)
			// Chat-specific routes using middleware to ensure user is a chat member
			r.Route("/chats/{id}", func(r chi.Router) {
				r.Use(RequireChatMemberMiddleware(h.storage, h.logger))

				r.Get("/export", h.exportChat)
				r.Post("/avatar", h.uploadChatAvatar)
				r.Get("/", h.getChatByID)
				r.Put("/", h.updateChat)
				r.Delete("/", h.deleteChat)
				r.Post("/members", h.addChatMember)
				r.Delete("/members/{userID}", h.removeChatMember)
				r.Post("/mute", h.muteChat)
				r.Post("/unmute", h.unmuteChat)
				r.Post("/pin", h.pinChat)
				r.Post("/unpin", h.unpinChat)
				r.Post("/archive", h.archiveChat)
				r.Post("/unarchive", h.unarchiveChat)
				r.Delete("/me", h.softDeleteChat)
				r.Delete("/history", h.clearChatHistory)

				r.Post("/messages", h.sendMessage)
				r.Get("/messages", h.getChatMessages)
				r.Get("/messages/search", h.searchMessages)
				r.Put("/messages/{msgID}", h.editMessage)
				r.Delete("/messages/{msgID}", h.deleteMessage)
				r.Post("/messages/{msgID}/files", h.uploadFile)
				r.Post("/messages/pin", h.pinMessage)
				r.Post("/messages/unpin", h.unpinMessage)
				r.Get("/messages/pinned", h.getPinnedMessages)
			})

			r.Post("/messages/{id}/reply", h.replyMessage)
			r.Post("/messages/{id}/forward", h.forwardMessage)
			r.Post("/messages/{id}/thread", h.startThread)
			r.Get("/threads/{id}", h.getThreadMessages)
			r.Get("/messages/{id}/reactions", h.getMessageReactions)
			r.Post("/reactions", h.addReaction)
			r.Delete("/reactions", h.removeReaction)

			r.Post("/typing", h.sendTypingIndicator)

			r.Get("/messages/search", h.searchAllMessages)

			// Notifications
			r.Post("/devices/register", h.registerDevice)
			r.Get("/notifications/settings", h.getNotificationSettings)
			r.Put("/notifications/settings", h.updateNotificationSettings)
			r.Get("/notifications/unread", h.getUnreadCount)
			r.Post("/notifications/web-push", h.registerWebPushSubscription)
			r.Delete("/notifications/web-push", h.unregisterWebPushSubscription)

			// E2E Encryption
			r.Post("/encryption/keys", h.registerPublicKey)
			r.Get("/encryption/keys/me", h.getMyPublicKey)
			r.Get("/encryption/keys/user", h.getUserPublicKey)
			r.Get("/encryption/keys/chat", h.getChatPublicKeys)

			// Bookmarks
			r.Post("/bookmarks", h.addBookmark)
			r.Delete("/bookmarks/{id}", h.removeBookmark)
			r.Get("/bookmarks", h.getBookmarks)

			r.NotFound(h.notFound)
		})
	})

	return r
}

// serveUploadedFile serves files from the uploads directory, decrypting them
// on-the-fly with AES-256-GCM when the master encryption key is available.
func (h *Handler) serveUploadedFile(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	filename := chi.URLParam(r, "filename")
	if filename == "" {
		WriteErrorCode(w, stdhttp.StatusBadRequest, "invalid_request", "Filename is required")
		return
	}
	// Prevent path traversal
	filename = filepath.Base(filename)

	filePath := filepath.Join("./uploads", filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			WriteErrorCode(w, stdhttp.StatusNotFound, "not_found", "File not found")
		} else {
			h.logger.Error("failed to read uploaded file", "error", err, "filename", filename)
			WriteErrorCode(w, stdhttp.StatusInternalServerError, "internal", "Failed to read file")
		}
		return
	}

	// Attempt decryption if master key is available; fall back to raw bytes otherwise
	plaintext := data
	if crypto.IsInitialized() {
		if decrypted, decErr := crypto.DecryptBytes(data); decErr == nil {
			plaintext = decrypted
		}
		// If decryption fails the file was likely stored before encryption was enabled —
		// serve the raw bytes so existing files remain accessible.
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	mimeTypes := map[string]string{
		"jpg": "image/jpeg", "jpeg": "image/jpeg", "png": "image/png",
		"gif": "image/gif", "webp": "image/webp", "bmp": "image/bmp",
		"pdf": "application/pdf", "txt": "text/plain", "mp3": "audio/mpeg",
		"mp4": "video/mp4", "webm": "video/webm", "ogg": "audio/ogg",
	}
	contentType := mimeTypes[ext]
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "inline; filename=\""+filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(plaintext)))
	w.WriteHeader(stdhttp.StatusOK)
	w.Write(plaintext) //nolint:errcheck
}

func (h *Handler) corsMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		origin := strings.ToLower(r.Header.Get("Origin"))

		// Validate and normalize origin format using URL parsing
		normalizedOrigin := ""
		if origin != "" {
			parsedURL, err := url.Parse(origin)
			if err == nil {
				// Only allow http and https schemes
				if parsedURL.Scheme == "http" || parsedURL.Scheme == "https" {
					normalizedOrigin = parsedURL.Scheme + "://" + parsedURL.Host
				}
			}
		}

		// Only set CORS headers for whitelisted origins (case-insensitive)
		if h.corsOrigins[normalizedOrigin] {
			w.Header().Set("Access-Control-Allow-Origin", normalizedOrigin)
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

func (h *Handler) getTurnConfig(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, stdhttp.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Return TURN server configuration with credentials
	// Note: For production, implement TURN REST API for temporary credentials
	config := map[string]interface{}{}
	if h.turnServerURI != "" {
		// Validate that TURN credentials are actually configured
		if h.turnUsername == "" || h.turnPassword == "" {
			h.logger.Error("TURN server URI configured but credentials are missing")
			WriteErrorCode(w, stdhttp.StatusServiceUnavailable, "turn_misconfigured", "TURN server is not properly configured")
			return
		}
		// Generate TURN REST API credentials
		// turnPassword acts as the shared secret configured in coturn (static-auth-secret)
		ttl := 24 * time.Hour
		timestamp := time.Now().Add(ttl).Unix()

		var userID string
		if claims != nil {
			userID = claims.UserID.String()
		} else {
			userID = "guest"
		}

		turnUser := fmt.Sprintf("%d:%s", timestamp, userID)
		mac := hmac.New(sha1.New, []byte(h.turnPassword))
		mac.Write([]byte(turnUser))
		turnPass := base64.StdEncoding.EncodeToString(mac.Sum(nil))

		config["turn_server_uri"] = h.turnServerURI
		config["turn_username"] = turnUser
		config["turn_password"] = turnPass
	}

	WriteJSON(w, stdhttp.StatusOK, map[string]interface{}{
		"user_id": claims.UserID,
		"turn":    config,
	})
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
			WriteErrorCode(w, stdhttp.StatusUnauthorized, "unauthorized", "Authentication required for WebSocket")
			return
		}
		var err error
		claims, err = h.jwt.ParseToken(token)
		if err != nil {
			WriteErrorCode(w, stdhttp.StatusUnauthorized, "invalid_token", "Invalid or expired token")
			return
		}
	}

	if err := websocket.ServeWs(h.hub, w, r, claims.UserID); err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		WriteErrorCode(w, stdhttp.StatusInternalServerError, "websocket_error", "Failed to upgrade connection")
		return
	}
}
