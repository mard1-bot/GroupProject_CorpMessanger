package http

import (
	"context"
	"encoding/json"
	"fmt"
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

// validatePassword checks password strength requirements
func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	// Check for common passwords (top 100 most common)
	commonPasswords := map[string]bool{
		"password": true, "123456": true, "12345678": true, "qwerty": true,
		"abc123": true, "password123": true, "admin": true, "welcome": true,
		"monkey": true, "letmein": true, "dragon": true, "master": true,
		"hello": true, "login": true, "football": true, "iloveyou": true,
		"princess": true, "starwars": true, "123123": true, "password1": true,
		"123qwe": true, "qwerty123": true, "1q2w3e4r": true, "baseball": true,
		"superman": true, "whatever": true, "trustno1": true, "michael": true,
	}
	lowerPassword := strings.ToLower(password)
	if commonPasswords[lowerPassword] {
		return fmt.Errorf("password is too common, please choose a stronger password")
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, ch := range password {
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
		return fmt.Errorf("password must contain at least one uppercase letter, one lowercase letter, and one digit")
	}

	return nil
}

type RegisterRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username,omitempty"`
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
	// Validate JSON body with size limit
	var req RegisterRequest
	if err := ValidateJSONBody(r, &req, 64*1024); err != nil {
		if validationErr, ok := err.(*ValidationError); ok {
			WriteError(w, ErrInvalidInput, validationErr.Message)
		} else {
			WriteError(w, ErrInvalidInput, "Invalid request body")
		}
		return
	}
	defer r.Body.Close()

	// Validate required fields
	if req.Email == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_email", "Email is required")
		return
	}
	if !ValidateEmail(req.Email) {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_email", "Invalid email format")
		return
	}
	if req.Password == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_password", "Password is required")
		return
	}
	if err := ValidatePassword(req.Password); err != nil {
		if validationErr, ok := err.(*ValidationError); ok {
			WriteError(w, ErrInvalidInput, validationErr.Message)
		} else {
			WriteError(w, ErrInvalidInput, "Password does not meet requirements")
		}
		return
	}

	existingUser, _, err := h.storage.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("failed to check existing user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to check user existence")
		return
	}
	if existingUser != nil {
		WriteErrorCode(w, http.StatusConflict, "email_exists", "User with this email already exists")
		return
	}

	// Validate and check username uniqueness
	if req.Username != "" {
		existingUsername, err := h.storage.GetUserByUsername(r.Context(), req.Username)
		if err != nil {
			h.logger.Error("failed to check existing username", "error", err)
			WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to check username existence")
			return
		}
		if existingUsername != nil {
			WriteErrorCode(w, http.StatusConflict, "username_exists", "Username is already taken")
			return
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Internal server error")
		return
	}

	user := &models.User{
		Email:     req.Email,
		Phone:     req.Phone,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Status:    models.UserStatusActive,
		Role:      models.UserRoleUser,
	}
	// Only set MiddleName if it's not empty
	if req.MiddleName != "" {
		user.MiddleName = &req.MiddleName
	}
	// Set username if provided
	if req.Username != "" {
		user.Username = &req.Username
	}

	if err := h.storage.CreateUser(r.Context(), user, string(hashedPassword)); err != nil {
		h.logger.Error("failed to create user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to create user")
		return
	}

	// Create XMPP user
	if err := h.ejabberd.CreateUser(user.ID, req.Password); err != nil {
		h.logger.Error("failed to create XMPP user", "error", err)
		// Continue anyway - XMPP is optional
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role, h.sessionDuration)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to generate token")
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
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to create session")
		return
	}

	// Clean up old sessions for this user (keep only 5 most recent) - synchronous to avoid goroutine leak
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := h.storage.DeleteOldSessionsForUser(cleanupCtx, user.ID, 5); err != nil {
		h.logger.Error("failed to cleanup old sessions", "error", err)
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     user.ID,
		Action:     "user_registered",
		Resource:   "user",
		ResourceID: user.ID.String(),
		IPAddress:  getClientIP(r),
		UserAgent:  r.Header.Get("User-Agent"),
	})

	WriteJSON(w, http.StatusCreated, AuthResponse{Token: token, User: user})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_fields", "Email and password are required")
		return
	}

	user, passwordHash, err := h.storage.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		h.logger.Error("login: failed to get user by email", "email", req.Email, "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to authenticate user")
		return
	}
	if user == nil {
		h.logger.Warn("login: user not found", "email", req.Email)
		WriteErrorCode(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	h.logger.Info("login: user found", "email", req.Email, "user_id", user.ID, "hash_len", len(passwordHash))

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		h.logger.Warn("login: password mismatch", "email", req.Email, "error", err)

		// Track failed login attempt for account lockout
		if err := h.trackFailedLoginAttempt(r.Context(), req.Email, getClientIP(r)); err != nil {
			h.logger.Error("failed to record login attempt", "error", err)
		}

		// Check if account should be locked
		locked, err := h.isAccountLocked(r.Context(), req.Email)
		if err != nil {
			h.logger.Error("failed to check account lock status", "error", err)
		}
		if locked {
			h.logger.Warn("login: account locked due to too many failed attempts", "email", req.Email)
			WriteErrorCode(w, http.StatusTooManyRequests, "account_locked", "Account temporarily locked due to too many failed login attempts. Please try again later.")
			return
		}

		WriteErrorCode(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	h.logger.Info("login: password verified", "email", req.Email)

	// Record successful login attempt
	if err := h.storage.RecordLoginAttempt(r.Context(), req.Email, getClientIP(r), &user.ID, true); err != nil {
		h.logger.Error("failed to record successful login attempt", "error", err)
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role, h.sessionDuration)
	if err != nil {
		h.logger.Error("failed to generate token", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to generate token")
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
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to create session")
		return
	}

	// Clean up old sessions for this user (keep only 5 most recent) - synchronous to avoid goroutine leak
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := h.storage.DeleteOldSessionsForUser(cleanupCtx, user.ID, 5); err != nil {
		h.logger.Error("failed to cleanup old sessions", "error", err)
	}

	// Audit log
	h.storage.CreateAuditLog(r.Context(), &models.AuditLog{
		UserID:     user.ID,
		Action:     "user_logged_in",
		Resource:   "auth",
		ResourceID: session.ID.String(),
		IPAddress:  getClientIP(r),
		UserAgent:  r.Header.Get("User-Agent"),
	})

	WriteJSON(w, http.StatusOK, AuthResponse{Token: token, User: user})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") && len(authHeader) > 7 {
		token := authHeader[7:]

		// Verify the session belongs to the current user before deleting
		session, err := h.storage.GetSessionByToken(r.Context(), token)
		if err != nil {
			h.logger.Error("failed to get session", "error", err)
			WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to validate session")
			return
		}
		if session == nil {
			WriteErrorCode(w, http.StatusUnauthorized, "session_not_found", "Session not found")
			return
		}
		if session.UserID != claims.UserID {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Cannot logout another user's session")
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
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		WriteErrorCode(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.OldPassword == "" || req.NewPassword == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_fields", "Old password and new password are required")
		return
	}

	// Validate new password strength
	if err := validatePassword(req.NewPassword); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "password_too_weak", err.Error())
		return
	}

	// Get current user with password hash using UserID instead of Email
	// This handles the case where user changes email after JWT was issued
	user, passwordHash, err := h.storage.GetUserByIDWithPassword(r.Context(), claims.UserID)
	if err != nil || user == nil {
		WriteErrorCode(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.OldPassword)); err != nil {
		WriteErrorCode(w, http.StatusUnauthorized, "invalid_password", "Invalid old password")
		return
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Internal server error")
		return
	}

	// Update password in database
	if err := h.storage.UpdateUserPassword(r.Context(), user.ID, string(hashedPassword)); err != nil {
		h.logger.Error("failed to update password", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to update password")
		return
	}

	// Update password in XMPP
	if err := h.ejabberd.UpdateUserPassword(user.ID, req.NewPassword); err != nil {
		h.logger.Error("failed to update XMPP password", "error", err)
		// Continue anyway - XMPP is optional
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *Handler) deleteUser(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Limit request body to 64KB to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	// Verify old password before deletion
	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	if req.Password == "" {
		WriteErrorCode(w, http.StatusBadRequest, "missing_password", "Password is required for deletion")
		return
	}

	// Get current user with password hash using UserID (consistent with changePassword)
	user, passwordHash, err := h.storage.GetUserByIDWithPassword(r.Context(), claims.UserID)
	if err != nil || user == nil {
		WriteErrorCode(w, http.StatusNotFound, "user_not_found", "User not found")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		WriteErrorCode(w, http.StatusUnauthorized, "invalid_password", "Invalid password")
		return
	}

	// Delete user from database first (source of truth)
	deleteErr := h.storage.DeleteUser(r.Context(), user.ID)
	if deleteErr != nil {
		h.logger.Error("failed to delete user", "error", deleteErr)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to delete user")
		return
	}

	// Delete XMPP user (optional, best effort)
	if err := h.ejabberd.DeleteUser(user.ID); err != nil {
		h.logger.Error("failed to delete XMPP user", "error", err)
		// Continue anyway - XMPP is optional and user is already deleted from database
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func AuthMiddleware(jwtService *auth.JWTService, sessionStorage storage.SessionStorage) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || len(authHeader) <= 7 {
				WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid authorization header")
				return
			}

			token := authHeader[7:]
			claims, err := jwtService.ParseToken(token)
			if err != nil {
				WriteErrorCode(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
				return
			}

			// Verify session exists and hasn't expired in database
			if sessionStorage != nil {
				session, err := sessionStorage.GetSessionByToken(r.Context(), token)
				if err != nil {
					WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to validate session")
					return
				}
				if session == nil {
					WriteErrorCode(w, http.StatusUnauthorized, "session_expired", "Session has been revoked or expired")
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
			WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
			return
		}
		if claims.Role != models.UserRoleAdmin {
			WriteErrorCode(w, http.StatusForbidden, "forbidden", "Admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
