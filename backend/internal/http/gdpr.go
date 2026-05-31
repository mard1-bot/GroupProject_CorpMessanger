package http

import (
	"net/http"
	"time"

	"corp-messenger/backend/internal/auth"
	"corp-messenger/backend/internal/models"
)

// ConsentRequest represents a user consent submission
type ConsentRequest struct {
	ConsentType    string `json:"consent_type"` // 'data_processing', 'marketing', 'analytics'
	ConsentGiven   bool   `json:"consent_given"`
	ConsentVersion string `json:"consent_version"` // Version of consent text
}

// ConsentResponse represents consent status
type ConsentResponse struct {
	ConsentType    string    `json:"consent_type"`
	ConsentGiven   bool      `json:"consent_given"`
	ConsentVersion string    `json:"consent_version"`
	ConsentedAt    time.Time `json:"consented_at,omitempty"`
	RevokedAt      time.Time `json:"revoked_at,omitempty"`
}

// DataExportRequest represents a request to export user data
type DataExportRequest struct {
	RequestType string `json:"request_type"` // 'export' or 'delete'
}

// DataExportResponse represents the status of a data export request
type DataExportResponse struct {
	ID          string    `json:"id"`
	RequestType string    `json:"request_type"`
	Status      string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	ExportURL   string    `json:"export_url,omitempty"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

// giveConsent handles user consent submission
func (h *Handler) giveConsent(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req ConsentRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validate consent type
	validTypes := map[string]bool{
		"data_processing": true,
		"marketing":       true,
		"analytics":       true,
	}
	if !validTypes[req.ConsentType] {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_consent_type", "Invalid consent type")
		return
	}

	// Get client IP and user agent for audit trail
	clientIP := getClientIP(r)
	userAgent := r.UserAgent()

	consent := &models.UserConsent{
		UserID:         claims.UserID,
		ConsentType:    req.ConsentType,
		ConsentGiven:   req.ConsentGiven,
		ConsentVersion: req.ConsentVersion,
		IPAddress:      clientIP,
		UserAgent:      userAgent,
	}

	if req.ConsentGiven {
		consent.ConsentedAt = time.Now()
	} else {
		consent.RevokedAt = time.Now()
	}

	if err := h.storage.SaveUserConsent(r.Context(), consent); err != nil {
		h.logger.Error("failed to save consent", "error", err, "user_id", claims.UserID)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to save consent")
		return
	}

	// Log audit event
	h.logAudit(r.Context(), claims.UserID, "consent_updated", map[string]interface{}{
		"consent_type":  req.ConsentType,
		"consent_given": req.ConsentGiven,
		"ip_address":    clientIP,
	})

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Consent updated successfully",
		"consent_type": req.ConsentType,
		"given":        req.ConsentGiven,
	})
}

// getConsents retrieves all consents for the current user
func (h *Handler) getConsents(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	consents, err := h.storage.GetUserConsents(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get consents", "error", err, "user_id", claims.UserID)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get consents")
		return
	}

	response := make([]ConsentResponse, 0, len(consents))
	for _, c := range consents {
		response = append(response, ConsentResponse{
			ConsentType:    c.ConsentType,
			ConsentGiven:   c.ConsentGiven,
			ConsentVersion: c.ConsentVersion,
			ConsentedAt:    c.ConsentedAt,
			RevokedAt:      c.RevokedAt,
		})
	}

	WriteJSON(w, http.StatusOK, response)
}

// requestDataExport handles GDPR data export/deletion requests
func (h *Handler) requestDataExport(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	var req DataExportRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Validate request type
	if req.RequestType != "export" && req.RequestType != "delete" {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_request_type", "Request type must be 'export' or 'delete'")
		return
	}

	// For deletion requests, require additional confirmation
	if req.RequestType == "delete" {
		// Check if user has data_processing consent
		hasConsent, err := h.storage.HasValidConsent(r.Context(), claims.UserID, "data_processing")
		if err != nil {
			h.logger.Error("failed to check consent", "error", err)
		}

		if !hasConsent {
			WriteErrorCode(w, http.StatusBadRequest, "no_consent", "Cannot delete data without prior consent record")
			return
		}
	}

	exportReq := &models.DataExportRequest{
		UserID:      claims.UserID,
		RequestType: req.RequestType,
		Status:      "pending",
		RequestedAt: time.Now(),
	}

	if err := h.storage.CreateDataExportRequest(r.Context(), exportReq); err != nil {
		h.logger.Error("failed to create export request", "error", err, "user_id", claims.UserID)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to create export request")
		return
	}

	// Log audit event
	h.logAudit(r.Context(), claims.UserID, "data_export_requested", map[string]interface{}{
		"request_type": req.RequestType,
		"request_id":   exportReq.ID.String(),
	})

	// TODO: Trigger background job to process export/deletion
	// This should be done asynchronously via a job queue

	WriteJSON(w, http.StatusAccepted, DataExportResponse{
		ID:          exportReq.ID.String(),
		RequestType: exportReq.RequestType,
		Status:      exportReq.Status,
		RequestedAt: exportReq.RequestedAt,
	})
}

// getDataExportStatus retrieves the status of a data export request
func (h *Handler) getDataExportStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	requests, err := h.storage.GetDataExportRequests(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get export requests", "error", err, "user_id", claims.UserID)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to get export requests")
		return
	}

	response := make([]DataExportResponse, 0, len(requests))
	for _, req := range requests {
		resp := DataExportResponse{
			ID:          req.ID.String(),
			RequestType: req.RequestType,
			Status:      req.Status,
			RequestedAt: req.RequestedAt,
		}

		if !req.CompletedAt.IsZero() {
			resp.CompletedAt = req.CompletedAt
		}
		if req.ExportURL != "" {
			resp.ExportURL = req.ExportURL
		}
		if !req.ExpiresAt.IsZero() {
			resp.ExpiresAt = req.ExpiresAt
		}

		response = append(response, resp)
	}

	WriteJSON(w, http.StatusOK, response)
}

// deleteMyData handles immediate data deletion (GDPR Right to Erasure)
func (h *Handler) deleteMyData(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		WriteErrorCode(w, http.StatusUnauthorized, "unauthorized", "Unauthorized")
		return
	}

	// Verify password for security
	var req struct {
		Password string `json:"password"`
		Confirm  string `json:"confirm"` // User must type "DELETE" to confirm
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		WriteErrorCode(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}
	defer r.Body.Close()

	// Verify password
	if err := h.storage.VerifyUserPassword(r.Context(), claims.UserID, req.Password); err != nil {
		WriteErrorCode(w, http.StatusUnauthorized, "invalid_password", "Invalid password")
		return
	}

	// Require explicit confirmation
	if req.Confirm != "DELETE" {
		WriteErrorCode(w, http.StatusBadRequest, "confirmation_required", "Please type DELETE to confirm")
		return
	}

	// Get user info for logging before deletion
	user, err := h.storage.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to get user", "error", err)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to process request")
		return
	}

	// Anonymize user data (keeps messages for chat history but removes personal info)
	recordsAffected, err := h.storage.AnonymizeUserData(r.Context(), claims.UserID)
	if err != nil {
		h.logger.Error("failed to anonymize user data", "error", err, "user_id", claims.UserID)
		WriteErrorCode(w, http.StatusInternalServerError, "internal", "Failed to delete data")
		return
	}

	// Log deletion for compliance
	h.logAudit(r.Context(), claims.UserID, "data_deleted", map[string]interface{}{
		"email":            user.Email,
		"records_affected": recordsAffected,
		"deletion_type":    "anonymization",
	})

	// Invalidate all user sessions
	if err := h.storage.DeleteAllUserSessions(r.Context(), claims.UserID); err != nil {
		h.logger.Error("failed to delete sessions", "error", err)
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "Your data has been anonymized successfully",
		"records_affected": recordsAffected,
	})
}
