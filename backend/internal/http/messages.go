package http

import (
	"context"
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
	defer r.Body.Close()

	trimmedContent := strings.TrimSpace(req.Content)
	if trimmedContent == "" {
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

	msg := &models.Message{
		ChatID:   chatID,
		SenderID: claims.UserID,
		Type:     msgType,
		Content:  req.Content,
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
				CreatedAt: time.Now(),
				ReplyTo:   msg.ReplyTo,
			},
			ExcludeSender: &claims.UserID,
		})
	}

	// Send push notifications to offline users
	if h.notificationSvc != nil {
		go func() {
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
			sender, _ := h.storage.GetUserByID(ctx, claims.UserID)
			senderName := "Кто-то"
			if sender != nil {
				senderName = sender.FirstName + " " + sender.LastName
			}
			h.notifyMessageReceived(ctx, msg, senderName)
		}()
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

	messages, err := h.storage.GetMessagesByChat(r.Context(), chatID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get messages", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get messages")
		return
	}

	// Populate read status for each message
	for _, msg := range messages {
		readBy, err := h.storage.GetMessageReadStatus(r.Context(), msg.ID)
		if err == nil {
			msg.ReadBy = readBy
		}
	}

	// Mark messages as read for current user
	for _, msg := range messages {
		if msg.SenderID != claims.UserID {
			h.storage.MarkMessageAsRead(r.Context(), msg.ID, claims.UserID)
		}
	}

	// Broadcast read receipt
	if h.hub != nil {
		unreadCount := 0
		for _, msg := range messages {
			if msg.SenderID == claims.UserID {
				readBy, _ := h.storage.GetMessageReadStatus(r.Context(), msg.ID)
				if len(readBy) > 0 {
					unreadCount++
				}
			}
		}
		if unreadCount > 0 {
			h.hub.BroadcastToChat(&websocket.BroadcastMessage{
				Type:   websocket.EventReadReceipt,
				ChatID: chatID,
				Payload: map[string]interface{}{
					"reader_id": claims.UserID,
					"read_at":   time.Now(),
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
	r.ParseMultipartForm(50 << 20)
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
		"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true,
		"pdf": true, "doc": true, "docx": true, "txt": true,
		"mp4": true, "mp3": true, "mov": true, "avi": true,
		"zip": true, "rar": true, "7z": true,
	}

	// Validate extension - files without extension are allowed (no whitelist check)
	// Files with extension must be in whitelist
	var filename string
	fileID := uuid.New()
	if ext == "" {
		// No extension - use UUID only
		filename = fileID.String()
	} else {
		extLower := strings.ToLower(ext)
		if !allowedExts[extLower] {
			WriteError(w, http.StatusBadRequest, "invalid_extension", "File extension not allowed")
			return
		}
		filename = fileID.String() + "." + extLower
	}

	// Create uploads directory if not exists
	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		h.logger.Error("failed to create upload dir", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save file")
		return
	}

	// Save file
	filepath := filepath.Join(uploadDir, filename)
	out, err := os.Create(filepath)
	if err != nil {
		h.logger.Error("failed to create file", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save file")
		return
	}
	defer out.Close()

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
	fileRecord := &models.File{
		ID:         fileID,
		MessageID:  messageID,
		Name:       header.Filename,
		Size:       header.Size,
		MimeType:   mimeType,
		URL:        "/uploads/" + filename,
		UploadedAt: time.Now(),
	}

	if err := h.storage.CreateFile(r.Context(), fileRecord); err != nil {
		h.logger.Error("failed to save file record", "error", err)
		os.Remove(filepath) // Clean up
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
