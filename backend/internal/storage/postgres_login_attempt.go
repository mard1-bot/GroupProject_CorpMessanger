package storage

import (
	"context"

	"time"

	"github.com/google/uuid"
)

// RecordLoginAttempt records a login attempt in the database
func (s *PostgresStorage) RecordLoginAttempt(ctx context.Context, email, ipAddress string, userID *uuid.UUID, success bool) error {
	query := `
		INSERT INTO login_attempts (email, ip_address, user_id, success)
		VALUES ($1, $2, $3, $4)
	`
	_, err := s.db.ExecContext(ctx, query, email, ipAddress, userID, success)
	return err
}

// GetFailedLoginAttempts counts failed login attempts for an email since a given time
func (s *PostgresStorage) GetFailedLoginAttempts(ctx context.Context, email string, since time.Time) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM login_attempts 
		WHERE email = $1 AND success = false AND created_at >= $2
	`
	var count int
	err := s.db.QueryRowContext(ctx, query, email, since).Scan(&count)
	return count, err
}

// CleanupOldLoginAttempts removes login attempts older than the specified time
func (s *PostgresStorage) CleanupOldLoginAttempts(ctx context.Context, olderThan time.Time) error {
	query := `
		DELETE FROM login_attempts WHERE created_at < $1
	`
	_, err := s.db.ExecContext(ctx, query, olderThan)
	return err
}
