package http

import (
	"net/http"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CreateChatRequest struct {
	Type        string   `json:"type"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	MemberIDs   []string `json:"member_ids,omitempty"`
}

type ChatResponse struct {
	*models.Chat
	Members []*models.ChatMember `json:"members,omitempty"`
}

func (h *Handler) createChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req CreateChatRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Type == "" {
		req.Type = models.ChatTypeDirect
	}

	chat := &models.Chat{
		Type:        req.Type,
		Title:       req.Title,
		Description: req.Description,
		CreatorID:   claims.UserID,
	}

	if err := h.storage.CreateChat(r.Context(), chat); err != nil {
		h.logger.Error("failed to create chat", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to create chat")
		return
	}

	ownerMember := &models.ChatMember{
		ChatID: chat.ID,
		UserID: claims.UserID,
		Role:   models.ChatRoleOwner,
	}
	if err := h.storage.AddChatMember(r.Context(), ownerMember); err != nil {
		h.logger.Error("failed to add creator as member", "error", err)
	}

	for _, memberIDStr := range req.MemberIDs {
		memberID, err := uuid.Parse(memberIDStr)
		if err != nil {
			continue
		}
		member := &models.ChatMember{
			ChatID: chat.ID,
			UserID: memberID,
			Role:   models.ChatRoleMember,
		}
		if err := h.storage.AddChatMember(r.Context(), member); err != nil {
			h.logger.Error("failed to add member", "error", err, "member_id", memberID)
		}
	}

	members, err := h.storage.GetChatMembers(r.Context(), chat.ID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	WriteJSON(w, http.StatusCreated, ChatResponse{Chat: chat, Members: members})
}

func (h *Handler) getUserChats(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chats, err := h.storage.GetUserChats(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get user chats", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chats")
		return
	}

	WriteJSON(w, http.StatusOK, chats)
}

func (h *Handler) getChatByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

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

	WriteJSON(w, http.StatusOK, ChatResponse{Chat: chat, Members: members})
}

func (h *Handler) addChatMember(w http.ResponseWriter, r *http.Request) {
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

	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role,omitempty"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	// Validate role
	validRoles := map[string]bool{
		models.ChatRoleOwner:  true,
		models.ChatRoleAdmin:  true,
		models.ChatRoleMember: true,
	}
	if req.Role == "" {
		req.Role = models.ChatRoleMember
	} else if !validRoles[req.Role] {
		WriteError(w, http.StatusBadRequest, "invalid_role", "Invalid role. Allowed: owner, admin, member")
		return
	}

	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	// Check if caller is a member and get their role
	var callerRole string
	isMember := false
	for _, m := range members {
		if m.UserID == claims.UserID {
			isMember = true
			callerRole = m.Role
			break
		}
	}
	if !isMember {
		WriteError(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	// Only owner/admin can add members
	if callerRole != models.ChatRoleOwner && callerRole != models.ChatRoleAdmin {
		WriteError(w, http.StatusForbidden, "forbidden", "Only chat owner or admin can add members")
		return
	}

	// Only owner can assign owner/admin roles
	if (req.Role == models.ChatRoleOwner || req.Role == models.ChatRoleAdmin) && callerRole != models.ChatRoleOwner {
		WriteError(w, http.StatusForbidden, "forbidden", "Only chat owner can assign owner or admin roles")
		return
	}

	member := &models.ChatMember{
		ChatID: chatID,
		UserID: userID,
		Role:   req.Role,
	}
	if err := h.storage.AddChatMember(r.Context(), member); err != nil {
		h.logger.Error("failed to add member", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to add member")
		return
	}

	WriteJSON(w, http.StatusOK, member)
}
