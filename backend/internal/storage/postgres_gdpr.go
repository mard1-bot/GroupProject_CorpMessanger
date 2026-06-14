package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

// SaveUserConsent saves or updates user consent
func (s *PostgresStorage) SaveUserConsent(ctx context.Context, consent *models.UserConsent) error {
	query := `
		INSERT INTO user_consents (user_id, consent_type, consent_given, consent_text, consent_version, ip_address, user_agent, consented_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (user_id, consent_type) 
		DO UPDATE SET 
			consent_given = $3,
			consent_text = $4,
			consent_version = $5,
			ip_address = $6,
			user_agent = $7,
			consented_at = $8,
			revoked_at = $9,
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(ctx, query,
		consent.UserID, consent.ConsentType, consent.ConsentGiven,
		consent.ConsentText, consent.ConsentVersion, consent.IPAddress,
		consent.UserAgent, consent.ConsentedAt, consent.RevokedAt,
	)
	return err
}

// GetUserConsents retrieves all consents for a user
func (s *PostgresStorage) GetUserConsents(ctx context.Context, userID uuid.UUID) ([]*models.UserConsent, error) {
	query := `
		SELECT id, user_id, consent_type, consent_given, consent_text, consent_version, 
		       ip_address, user_agent, consented_at, revoked_at, created_at, updated_at
		FROM user_consents
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var consents []*models.UserConsent
	for rows.Next() {
		var c models.UserConsent
		err := rows.Scan(
			&c.ID, &c.UserID, &c.ConsentType, &c.ConsentGiven,
			&c.ConsentText, &c.ConsentVersion, &c.IPAddress, &c.UserAgent,
			&c.ConsentedAt, &c.RevokedAt, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		consents = append(consents, &c)
	}
	return consents, rows.Err()
}

// HasValidConsent checks if user has valid consent for a type
func (s *PostgresStorage) HasValidConsent(ctx context.Context, userID uuid.UUID, consentType string) (bool, error) {
	query := `SELECT has_valid_consent($1, $2)`
	var hasConsent bool
	err := s.db.QueryRowContext(ctx, query, userID, consentType).Scan(&hasConsent)
	return hasConsent, err
}

// CreateDataExportRequest creates a new data export/deletion request
func (s *PostgresStorage) CreateDataExportRequest(ctx context.Context, request *models.DataExportRequest) error {
	query := `
		INSERT INTO data_export_requests (user_id, request_type, status, requested_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	return s.db.QueryRowContext(ctx, query,
		request.UserID, request.RequestType, request.Status, request.RequestedAt,
	).Scan(&request.ID)
}

// GetDataExportRequests retrieves all export requests for a user
func (s *PostgresStorage) GetDataExportRequests(ctx context.Context, userID uuid.UUID) ([]*models.DataExportRequest, error) {
	query := `
		SELECT id, user_id, request_type, status, requested_at, completed_at, 
		       export_url, expires_at, error_message, created_at, updated_at
		FROM data_export_requests
		WHERE user_id = $1
		ORDER BY requested_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []*models.DataExportRequest
	for rows.Next() {
		var r models.DataExportRequest
		err := rows.Scan(
			&r.ID, &r.UserID, &r.RequestType, &r.Status, &r.RequestedAt,
			&r.CompletedAt, &r.ExportURL, &r.ExpiresAt, &r.ErrorMsg,
			&r.CreatedAt, &r.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		requests = append(requests, &r)
	}
	return requests, rows.Err()
}

// AnonymizeUserData anonymizes user data (GDPR Right to Erasure)
func (s *PostgresStorage) AnonymizeUserData(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error) {
	query := `SELECT anonymize_user_data($1)`
	var result []byte
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("anonymize user data: %w", err)
	}

	var recordsAffected map[string]interface{}
	if err := json.Unmarshal(result, &recordsAffected); err != nil {
		return nil, fmt.Errorf("parse anonymization result: %w", err)
	}

	return recordsAffected, nil
}
