package storage

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

// CreateTwoFactorSettings creates 2FA settings for a user
func (s *PostgresStorage) CreateTwoFactorSettings(ctx context.Context, settings *models.TwoFactorSettings) error {
	query := `
		INSERT INTO two_factor_settings (user_id, secret, backup_codes, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	backupCodesJSON, _ := json.Marshal(settings.BackupCodes)
	_, err := s.db.ExecContext(ctx, query, settings.UserID, settings.Secret, backupCodesJSON, settings.Enabled, settings.CreatedAt, settings.UpdatedAt)
	return err
}

// GetTwoFactorSettings retrieves 2FA settings for a user
func (s *PostgresStorage) GetTwoFactorSettings(ctx context.Context, userID uuid.UUID) (*models.TwoFactorSettings, error) {
	query := `
		SELECT user_id, secret, backup_codes, enabled, created_at, updated_at
		FROM two_factor_settings
		WHERE user_id = $1
	`
	var settings models.TwoFactorSettings
	var backupCodesJSON []byte
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.UserID, &settings.Secret, &backupCodesJSON,
		&settings.Enabled, &settings.CreatedAt, &settings.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(backupCodesJSON) > 0 {
		json.Unmarshal(backupCodesJSON, &settings.BackupCodes)
	}
	return &settings, nil
}

// UpdateTwoFactorSettings updates 2FA settings for a user
func (s *PostgresStorage) UpdateTwoFactorSettings(ctx context.Context, settings *models.TwoFactorSettings) error {
	query := `
		UPDATE two_factor_settings
		SET secret = $1, backup_codes = $2, enabled = $3, updated_at = $4
		WHERE user_id = $5
	`
	backupCodesJSON, _ := json.Marshal(settings.BackupCodes)
	_, err := s.db.ExecContext(ctx, query, settings.Secret, backupCodesJSON, settings.Enabled, settings.UpdatedAt, settings.UserID)
	return err
}

// DeleteTwoFactorSettings deletes 2FA settings for a user
func (s *PostgresStorage) DeleteTwoFactorSettings(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM two_factor_settings WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, query, userID)
	return err
}
