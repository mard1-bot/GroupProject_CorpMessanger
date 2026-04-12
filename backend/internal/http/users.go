package http

import (
	"net/http"

	"corp-messenger/backend/internal/auth"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

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

	user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
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

	WriteJSON(w, http.StatusOK, user)
}
