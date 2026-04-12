package http

import (
	"encoding/json"
	"net/http"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

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
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" || req.FirstName == "" || req.LastName == "" {
		WriteError(w, http.StatusBadRequest, "missing_fields", "Email, password, first_name and last_name are required")
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

	token, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role, 7*24*time.Hour)
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
		ExpiresAt:  time.Now().Add(7 * 24 * time.Hour),
	}
	_ = h.storage.CreateSession(r.Context(), session)

	WriteJSON(w, http.StatusCreated, AuthResponse{Token: token, User: user})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("failed to get user by email", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal", "Failed to authenticate user")
		return
	}
	if user == nil {
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
		return
	}

	token, err := h.jwt.GenerateToken(user.ID, user.Email, user.Role, 7*24*time.Hour)
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
		ExpiresAt:  time.Now().Add(7 * 24 * time.Hour),
	}
	_ = h.storage.CreateSession(r.Context(), session)

	WriteJSON(w, http.StatusOK, AuthResponse{Token: token, User: user})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token := authHeader[7:]
		_ = h.storage.DeleteSession(r.Context(), token)
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

func AuthMiddleware(jwtService *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || len(authHeader) < 8 || authHeader[:7] != "Bearer " {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid authorization header")
				return
			}

			token := authHeader[7:]
			claims, err := jwtService.ParseToken(token)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "invalid_token", "Invalid or expired token")
				return
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
