package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// isValidEmail uses net/mail for RFC-compliant validation
func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

type RegisterRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	MiddleName string `json:"middle_name,omitempty"`
	Phone      string `json:"phone,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		WriteError(w, http.StatusBadRequest, "missing_fields", "Email, password, first_name and last_name are required")
		return
	}

	if !isValidEmail(req.Email) {
		WriteError(w, http.StatusBadRequest, "invalid_email", "Invalid email format")
		return
	}

	// Password strength validation
	if len(req.Password) < 8 {
		WriteError(w, http.StatusBadRequest, "password_too_short", "Password must be at least 8 characters long")
		return
	}
	// Check for at least one uppercase, one lowercase, one digit (Unicode-aware)
	var hasUpper, hasLower, hasDigit bool
	for _, ch := range req.Password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		WriteError(w, http.StatusBadRequest, "password_too_weak", "Password must contain at least one uppercase letter, one lowercase letter, and one digit")
		return
	}

	existingUser, _, err := h.storage.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("failed to check existing user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to check user existence")
		return
	}
	if existingUser != nil {
		WriteError(w, http.StatusConflict, "email_exists", "User with this email already exists")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Internal server error")
		return
	}

	user := &models.User{
		Email:      req.Email,
		Phone:      req.Phone,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		MiddleName: req.MiddleName,
		Status:     models.UserStatusActive,
		Role:       models.UserRoleUser,
	}

	if err := h.storage.CreateUser(r.Context(), user, string(hashedPassword)); err != nil {
		h.logger.Error("failed to create user", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to create user")
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role, h.sessionDuration)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to generate token")
		return
	}

	session := &models.Session{
		ID:         uuid.New(),
		UserID:     user.ID,
		Token:      token,
		DeviceInfo: r.UserAgent(),
		IP:         r.RemoteAddr,
		ExpiresAt:  time.Now().Add(h.sessionDuration),
	}
	if err := h.storage.CreateSession(r.Context(), session); err != nil {
		h.logger.Error("failed to create session", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to create session")
		return
	}

	// Clean up old sessions for this user (keep only 5 most recent) - synchronous to avoid goroutine leak
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.storage.DeleteOldSessionsForUser(cleanupCtx, user.ID, 5); err != nil {
		h.logger.Error("failed to cleanup old sessions", "error", err)
	}

	WriteJSON(w, http.StatusCreated, AuthResponse{Token: token, User: user})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteError(w, http.StatusBadRequest, "missing_fields", "Email and password are required")
		return
	}

	user, passwordHash, err := h.storage.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("login: failed to get user by email", "email", req.Email, "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to authenticate user")
		return
	}
	if user == nil {
		h.logger.Warn("login: user not found", "email", req.Email)
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	h.logger.Info("login: user found", "email", req.Email, "user_id", user.ID, "hash_len", len(passwordHash))

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		h.logger.Warn("login: password mismatch", "email", req.Email, "error", err)
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	h.logger.Info("login: password verified", "email", req.Email)

	token, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role, h.sessionDuration)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to generate token")
		return
	}

	session := &models.Session{
		ID:         uuid.New(),
		UserID:     user.ID,
		Token:      token,
		DeviceInfo: r.UserAgent(),
		IP:         r.RemoteAddr,
		ExpiresAt:  time.Now().Add(h.sessionDuration),
	}
	if err := h.storage.CreateSession(r.Context(), session); err != nil {
		h.logger.Error("failed to create session", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to create session")
		return
	}

	// Clean up old sessions for this user (keep only 5 most recent) - synchronous to avoid goroutine leak
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.storage.DeleteOldSessionsForUser(cleanupCtx, user.ID, 5); err != nil {
		h.logger.Error("failed to cleanup old sessions", "error", err)
	}

	WriteJSON(w, http.StatusOK, AuthResponse{Token: token, User: user})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") && len(authHeader) > 7 {
		token := authHeader[7:]

		// Verify the session belongs to the current user before deleting
		session, err := h.storage.GetSessionByToken(r.Context(), token)
		if err != nil {
			h.logger.Error("failed to get session", "error", err)
			WriteError(w, http.StatusInternalServerError, "internal", "Failed to validate session")
			return
		}
		if session == nil {
			WriteError(w, http.StatusUnauthorized, "session_not_found", "Session not found")
			return
		}
		if session.UserID != claims.UserID {
			WriteError(w, http.StatusForbidden, "forbidden", "Cannot logout another user's session")
			return
		}

		if err := h.storage.DeleteSession(r.Context(), token); err != nil {
			h.logger.Error("failed to delete session", "error", err)
			// Don't return error to client, just log it
		}
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

func (h *Handler) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		WriteError(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

func AuthMiddleware(jwtService *auth.JWTService, sessionStorage storage.SessionStorage) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || len(authHeader) <= 7 {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid authorization header")
				return
			}

			token := authHeader[7:]
			claims, err := jwtService.ParseToken(token)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
				return
			}

			// Verify session exists and hasn't expired in database
			if sessionStorage != nil {
				session, err := sessionStorage.GetSessionByToken(r.Context(), token)
				if err != nil {
					WriteError(w, http.StatusInternalServerError, "internal", "Failed to validate session")
					return
				}
				if session == nil {
					WriteError(w, http.StatusUnauthorized, "session_expired", "Session has been revoked or expired")
					return
				}
			}

			ctx := auth.ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			WriteError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
			return
		}
		if claims.Role != models.UserRoleAdmin {
			WriteError(w, http.StatusForbidden, "forbidden", "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
