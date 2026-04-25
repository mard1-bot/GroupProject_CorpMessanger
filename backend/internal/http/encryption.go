package http

import (
	"encoding/base64"
	"net/http"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/google/uuid"
)

// Current key version - increment when implementing key rotation
// This value should be stored in the database and incremented when
// a new key version is introduced. For now, it's static.
const currentKeyVersion = 1

// RegisterPublicKeyRequest represents a request to register user's public key
type RegisterPublicKeyRequest struct {
	PublicKey  string `json:"public_key"`            // Base64 encoded X25519 public key
	PrivateKey string `json:"private_key,omitempty"` // Optional: encrypted private key for backup
}

// PublicKeyResponse represents a user's public key response
type PublicKeyResponse struct {
	UserID     string `json:"user_id"`
	PublicKey  string `json:"public_key"`
	KeyVersion int    `json:"key_version"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// GetChatKeysResponse represents public keys for all chat members
type GetChatKeysResponse struct {
	Keys []PublicKeyResponse `json:"keys"`
}

func (h *Handler) registerPublicKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 1KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req RegisterPublicKeyRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.PublicKey == "" {
		WriteError(w, http.StatusBadRequest, "missing_key", "Public key is required")
		return
	}

	// Validate public key is valid base64
	decodedKey, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_base64", "Public key must be valid base64")
		return
	}

	// X25519 public key should be 32 bytes
	if len(decodedKey) != 32 {
		WriteError(w, http.StatusBadRequest, "invalid_key_length", "Public key must be 32 bytes (X25519)")
		return
	}

	// Validate private key if provided
	if req.PrivateKey != "" {
		decodedPrivateKey, err := base64.StdEncoding.DecodeString(req.PrivateKey)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_base64", "Private key must be valid base64")
			return
		}
		// X25519 private key should be 32 bytes
		if len(decodedPrivateKey) != 32 {
			WriteError(w, http.StatusBadRequest, "invalid_key_length", "Private key must be 32 bytes (X25519)")
			return
		}
	}

	// Check if user already has a key registered
	existingKey, err := h.storage.GetUserPublicKey(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to check existing public key", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to check existing key")
		return
	}
	if existingKey != nil {
		// Key rotation is not yet implemented - prevent overwriting
		// In the future, this should implement proper key rotation with version increment
		WriteError(w, http.StatusConflict, "key_exists", "Encryption key already registered. Key rotation is not yet supported.")
		return
	}

	key := &models.EncryptionKey{
		UserID:     claims.UserID,
		PublicKey:  req.PublicKey,
		PrivateKey: req.PrivateKey,
		KeyVersion: currentKeyVersion,
	}

	if err := h.storage.SaveUserPublicKey(r.Context(), key); err != nil {
		h.logger.Error("failed to save public key", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to save public key")
		return
	}

	WriteJSON(w, http.StatusCreated, PublicKeyResponse{
		UserID:     key.UserID.String(),
		PublicKey:  key.PublicKey,
		KeyVersion: key.KeyVersion,
		CreatedAt:  key.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *Handler) getMyPublicKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	key, err := h.storage.GetUserPublicKey(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get public key", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get public key")
		return
	}

	if key == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Public key not found")
		return
	}

	WriteJSON(w, http.StatusOK, PublicKeyResponse{
		UserID:     key.UserID.String(),
		PublicKey:  key.PublicKey,
		KeyVersion: key.KeyVersion,
		CreatedAt:  key.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *Handler) getUserPublicKey(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		WriteError(w, http.StatusBadRequest, "missing_param", "user_id parameter is required")
		return
	}

	// Validate query parameter length to prevent DoS
	if len(userIDStr) > 100 {
		WriteError(w, http.StatusBadRequest, "invalid_param", "user_id parameter is too long")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid user ID")
		return
	}

	// Users can only get public keys of users they share chats with
	// This is a privacy measure - check if they have common chats
	hasCommonChat, err := h.storage.UsersShareChat(r.Context(), claims.UserID, userID)
	if err != nil {
		h.logger.Error("failed to check shared chat", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to verify access")
		return
	}

	if !hasCommonChat {
		WriteError(w, http.StatusForbidden, "access_denied", "You don't share any chats with this user")
		return
	}

	key, err := h.storage.GetUserPublicKey(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get public key", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get public key")
		return
	}

	if key == nil {
		WriteError(w, http.StatusNotFound, "not_found", "Public key not found for this user")
		return
	}

	WriteJSON(w, http.StatusOK, PublicKeyResponse{
		UserID:     key.UserID.String(),
		PublicKey:  key.PublicKey,
		KeyVersion: key.KeyVersion,
		CreatedAt:  key.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *Handler) getChatPublicKeys(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	chatIDStr := r.URL.Query().Get("chat_id")
	if chatIDStr == "" {
		WriteError(w, http.StatusBadRequest, "missing_param", "chat_id parameter is required")
		return
	}

	chatID, err := uuid.Parse(chatIDStr)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_id", "Invalid chat ID")
		return
	}

	// Verify user is a member of this chat
	members, err := h.storage.GetChatMembers(r.Context(), chatID)
	if err != nil {
		h.logger.Error("failed to get chat members", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get chat members")
		return
	}

	isMember := false
	memberIDs := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		memberIDs = append(memberIDs, m.UserID)
		if m.UserID == claims.UserID {
			isMember = true
		}
	}

	if !isMember {
		WriteError(w, http.StatusForbidden, "access_denied", "You are not a member of this chat")
		return
	}

	// Get public keys for all members
	keys, err := h.storage.GetUsersPublicKeys(r.Context(), memberIDs)
	if err != nil {
		h.logger.Error("failed to get public keys", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to get public keys")
		return
	}

	response := GetChatKeysResponse{
		Keys: make([]PublicKeyResponse, 0, len(keys)),
	}

	for _, key := range keys {
		response.Keys = append(response.Keys, PublicKeyResponse{
			UserID:     key.UserID.String(),
			PublicKey:  key.PublicKey,
			KeyVersion: key.KeyVersion,
			CreatedAt:  key.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	WriteJSON(w, http.StatusOK, response)
}
