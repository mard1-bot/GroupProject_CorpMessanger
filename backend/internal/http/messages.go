package http

import (
	"net/http"
	"strconv"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SendMessageRequest struct {
	Content string `json:"content"`
	Type    string `json:"type,omitempty"`
	ReplyTo string `json:"reply_to,omitempty"`
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	var req SendMessageRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "missing_content", "Message content is required")
		return
	}

	members, _ := h.storage.GetChatMembers(r.Context(), chatID)
	isMember := false
	for _, m := range members {
		if m.UserID == claims.UserID {
			isMember = true
			break
		}
	}
	if !isMember {
		WriteError(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	msgType := req.Type
	if msgType == "" {
		msgType = models.MessageTypeText
	}

	msg := &models.Message{
		ChatID:   chatID,
		SenderID: claims.UserID,
		Type:     msgType,
		Content:  req.Content,
	}

	if req.ReplyTo != "" {
		replyToID, err := uuid.Parse(req.ReplyTo)
		if err == nil {
			msg.ReplyTo = &replyToID
		}
	}

	if err := h.storage.CreateMessage(r.Context(), msg); err != nil {
		h.logger.Error("failed to create message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to send message")
		return
	}

	WriteJSON(w, http.StatusCreated, msg)
}

func (h *Handler) getChatMessages(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	members, _ := h.storage.GetChatMembers(r.Context(), chatID)
	isMember := false
	for _, m := range members {
		if m.UserID == claims.UserID {
			isMember = true
			break
		}
	}
	if !isMember {
		WriteError(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	limit := 50
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	messages, err := h.storage.GetMessagesByChat(r.Context(), chatID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get messages", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get messages")
		return
	}

	WriteJSON(w, http.StatusOK, messages)
}
