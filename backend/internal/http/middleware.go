package http

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net"
	stdhttp "net/http"
	"strings"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	ChatMembersKey  contextKey = "chat_members"
	ChatUserRoleKey contextKey = "chat_user_role"
)

type responseWriter struct {
	stdhttp.ResponseWriter
	status  int
	written bool
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
		if rw.status == 0 {
			rw.status = stdhttp.StatusOK
		}
	}
	return rw.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(stdhttp.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}
func AccessLogMiddleware(logger *slog.Logger) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			startedAt := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: stdhttp.StatusOK}
			next.ServeHTTP(rw, r)
			attrs := []any{"method", r.Method, "path", r.URL.Path, "status", rw.status, "duration", time.Since(startedAt).String()}
			switch {
			case rw.status >= 500:
				logger.Error("request completed", attrs...)
			case rw.status >= 400:
				logger.Warn("request completed", attrs...)
			default:
				logger.Info("request completed", attrs...)
			}
		})
	}
}

// SecurityHeadersMiddleware adds security headers to all responses
func SecurityHeadersMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Content Security Policy
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none';")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// XSS Protection
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (formerly Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Strict-Transport-Security (HSTS) - only add if HTTPS
		if r.URL.Scheme == "https" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		next.ServeHTTP(w, r)
	})
}

// CSRFProtectionMiddleware adds CSRF protection for state-changing requests.
// Requests with JWT Bearer tokens in the Authorization header are inherently
// CSRF-safe because browsers do not automatically attach custom headers.
// For other requests, we require X-Requested-With header.
func CSRFProtectionMiddleware(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Skip CSRF for GET, HEAD, OPTIONS, TRACE (safe methods)
		if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" || r.Method == "TRACE" {
			next.ServeHTTP(w, r)
			return
		}

		// Requests with Bearer token in Authorization header are CSRF-safe
		// because browsers cannot automatically attach custom Authorization headers
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		// For non-Bearer requests, require X-Requested-With header
		if r.Header.Get("X-Requested-With") == "" {
			WriteErrorCode(w, stdhttp.StatusForbidden, "csrf_failed", "CSRF validation failed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequireChatMemberMiddleware verifies that the authenticated user is a member of the chat
// specified by the "id" URL parameter. It adds the chat members and the user's role to the context.
func RequireChatMemberMiddleware(stg storage.Storage, logger *slog.Logger) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				WriteErrorCode(w, stdhttp.StatusUnauthorized, "unauthorized", "Unauthorized")
				return
			}

			chatIDStr := chi.URLParam(r, "id")
			if chatIDStr == "" {
				// Fallback if parameter is named differently in some routes
				chatIDStr = chi.URLParam(r, "chat_id")
			}

			if chatIDStr == "" {
				WriteErrorCode(w, stdhttp.StatusBadRequest, "missing_id", "Chat ID is required")
				return
			}

			chatID, err := uuid.Parse(chatIDStr)
			if err != nil {
				WriteErrorCode(w, stdhttp.StatusBadRequest, "invalid_id", "Invalid chat ID")
				return
			}

			members, err := stg.GetChatMembers(r.Context(), chatID)
			if err != nil {
				logger.Error("failed to get chat members in middleware", "error", err, "chat_id", chatID)
				WriteErrorCode(w, stdhttp.StatusInternalServerError, "internal", "Failed to verify chat membership")
				return
			}

			isMember := false
			var userRole string
			for _, m := range members {
				if m.UserID == claims.UserID {
					isMember = true
					userRole = m.Role
					break
				}
			}

			if !isMember {
				WriteErrorCode(w, stdhttp.StatusForbidden, "forbidden", "You are not a member of this chat")
				return
			}

			ctx := context.WithValue(r.Context(), ChatMembersKey, members)
			ctx = context.WithValue(ctx, ChatUserRoleKey, userRole)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
