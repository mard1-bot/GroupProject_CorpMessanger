package storage

import (
	"context"
	"fmt"
	"log"
	"os"

	"time"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"
)

func (s *PostgresStorage) migrate() error {
	// Create base schema first
	if err := s.createBaseSchema(); err != nil {
		return fmt.Errorf("create base schema: %w", err)
	}

	// Run migration files
	if err := s.runMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

func (s *PostgresStorage) createBaseSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone VARCHAR(50),
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    middle_name VARCHAR(100),
    avatar VARCHAR(500),
    status VARCHAR(20) DEFAULT 'active',
    custom_status VARCHAR(200),
    role VARCHAR(20) DEFAULT 'user',
    last_online TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

CREATE TABLE IF NOT EXISTS user_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(255),
    description TEXT,
    avatar VARCHAR(500),
    creator_id UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) DEFAULT 'member',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_read_at TIMESTAMP WITH TIME ZONE,
    muted BOOLEAN DEFAULT false,
    pinned BOOLEAN DEFAULT false,
    archived BOOLEAN DEFAULT false,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(chat_id, user_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    sender_id UUID REFERENCES users(id) ON DELETE SET NULL,
    type VARCHAR(20) DEFAULT 'text',
    content TEXT NOT NULL,
    file_url TEXT,
    reply_to UUID REFERENCES messages(id) ON DELETE SET NULL,
    reply_to_content TEXT,
    reply_to_sender_name VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    read_by UUID[],
    pinned BOOLEAN DEFAULT false,
    duration FLOAT,
    scheduled_at TIMESTAMP WITH TIME ZONE,
    thread_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    forwarded_from UUID REFERENCES users(id) ON DELETE SET NULL,
    forwarded_sender_name VARCHAR(255),
    encrypted_content TEXT,
    encryption_key_id UUID,
    encrypted_keys JSONB,
    location JSONB,
    synced_to_xmpp BOOLEAN DEFAULT false,
    xmpp_message_id VARCHAR(255)
);

-- Create notification_settings table if it doesn't exist
CREATE TABLE IF NOT EXISTS notification_settings (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    push_enabled BOOLEAN DEFAULT true,
    email_enabled BOOLEAN DEFAULT true,
    email VARCHAR(255),
    quiet_hours_start TIME,
    quiet_hours_end TIME,
    quiet_hours_enabled BOOLEAN DEFAULT false,
    protocol_preference VARCHAR(20) DEFAULT 'websocket',
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    url TEXT NOT NULL,
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_chat_members_chat_id ON chat_members(chat_id);
CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id);
CREATE INDEX IF NOT EXISTS idx_files_message_id ON files(message_id);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(500) NOT NULL UNIQUE,
    device_info TEXT,
    ip VARCHAR(45),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *PostgresStorage) runMigrations() error {
	// Create migrations table if it doesn't exist
	migrationTableSQL := `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);`
	if _, err := s.db.Exec(migrationTableSQL); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Get list of migration files - only include tables not in base schema
	migrationFiles := []string{
		"003_message_reads.sql",
		"006_audit_logs.sql",
		"010_mentions.sql",
		"011_encryption_keys.sql",
		"013_bookmarks_blocked_deleted.sql",
		"015_reactions.sql",
		"021_web_push_subscriptions.sql",
		"024_xmpp_reconciliation.sql",
		"025_xmpp_sync_dead_letter.sql",
		"026_scheduler_failures.sql",
		"027_login_attempts.sql",
		"029_two_factor_auth.sql",
		"031_key_rotation.sql",
		"031_move_password_to_credentials.sql",
		"032_gdpr_consent.sql",
		"033_fix_schema_gaps.sql",
		"034_ensure_all_columns.sql",
		"035_add_username.sql",
	}

	for _, filename := range migrationFiles {
		// Check if migration is already applied
		var applied bool
		err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", filename).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}

		if applied {
			continue // Migration already applied
		}

		// Read and execute migration file
		migrationPath := fmt.Sprintf("migrations/%s", filename)
		migrationSQL, err := os.ReadFile(migrationPath)
		if err != nil {
			log.Printf("Warning: Could not read migration file %s: %v", filename, err)
			continue
		}

		// Execute migration
		if _, err := s.db.Exec(string(migrationSQL)); err != nil {
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		// Mark migration as applied
		if _, err := s.db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		log.Printf("Applied migration: %s", filename)
	}

	return nil
}

func (s *PostgresStorage) Ready(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) ResolveReconciliation(ctx context.Context, chatID uuid.UUID, reason string) error {
	query := `
		UPDATE xmpp_reconciliation
		SET resolved_at = NOW()
		WHERE chat_id = $1 AND reason = $2 AND resolved_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, chatID, reason)
	return err
}

func (s *PostgresStorage) GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	query := `
		SELECT COALESCE(SUM(count), 0)
		FROM unread_counts
		WHERE user_id = $1
	`
	var count int
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&count)
	return count, err
}

// EncryptionKey methods for E2E encryption

func (s *PostgresStorage) CreateWebPushSubscription(ctx context.Context, sub *models.WebPushSubscription) error {
	query := `
		INSERT INTO web_push_subscriptions (id, user_id, endpoint, key, auth, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, endpoint) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, sub.ID, sub.UserID, sub.Endpoint, sub.Key, sub.Auth, sub.CreatedAt)
	return err
}

func (s *PostgresStorage) GetWebPushSubscriptions(ctx context.Context, userID uuid.UUID) ([]*models.WebPushSubscription, error) {
	query := `
		SELECT id, user_id, endpoint, key, auth, created_at
		FROM web_push_subscriptions
		WHERE user_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []*models.WebPushSubscription
	for rows.Next() {
		var sub models.WebPushSubscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.Key, &sub.Auth, &sub.CreatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, &sub)
	}
	return subs, rows.Err()
}

func (s *PostgresStorage) DeleteWebPushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	query := `DELETE FROM web_push_subscriptions WHERE user_id = $1 AND endpoint = $2`
	_, err := s.db.ExecContext(ctx, query, userID, endpoint)
	return err
}

func (s *PostgresStorage) AddToXMPPSyncDeadLetter(ctx context.Context, messageID, chatID uuid.UUID, errorMessage string) error {
	query := `
		INSERT INTO xmpp_sync_dead_letter (message_id, chat_id, error_message, error_details)
		VALUES ($1, $2, $3, '{"error": $3}')
		ON CONFLICT (message_id) 
		DO UPDATE SET 
			error_message = EXCLUDED.error_message,
			attempts = EXCLUDED.attempts + 1,
			last_attempt_at = NOW()
		WHERE xmpp_sync_dead_letter.resolved_at IS NULL AND xmpp_sync_dead_letter.attempts < xmpp_sync_dead_letter.max_attempts
	`
	_, err := s.db.ExecContext(ctx, query, messageID, chatID, errorMessage)
	return err
}

func (s *PostgresStorage) GetXMPPSyncDeadLetters(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, message_id, chat_id, error_message, error_details, attempts, last_attempt_at, created_at
		FROM xmpp_sync_dead_letter
		WHERE resolved_at IS NULL AND attempts < max_attempts
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, messageID, chatID uuid.UUID
		var errorMessage string
		var errorDetails map[string]interface{}
		var attempts int
		var lastAttemptAt, createdAt time.Time

		if err := rows.Scan(&id, &messageID, &chatID, &errorMessage, &errorDetails, &attempts, &lastAttemptAt, &createdAt); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"id":            id,
			"message_id":    messageID,
			"chat_id":       chatID,
			"error_message": errorMessage,
			"error_details": errorDetails,
			"attempts":      attempts,
			"last_attempt":  lastAttemptAt,
			"created_at":    createdAt,
		})
	}
	return results, rows.Err()
}

func (s *PostgresStorage) ResolveXMPPSyncDeadLetter(ctx context.Context, messageID uuid.UUID) error {
	query := `
		UPDATE xmpp_sync_dead_letter
		SET resolved_at = NOW()
		WHERE message_id = $1 AND resolved_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, messageID)
	return err
}

func (s *PostgresStorage) AddToSchedulerFailures(ctx context.Context, messageID, chatID uuid.UUID, errorMessage string) error {
	query := `
		INSERT INTO scheduler_failures (message_id, chat_id, error_message, error_details, next_retry_at)
		VALUES ($1, $2, $3, '{"error": $3}', NOW() + INTERVAL '1 minute' * POWER(2, LEAST(attempts, 4)))
		ON CONFLICT (message_id) 
		DO UPDATE SET 
			error_message = EXCLUDED.error_message,
			attempts = EXCLUDED.attempts + 1,
			next_retry_at = NOW() + INTERVAL '1 minute' * POWER(2, LEAST(EXCLUDED.attempts + 1, 4)),
			last_attempt_at = NOW()
		WHERE scheduler_failures.resolved_at IS NULL AND scheduler_failures.attempts < scheduler_failures.max_attempts
	`
	_, err := s.db.ExecContext(ctx, query, messageID, chatID, errorMessage)
	return err
}

func (s *PostgresStorage) GetSchedulerFailures(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, message_id, chat_id, error_message, error_details, attempts, next_retry_at, last_attempt_at, created_at
		FROM scheduler_failures
		WHERE resolved_at IS NULL AND attempts < max_attempts AND next_retry_at <= NOW()
		ORDER BY next_retry_at ASC
		LIMIT $1
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, messageID, chatID uuid.UUID
		var errorMessage string
		var errorDetails map[string]interface{}
		var attempts int
		var nextRetryAt, lastAttemptAt, createdAt time.Time

		if err := rows.Scan(&id, &messageID, &chatID, &errorMessage, &errorDetails, &attempts, &nextRetryAt, &lastAttemptAt, &createdAt); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"id":            id,
			"message_id":    messageID,
			"chat_id":       chatID,
			"error_message": errorMessage,
			"error_details": errorDetails,
			"attempts":      attempts,
			"next_retry":    nextRetryAt,
			"last_attempt":  lastAttemptAt,
			"created_at":    createdAt,
		})
	}
	return results, rows.Err()
}

func (s *PostgresStorage) ResolveSchedulerFailure(ctx context.Context, messageID uuid.UUID) error {
	query := `
		UPDATE scheduler_failures
		SET resolved_at = NOW()
		WHERE message_id = $1 AND resolved_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, messageID)
	return err
}
