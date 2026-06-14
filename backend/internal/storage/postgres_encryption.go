package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) CreateDeviceToken(ctx context.Context, token *models.DeviceToken) error {
	query := `
		INSERT INTO device_tokens (id, user_id, token, platform, device_name, created_at, last_used_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		ON CONFLICT (user_id, token) DO UPDATE SET last_used_at = EXCLUDED.last_used_at
		RETURNING id, created_at, last_used_at
	`
	return s.db.QueryRowContext(ctx, query,
		token.ID, token.UserID, token.Token, token.Platform, token.DeviceName, token.CreatedAt,
	).Scan(&token.ID, &token.CreatedAt, &token.LastUsedAt)
}

func (s *PostgresStorage) GetDeviceTokens(ctx context.Context, userID uuid.UUID) ([]*models.DeviceToken, error) {
	query := `
		SELECT id, user_id, token, platform, device_name, created_at, last_used_at
		FROM device_tokens
		WHERE user_id = $1
		ORDER BY last_used_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*models.DeviceToken
	for rows.Next() {
		t := &models.DeviceToken{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.DeviceName, &t.CreatedAt, &t.LastUsedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (s *PostgresStorage) DeleteDeviceToken(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM device_tokens WHERE token = $1`, token)
	return err
}

func (s *PostgresStorage) GetUserPublicKey(ctx context.Context, userID uuid.UUID) (*models.EncryptionKey, error) {
	query := `
		SELECT user_id, public_key, private_key, key_version, created_at, updated_at
		FROM encryption_keys
		WHERE user_id = $1
	`
	key := &models.EncryptionKey{}
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&key.UserID, &key.PublicKey, &key.PrivateKey, &key.KeyVersion, &key.CreatedAt, &key.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return key, nil
}

func (s *PostgresStorage) GetUsersPublicKeys(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*models.EncryptionKey, error) {
	if len(userIDs) == 0 {
		return make(map[uuid.UUID]*models.EncryptionKey), nil
	}

	// Build placeholders for IN clause
	placeholders := make([]string, len(userIDs))
	args := make([]interface{}, len(userIDs))
	for i, id := range userIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT user_id, public_key, private_key, key_version, created_at, updated_at
		FROM encryption_keys
		WHERE user_id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make(map[uuid.UUID]*models.EncryptionKey)
	for rows.Next() {
		key := &models.EncryptionKey{}
		err := rows.Scan(
			&key.UserID, &key.PublicKey, &key.PrivateKey, &key.KeyVersion, &key.CreatedAt, &key.UpdatedAt,
		)
		if err != nil {
			continue
		}
		keys[key.UserID] = key
	}
	return keys, rows.Err()
}

func (s *PostgresStorage) SaveUserPublicKey(ctx context.Context, key *models.EncryptionKey) error {
	query := `
		INSERT INTO encryption_keys (user_id, public_key, private_key, key_version)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			public_key = EXCLUDED.public_key,
			private_key = EXCLUDED.private_key,
			key_version = EXCLUDED.key_version,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		key.UserID, key.PublicKey, key.PrivateKey, key.KeyVersion,
	).Scan(&key.CreatedAt, &key.UpdatedAt)
}

// RotateEncryptionKey rotates a user's encryption key by archiving the old key and creating a new one
func (s *PostgresStorage) RotateEncryptionKey(ctx context.Context, userID uuid.UUID, newPublicKey, newPrivateKey, reason string) (int, error) {
	query := `SELECT new_version, archived_id FROM rotate_encryption_key($1, $2, $3, $4)`
	var newVersion int
	var archivedID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, userID, newPublicKey, newPrivateKey, reason).Scan(&newVersion, &archivedID)
	if err != nil {
		return 0, fmt.Errorf("rotate encryption key: %w", err)
	}
	return newVersion, nil
}

// GetEncryptionKeyByVersion retrieves an encryption key by version (current or archived)
func (s *PostgresStorage) GetEncryptionKeyByVersion(ctx context.Context, userID uuid.UUID, version int) (*models.EncryptionKey, error) {
	query := `SELECT public_key, private_key, key_version FROM get_encryption_key_by_version($1, $2)`
	var key models.EncryptionKey
	var privateKey sql.NullString

	err := s.db.QueryRowContext(ctx, query, userID, version).Scan(&key.PublicKey, &privateKey, &key.KeyVersion)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get encryption key by version: %w", err)
	}

	key.UserID = userID
	if privateKey.Valid {
		key.PrivateKey = privateKey.String
	}

	return &key, nil
}
