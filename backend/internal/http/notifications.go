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
	PushEnabled        bool    `json:"push_enabled"`
	EmailEnabled       bool    `json:"email_enabled"`
	Email              string  `json:"email,omitempty"`
	QuietHoursStart    *string `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd      *string `json:"quiet_hours_end,omitempty"`
	QuietHoursEnabled  bool    `json:"quiet_hours_enabled"`
	ProtocolPreference string  `json:"protocol_preference,omitempty"` // "websocket" or "xmpp"
	HybridModeEnabled  bool    `json:"hybrid_mode_enabled"`           // Enable XMPP hybrid mode
}

type WebPushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Key      string `json:"key"`
	Auth     string `json:"auth"`
}

func (h *Handler) registerDevice(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req RegisterDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Token == "" || req.Platform == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_fields", "token and platform are required")
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
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to register device")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]string{"status": "registered"})
}

func (h *Handler) getNotificationSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	settings, err := h.storage.GetNotificationSettings(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get settings", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get settings")
		return
	}

	WriteJSON(w, http.StatusOK, settings)
}

func (h *Handler) updateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req NotificationSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode notification settings request", "error", err)
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	h.logger.Info("updateNotificationSettings called",
		"user_id", claims.UserID,
		"push_enabled", req.PushEnabled,
		"email_enabled", req.EmailEnabled,
		"quiet_hours_enabled", req.QuietHoursEnabled,
		"quiet_hours_start", req.QuietHoursStart,
		"quiet_hours_end", req.QuietHoursEnd,
		"hybrid_mode_enabled", req.HybridModeEnabled)

	// Validate quiet hours format (HH:MM)
	timeRegex := regexp.MustCompile(`^([01]?[0-9]|2[0-3]):([0-5][0-9])$`)
	if req.QuietHoursEnabled {
		if req.QuietHoursStart != nil && !timeRegex.MatchString(*req.QuietHoursStart) {
			h.logger.Error("invalid quiet_hours_start format", "value", *req.QuietHoursStart)
			WriteErrorCode(w, http.StatusBadRequest, "invalid_time", "Quiet hours start must be in HH:MM format")
			return
		}
		if req.QuietHoursEnd != nil && !timeRegex.MatchString(*req.QuietHoursEnd) {
			h.logger.Error("invalid quiet_hours_end format", "value", *req.QuietHoursEnd)
			WriteErrorCode(w, http.StatusBadRequest, "invalid_time", "Quiet hours end must be in HH:MM format")
			return
		}
	}

	// Validate protocol preference
	if req.ProtocolPreference != "" && req.ProtocolPreference != "websocket" && req.ProtocolPreference != "xmpp" {
		h.logger.Error("invalid protocol preference", "value", req.ProtocolPreference)
		WriteErrorCode(w, http.StatusBadRequest, "invalid_protocol", "Protocol preference must be 'websocket' or 'xmpp'")
		return
	}

	// Set default protocol_preference if not provided
	protocolPreference := req.ProtocolPreference
	if protocolPreference == "" {
		protocolPreference = "websocket"
	}

	h.logger.Info("Creating notification settings",
		"protocol_preference", protocolPreference,
		"original_value", req.ProtocolPreference)

	settings := &models.NotificationSettings{
		UserID:             claims.UserID,
		PushEnabled:        req.PushEnabled,
		EmailEnabled:       req.EmailEnabled,
		Email:              req.Email,
		QuietHoursStart:    req.QuietHoursStart,
		QuietHoursEnd:      req.QuietHoursEnd,
		QuietHoursEnabled:  req.QuietHoursEnabled,
		ProtocolPreference: protocolPreference,
		HybridModeEnabled:  req.HybridModeEnabled,
	}

	if err := h.storage.UpdateNotificationSettings(r.Context(), settings); err != nil {
		h.logger.Error("failed to update notification settings in storage", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to update settings")
		return
	}

	h.logger.Info("notification settings updated successfully", "user_id", claims.UserID)
	WriteJSON(w, http.StatusOK, settings)
}

func (h *Handler) getUnreadCount(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	count, err := h.storage.GetTotalUnreadCount(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get unread count", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get unread count")
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

func (h *Handler) registerWebPushSubscription(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req WebPushSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Endpoint == "" || req.Key == "" || req.Auth == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_fields", "endpoint, key, and auth are required")
		return
	}

	// Add length validation to prevent database bloat and DoS
	const (
		maxEndpointLength = 2048
		maxKeyLength      = 256
		maxAuthLength     = 256
	)

	if len(req.Endpoint) > maxEndpointLength {
		WriteErrorCode(w, http.StatusBadRequest, "endpoint_too_long", fmt.Sprintf("endpoint must be at most %d characters", maxEndpointLength))
		return
	}
	if len(req.Key) > maxKeyLength {
		WriteErrorCode(w, http.StatusBadRequest, "key_too_long", fmt.Sprintf("key must be at most %d characters", maxKeyLength))
		return
	}
	if len(req.Auth) > maxAuthLength {
		WriteErrorCode(w, http.StatusBadRequest, "auth_too_long", fmt.Sprintf("auth must be at most %d characters", maxAuthLength))
		return
	}

	subscription := &models.WebPushSubscription{
		ID:        uuid.New(),
		UserID:    claims.UserID,
		Endpoint:  req.Endpoint,
		Key:       req.Key,
		Auth:      req.Auth,
		CreatedAt: time.Now(),
	}

	if err := h.storage.CreateWebPushSubscription(r.Context(), subscription); err != nil {
		h.logger.Error("failed to create web push subscription", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to create subscription")
		return
	}

	WriteJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Web push subscription registered",
	})
}

func (h *Handler) unregisterWebPushSubscription(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req WebPushSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Endpoint == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_fields", "endpoint is required")
		return
	}

	if err := h.storage.DeleteWebPushSubscription(r.Context(), claims.UserID, req.Endpoint); err != nil {
		h.logger.Error("failed to delete web push subscription", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to delete subscription")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Web push subscription deleted",
	})
}
