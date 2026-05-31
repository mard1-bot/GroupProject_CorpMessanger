package http

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"

	"golang.org/x/crypto/bcrypt"
)

// TwoFactorSetup represents 2FA setup response
type TwoFactorSetup struct {
	Secret      string   `json:"secret"`
	QRCode      string   `json:"qr_code"`
	BackupCodes []string `json:"backup_codes"`
}

// TwoFactorVerifyRequest represents 2FA verification request
type TwoFactorVerifyRequest struct {
	Code string `json:"code"`
}

// TwoFactorEnableRequest represents 2FA enable request
type TwoFactorEnableRequest struct {
	Code string `json:"code"`
}

// TwoFactorDisableRequest represents 2FA disable request
type TwoFactorDisableRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

// generateSecret generates a new TOTP secret
func (h *Handler) generateSecret() (string, error) {
	secret := make([]byte, 20)
	_, err := rand.Read(secret)
	if err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// generateBackupCodes generates backup codes for 2FA
func (h *Handler) generateBackupCodes() ([]string, error) {
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code := make([]byte, 4)
		_, err := rand.Read(code)
		if err != nil {
			return nil, err
		}
		codes[i] = fmt.Sprintf("%02x%02x", code[0], code[1])
	}
	return codes, nil
}

// hashBackupCodes hashes backup codes for storage
func (h *Handler) hashBackupCodes(codes []string) ([]string, error) {
	hashedCodes := make([]string, len(codes))
	for i, code := range codes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hashedCodes[i] = string(hash)
	}
	return hashedCodes, nil
}

// verifyBackupCode verifies a backup code
func (h *Handler) verifyBackupCode(hashedCodes []string, providedCode string) bool {
	for _, hashedCode := range hashedCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(providedCode)) == nil {
			return true
		}
	}
	return false
}

// generateQRCode generates QR code for TOTP setup
func (h *Handler) generateQRCode(secret, email string) (string, error) {
	// Create otpauth URL for QR code
	otpauthURL := fmt.Sprintf("otpauth://totp/CorpMessenger:%s?secret=%s&issuer=CorpMessenger", email, secret)

	// Return the otpauth URL as base64 for now
	// In production, integrate a proper QR code library
	return base64.StdEncoding.EncodeToString([]byte(otpauthURL)), nil
}

// setupTwoFactor handles 2FA setup request
func (h *Handler) setupTwoFactor(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, "Unauthorized")
		return
	}

	// Check if 2FA is already enabled
	existing, err := h.storage.GetTwoFactorSettings(r.Context(), claims.UserID)
	if err == nil && existing != nil && existing.Enabled {
		WriteError(w, ErrConflict, "2FA is already enabled")
		return
	}

	// Generate secret and backup codes
	secret, err := h.generateSecret()
	if err != nil {
		h.logger.Error("Failed to generate 2FA secret", "error", err)
		WriteError(w, ErrInternal, "Failed to generate secret")
		return
	}

	backupCodes, err := h.generateBackupCodes()
	if err != nil {
		h.logger.Error("Failed to generate backup codes", "error", err)
		WriteError(w, ErrInternal, "Failed to generate backup codes")
		return
	}

	// Generate QR code
	qrCode, err := h.generateQRCode(secret, claims.Email)
	if err != nil {
		h.logger.Error("Failed to generate QR code", "error", err)
		WriteError(w, ErrInternal, "Failed to generate QR code")
		return
	}

	// Store settings temporarily (not enabled yet)
	settings := &models.TwoFactorSettings{
		UserID:      claims.UserID,
		Secret:      secret,
		BackupCodes: backupCodes, // Store plain codes for now, will hash when enabled
		Enabled:     false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = h.storage.CreateTwoFactorSettings(r.Context(), settings)
	if err != nil {
		h.logger.Error("Failed to create 2FA settings", "error", err)
		WriteError(w, ErrInternal, "Failed to save settings")
		return
	}

	response := TwoFactorSetup{
		Secret:      secret,
		QRCode:      qrCode,
		BackupCodes: backupCodes,
	}

	WriteJSON(w, http.StatusOK, response)
}

// enableTwoFactor handles 2FA enable request
func (h *Handler) enableTwoFactor(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, "Unauthorized")
		return
	}

	var req TwoFactorEnableRequest
	if err := ValidateJSONBody(r, &req, 1024); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request format")
		return
	}

	// Get existing settings
	settings, err := h.storage.GetTwoFactorSettings(r.Context(), claims.UserID)
	if err != nil {
		WriteError(w, ErrNotFound, "2FA settings not found")
		return
	}

	if settings.Enabled {
		WriteError(w, ErrConflict, "2FA is already enabled")
		return
	}

	// Verify TOTP code
	if !ValidateTOTP(settings.Secret, req.Code) {
		WriteError(w, ErrInvalid2FACode, "Invalid verification code")
		return
	}

	// Hash backup codes
	hashedCodes, err := h.hashBackupCodes(settings.BackupCodes)
	if err != nil {
		h.logger.Error("Failed to hash backup codes", "error", err)
		WriteError(w, ErrInternal, "Failed to process backup codes")
		return
	}

	// Enable 2FA
	settings.Enabled = true
	settings.BackupCodes = hashedCodes
	settings.UpdatedAt = time.Now()

	err = h.storage.UpdateTwoFactorSettings(r.Context(), settings)
	if err != nil {
		h.logger.Error("Failed to enable 2FA", "error", err)
		WriteError(w, ErrInternal, "Failed to enable 2FA")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"message": "2FA enabled successfully",
	})
}

// disableTwoFactor handles 2FA disable request
func (h *Handler) disableTwoFactor(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, "Unauthorized")
		return
	}

	var req TwoFactorDisableRequest
	if err := ValidateJSONBody(r, &req, 1024); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request format")
		return
	}

	// Verify password
	err := h.storage.VerifyUserPassword(r.Context(), claims.UserID, req.Password)
	if err != nil {
		WriteError(w, ErrInvalidPassword, "Invalid password")
		return
	}

	// Get settings
	settings, err := h.storage.GetTwoFactorSettings(r.Context(), claims.UserID)
	if err != nil {
		WriteError(w, ErrNotFound, "2FA settings not found")
		return
	}

	if !settings.Enabled {
		WriteError(w, ErrConflict, "2FA is not enabled")
		return
	}

	// Verify current TOTP code if provided
	if req.Code != "" {
		if !ValidateTOTP(settings.Secret, req.Code) {
			WriteError(w, ErrInvalid2FACode, "Invalid verification code")
			return
		}
	}

	// Disable 2FA
	settings.Enabled = false
	settings.UpdatedAt = time.Now()

	err = h.storage.UpdateTwoFactorSettings(r.Context(), settings)
	if err != nil {
		h.logger.Error("Failed to disable 2FA", "error", err)
		WriteError(w, ErrInternal, "Failed to disable 2FA")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": false,
		"message": "2FA disabled successfully",
	})
}

// verifyTwoFactor handles 2FA verification during login
func (h *Handler) verifyTwoFactor(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, "Unauthorized")
		return
	}

	var req TwoFactorVerifyRequest
	if err := ValidateJSONBody(r, &req, 1024); err != nil {
		WriteError(w, ErrInvalidInput, "Invalid request format")
		return
	}

	// Get settings
	settings, err := h.storage.GetTwoFactorSettings(r.Context(), claims.UserID)
	if err != nil {
		WriteError(w, ErrNotFound, "2FA settings not found")
		return
	}

	if !settings.Enabled {
		WriteError(w, ErrConflict, "2FA is not enabled")
		return
	}

	// Try TOTP code first
	if ValidateTOTP(settings.Secret, req.Code) {
		// Generate new token
		token, err := h.jwt.GenerateToken(claims.UserID, claims.Email, claims.Role, 24*time.Hour)
		if err != nil {
			h.logger.Error("Failed to generate token", "error", err)
			WriteError(w, ErrInternal, "Failed to generate token")
			return
		}

		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"token":    token,
			"verified": true,
		})
		return
	}

	// Try backup codes
	if h.verifyBackupCode(settings.BackupCodes, req.Code) {
		// Generate new token
		token, err := h.jwt.GenerateToken(claims.UserID, claims.Email, claims.Role, 24*time.Hour)
		if err != nil {
			h.logger.Error("Failed to generate token", "error", err)
			WriteError(w, ErrInternal, "Failed to generate token")
			return
		}

		// Remove used backup code
		h.logger.Info("Backup code used for 2FA", "user_id", claims.UserID)

		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"token":            token,
			"verified":         true,
			"backup_code_used": true,
		})
		return
	}

	WriteError(w, ErrInvalid2FACode, "Invalid verification code")
}

// getTwoFactorStatus handles getting 2FA status
func (h *Handler) getTwoFactorStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, "Unauthorized")
		return
	}

	settings, err := h.storage.GetTwoFactorSettings(r.Context(), claims.UserID)
	if err != nil {
		// No settings found means 2FA is not set up
		WriteJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"setup":   false,
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"enabled":    settings.Enabled,
		"setup":      true,
		"created_at": settings.CreatedAt,
	})
}

// regenerateBackupCodes handles regenerating backup codes
func (h *Handler) regenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteError(w, ErrUnauthorized, "Unauthorized")
		return
	}

	settings, err := h.storage.GetTwoFactorSettings(r.Context(), claims.UserID)
	if err != nil || !settings.Enabled {
		WriteError(w, ErrNotFound, "2FA not enabled")
		return
	}

	// Generate new backup codes
	backupCodes, err := h.generateBackupCodes()
	if err != nil {
		h.logger.Error("Failed to generate backup codes", "error", err)
		WriteError(w, ErrInternal, "Failed to generate backup codes")
		return
	}

	// Hash backup codes
	hashedCodes, err := h.hashBackupCodes(backupCodes)
	if err != nil {
		h.logger.Error("Failed to hash backup codes", "error", err)
		WriteError(w, ErrInternal, "Failed to process backup codes")
		return
	}

	// Update settings
	settings.BackupCodes = hashedCodes
	settings.UpdatedAt = time.Now()

	err = h.storage.UpdateTwoFactorSettings(r.Context(), settings)
	if err != nil {
		h.logger.Error("Failed to update backup codes", "error", err)
		WriteError(w, ErrInternal, "Failed to update backup codes")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"backup_codes": backupCodes,
		"message":      "Backup codes regenerated successfully",
	})
}
