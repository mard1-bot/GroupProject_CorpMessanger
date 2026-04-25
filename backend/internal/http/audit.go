package http

import (
	"context"
	stdhttp "net/http"
	"strconv"
	"strings"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// AuditMiddleware logs user actions to the audit log
func AuditMiddleware(storage storage.Storage, logger *slog.Logger) func(stdhttp.Handler) stdhttp.Handler {
	return func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			startedAt := time.Now()

			// Wrap response writer to capture status code
			rw := &responseWriter{ResponseWriter: w, status: stdhttp.StatusOK}

			next.ServeHTTP(rw, r)

			// Get user claims if available
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok {
				// No user context, skip audit logging
				return
			}

			// Determine action based on method and path
			action := determineAction(r.Method, r.URL.Path)
			if action == "" {
				// Skip logging for non-auditable actions
				return
			}

			// Get IP address using the same function as rate limiting for consistency
			ipAddress := getClientIP(r)

			// Get user agent
			userAgent := r.Header.Get("User-Agent")

			// Create audit log entry
			log := &models.AuditLog{
				ID:         uuid.New(),
				UserID:     claims.UserID,
				Action:     action,
				Resource:   determineResource(r.URL.Path),
				ResourceID: extractResourceID(r.URL.Path),
				IPAddress:  ipAddress,
				UserAgent:  userAgent,
				CreatedAt:  startedAt,
			}

			// Log asynchronously to avoid blocking response
			// Copy values to avoid data race with request scope
			userID := claims.UserID
			actionName := action
			go func() {
				// Panic recovery to ensure goroutine always terminates
				defer func() {
					if r := recover(); r != nil {
						logger.Error("panic in audit log goroutine", "recover", r)
					}
				}()

				ctx := context.Background()
				if err := storage.CreateAuditLog(ctx, log); err != nil {
					logger.Error("failed to create audit log", "error", err, "user_id", userID, "action", actionName)
				}
			}()
		})
	}
}

// determineAction maps HTTP method and path to audit action
func determineAction(method, path string) string {
	// Skip GET requests for listing/viewing as they're less critical
	if method == "GET" && !strings.Contains(path, "/export") {
		return ""
	}

	// Map common patterns to actions
	switch {
	case method == "POST" && strings.Contains(path, "/auth/register"):
		return "user_registered"
	case method == "POST" && strings.Contains(path, "/auth/login"):
		return "user_logged_in"
	case method == "POST" && strings.Contains(path, "/auth/logout"):
		return "user_logged_out"
	case method == "POST" && strings.Contains(path, "/chats"):
		return "chat_created"
	case method == "DELETE" && strings.Contains(path, "/chats"):
		return "chat_deleted"
	case method == "POST" && strings.Contains(path, "/messages"):
		return "message_sent"
	case method == "PUT" && strings.Contains(path, "/messages"):
		return "message_updated"
	case method == "DELETE" && strings.Contains(path, "/messages"):
		return "message_deleted"
	case method == "POST" && strings.Contains(path, "/users/block"):
		return "user_blocked"
	case method == "DELETE" && strings.Contains(path, "/users/block"):
		return "user_unblocked"
	case method == "POST" && strings.Contains(path, "/export"):
		return "chat_exported"
	case method == "DELETE" && strings.Contains(path, "/history"):
		return "chat_history_cleared"
	default:
		return ""
	}
}

// determineResource extracts resource type from path
func determineResource(path string) string {
	switch {
	case strings.Contains(path, "/auth/"):
		return "auth"
	case strings.Contains(path, "/users/"):
		return "user"
	case strings.Contains(path, "/chats/"):
		return "chat"
	case strings.Contains(path, "/messages/"):
		return "message"
	case strings.Contains(path, "/reactions/"):
		return "reaction"
	default:
		return "unknown"
	}
}

// extractResourceID extracts resource ID from path
func extractResourceID(path string) string {
	// Simple extraction - could be improved with proper parsing
	// For now, return empty string as it's optional
	return ""
}

// getAuditLogs returns audit logs for the current user (admin only)
func (h *Handler) getAuditLogs(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, stdhttp.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Parse query parameters
	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	logs, err := h.storage.GetAuditLogs(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get audit logs", "error", err)
		WriteError(w, stdhttp.StatusInternalServerError, "internal", "Failed to get audit logs")
		return
	}

	WriteJSON(w, stdhttp.StatusOK, logs)
}

// getAllAuditLogs returns all audit logs (admin only)
func (h *Handler) getAllAuditLogs(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	// Parse query parameters
	limit := 100
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	logs, err := h.storage.GetAllAuditLogs(r.Context(), limit, offset)
	if err != nil {
		h.logger.Error("failed to get all audit logs", "error", err)
		WriteError(w, stdhttp.StatusInternalServerError, "internal", "Failed to get audit logs")
		return
	}

	WriteJSON(w, stdhttp.StatusOK, logs)
}
