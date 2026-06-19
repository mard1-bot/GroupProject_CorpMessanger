package http

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AdminCreateUserRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password"`
	Phone      string `json:"phone,omitempty"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	MiddleName string `json:"middle_name,omitempty"`
	Role       string `json:"role,omitempty"`
	Status     string `json:"status,omitempty"`
}

type AdminUpdateUserRequest struct {
	Email      string  `json:"email,omitempty"`
	Username   *string `json:"username,omitempty"`
	Phone      string  `json:"phone,omitempty"`
	FirstName  string  `json:"first_name,omitempty"`
	LastName   string  `json:"last_name,omitempty"`
	MiddleName *string `json:"middle_name,omitempty"`
	Role       string  `json:"role,omitempty"`
	Status     string  `json:"status,omitempty"`
}

// AdminChangeRoleRequest represents role change
type AdminChangeRoleRequest struct {
	Role string `json:"role"`
}

// AdminBlockUserRequest represents user block/unblock
type AdminBlockUserRequest struct {
	Reason string `json:"reason,omitempty"`
}

// adminGetUsers returns all users with pagination (admin only)
func (h *Handler) adminGetUsers(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	roleFilter := r.URL.Query().Get("role")
	statusFilter := r.URL.Query().Get("status")

	limit := 50
	offset := 0
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
		limit = l
	}
	if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
		offset = o
	}

	users, err := h.storage.GetUsers(r.Context(), search, "", limit)
	if err != nil {
		h.logger.Error("failed to get users", "error", err)
		WriteError(w, ErrInternal, "Failed to get users")
		return
	}

	// Filter by role/status if specified
	filtered := make([]*models.User, 0)
	for _, u := range users {
		if roleFilter != "" && u.Role != roleFilter {
			continue
		}
		if statusFilter != "" && u.Status != statusFilter {
			continue
		}
		filtered = append(filtered, u)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"users":  filtered,
		"count":  len(filtered),
		"limit":  limit,
		"offset": offset,
	})
}

// adminGetUserByID returns a specific user by ID (admin only)
func (h *Handler) adminGetUserByID(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

// adminCreateUser creates a new user account (admin only)
func (h *Handler) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	var req AdminCreateUserRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Email == "" {
		WriteError(w, ErrInvalidInput, "Email is required")
		return
	}
	if !ValidateEmail(req.Email) {
		WriteError(w, ErrInvalidInput, "Invalid email format")
		return
	}
	if req.Password == "" {
		WriteError(w, ErrInvalidInput, "Password is required")
		return
	}
	if err := ValidatePassword(req.Password); err != nil {
		WriteError(w, ErrInvalidInput, "Password does not meet requirements")
		return
	}
	if req.FirstName == "" {
		WriteError(w, ErrInvalidInput, "First name is required")
		return
	}
	if req.LastName == "" {
		WriteError(w, ErrInvalidInput, "Last name is required")
		return
	}

	// Validate role
	role := req.Role
	if role == "" {
		role = models.UserRoleUser
	}
	validRoles := map[string]bool{
		models.UserRoleUser:      true,
		models.UserRoleAdmin:     true,
		models.UserRoleModerator: true,
	}
	if !validRoles[role] {
		WriteError(w, ErrInvalidInput, "Invalid role. Must be user, admin, or moderator")
		return
	}

	// Validate status
	status := req.Status
	if status == "" {
		status = models.UserStatusActive
	}
	validStatuses := map[string]bool{
		models.UserStatusActive:   true,
		models.UserStatusInactive: true,
	}
	if !validStatuses[status] {
		WriteError(w, ErrInvalidInput, "Invalid status for new user")
		return
	}

	// Check if email already exists
	existing, _, err := h.storage.GetUserByEmail(r.Context(), req.Email)
	if err == nil && existing != nil {
		WriteError(w, ErrConflict, "Email already registered")
		return
	}

	// Validate and check username uniqueness
	req.Username = strings.TrimSpace(req.Username)
	if strings.HasPrefix(req.Username, "@") {
		req.Username = req.Username[1:]
	}
	if req.Username != "" {
		importRegexp := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
		if len(req.Username) < 3 || len(req.Username) > 50 || !importRegexp.MatchString(req.Username) {
			WriteError(w, ErrInvalidInput, "Username must be 3-50 characters long and contain only letters, numbers, and underscores")
			return
		}
		existingUsername, err := h.storage.GetUserByUsername(r.Context(), req.Username)
		if err == nil && existingUsername != nil {
			WriteError(w, ErrConflict, "Username is already taken")
			return
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		WriteError(w, ErrInternal, "Failed to create user")
		return
	}

	user := &models.User{
		Email:     req.Email,
		Phone:     req.Phone,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Status:    status,
		Role:      role,
	}
	if req.MiddleName != "" {
		user.MiddleName = &req.MiddleName
	}
	if req.Username != "" {
		user.Username = &req.Username
	}

	if err := h.storage.CreateUser(r.Context(), user, string(hashedPassword)); err != nil {
		h.logger.Error("failed to create user", "error", err)
		WriteError(w, ErrInternal, "Failed to create user")
		return
	}

	// Create XMPP user
	if err := h.ejabberd.CreateUser(user.ID, req.Password); err != nil {
		h.logger.Error("failed to create XMPP user", "error", err)
	}

	// Audit log
	claims, _ := auth.ClaimsFromContext(r.Context())
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_create_user",
		Resource:   "user",
		ResourceID: user.ID.String(),
		Details:    `{"role":"` + role + `"}`,
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusCreated, user)
}

// adminUpdateUser updates a user account (admin only)
func (h *Handler) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req AdminUpdateUserRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request body")
		return
	}
	defer r.Body.Close()

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	// Admin cannot modify their own account through this endpoint
	claims, _ := auth.ClaimsFromContext(r.Context())
	if claims.UserID == userID {
		WriteError(w, ErrConflict, "Cannot modify your own account through admin panel")
		return
	}

	// Update fields
	if req.Email != "" && req.Email != user.Email {
		if !ValidateEmail(req.Email) {
			WriteError(w, ErrInvalidInput, "Invalid email format")
			return
		}
		// Check uniqueness
		existing, _, err := h.storage.GetUserByEmail(r.Context(), req.Email)
		if err == nil && existing != nil && existing.ID != user.ID {
			WriteError(w, ErrConflict, "Email is already taken by another user")
			return
		}
		user.Email = req.Email
	}
	
	if req.Username != nil {
		newUsername := strings.TrimSpace(*req.Username)
		if strings.HasPrefix(newUsername, "@") {
			newUsername = newUsername[1:]
		}
		if newUsername != "" {
			importRegexp := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
			if len(newUsername) < 3 || len(newUsername) > 50 || !importRegexp.MatchString(newUsername) {
				WriteError(w, ErrInvalidInput, "Username must be 3-50 characters long and contain only letters, numbers, and underscores")
				return
			}
			if user.Username == nil || *user.Username != newUsername {
				// Check uniqueness
				existingUsername, err := h.storage.GetUserByUsername(r.Context(), newUsername)
				if err == nil && existingUsername != nil && existingUsername.ID != user.ID {
					WriteError(w, ErrConflict, "Username is already taken")
					return
				}
				user.Username = &newUsername
			}
		} else {
			// If explicitly set to empty string, it might mean unset (but database requires UNIQUE index with WHERE IS NOT NULL)
			// we can set it to nil or just not update it. Let's not allow removing username for now, or just set to nil
			// actually we will let them pass empty string to unset it
			user.Username = nil
		}
	}
	
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.MiddleName != nil {
		if *req.MiddleName == "" {
			user.MiddleName = nil
		} else {
			user.MiddleName = req.MiddleName
		}
	}

	if err := h.storage.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to update user", "error", err)
		WriteError(w, ErrInternal, "Failed to update user")
		return
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_update_user",
		Resource:   "user",
		ResourceID: userID.String(),
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusOK, user)
}

// adminChangeRole changes a user's role (admin only)
func (h *Handler) adminChangeRole(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req AdminChangeRoleRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request body")
		return
	}
	defer r.Body.Close()

	validRoles := map[string]bool{
		models.UserRoleUser:      true,
		models.UserRoleAdmin:     true,
		models.UserRoleModerator: true,
	}
	if !validRoles[req.Role] {
		WriteError(w, ErrInvalidInput, "Invalid role. Must be user, admin, or moderator")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())

	// Cannot change own role
	if claims.UserID == userID {
		WriteError(w, ErrConflict, "Cannot change your own role")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	oldRole := user.Role
	user.Role = req.Role
	if err := h.storage.UpdateUserRole(r.Context(), userID, req.Role); err != nil {
		h.logger.Error("failed to update user role", "error", err)
		WriteError(w, ErrInternal, "Failed to update role")
		return
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_change_role",
		Resource:   "user",
		ResourceID: userID.String(),
		Details:    `{"old_role":"` + oldRole + `","new_role":"` + req.Role + `"}`,
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":  userID,
		"old_role": oldRole,
		"new_role": req.Role,
	})
}

// adminBlockUser blocks a user account (admin only)
func (h *Handler) adminBlockUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req AdminBlockUserRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		// Reason is optional, continue without it
		req.Reason = ""
	}
	r.Body.Close()

	claims, _ := auth.ClaimsFromContext(r.Context())

	// Cannot block self
	if claims.UserID == userID {
		WriteError(w, ErrConflict, "Cannot block your own account")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	// Cannot block another admin
	if user.Role == models.UserRoleAdmin {
		WriteError(w, ErrConflict, "Cannot block another admin")
		return
	}

	if user.Status == models.UserStatusBlocked {
		WriteError(w, ErrConflict, "User is already blocked")
		return
	}

	if err := h.storage.UpdateUserStatus(r.Context(), userID, models.UserStatusBlocked, req.Reason); err != nil {
		h.logger.Error("failed to block user", "error", err)
		WriteError(w, ErrInternal, "Failed to block user")
		return
	}

	// Revoke all sessions for blocked user
	h.storage.DeleteAllUserSessions(r.Context(), userID)

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_block_user",
		Resource:   "user",
		ResourceID: userID.String(),
		Details:    `{"reason":"` + req.Reason + `"}`,
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"status":  models.UserStatusBlocked,
	})
}

// adminUnblockUser unblocks a user account (admin only)
func (h *Handler) adminUnblockUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	if user.Status != models.UserStatusBlocked {
		WriteError(w, ErrConflict, "User is not blocked")
		return
	}

	if err := h.storage.UpdateUserStatus(r.Context(), userID, models.UserStatusActive, ""); err != nil {
		h.logger.Error("failed to unblock user", "error", err)
		WriteError(w, ErrInternal, "Failed to unblock user")
		return
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_unblock_user",
		Resource:   "user",
		ResourceID: userID.String(),
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"user_id": userID,
		"status":  models.UserStatusActive,
	})
}

// adminDeleteUser deletes a user account from the system (admin only)
func (h *Handler) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())

	// Cannot delete self
	if claims.UserID == userID {
		WriteError(w, ErrConflict, "Cannot delete your own account")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	// Cannot delete another admin
	if user.Role == models.UserRoleAdmin {
		WriteError(w, ErrConflict, "Cannot delete another admin")
		return
	}

	// Revoke all sessions
	h.storage.DeleteAllUserSessions(r.Context(), userID)

	// Delete XMPP user
	if err := h.ejabberd.DeleteUser(userID); err != nil {
		h.logger.Error("failed to delete XMPP user", "error", err)
	}

	// Delete user from database
	if err := h.storage.DeleteUser(r.Context(), userID); err != nil {
		h.logger.Error("failed to delete user", "error", err)
		WriteError(w, ErrInternal, "Failed to delete user")
		return
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_delete_user",
		Resource:   "user",
		ResourceID: userID.String(),
		Details:    `{"deleted_email":"` + user.Email + `"}`,
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "User deleted successfully",
		"user_id": userID,
	})
}

// adminResetPassword resets a user's password (admin only)
func (h *Handler) adminResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, ErrInvalidInput, "Invalid user ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Password == "" {
		WriteError(w, ErrInvalidInput, "Password is required")
		return
	}
	if err := ValidatePassword(req.Password); err != nil {
		WriteError(w, ErrInvalidInput, "Password does not meet requirements")
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())

	// Cannot reset own password here (use /auth/change-password)
	if claims.UserID == userID {
		WriteError(w, ErrConflict, "Use /auth/change-password to change your own password")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	// Get current password hash to check if new password is different
	userWithPassword, passwordHash, err := h.storage.GetUserByIDWithPassword(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get user password", "error", err)
		WriteError(w, ErrInternal, "Failed to get user")
		return
	}
	if userWithPassword == nil {
		WriteError(w, ErrNotFound, "User not found")
		return
	}

	// Check if new password is same as old password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err == nil {
		WriteError(w, ErrInvalidInput, "New password must be different from current password")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		WriteError(w, ErrInternal, "Failed to reset password")
		return
	}

	if err := h.storage.UpdateUserPassword(r.Context(), userID, string(hashedPassword)); err != nil {
		h.logger.Error("failed to update password", "error", err)
		WriteError(w, ErrInternal, "Failed to reset password")
		return
	}

	// Revoke all sessions so user must re-login
	h.storage.DeleteAllUserSessions(r.Context(), userID)

	// Update XMPP password
	if err := h.ejabberd.UpdateUserPassword(userID, req.Password); err != nil {
		h.logger.Error("failed to update XMPP password", "error", err)
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     claims.UserID,
		Action:     "admin_reset_password",
		Resource:   "user",
		ResourceID: userID.String(),
		IPAddress:  getClientIP(r),
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Password reset successfully",
		"user_id": userID,
	})
}

// adminGetStats returns system statistics (admin only)
func (h *Handler) adminGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.storage.GetAdminStats(r.Context())
	if err != nil {
		h.logger.Error("failed to get admin stats", "error", err)
		WriteError(w, ErrInternal, "Failed to get statistics")
		return
	}

	WriteJSON(w, http.StatusOK, stats)
}

// adminGetAuditLogs returns all audit logs (admin only)
func (h *Handler) adminGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	h.getAllAuditLogs(w, r)
}
