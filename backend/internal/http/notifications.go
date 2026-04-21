package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/notifications"

	"github.com/google/uuid"
)

type RegisterDeviceRequest struct {
	Token      string `json:"token"`
	Platform   string `json:"platform"`
	DeviceName string `json:"device_name,omitempty"`
}

type NotificationSettingsRequest struct {
	PushEnabled       bool    `json:"push_enabled"`
	EmailEnabled      bool    `json:"email_enabled"`
	Email             string  `json:"email,omitempty"`
	QuietHoursStart   *string `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd     *string `json:"quiet_hours_end,omitempty"`
	QuietHoursEnabled bool    `json:"quiet_hours_enabled"`
}

func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Token == "" || req.Platform == "" {
		WriteError(w, http.StatusBadRequest, "missing_fields", "token and platform are required")
		return
	}

	token := &models.DeviceToken{
		ID:         uuid.New(),
		UserID:     claims.UserID,
		Token:      req.Token,
		Platform:   req.Platform,
		DeviceName: req.DeviceName,
		CreatedAt:  time.Now(),
	}

	if err := h.storage.CreateDeviceToken(r.Context(), token); err != nil {
		h.logger.Error("failed to register device", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to register device")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

func (h *Handler) getNotificationSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	settings, err := h.storage.GetNotificationSettings(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get settings", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get settings")
		return
	}

	WriteJSON(w, http.StatusOK, settings)
}

func (h *Handler) updateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req NotificationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validate quiet hours format (HH:MM)
	timeRegex := regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)
	if req.QuietHoursEnabled {
		if req.QuietHoursStart != nil && !timeRegex.MatchString(*req.QuietHoursStart) {
			WriteError(w, http.StatusBadRequest, "invalid_time", "Quiet hours start must be in HH:MM format")
			return
		}
		if req.QuietHoursEnd != nil && !timeRegex.MatchString(*req.QuietHoursEnd) {
			WriteError(w, http.StatusBadRequest, "invalid_time", "Quiet hours end must be in HH:MM format")
			return
		}
	}

	settings := &models.NotificationSettings{
		UserID:            claims.UserID,
		PushEnabled:       req.PushEnabled,
		EmailEnabled:      req.EmailEnabled,
		Email:             req.Email,
		QuietHoursStart:   req.QuietHoursStart,
		QuietHoursEnd:     req.QuietHoursEnd,
		QuietHoursEnabled: req.QuietHoursEnabled,
	}

	if err := h.storage.UpdateNotificationSettings(r.Context(), settings); err != nil {
		h.logger.Error("failed to update settings", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update settings")
		return
	}

	WriteJSON(w, http.StatusOK, settings)
}

func (h *Handler) getUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	count, err := h.storage.GetTotalUnreadCount(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get unread count", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get unread count")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]int{"total_unread": count})
}

// Helper to send notification when message is created
func (h *Handler) notifyMessageReceived(ctx context.Context, msg *models.Message, senderName string) {
	if h.notificationSvc == nil {
		return
	}

	members, err := h.storage.GetChatMembers(ctx, msg.ChatID)
	if err != nil {
		h.logger.Error("failed to get members for notification", "error", err)
		return
	}

	for _, member := range members {
		if member.UserID == msg.SenderID {
			continue // Don't notify sender
		}

		payload := &notifications.NotificationPayload{
			Title:  fmt.Sprintf("%s", senderName),
			Body:   truncateString(msg.Content, 100),
			ChatID: msg.ChatID,
			Type:   "message",
			Data: map[string]string{
				"chat_id":    msg.ChatID.String(),
				"message_id": msg.ID.String(),
				"type":       "new_message",
			},
		}

		if err := h.notificationSvc.SendToUser(ctx, member.UserID, payload); err != nil {
			h.logger.Error("failed to send notification", "error", err, "user_id", member.UserID)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
