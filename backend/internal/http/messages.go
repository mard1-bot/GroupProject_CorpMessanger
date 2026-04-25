package http

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type SendMessageRequest struct {
	Content          string                  `json:"content"`
	Type             string                  `json:"type,omitempty"`
	ReplyTo          string                  `json:"reply_to,omitempty"`
	EncryptedContent string                  `json:"encrypted_content,omitempty"`
	EncryptedKeys    map[string]string       `json:"encrypted_keys,omitempty"`
	IV               string                  `json:"iv,omitempty"`
	Location         *models.MessageLocation `json:"location,omitempty"`
	Duration         *float64                `json:"duration,omitempty"`     // For voice messages in seconds
	ScheduledAt      *time.Time              `json:"scheduled_at,omitempty"` // For scheduled messages
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

	// Limit request body to 1MB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req SendMessageRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	trimmedContent := strings.TrimSpace(req.Content)
	if trimmedContent == "" && req.EncryptedContent == "" {
		WriteError(w, http.StatusBadRequest, "missing_content", "Message content is required (cannot be empty or only whitespace)")
		return
	}
	req.Content = trimmedContent

	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

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

	// Check if sender is blocked by any chat member
	for _, m := range members {
		if m.UserID != claims.UserID {
			blocked, err := h.storage.IsUserBlocked(r.Context(), m.UserID, claims.UserID)
			if err != nil {
				h.logger.Error("failed to check blocking status", "error", err)
				WriteError(w, http.StatusInternalServerError, "internal", "Failed to check blocking status")
				return
			}
			if blocked {
				WriteError(w, http.StatusForbidden, "blocked", "You are blocked by a member of this chat")
				return
			}
		}
	}

	msgType := req.Type
	if msgType == "" {
		msgType = models.MessageTypeText
	}

	// Validate message type
	validTypes := map[string]bool{
		models.MessageTypeText:  true,
		models.MessageTypeImage: true,
		models.MessageTypeFile:  true,
		models.MessageTypeVoice: true,
		models.MessageTypeVideo: true,
	}
	if !validTypes[msgType] {
		WriteError(w, http.StatusBadRequest, "invalid_type", "Invalid message type")
		return
	}

	// Validate message content size (prevent DoS)
	const maxMessageContentSize = 10000 // 10KB
	if len(req.Content) > maxMessageContentSize {
		WriteError(w, http.StatusBadRequest, "content_too_large", "Message content exceeds maximum size (10KB)")
		return
	}

	// Validate encrypted keys size (prevent DoS)
	if len(req.EncryptedKeys) > 100 {
		WriteError(w, http.StatusBadRequest, "too_many_keys", "Too many encrypted keys (max 100)")
		return
	}
	for userID, key := range req.EncryptedKeys {
		if _, err := uuid.Parse(userID); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID in encrypted keys")
			return
		}
		if len(key) > 10000 {
			WriteError(w, http.StatusBadRequest, "key_too_large", "Encrypted key exceeds maximum size (10KB)")
			return
		}
	}

	// Validate duration if provided
	if req.Duration != nil {
		if *req.Duration < 0 || *req.Duration > 3600 {
			WriteError(w, http.StatusBadRequest, "invalid_duration", "Duration must be between 0 and 3600 seconds (1 hour)")
			return
		}
	}

	// NOTE: Scheduled message processing is not yet implemented.
	// The ScheduledAt field is reserved for future use but currently ignored.
	// When implemented, a background worker will be needed to process scheduled messages.

	// Validate location if provided
	if req.Location != nil {
		if req.Location.Latitude < -90 || req.Location.Latitude > 90 {
			WriteError(w, http.StatusBadRequest, "invalid_latitude", "Latitude must be between -90 and 90")
			return
		}
		if req.Location.Longitude < -180 || req.Location.Longitude > 180 {
			WriteError(w, http.StatusBadRequest, "invalid_longitude", "Longitude must be between -180 and 180")
			return
		}
		if len(req.Location.Address) > 500 {
			WriteError(w, http.StatusBadRequest, "address_too_large", "Address exceeds maximum size (500 characters)")
			return
		}
	}

	if req.EncryptedContent != "" {
		// Validate encrypted content is not empty after trimming
		trimmedEncrypted := strings.TrimSpace(req.EncryptedContent)
		if trimmedEncrypted == "" {
			WriteError(w, http.StatusBadRequest, "invalid_encrypted_content", "Encrypted content cannot be empty")
			return
		}
		req.EncryptedContent = trimmedEncrypted

		// Validate encrypted content size (prevent DoS)
		const maxEncryptedContentSize = 1024 * 1024 // 1MB
		if len(req.EncryptedContent) > maxEncryptedContentSize {
			WriteError(w, http.StatusBadRequest, "encrypted_content_too_large", "Encrypted content exceeds maximum size (1MB)")
			return
		}

		// Validate IV is provided when encrypted content is present
		if req.IV == "" {
			WriteError(w, http.StatusBadRequest, "missing_iv", "IV (Initialization Vector) is required when using encrypted content")
			return
		}

		// Validate IV is valid base64
		decodedIV, err := base64.StdEncoding.DecodeString(req.IV)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_iv", "IV must be valid base64")
			return
		}

		// IV should be 12 or 16 bytes for AES-GCM (common sizes)
		if len(decodedIV) != 12 && len(decodedIV) != 16 {
			WriteError(w, http.StatusBadRequest, "invalid_iv_length", "IV must be 12 or 16 bytes (AES-GCM)")
			return
		}
	}

	msg := &models.Message{
		ChatID:           chatID,
		SenderID:         claims.UserID,
		Type:             msgType,
		Content:          req.Content,
		EncryptedContent: req.EncryptedContent,
		EncryptedKeys:    req.EncryptedKeys,
		Location:         req.Location,
		ScheduledAt:      req.ScheduledAt,
		Duration:         req.Duration,
	}

	if req.ReplyTo != "" {
		replyToID, err := uuid.Parse(req.ReplyTo)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_reply_to", "Invalid reply_to message ID")
			return
		}
		// Verify the replied-to message exists in the same chat
		replyMsg, err := h.storage.GetMessageByID(r.Context(), replyToID)
		if err != nil {
			h.logger.Error("failed to get reply message", "error", err)
			WriteError(w, http.StatusInternalServerError, "internal", "Failed to validate reply_to")
			return
		}
		if replyMsg == nil {
			WriteError(w, http.StatusBadRequest, "reply_not_found", "Reply-to message not found")
			return
		}
		if replyMsg.ChatID != chatID {
			WriteError(w, http.StatusBadRequest, "reply_wrong_chat", "Reply-to message is not in this chat")
			return
		}
		msg.ReplyTo = &replyToID
	}

	if err := h.storage.CreateMessage(r.Context(), msg); err != nil {
		h.logger.Error("failed to create message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to send message")
		return
	}

	// Broadcast message via WebSocket for realtime delivery
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventNewMessage,
			ChatID: chatID,
			Payload: websocket.MessagePayload{
				ID:        msg.ID,
				ChatID:    msg.ChatID,
				SenderID:  msg.SenderID,
				Type:      msg.Type,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
				ReplyTo:   msg.ReplyTo,
			},
			ExcludeSender: &claims.UserID,
		})
	}

	// Send push notifications to offline users
	if h.notificationSvc != nil {
		// Capture values to avoid data race with request scope
		senderID := claims.UserID
		msgCopy := *msg // Copy message to avoid race with potential future modifications
		go func(userID uuid.UUID, msg models.Message) {
			// Panic recovery to ensure goroutine always terminates
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("panic in notification goroutine", "recover", r)
				}
			}()

			// Use background context with timeout to avoid request cancellation
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Get sender name for notification
			sender, _ := h.storage.GetUserByID(ctx, userID)
			senderName := "Кто-то"
			if sender != nil {
				senderName = sender.FirstName + " " + sender.LastName
			}
			h.notifyMessageReceived(ctx, &msg, senderName)
		}(senderID, msgCopy)
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

	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

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

	messages, err := h.storage.GetMessagesByChatForUser(r.Context(), chatID, claims.UserID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get messages", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get messages")
		return
	}

	// Populate read status for each message using batch query
	messageIDs := make([]uuid.UUID, len(messages))
	for i, msg := range messages {
		messageIDs[i] = msg.ID
	}
	readStatusMap, err := h.storage.GetMessagesReadStatus(r.Context(), messageIDs)
	if err != nil {
		h.logger.Error("failed to get messages read status", "error", err, "chat_id", chatID, "user_id", claims.UserID)
	} else {
		for _, msg := range messages {
			msg.ReadBy = readStatusMap[msg.ID]
		}
	}

	// Batch mark messages as read
	messagesToMark := []uuid.UUID{}
	for _, msg := range messages {
		if msg.SenderID != claims.UserID {
			messagesToMark = append(messagesToMark, msg.ID)
		}
	}
	if len(messagesToMark) > 0 {
		if err := h.storage.MarkMessagesAsRead(r.Context(), messagesToMark, claims.UserID); err != nil {
			h.logger.Error("failed to mark messages as read", "error", err, "user_id", claims.UserID, "count", len(messagesToMark))
		}
	}

	// Broadcast read receipt
	if h.hub != nil {
		// Count messages received by current user that were just marked as read
		readCount := 0
		for _, msg := range messages {
			if msg.SenderID != claims.UserID {
				readCount++
			}
		}
		if readCount > 0 {
			h.hub.BroadcastToChat(&websocket.BroadcastMessage{
				Type:   websocket.EventReadReceipt,
				ChatID: chatID,
				Payload: map[string]interface{}{
					"reader_id": claims.UserID,
					"read_at":   time.Now(),
					"count":     readCount,
				},
			})
		}
	}

	WriteJSON(w, http.StatusOK, messages)
}

func (h *Handler) searchMessages(w http.ResponseWriter, r *http.Request) {
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

	// Check membership
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

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

	// Get search query
	query := r.URL.Query().Get("q")
	if query == "" {
		WriteError(w, http.StatusBadRequest, "missing_query", "Search query is required")
		return
	}

	// Pagination
	limit := 20
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

	messages, err := h.storage.SearchMessages(r.Context(), chatID, query, limit, offset)
	if err != nil {
		h.logger.Error("failed to search messages", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to search messages")
		return
	}

	WriteJSON(w, http.StatusOK, messages)
}

func (h *Handler) searchAllMessages(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		WriteError(w, http.StatusBadRequest, "missing_query", "Query parameter 'q' is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	messages, err := h.storage.SearchAllMessages(r.Context(), claims.UserID, query, limit, offset)
	if err != nil {
		h.logger.Error("failed to search all messages", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to search messages")
		return
	}

	WriteJSON(w, http.StatusOK, messages)
}

func (h *Handler) getUserMentions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20
	offset := 0
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	mentions, err := h.storage.GetUserMentions(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get mentions", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get mentions")
		return
	}

	WriteJSON(w, http.StatusOK, mentions)
}

func (h *Handler) addReaction(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
		Emoji     string `json:"emoji"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	messageID, err := uuid.Parse(req.MessageID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_message_id", "Invalid message ID")
		return
	}

	// Validate emoji - must be a valid emoji character
	if req.Emoji == "" {
		WriteError(w, http.StatusBadRequest, "invalid_emoji", "Emoji is required")
		return
	}

	// Check if emoji is a valid single emoji character (basic validation)
	// Emojis are typically in the range U+1F600 to U+1F64F (emoticons), U+1F300 to U+1F5FF (symbols), etc.
	// Also check for common emoji ranges and multi-byte UTF-8 sequences
	emoji := []rune(req.Emoji)
	if len(emoji) == 0 || len(emoji) > 4 {
		WriteError(w, http.StatusBadRequest, "invalid_emoji", "Invalid emoji format")
		return
	}

	// Basic check for emoji-like characters (this is a simplified validation)
	// In production, you might want to use a proper emoji library
	hasValidEmoji := false
	for _, r := range emoji {
		// Check for emoji ranges (simplified)
		if (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
			(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols
			(r >= 0x1F680 && r <= 0x1F6FF) || // Transport & Map
			(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
			(r >= 0x2700 && r <= 0x27BF) || // Dingbats
			(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols
			(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
			(r >= 0x1F000 && r <= 0x1F02F) { // Mahjong Tiles
			hasValidEmoji = true
		}
	}
	if !hasValidEmoji {
		WriteError(w, http.StatusBadRequest, "invalid_emoji", "Invalid emoji character")
		return
	}

	// Remove existing reaction by this user (only one reaction per user per message)
	existingReactions, err := h.storage.GetMessageReactions(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get existing reactions", "error", err)
	} else {
		for _, reaction := range existingReactions {
			if reaction.UserID == claims.UserID {
				// Remove existing reaction
				if err := h.storage.RemoveReaction(r.Context(), messageID, reaction.UserID, reaction.Emoji); err != nil {
					h.logger.Error("failed to remove existing reaction", "error", err)
				} else {
					// Broadcast reaction removal
					if h.hub != nil {
						message, err := h.storage.GetMessageByID(r.Context(), messageID)
						if err == nil {
							h.hub.BroadcastToChat(&websocket.BroadcastMessage{
								Type:   "reaction_removed",
								ChatID: message.ChatID,
								Payload: map[string]interface{}{
									"message_id": messageID.String(),
									"user_id":    claims.UserID.String(),
									"emoji":      reaction.Emoji,
								},
							})
						}
					}
				}
			}
		}
	}

	if err := h.storage.AddReaction(r.Context(), messageID, claims.UserID, req.Emoji); err != nil {
		h.logger.Error("failed to add reaction", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to add reaction")
		return
	}

	// Get message details for broadcast
	message, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message for reaction broadcast", "error", err)
	} else {
		// Get user details for broadcast
		user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
		if err != nil {
			h.logger.Error("failed to get user for reaction broadcast", "error", err)
		} else {
			// Broadcast reaction to chat
			if h.hub != nil {
				h.hub.BroadcastToChat(&websocket.BroadcastMessage{
					Type:   "reaction_added",
					ChatID: message.ChatID,
					Payload: map[string]interface{}{
						"message_id": messageID.String(),
						"user_id":    claims.UserID.String(),
						"emoji":      req.Emoji,
						"user": map[string]interface{}{
							"id":         user.ID.String(),
							"first_name": user.FirstName,
							"last_name":  user.LastName,
						},
					},
				})
			}
		}
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "reacted"})
}

func (h *Handler) removeReaction(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req struct {
		MessageID string `json:"message_id"`
		Emoji     string `json:"emoji"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	messageID, err := uuid.Parse(req.MessageID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_message_id", "Invalid message ID")
		return
	}

	// Validate emoji - must be a valid emoji character
	if req.Emoji == "" {
		WriteError(w, http.StatusBadRequest, "invalid_emoji", "Emoji is required")
		return
	}

	// Check if emoji is a valid single emoji character (basic validation)
	emoji := []rune(req.Emoji)
	if len(emoji) == 0 || len(emoji) > 4 {
		WriteError(w, http.StatusBadRequest, "invalid_emoji", "Invalid emoji format")
		return
	}

	// Basic check for emoji-like characters (this is a simplified validation)
	hasValidEmoji := false
	for _, r := range emoji {
		// Check for emoji ranges (simplified)
		if (r >= 0x1F600 && r <= 0x1F64F) || // Emoticons
			(r >= 0x1F300 && r <= 0x1F5FF) || // Misc Symbols
			(r >= 0x1F680 && r <= 0x1F6FF) || // Transport & Map
			(r >= 0x2600 && r <= 0x26FF) || // Misc symbols
			(r >= 0x2700 && r <= 0x27BF) || // Dingbats
			(r >= 0x1F900 && r <= 0x1F9FF) || // Supplemental Symbols
			(r >= 0x1FA00 && r <= 0x1FA6F) || // Chess Symbols
			(r >= 0x1F000 && r <= 0x1F02F) { // Mahjong Tiles
			hasValidEmoji = true
		}
	}
	if !hasValidEmoji {
		WriteError(w, http.StatusBadRequest, "invalid_emoji", "Invalid emoji character")
		return
	}

	if err := h.storage.RemoveReaction(r.Context(), messageID, claims.UserID, req.Emoji); err != nil {
		h.logger.Error("failed to remove reaction", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to remove reaction")
		return
	}

	// Get message details for broadcast
	message, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message for reaction removal broadcast", "error", err)
	} else {
		// Broadcast reaction removal to chat
		if h.hub != nil {
			h.hub.BroadcastToChat(&websocket.BroadcastMessage{
				Type:   "reaction_removed",
				ChatID: message.ChatID,
				Payload: map[string]interface{}{
					"message_id": messageID.String(),
					"user_id":    claims.UserID.String(),
					"emoji":      req.Emoji,
				},
			})
		}
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unreacted"})
}

func (h *Handler) getMessageReactions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	messageIDStr := chi.URLParam(r, "id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid message ID")
		return
	}

	// Get message to check chat membership
	message, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteError(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}

	// Verify user is member of the chat
	members, err := h.storage.GetChatMembers(r.Context(), message.ChatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify chat membership")
		return
	}

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

	reactions, err := h.storage.GetMessageReactions(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get reactions", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get reactions")
		return
	}

	WriteJSON(w, http.StatusOK, reactions)
}

func (h *Handler) forwardMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req struct {
		ChatID string `json:"chat_id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	messageIDStr := chi.URLParam(r, "id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_message_id", "Invalid message ID")
		return
	}

	chatID, err := uuid.Parse(req.ChatID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid target chat ID")
		return
	}

	// Get original message
	originalMsg, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get message")
		return
	}
	if originalMsg == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}

	// Verify user is member of source chat (original message's chat)
	sourceMembers, err := h.storage.GetChatMembers(r.Context(), originalMsg.ChatID)
	if err != nil {
		h.logger.Error("failed to get source chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify source chat membership")
		return
	}

	isSourceMember := false
	for _, m := range sourceMembers {
		if m.UserID == claims.UserID {
			isSourceMember = true
			break
		}
	}
	if !isSourceMember {
		WriteError(w, http.StatusForbidden, "forbidden", "You are not a member of the source chat")
		return
	}

	// Verify user is member of target chat
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify chat membership")
		return
	}

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

	// Create forwarded message
	forwardedMsg := &models.Message{
		ChatID:   chatID,
		SenderID: claims.UserID,
		Type:     originalMsg.Type,
		Content:  originalMsg.Content,
		FileURL:  originalMsg.FileURL,
	}

	if err := h.storage.CreateMessage(r.Context(), forwardedMsg); err != nil {
		h.logger.Error("failed to create forwarded message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to forward message")
		return
	}

	// Broadcast to chat
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventNewMessage,
			ChatID: forwardedMsg.ChatID,
			Payload: websocket.MessagePayload{
				ID:        forwardedMsg.ID,
				ChatID:    forwardedMsg.ChatID,
				SenderID:  forwardedMsg.SenderID,
				Type:      forwardedMsg.Type,
				Content:   forwardedMsg.Content,
				CreatedAt: forwardedMsg.CreatedAt,
			},
		})
	}

	WriteJSON(w, http.StatusOK, forwardedMsg)
}

type ReplyMessageRequest struct {
	Content string `json:"content"`
}

func (h *Handler) replyMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req ReplyMessageRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	trimmedContent := strings.TrimSpace(req.Content)
	if trimmedContent == "" {
		WriteError(w, http.StatusBadRequest, "missing_content", "Reply content is required")
		return
	}

	messageIDStr := chi.URLParam(r, "id")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_message_id", "Invalid message ID")
		return
	}

	// Get original message
	originalMsg, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get message")
		return
	}
	if originalMsg == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}

	// Verify user is a member of the chat
	members, err := h.storage.GetChatMembers(r.Context(), originalMsg.ChatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify chat membership")
		return
	}

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

	// Check if sender is blocked by any chat member
	for _, m := range members {
		if m.UserID != claims.UserID {
			blocked, err := h.storage.IsUserBlocked(r.Context(), m.UserID, claims.UserID)
			if err != nil {
				h.logger.Error("failed to check blocking status", "error", err)
				WriteError(w, http.StatusInternalServerError, "internal", "Failed to check blocking status")
				return
			}
			if blocked {
				WriteError(w, http.StatusForbidden, "blocked", "You are blocked by a member of this chat")
				return
			}
		}
	}

	// Create reply message
	replyMsg := &models.Message{
		ChatID:   originalMsg.ChatID,
		SenderID: claims.UserID,
		Type:     models.MessageTypeText,
		Content:  trimmedContent,
		ReplyTo:  &messageID,
	}

	if err := h.storage.CreateMessage(r.Context(), replyMsg); err != nil {
		h.logger.Error("failed to create reply message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to send reply")
		return
	}

	// Broadcast message via WebSocket
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventNewMessage,
			ChatID: replyMsg.ChatID,
			Payload: websocket.MessagePayload{
				ID:        replyMsg.ID,
				ChatID:    replyMsg.ChatID,
				SenderID:  replyMsg.SenderID,
				Type:      replyMsg.Type,
				Content:   replyMsg.Content,
				CreatedAt: replyMsg.CreatedAt,
				ReplyTo:   replyMsg.ReplyTo,
			},
			ExcludeSender: &claims.UserID,
		})
	}

	WriteJSON(w, http.StatusCreated, replyMsg)
}

type EditMessageRequest struct {
	Content string `json:"content"`
}

func (h *Handler) editMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	messageIDStr := chi.URLParam(r, "msgID")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid message ID")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req EditMessageRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	trimmedContent := strings.TrimSpace(req.Content)
	if trimmedContent == "" {
		WriteError(w, http.StatusBadRequest, "missing_content", "Message content is required")
		return
	}

	// Get message
	msg, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get message")
		return
	}
	if msg == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}

	// Verify message belongs to the chat
	chatIDStr := chi.URLParam(r, "id")
	chatID, _ := uuid.Parse(chatIDStr)
	if msg.ChatID != chatID {
		WriteError(w, http.StatusForbidden, "forbidden", "Message does not belong to this chat")
		return
	}

	// Check sender
	if msg.SenderID != claims.UserID {
		WriteError(w, http.StatusForbidden, "forbidden", "Can only edit own messages")
		return
	}

	// Update message
	msg.Content = trimmedContent
	msg.UpdatedAt = time.Now()

	if err := h.storage.UpdateMessage(r.Context(), msg); err != nil {
		h.logger.Error("failed to update message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update message")
		return
	}

	// Broadcast update
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventMessageUpdated,
			ChatID: msg.ChatID,
			Payload: websocket.MessagePayload{
				ID:        msg.ID,
				ChatID:    msg.ChatID,
				SenderID:  msg.SenderID,
				Type:      msg.Type,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
				UpdatedAt: &msg.UpdatedAt,
				ReplyTo:   msg.ReplyTo,
			},
		})
	}

	WriteJSON(w, http.StatusOK, msg)
}

func (h *Handler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	messageIDStr := chi.URLParam(r, "msgID")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid message ID")
		return
	}

	// Get message
	msg, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get message")
		return
	}
	if msg == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}

	// Verify message belongs to the chat
	chatIDStr := chi.URLParam(r, "id")
	chatID, _ := uuid.Parse(chatIDStr)
	if msg.ChatID != chatID {
		WriteError(w, http.StatusForbidden, "forbidden", "Message does not belong to this chat")
		return
	}

	// Get chat to check type
	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat")
		return
	}
	if chat == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Chat not found")
		return
	}

	// Get chat members to check caller's role
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	// Check caller's role in chat
	var callerRole string
	for _, m := range members {
		if m.UserID == claims.UserID {
			callerRole = m.Role
			break
		}
	}

	// Check permissions:
	// - In direct chats: any member can delete any message
	// - In group chats: owner/admin can delete any message, members only their own
	isOwnMessage := msg.SenderID == claims.UserID
	isDirectChat := chat.Type == "direct"
	if !isOwnMessage && !isDirectChat && callerRole != models.ChatRoleOwner && callerRole != models.ChatRoleAdmin {
		WriteError(w, http.StatusForbidden, "forbidden", "Can only delete own messages")
		return
	}

	// Delete message
	if err := h.storage.DeleteMessage(r.Context(), messageID); err != nil {
		h.logger.Error("failed to delete message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to delete message")
		return
	}

	// Broadcast deletion
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventMessageDeleted,
			ChatID: msg.ChatID,
			Payload: map[string]string{
				"message_id": messageID.String(),
			},
		})
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request) {
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

	messageIDStr := chi.URLParam(r, "msgID")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_msg_id", "Invalid message ID")
		return
	}

	// Verify message belongs to chat and user is member
	msg, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get message")
		return
	}
	if msg == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Message not found")
		return
	}
	if msg.ChatID != chatID {
		WriteError(w, http.StatusForbidden, "forbidden", "Message does not belong to this chat")
		return
	}

	// Check if user is chat member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}
	var isMember bool
	for _, m := range members {
		if m.UserID == claims.UserID {
			isMember = true
			break
		}
	}
	if !isMember {
		WriteError(w, http.StatusForbidden, "forbidden", "Not a chat member")
		return
	}

	// Parse multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		h.logger.Error("failed to parse multipart form", "error", err)
		WriteError(w, http.StatusBadRequest, "invalid_form", "Failed to parse form data")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "no_file", "No file provided")
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			h.logger.Error("failed to close uploaded file", "error", cerr)
		}
	}()

	// Limit file size
	if header.Size > 50<<20 {
		WriteError(w, http.StatusBadRequest, "file_too_large", "File too large (max 50MB)")
		return
	}

	// Validate and sanitize file extension
	ext := filepath.Ext(header.Filename)
	// Remove any path traversal or special characters
	ext = strings.ReplaceAll(ext, "/", "")
	ext = strings.ReplaceAll(ext, "\\", "")
	ext = strings.TrimPrefix(ext, ".")

	// Whitelist of allowed extensions
	allowedExts := map[string]bool{
		// Images
		"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true, "bmp": true, "svg": true, "ico": true,
		// Documents
		"pdf": true, "doc": true, "docx": true, "txt": true, "rtf": true, "odt": true, "xls": true, "xlsx": true, "ppt": true, "pptx": true,
		// Audio
		"mp3": true, "wav": true, "ogg": true, "flac": true, "aac": true, "m4a": true,
		// Video
		"mp4": true, "mov": true, "avi": true, "mkv": true, "webm": true, "flv": true, "wmv": true,
		// Archives
		"zip": true, "rar": true, "7z": true, "tar": true, "gz": true, "bz2": true,
		// Code
		"json": true, "xml": true, "yaml": true, "yml": true, "csv": true,
	}

	// Validate extension - files without extension are not allowed for security
	var filename string
	fileID := uuid.New()
	if ext == "" {
		// No extension - reject for security
		WriteError(w, http.StatusBadRequest, "invalid_file", "File must have an extension")
		return
	}

	extLower := strings.ToLower(ext)
	if !allowedExts[extLower] {
		WriteError(w, http.StatusBadRequest, "invalid_extension", "File extension not allowed")
		return
	}
	filename = fileID.String() + "." + extLower

	// Create uploads directory if not exists
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("failed to create upload dir", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save file")
		return
	}

	// Save file
	filePath := filepath.Join(uploadDir, filename)
	out, err := os.Create(filePath)
	if err != nil {
		h.logger.Error("failed to create file", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save file")
		return
	}
	defer func() {
		if cerr := out.Close(); cerr != nil {
			h.logger.Error("failed to close file", "error", cerr)
		}
	}()

	if _, err := io.Copy(out, file); err != nil {
		h.logger.Error("failed to write file", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save file")
		return
	}

	// Determine mime type
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Save file record
	// Sanitize original filename to prevent path traversal
	safeFilename := filepath.Base(header.Filename)
	fileRecord := &models.File{
		ID:           fileID,
		MessageID:    messageID,
		Name:         safeFilename,
		Size:         header.Size,
		MimeType:     mimeType,
		URL:          h.baseURL + "/uploads/" + filename,
		ThumbnailURL: nil,
		UploadedAt:   time.Now(),
	}

	if err := h.storage.CreateFile(r.Context(), fileRecord); err != nil {
		h.logger.Error("failed to save file record", "error", err)
		os.Remove(filePath) // Clean up
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save file record")
		return
	}

	// Update message with file URL
	if err := h.storage.UpdateMessageFileURL(r.Context(), messageID, fileRecord.URL); err != nil {
		h.logger.Error("failed to update message file_url", "error", err)
		// Don't fail the upload, just log the error
	}

	// Broadcast file attachment to chat
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventMessageUpdated,
			ChatID: chatID,
			Payload: map[string]interface{}{
				"id":       messageID.String(),
				"file_url": fileRecord.URL,
			},
		})
	}

	WriteJSON(w, http.StatusCreated, fileRecord)
}

func (h *Handler) sendTypingIndicator(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req struct {
		ChatID string `json:"chat_id"`
		Typing bool   `json:"typing"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	chatID, err := uuid.Parse(req.ChatID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_chat_id", "Invalid chat ID")
		return
	}

	// Verify user is member of chat
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify chat membership")
		return
	}

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

	// Broadcast typing indicator to chat members
	if h.hub != nil {
		h.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventTyping,
			ChatID: chatID,
			Payload: map[string]interface{}{
				"user_id": claims.UserID,
				"typing":  req.Typing,
			},
		})
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
