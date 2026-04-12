package http

import (
	"net/http"
	"regexp"
	"strings"

	"corp-messenger/backend/internal/auth"

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
	users, err := h.storage.GetUsers(r.Context())
	if err != nil {
		h.logger.Error("failed to get users", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get users")
		return
	}
	WriteJSON(w, http.StatusOK, users)
}

func (h *Handler) getUserByID(w http.ResponseWriter, r *http.Request) {
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
	WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

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

	if updates.FirstName != "" {
		user.FirstName = updates.FirstName
	}
	if updates.LastName != "" {
		user.LastName = updates.LastName
	}
	if updates.MiddleName != "" {
		user.MiddleName = updates.MiddleName
	}
	if updates.Phone != "" {
		user.Phone = updates.Phone
	}
	if updates.Avatar != "" {
		user.Avatar = updates.Avatar
	}

	if err := h.storage.UpdateUser(r.Context(), user); err != nil {
		h.logger.Error("failed to update user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to update user")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}
