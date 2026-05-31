package http

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/websocket"

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
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req CreateChatRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Type == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_chat_type", "Chat type is required (direct or group)")
		return
	}

	// Validate input lengths to prevent DoS and XSS
	const maxChatTitleLength = 200
	const maxChatDescriptionLength = 1000
	if len(req.Title) > maxChatTitleLength {
		WriteErrorCode(w, http.StatusBadRequest, "title_too_long", "Chat title exceeds maximum length (200 characters)")
		return
	}
	if len(req.Description) > maxChatDescriptionLength {
		WriteErrorCode(w, http.StatusBadRequest, "description_too_long", "Chat description exceeds maximum length (1000 characters)")
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
			WriteErrorCode(w, http.StatusBadRequest, "invalid_member_id", "Invalid member ID: "+memberIDStr)
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
			WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to verify member")
			return
		}
		if user == nil {
			WriteErrorCode(w, http.StatusNotFound, "member_not_found", "User not found: "+member.UserID.String())
			return
		}
	}

	// Validate direct chat has exactly 2 members (creator + 1 other)
	// This is the consolidated validation - done once after member list is built
	if chat.Type == models.ChatTypeDirect && len(membersList) != 2 {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_direct_chat", "Direct chats must have exactly 2 members")
		return
	}

	// Atomic creation: chat + members in one transaction
	if err := h.storage.CreateChatWithMembers(r.Context(), chat, membersList); err != nil {
		h.logger.Error("failed to create chat with members", "error", err)
		if err.Error() == "direct chat already exists" {
			WriteErrorCode(w, http.StatusConflict, "chat_exists", "Direct chat with this user already exists")
			return
		}
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to create chat")
		return
	}

	// Create XMPP chat room (only for group chats)
	// Note: This happens after PostgreSQL commit. In case of failure, mark for reconciliation
	// to sync XMPP rooms with PostgreSQL chats to fix inconsistencies.
	if chat.Type == models.ChatTypeGroup {
		xmppRoomCreated := false
		if err := h.ejabberd.CreateChatRoom(chat.ID, chat.Title, claims.UserID); err != nil {
			h.logger.Error("XMPP room creation failed - marking for reconciliation", "chat_id", chat.ID, "error", err)
			// Mark for reconciliation
			if err := h.storage.MarkChatForReconciliation(r.Context(), chat.ID, "room_not_created", map[string]interface{}{
				"error":      err.Error(),
				"title":      chat.Title,
				"creator_id": claims.UserID,
			}); err != nil {
				h.logger.Error("failed to mark chat for reconciliation", "chat_id", chat.ID, "error", err)
			}
		} else {
			xmppRoomCreated = true
		}

		// Add members to XMPP room only if room was created
		if xmppRoomCreated {
			for _, member := range membersList {
				role := "member"
				if member.Role == models.ChatRoleOwner {
					role = "owner"
				} else if member.Role == models.ChatRoleAdmin {
					role = "admin"
				}
				if err := h.ejabberd.AddMemberToRoom(chat.ID, member.UserID, role); err != nil {
					h.logger.Error("XMPP member sync failed - marking for reconciliation", "chat_id", chat.ID, "user_id", member.UserID, "error", err)
					// Mark for reconciliation
					if err := h.storage.MarkChatForReconciliation(r.Context(), chat.ID, "member_not_added", map[string]interface{}{
						"error":   err.Error(),
						"user_id": member.UserID,
						"role":    role,
					}); err != nil {
						h.logger.Error("failed to mark chat for reconciliation", "chat_id", chat.ID, "error", err)
					}
				}
			}
		}
	}

	members, err := h.storage.GetChatMembersWithUsers(r.Context(), chat.ID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	// Return chat with members embedded
	chatWithMembers := map[string]interface{}{
		"id":          chat.ID,
		"type":        chat.Type,
		"title":       chat.Title,
		"description": chat.Description,
		"avatar":      chat.Avatar,
		"creator_id":  chat.CreatorID,
		"created_at":  chat.CreatedAt,
		"updated_at":  chat.UpdatedAt,
		"members":     members,
	}

	// Manual audit log with correct resource_id (middleware can't extract ID from POST /chats)
	claims, _ = auth.ClaimsFromContext(r.Context())
	if claims.UserID != uuid.Nil {
		h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
			UserID:     claims.UserID,
			Action:     "chat_created",
			Resource:   "chat",
			ResourceID: chat.ID.String(),
			Details:    `{"type":"` + chat.Type + `","title":"` + chat.Title + `"}`,
			IPAddress:  getClientIP(r),
			UserAgent:  r.Header.Get("User-Agent"),
		})
	}

	// Broadcast chat creation to all members
	if h.hub != nil {
		for _, member := range members {
			h.hub.BroadcastToUser(member.UserID, &websocket.BroadcastMessage{
				Type:   websocket.EventChatCreated,
				ChatID: chat.ID,
				Payload: map[string]interface{}{
					"chat": chatWithMembers,
				},
			})
		}
	}

	WriteJSON(w, http.StatusCreated, chatWithMembers)
}

func (h *Handler) getUserChats(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chats, err := h.storage.GetUserChats(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get user chats", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chats")
		return
	}

	// Enrich chats with display names and members
	chatsWithMembers := make([]map[string]interface{}, 0, len(chats))
	for _, chat := range chats {
		chatMap := map[string]interface{}{
			"id":              chat.ID,
			"type":            chat.Type,
			"title":           chat.Title,
			"name":            chat.Title, // Add name field for compatibility
			"description":     chat.Description,
			"avatar":          chat.Avatar,
			"creator_id":      chat.CreatorID,
			"created_at":      chat.CreatedAt,
			"updated_at":      chat.UpdatedAt,
			"last_message":    chat.LastMessage,
			"last_message_at": chat.LastMessageAt,
		}

		// Get members for display name construction
		members, err := h.storage.GetChatMembersWithUsers(r.Context(), chat.ID)
		if err != nil {
			h.logger.Error("failed to get chat members with users", "chat_id", chat.ID, "error", err)
			// Continue without member data - use default display name
		} else {
			// Find other participant for direct chats
			if chat.Type != models.ChatTypeGroup && len(members) == 2 {
				for _, member := range members {
					if member.UserID != claims.UserID && member.User != nil {
						displayName := member.User.FirstName + " " + member.User.LastName
						if displayName == " " || displayName == "" {
							displayName = member.User.Email
						}
						if displayName == "" {
							displayName = "Чат"
						}
						chatMap["display_name"] = displayName
						chatMap["name"] = displayName // Also update name field
						break
					}
				}
			}
			// For group chats, use title if available
			if chat.Type == "group" && chat.Title == "" {
				displayName := "Групповой чат"
				chatMap["title"] = displayName
				chatMap["name"] = displayName // Also update name field
			}
			chatMap["members"] = members
		}

		chatsWithMembers = append(chatsWithMembers, chatMap)
	}

	WriteJSON(w, http.StatusOK, chatsWithMembers)
}

func (h *Handler) getChatByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(idStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat")
		return
	}
	if chat == nil {
		WriteErrorCode(w, http.StatusNotFound, "not_found", "Chat not found")
		return
	}

	// Always load members with user data for API response
	members, err := h.storage.GetChatMembersWithUsers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members with users", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}
	if len(members) > 0 {
		h.logger.Info("getChatById: loaded members", "count", len(members), "first_user", members[0].User)
	} else {
		h.logger.Info("getChatById: loaded members", "count", 0)
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	// Return chat with members embedded
	chatWithMembers := map[string]interface{}{
		"id":          chat.ID,
		"type":        chat.Type,
		"title":       chat.Title,
		"description": chat.Description,
		"avatar":      chat.Avatar,
		"creator_id":  chat.CreatorID,
		"created_at":  chat.CreatedAt,
		"updated_at":  chat.UpdatedAt,
		"members":     members,
	}
	WriteJSON(w, http.StatusOK, chatWithMembers)
}

func (h *Handler) deleteChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Get chat to check type
	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat")
		return
	}
	if chat == nil {
		WriteErrorCode(w, http.StatusNotFound, "not_found", "Chat not found")
		return
	}

	// Get chat members to verify membership
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	// Check if caller is a member and get their role
	var isMember bool
	var callerRole string
	for _, m := range members {
		if m.UserID == claims.UserID {
			isMember = true
			callerRole = m.Role
			break
		}
	}

	if !isMember {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	// For group chats: only owner can delete
	// For direct chats: any member can delete
	if chat.Type == models.ChatTypeGroup && callerRole != models.ChatRoleOwner {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Only chat owner can delete group chat")
		return
	}

	// Delete the chat
	if err := h.storage.DeleteChat(r.Context(), chatID); err != nil {
		h.logger.Error("failed to delete chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to delete chat")
		return
	}

	// Broadcast chat deletion to all members
	if h.hub != nil {
		for _, member := range members {
			h.hub.BroadcastToUser(member.UserID, &websocket.BroadcastMessage{
				Type:   websocket.EventChatDeleted,
				ChatID: chatID,
				Payload: map[string]interface{}{
					"chat_id": chatID.String(),
				},
			})
		}
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) addChatMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		UserID string `json:"user_id"`
		Role   string `json:"role,omitempty"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	// Verify the target user exists
	targetUser, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get target user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to verify user")
		return
	}
	if targetUser == nil {
		WriteErrorCode(w, http.StatusNotFound, "user_not_found", "User not found")
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
		WriteErrorCode(w, http.StatusBadRequest, "invalid_role", "Invalid role. Allowed: owner, admin, member")
		return
	}

	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	// Check if caller is a member and get their role
	var callerRole string
	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
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
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Only chat owner or admin can add members")
		return
	}

	// Only owner can assign owner/admin roles
	if (req.Role == models.ChatRoleOwner || req.Role == models.ChatRoleAdmin) && callerRole != models.ChatRoleOwner {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Only chat owner can assign owner or admin roles")
		return
	}

	// Check if user is already a member
	existingMember := getChatMember(members, userID)
	if existingMember != nil {
		// User exists - check if we're changing their role
		if existingMember.Role == req.Role {
			WriteErrorCode(w, http.StatusConflict, "already_member", "User is already a member with this role")
			return
		}

		// Only owner can change roles
		if callerRole != models.ChatRoleOwner {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Only chat owner can change member roles")
			return
		}

		// Cannot change owner's role
		if existingMember.Role == models.ChatRoleOwner {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Cannot change chat owner's role")
			return
		}
	}

	member := &models.ChatMember{
		ChatID: chatID,
		UserID: userID,
		Role:   req.Role,
	}
	if err := h.storage.AddChatMember(r.Context(), member); err != nil {
		h.logger.Error("failed to add member", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to add member")
		return
	}

	// Add member to XMPP room
	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err == nil && chat != nil && chat.Type == models.ChatTypeGroup {
		// Map internal role to XMPP role
		xmppRole := "member"
		if req.Role == models.ChatRoleOwner {
			xmppRole = "owner"
		} else if req.Role == models.ChatRoleAdmin {
			xmppRole = "admin"
		}
		if err := h.ejabberd.AddMemberToRoom(chatID, userID, xmppRole); err != nil {
			h.logger.Error("failed to add member to XMPP room", "user_id", userID, "error", err)
			// Continue anyway - XMPP is optional
		}
	}

	WriteJSON(w, http.StatusOK, member)
}

func (h *Handler) updateChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validate input lengths
	const maxChatTitleLength = 200
	const maxChatDescriptionLength = 1000
	if len(req.Title) > maxChatTitleLength {
		WriteErrorCode(w, http.StatusBadRequest, "title_too_long", "Chat title exceeds maximum length (200 characters)")
		return
	}
	if len(req.Description) > maxChatDescriptionLength {
		WriteErrorCode(w, http.StatusBadRequest, "description_too_long", "Chat description exceeds maximum length (1000 characters)")
		return
	}

	// Get current chat
	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err != nil || chat == nil {
		WriteErrorCode(w, http.StatusNotFound, "chat_not_found", "Chat not found")
		return
	}

	// Check if user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	// Get user's role
	var userRole string
	for _, m := range members {
		if m.UserID == claims.UserID {
			userRole = m.Role
			break
		}
	}

	// Only owner and admin can update chat
	if userRole != models.ChatRoleOwner && userRole != models.ChatRoleAdmin {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Only owner or admin can update chat")
		return
	}

	// Sanitize inputs
	sanitizedTitle := html.EscapeString(strings.TrimSpace(req.Title))
	sanitizedDescription := html.EscapeString(strings.TrimSpace(req.Description))

	// Validate that title is not empty after sanitization
	if chat.Type == models.ChatTypeGroup && sanitizedTitle == "" {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_title", "Group chat title cannot be empty")
		return
	}

	// Update chat
	chat.Title = sanitizedTitle
	chat.Description = sanitizedDescription

	if err := h.storage.UpdateChat(r.Context(), chat); err != nil {
		h.logger.Error("failed to update chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to update chat")
		return
	}

	// Update XMPP chat room title (only for group chats)
	if chat.Type == models.ChatTypeGroup && chat.Title != "" {
		if err := h.ejabberd.UpdateChatRoomTitle(chat.ID, chat.Title); err != nil {
			h.logger.Error("failed to update XMPP room title", "chat_id", chat.ID, "error", err)
			// Continue anyway - XMPP is optional
		}
	}

	WriteJSON(w, http.StatusOK, chat)
}

func (h *Handler) removeChatMember(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	userIDStr := chi.URLParam(r, "userID")
	targetUserID, err := uuid.Parse(userIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	// Check if caller is a member and get their role
	var callerRole string
	var isCallerMember bool
	for _, m := range members {
		if m.UserID == claims.UserID {
			callerRole = m.Role
			isCallerMember = true
			break
		}
	}

	if !isCallerMember {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	// Check if target is a member
	var targetRole string
	var isTargetMember bool
	for _, m := range members {
		if m.UserID == targetUserID {
			targetRole = m.Role
			isTargetMember = true
			break
		}
	}

	if !isTargetMember {
		WriteErrorCode(w, http.StatusNotFound, "not_found", "User is not a member of this chat")
		return
	}

	// Rules for removal:
	// 1. User can remove themselves (leave chat)
	// 2. Owner can remove anyone except themselves (must transfer ownership first)
	// 3. Admin can remove regular members only
	isSelfRemoval := targetUserID == claims.UserID

	isLastMember := len(members) == 1

	if isSelfRemoval {
		// Owner can leave only if they are the last member (chat will be deleted)
		if callerRole == models.ChatRoleOwner && !isLastMember {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Owner must transfer ownership before leaving or remove all other members first")
			return
		}
	} else {
		// Removing someone else
		if callerRole != models.ChatRoleOwner && callerRole != models.ChatRoleAdmin {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Only owner or admin can remove members")
			return
		}

		// Admin cannot remove owner or other admins
		if callerRole == models.ChatRoleAdmin && (targetRole == models.ChatRoleOwner || targetRole == models.ChatRoleAdmin) {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Admin cannot remove owner or other admins")
			return
		}

		// Only owner can remove other owners/admins or transfer ownership
		if targetRole == models.ChatRoleOwner {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Cannot remove chat owner")
			return
		}
	}

	chatDeleted, err := h.storage.RemoveChatMember(r.Context(), chatID, targetUserID)
	if err != nil {
		h.logger.Error("failed to remove member", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to remove member")
		return
	}

	// Remove member from XMPP room
	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err == nil && chat != nil && chat.Type == models.ChatTypeGroup {
		if err := h.ejabberd.RemoveMemberFromRoom(chatID, targetUserID); err != nil {
			h.logger.Error("failed to remove member from XMPP room", "user_id", targetUserID, "error", err)
			// Continue anyway - XMPP is optional
		}
	}

	response := map[string]interface{}{
		"status":       "removed",
		"chat_deleted": chatDeleted,
	}
	WriteJSON(w, http.StatusOK, response)
}

func getChatMember(members []*models.ChatMember, userID uuid.UUID) *models.ChatMember {
	for _, m := range members {
		if m.UserID == userID {
			return m
		}
	}
	return nil
}

func (h *Handler) muteChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	if err := h.storage.MuteChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to mute chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to mute chat")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "muted"})
}

func (h *Handler) unmuteChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	if err := h.storage.UnmuteChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to unmute chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to unmute chat")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unmuted"})
}

func (h *Handler) pinChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Check if user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
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
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if err := h.storage.PinChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to pin chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to pin chat")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "pinned"})
}

func (h *Handler) unpinChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Check if user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
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
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if err := h.storage.UnpinChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to unpin chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to unpin chat")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unpinned"})
}

func (h *Handler) archiveChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024) // 1KB limit

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if err := h.storage.ArchiveChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to archive chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to archive chat")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "archived"})
}

func (h *Handler) unarchiveChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024) // 1KB limit

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if err := h.storage.UnarchiveChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to unarchive chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to unarchive chat")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unarchived"})
}

func (h *Handler) softDeleteChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024) // 1KB limit

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if err := h.storage.SoftDeleteChat(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to soft delete chat", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to delete chat")
		return
	}

	// Destroy XMPP chat room
	if err := h.ejabberd.DestroyChatRoom(chatID); err != nil {
		h.logger.Error("failed to destroy XMPP chat room", "error", err)
		// Continue anyway - XMPP is optional
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) clearChatHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024) // 1KB limit

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if err := h.storage.ClearChatHistory(r.Context(), chatID, claims.UserID); err != nil {
		h.logger.Error("failed to clear chat history", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to clear chat history")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (h *Handler) exportChat(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	// Parse pagination parameters (default to 1000 messages max to prevent memory exhaustion)
	limit := 1000
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 10000 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get messages with pagination
	messages, err := h.storage.GetMessagesByChatForUser(r.Context(), chatID, claims.UserID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get messages for export", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to export chat")
		return
	}

	// Format as JSON to match frontend expectations
	exportData := map[string]interface{}{
		"chat_id":        chatID,
		"export_date":    time.Now(),
		"messages":       messages,
		"total_messages": len(messages),
	}

	// Set headers for download
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=chat_%s_export.json", chatID))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(exportData)
}

func (h *Handler) addBookmark(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024) // 1KB limit

	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	messageID, err := uuid.Parse(req.MessageID)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_message_id", "Invalid message ID")
		return
	}

	// Verify message exists
	msg, err := h.storage.GetMessageByID(r.Context(), messageID)
	if err != nil {
		h.logger.Error("failed to get message", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get message")
		return
	}
	if msg == nil {
		WriteErrorCode(w, http.StatusNotFound, "message_not_found", "Message not found")
		return
	}

	// Verify user is a member of the chat the message belongs to
	members, err := h.storage.GetChatMembers(r.Context(), msg.ChatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to verify chat membership")
		return
	}

	if !isChatMember(members, claims.UserID) {
		WriteErrorCode(w, http.StatusForbidden, "forbidden", "You are not a member of this chat")
		return
	}

	if err := h.storage.AddBookmark(r.Context(), claims.UserID, messageID); err != nil {
		h.logger.Error("failed to add bookmark", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to add bookmark")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "bookmarked"})
}

func (h *Handler) removeBookmark(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	messageID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_message_id", "Invalid message ID")
		return
	}

	if err := h.storage.RemoveBookmark(r.Context(), claims.UserID, messageID); err != nil {
		h.logger.Error("failed to remove bookmark", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to remove bookmark")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

func (h *Handler) getBookmarks(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	messages, err := h.storage.GetBookmarks(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get bookmarks", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get bookmarks")
		return
	}

	WriteJSON(w, http.StatusOK, messages)
}

func (h *Handler) blockUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, 1024) // 1KB limit

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	blockedID, err := uuid.Parse(req.UserID)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	if blockedID == claims.UserID {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_action", "Cannot block yourself")
		return
	}

	// Verify the target user exists
	targetUser, err := h.storage.GetUserByID(r.Context(), blockedID)
	if err != nil {
		h.logger.Error("failed to get target user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to verify user")
		return
	}
	if targetUser == nil {
		WriteErrorCode(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	if err := h.storage.BlockUser(r.Context(), claims.UserID, blockedID); err != nil {
		h.logger.Error("failed to block user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to block user")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

func (h *Handler) unblockUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_user_id", "Invalid user ID")
		return
	}

	if err := h.storage.UnblockUser(r.Context(), claims.UserID, userID); err != nil {
		h.logger.Error("failed to unblock user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to unblock user")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

func (h *Handler) getBlockedUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	blockedIDs, err := h.storage.GetBlockedUsers(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get blocked users", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get blocked users")
		return
	}

	WriteJSON(w, http.StatusOK, blockedIDs)
}

func (h *Handler) getPinnedMessages(w http.ResponseWriter, r *http.Request) {
	chatIDStr := chi.URLParam(r, "id")
	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	messages, err := h.storage.GetPinnedMessages(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get pinned messages", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get pinned messages")
		return
	}

	WriteJSON(w, http.StatusOK, messages)
}
