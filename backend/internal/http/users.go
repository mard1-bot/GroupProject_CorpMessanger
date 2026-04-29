package http

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// phoneRegex requires at least 10 digits, allows +, spaces, dashes, parentheses, dots
// Examples: +1234567890, +1 (123) 456-7890, +1.202.555.0191
var phoneRegex = regexp.MustCompile(`^[+]?[\s\d\-\(\)\.]{10,25}$`)

func isValidPhone(phone string) bool {
	if !phoneRegex.MatchString(phone) {
		return false
	}
	// Count digits - must be at least 10
	digitCount := 0
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			digitCount++
		}
	}
	return digitCount >= 10
}

func (h *Handler) getUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Get query parameters
	search := r.URL.Query().Get("search")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Search users, excluding current user
	users, err := h.storage.GetUsers(r.Context(), search, claims.UserID.String(), limit)
	if err != nil {
		h.logger.Error("failed to get users", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get users")
		return
	}
	WriteJSON(w, http.StatusOK, users)
}

func (h *Handler) getUserByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid user ID")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	// Users can only view their own profile
	if user.ID != claims.UserID {
		WriteError(w, http.StatusForbidden, "forbidden", "You can only view your own profile")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var updates struct {
		FirstName  string `json:"first_name,omitempty"`
		LastName   string `json:"last_name,omitempty"`
		MiddleName string `json:"middle_name,omitempty"`
		Phone      string `json:"phone,omitempty"`
		Avatar     string `json:"avatar,omitempty"`
	}

	if err := decodeJSON(r.Body, &updates); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Sanitize inputs (trim whitespace)
	updates.FirstName = strings.TrimSpace(updates.FirstName)
	updates.LastName = strings.TrimSpace(updates.LastName)
	updates.MiddleName = strings.TrimSpace(updates.MiddleName)
	updates.Phone = strings.TrimSpace(updates.Phone)
	updates.Avatar = strings.TrimSpace(updates.Avatar)

	// Validate input lengths
	maxNameLength := 100
	if len(updates.FirstName) > maxNameLength {
		WriteError(w, http.StatusBadRequest, "first_name_too_long", "First name is too long")
		return
	}
	if len(updates.LastName) > maxNameLength {
		WriteError(w, http.StatusBadRequest, "last_name_too_long", "Last name is too long")
		return
	}
	if len(updates.MiddleName) > maxNameLength {
		WriteError(w, http.StatusBadRequest, "middle_name_too_long", "Middle name is too long")
		return
	}
	if len(updates.Avatar) > 2048 {
		WriteError(w, http.StatusBadRequest, "avatar_url_too_long", "Avatar URL is too long (max 2048 characters)")
		return
	}

	// Basic phone validation (if provided)
	if updates.Phone != "" && !isValidPhone(updates.Phone) {
		WriteError(w, http.StatusBadRequest, "invalid_phone", "Invalid phone number format")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get user by ID", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	// Only validate and update fields that are provided (partial updates allowed)
	if updates.FirstName != "" {
		user.FirstName = updates.FirstName
	}
	if updates.LastName != "" {
		user.LastName = updates.LastName
	}
	if updates.MiddleName != "" {
		trimmed := strings.TrimSpace(updates.MiddleName)
		user.MiddleName = &trimmed
	}
	if updates.Phone != "" {
		user.Phone = updates.Phone
	}
	if updates.Avatar != "" {
		user.Avatar = &updates.Avatar
	}

	if err := h.storage.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to update user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update user")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit upload size to 5MB
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)

	// Parse multipart form (max 32MB in memory)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_form", "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "missing_file", "Avatar file is required")
		return
	}
	defer file.Close()

	// Validate file type (only images)
	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	contentType := header.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		WriteError(w, http.StatusBadRequest, "invalid_type", "Only image files (JPEG, PNG, GIF, WebP) are allowed")
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "read_error", "Failed to read file")
		return
	}

	// Validate file size after reading (double-check)
	if len(content) > 5*1024*1024 {
		WriteError(w, http.StatusBadRequest, "file_too_large", "File size exceeds 5MB limit")
		return
	}

	// Validate actual file content using magic bytes
	if len(content) < 4 {
		WriteError(w, http.StatusBadRequest, "invalid_file", "File is too small to be a valid image")
		return
	}

	// Check magic bytes for common image formats
	magicBytes := content[:4]
	validMagic := false
	switch contentType {
	case "image/jpeg", "image/jpg":
		// JPEG starts with FF D8 FF
		if magicBytes[0] == 0xFF && magicBytes[1] == 0xD8 && magicBytes[2] == 0xFF {
			validMagic = true
		}
	case "image/png":
		// PNG starts with 89 50 4E 47
		if magicBytes[0] == 0x89 && magicBytes[1] == 0x50 && magicBytes[2] == 0x4E && magicBytes[3] == 0x47 {
			validMagic = true
		}
	case "image/gif":
		// GIF starts with 47 49 46 38 (GIF8)
		if magicBytes[0] == 0x47 && magicBytes[1] == 0x49 && magicBytes[2] == 0x46 && magicBytes[3] == 0x38 {
			validMagic = true
		}
	case "image/webp":
		// WebP starts with 52 49 46 46 (RIFF)
		if magicBytes[0] == 0x52 && magicBytes[1] == 0x49 && magicBytes[2] == 0x46 && magicBytes[3] == 0x46 {
			validMagic = true
		}
	}

	if !validMagic {
		WriteError(w, http.StatusBadRequest, "invalid_content", "File content does not match declared type")
		return
	}

	// Ensure avatars directory exists
	avatarsDir := "./uploads/avatars"
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		h.logger.Error("failed to create avatars directory", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save avatar")
		return
	}

	// Generate unique filename with safe extension
	ext := filepath.Ext(header.Filename)
	// Sanitize extension to only allow known image extensions
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	ext = strings.ToLower(ext)
	if !allowedExts[ext] {
		// Default to .jpg if extension is invalid
		ext = ".jpg"
	}
	filename := "avatar_" + claims.UserID.String() + ext
	fullPath := filepath.Join(avatarsDir, filename)

	// Delete old avatar if it exists
	user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get user")
		return
	}
	if user == nil {
		WriteError(w, http.StatusNotFound, "not_found", "User not found")
		return
	}

	if user.Avatar != nil && *user.Avatar != "" {
		// Extract filename from URL
		oldFilename := filepath.Base(*user.Avatar)
		oldFilepath := filepath.Join(avatarsDir, oldFilename)
		if err := os.Remove(oldFilepath); err != nil && !os.IsNotExist(err) {
			h.logger.Warn("failed to delete old avatar", "error", err)
			// Continue anyway, don't block upload
		}
	}

	// Save file to disk
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		h.logger.Error("failed to save avatar file", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save avatar")
		return
	}

	avatarURL := "/uploads/avatars/" + filename

	user.Avatar = &avatarURL
	if err := h.storage.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to update user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update user")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL})
}

func (h *Handler) uploadChatAvatar(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member and has permission (owner/admin)
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

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
		WriteError(w, http.StatusForbidden, "forbidden", "Not a member of this chat")
		return
	}

	if callerRole != models.ChatRoleOwner && callerRole != models.ChatRoleAdmin {
		WriteError(w, http.StatusForbidden, "forbidden", "Only owner or admin can update chat avatar")
		return
	}

	// Limit upload size to 5MB
	r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_form", "Failed to parse form")
		return
	}

	file, header, err := r.FormFile("avatar")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "missing_file", "Avatar file is required")
		return
	}
	defer file.Close()

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/jpg":  true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}
	contentType := header.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		WriteError(w, http.StatusBadRequest, "invalid_type", "Only image files (JPEG, PNG, GIF, WebP) are allowed")
		return
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "read_error", "Failed to read file")
		return
	}

	// Validate file size after reading (double-check)
	if len(content) > 5*1024*1024 {
		WriteError(w, http.StatusBadRequest, "file_too_large", "File size exceeds 5MB limit")
		return
	}

	// Validate actual file content using magic bytes
	if len(content) < 4 {
		WriteError(w, http.StatusBadRequest, "invalid_file", "File is too small to be a valid image")
		return
	}

	// Check magic bytes for common image formats
	magicBytes := content[:4]
	validMagic := false
	switch contentType {
	case "image/jpeg", "image/jpg":
		// JPEG starts with FF D8 FF
		if magicBytes[0] == 0xFF && magicBytes[1] == 0xD8 && magicBytes[2] == 0xFF {
			validMagic = true
		}
	case "image/png":
		// PNG starts with 89 50 4E 47
		if magicBytes[0] == 0x89 && magicBytes[1] == 0x50 && magicBytes[2] == 0x4E && magicBytes[3] == 0x47 {
			validMagic = true
		}
	case "image/gif":
		// GIF starts with 47 49 46 38 (GIF8)
		if magicBytes[0] == 0x47 && magicBytes[1] == 0x49 && magicBytes[2] == 0x46 && magicBytes[3] == 0x38 {
			validMagic = true
		}
	case "image/webp":
		// WebP starts with 52 49 46 46 (RIFF)
		if magicBytes[0] == 0x52 && magicBytes[1] == 0x49 && magicBytes[2] == 0x46 && magicBytes[3] == 0x46 {
			validMagic = true
		}
	}

	if !validMagic {
		WriteError(w, http.StatusBadRequest, "invalid_content", "File content does not match declared type")
		return
	}

	// Ensure avatars directory exists
	avatarsDir := "./uploads/avatars"
	if err := os.MkdirAll(avatarsDir, 0755); err != nil {
		h.logger.Error("failed to create avatars directory", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save avatar")
		return
	}

	ext := filepath.Ext(header.Filename)
	// Sanitize extension to only allow known image extensions
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	ext = strings.ToLower(ext)
	if !allowedExts[ext] {
		// Default to .png if extension is invalid
		ext = ".png"
	}
	filename := "chat_" + chatID.String() + ext
	fullPath := filepath.Join(avatarsDir, filename)

	// Delete old avatar if it exists
	chat, err := h.storage.GetChatByID(r.Context(), chatID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat")
		return
	}
	if chat == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Chat not found")
		return
	}

	if chat.Avatar != "" {
		// Extract filename from URL
		oldFilename := filepath.Base(chat.Avatar)
		oldFilepath := filepath.Join(avatarsDir, oldFilename)
		if err := os.Remove(oldFilepath); err != nil && !os.IsNotExist(err) {
			h.logger.Warn("failed to delete old chat avatar", "error", err)
			// Continue anyway, don't block upload
		}
	}

	// Save file to disk
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		h.logger.Error("failed to save chat avatar file", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save avatar")
		return
	}

	avatarURL := "/uploads/avatars/" + filename

	chat.Avatar = avatarURL
	if err := h.storage.UpdateChat(r.Context(), chat); err != nil {
		h.logger.Error("failed to update chat avatar", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update chat avatar")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"avatar_url": avatarURL})
}

func (h *Handler) updateUserStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req struct {
		Status       string `json:"status"`
		CustomStatus string `json:"custom_status"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	validStatuses := map[string]bool{"online": true, "away": true, "busy": true, "invisible": true}
	if req.Status != "" && !validStatuses[req.Status] {
		WriteError(w, http.StatusBadRequest, "invalid_status", "Invalid status value")
		return
	}

	// Validate custom status length
	if req.CustomStatus != "" && len(req.CustomStatus) > 200 {
		WriteError(w, http.StatusBadRequest, "custom_status_too_long", "Custom status exceeds maximum length (200 characters)")
		return
	}

	if err := h.storage.UpdateUserStatus(r.Context(), claims.UserID, req.Status, req.CustomStatus); err != nil {
		h.logger.Error("failed to update user status", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update status")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
