package http

import (
	"html"
	"net/http"
	"strings"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// isChatMember checks if userID is a member of the chat from the members list
func isChatMember(members []*models.ChatMember, userID uuid.UUID) bool {
	for _, m := range members {
		if m.UserID == userID {
			return true
		}
	}
	return false
}

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
	defer r.Body.Close()

	if req.Type == "" {
		WriteError(w, http.StatusBadRequest, "missing_chat_type", "Chat type is required (direct or group)")
		return
	}

	// Validate input lengths to prevent DoS and XSS
	const maxChatTitleLength = 200
	const maxChatDescriptionLength = 1000
	if len(req.Title) > maxChatTitleLength {
		WriteError(w, http.StatusBadRequest, "title_too_long", "Chat title exceeds maximum length (200 characters)")
		return
	}
	if len(req.Description) > maxChatDescriptionLength {
		WriteError(w, http.StatusBadRequest, "description_too_long", "Chat description exceeds maximum length (1000 characters)")
		return
	}

	// Sanitize inputs to prevent XSS
	sanitizedTitle := html.EscapeString(strings.TrimSpace(req.Title))
	sanitizedDescription := html.EscapeString(strings.TrimSpace(req.Description))

	chat := &models.Chat{
		Type:        req.Type,
		Title:       sanitizedTitle,
		Description: sanitizedDescription,
		CreatorID:   claims.UserID,
	}

	// Build members list (owner + additional members)
	var membersList []*models.ChatMember
	membersList = append(membersList, &models.ChatMember{
		UserID: claims.UserID,
		Role:   models.ChatRoleOwner,
	})

	// Validate and collect additional member IDs
	for _, memberIDStr := range req.MemberIDs {
		memberID, err := uuid.Parse(memberIDStr)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_member_id", "Invalid member ID: "+memberIDStr)
			return
		}
		// Don't add creator twice if they're in the member list
		if memberID != claims.UserID {
			membersList = append(membersList, &models.ChatMember{
				UserID: memberID,
				Role:   models.ChatRoleMember,
			})
		}
	}

	// Verify all member IDs exist in database
	for _, member := range membersList {
		if member.UserID == claims.UserID {
			continue // Skip creator check (we know they exist from auth)
		}
		user, err := h.storage.GetUserByID(r.Context(), member.UserID)
		if err != nil {
			h.logger.Error("failed to verify member", "user_id", member.UserID, "error", err)
			WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify member")
			return
		}
		if user == nil {
			WriteError(w, http.StatusNotFound, "member_not_found", "User not found: "+member.UserID.String())
			return
		}
	}

	// Validate direct chat has exactly 2 members (creator + 1 other)
	if chat.Type == models.ChatTypeDirect && len(membersList) != 2 {
		WriteError(w, http.StatusBadRequest, "invalid_direct_chat", "Direct chats must have exactly 2 members")
		return
	}

	// Atomic creation: chat + members in one transaction
	if err := h.storage.CreateChatWithMembers(r.Context(), chat, membersList); err != nil {
		h.logger.Error("failed to create chat with members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to create chat")
		return
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

	if !isChatMember(members, claims.UserID) {
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
	defer r.Body.Close()

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	// Verify the target user exists
	targetUser, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get target user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify user")
		return
	}
	if targetUser == nil {
		WriteError(w, http.StatusNotFound, "user_not_found", "User not found")
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
	if !isChatMember(members, claims.UserID) {
		WriteError(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}
	for _, m := range members {
		if m.UserID == claims.UserID {
			callerRole = m.Role
			break
		}
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

	// Check if user is already a member
	if isChatMember(members, userID) {
		WriteError(w, http.StatusConflict, "already_member", "User is already a member of this chat")
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
