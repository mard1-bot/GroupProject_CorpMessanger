package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"time"

	"github.com/google/uuid"

	"corp-messenger/backend/internal/models"

	_ "github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func NewPostgres(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	// Use context with timeout for Ping to avoid indefinite hang
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	storage := &PostgresStorage{db: db}
	if err := storage.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return storage, nil
}

func (s *PostgresStorage) migrate() error {
	// Note: This function creates only the base schema from 001_initial.sql
	// Additional schema changes should be applied via migration files
	// to ensure consistency across environments
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    status VARCHAR(20) DEFAULT 'active',
    role VARCHAR(20) DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(255),
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
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Note: forwarded_from and forwarded_sender_name columns are added via migration 023_forwarded_message_fields.sql
-- The DO block has been removed to prevent race conditions during concurrent migrations.
-- Ensure migrations are run before starting the application.

-- Create index for forwarded_from if it doesn't exist
CREATE INDEX IF NOT EXISTS idx_messages_forwarded_from ON messages(forwarded_from) WHERE forwarded_from IS NOT NULL;

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

-- Note: protocol_preference column addition moved to dedicated migration 022_protocol_preference.sql
-- to prevent race conditions during concurrent migrations.

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
`
	_, err := s.db.Exec(schema)
	return err
}

func (s *PostgresStorage) Ready(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) CreateUser(ctx context.Context, user *models.User, passwordHash string) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Handle rollback with error check
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("transaction error: %w; rollback failed: %v", err, rbErr)
			}
		}
	}()

	query := `
		INSERT INTO users (email, phone, first_name, last_name, middle_name, avatar, status, role)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`
	var middleName, avatar sql.NullString
	if user.MiddleName != nil {
		middleName.String = *user.MiddleName
		middleName.Valid = true
	}
	if user.Avatar != nil {
		avatar.String = *user.Avatar
		avatar.Valid = true
	}
	err = tx.QueryRowContext(ctx, query,
		user.Email, user.Phone, user.FirstName, user.LastName, middleName,
		avatar, user.Status, user.Role,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}

	credQuery := `INSERT INTO user_credentials (user_id, password_hash) VALUES ($1, $2)`
	_, err = tx.ExecContext(ctx, credQuery, user.ID, passwordHash)
	if err != nil {
		return fmt.Errorf("insert credentials: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *PostgresStorage) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE user_credentials SET password_hash = $1 WHERE user_id = $2`
	_, err := s.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *PostgresStorage) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Handle rollback with error check
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("transaction error: %w; rollback failed: %v", err, rbErr)
			}
		}
	}()

	// Delete user credentials
	_, err = tx.ExecContext(ctx, `DELETE FROM user_credentials WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}

	// Delete user
	_, err = tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *PostgresStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	user := &models.User{}
	var passwordHash string
	var middleName, phone, avatar sql.NullString

	query := `
		SELECT u.id, u.email, u.phone, u.first_name, u.last_name, u.middle_name,
		       u.avatar, u.status, u.role, u.created_at, u.updated_at, u.last_online, uc.password_hash
		FROM users u
		JOIN user_credentials uc ON u.id = uc.user_id
		WHERE u.email = $1
	`
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline, &passwordHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	return user, passwordHash, nil
}

func (s *PostgresStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	var middleName, phone, avatar sql.NullString

	query := `
		SELECT id, email, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at, last_online
		FROM users WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	return user, nil
}

func (s *PostgresStorage) GetUserByIDWithPassword(ctx context.Context, id uuid.UUID) (*models.User, string, error) {
	user := &models.User{}
	var middleName, phone, avatar sql.NullString
	var passwordHash sql.NullString

	query := `
		SELECT u.id, u.email, u.phone, u.first_name, u.last_name, u.middle_name,
		       u.avatar, u.status, u.role, u.created_at, u.updated_at, u.last_online,
		       p.password_hash
		FROM users u
		LEFT JOIN user_credentials p ON u.id = p.user_id
		WHERE u.id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &phone, &user.FirstName, &user.LastName,
		&middleName, &avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
		&passwordHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	if middleName.Valid {
		user.MiddleName = &middleName.String
	}
	if phone.Valid {
		user.Phone = phone.String
	}
	if avatar.Valid {
		user.Avatar = &avatar.String
	}

	if passwordHash.Valid {
		return user, passwordHash.String, nil
	}
	return user, "", nil
}

func (s *PostgresStorage) UpdateUser(ctx context.Context, user *models.User) error {
	// Security: Only allow updating safe profile fields
	// Email, status, and role require separate admin endpoints
	query := `
		UPDATE users SET
			phone = $1, first_name = $2, last_name = $3,
			middle_name = $4, avatar = $5,
			updated_at = NOW()
		WHERE id = $6
		RETURNING updated_at
	`
	var middleName, avatar sql.NullString
	if user.MiddleName != nil {
		middleName.String = *user.MiddleName
		middleName.Valid = true
	}
	if user.Avatar != nil {
		avatar.String = *user.Avatar
		avatar.Valid = true
	}
	err := s.db.QueryRowContext(ctx, query,
		user.Phone, user.FirstName, user.LastName,
		middleName, avatar, user.ID,
	).Scan(&user.UpdatedAt)
	return err
}

func (s *PostgresStorage) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status, customStatus string) error {
	query := `
		UPDATE users SET
			status = $1, custom_status = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := s.db.ExecContext(ctx, query, status, customStatus, userID)
	return err
}

func (s *PostgresStorage) UsersShareChat(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM chat_members cm1
			JOIN chat_members cm2 ON cm1.chat_id = cm2.chat_id
			WHERE cm1.user_id = $1 AND cm2.user_id = $2
		)
	`
	var exists bool
	err := s.db.QueryRowContext(ctx, query, userID1, userID2).Scan(&exists)
	return exists, err
}

func (s *PostgresStorage) GetUsers(ctx context.Context, search string, excludeUserID string, limit int) ([]*models.User, error) {
	// Default limit to prevent unbounded results
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// Build query dynamically based on search
	var args []interface{}
	argIdx := 1

	whereClause := "WHERE status = 'active'"

	// Exclude current user if specified
	// SECURITY: Column name 'id' is hardcoded, only parameter index is dynamic ($%d)
	// This is safe from SQL injection as column names are not user-controlled
	if excludeUserID != "" {
		whereClause += fmt.Sprintf(" AND id != $%d", argIdx)
		args = append(args, excludeUserID)
		argIdx++
	}

	// Add search filter (email, first_name, last_name)
	if search != "" {
		searchPattern := "%" + strings.ToLower(search) + "%"
		whereClause += fmt.Sprintf(" AND (LOWER(email) LIKE $%d OR LOWER(first_name) LIKE $%d OR LOWER(last_name) LIKE $%d)", argIdx, argIdx+1, argIdx+2)
		args = append(args, searchPattern, searchPattern, searchPattern)
		argIdx += 3
	}

	// Add limit
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT id, email, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at, last_online
		FROM users %s
		ORDER BY created_at DESC
		LIMIT $%d
	`, whereClause, argIdx)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]*models.User, 0)
	for rows.Next() {
		user := &models.User{}
		var middleName, phone, avatar sql.NullString
		err := rows.Scan(
			&user.ID, &user.Email, &phone, &user.FirstName, &user.LastName,
			&middleName, &avatar, &user.Status, &user.Role,
			&user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
		)
		if err != nil {
			return nil, err
		}

		if middleName.Valid {
			user.MiddleName = &middleName.String
		}
		if phone.Valid {
			user.Phone = phone.String
		}
		if avatar.Valid {
			user.Avatar = &avatar.String
		}

		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *PostgresStorage) CreateChat(ctx context.Context, chat *models.Chat) error {
	query := `
		INSERT INTO chats (type, title, description, avatar, creator_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		chat.Type, chat.Title, chat.Description, chat.Avatar, chat.CreatorID,
	).Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
}

func (s *PostgresStorage) findExistingDirectChat(ctx context.Context, userID1, userID2 uuid.UUID) (uuid.UUID, error) {
	query := `
		SELECT c.id FROM chats c
		JOIN chat_members cm1 ON c.id = cm1.chat_id AND cm1.user_id = $1
		JOIN chat_members cm2 ON c.id = cm2.chat_id AND cm2.user_id = $2
		WHERE c.type = 'direct'
		LIMIT 1
	`
	var chatID uuid.UUID
	err := s.db.QueryRowContext(ctx, query, userID1, userID2).Scan(&chatID)
	if err == sql.ErrNoRows {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return chatID, nil
}

func (s *PostgresStorage) CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error {
	// For direct chats, check if one already exists between these users
	if chat.Type == models.ChatTypeDirect && len(members) == 2 {
		existingChatID, err := s.findExistingDirectChat(ctx, members[0].UserID, members[1].UserID)
		if err != nil {
			return fmt.Errorf("check existing direct chat: %w", err)
		}
		if existingChatID != uuid.Nil {
			return fmt.Errorf("direct chat already exists")
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Handle rollback with error check
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				err = fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
			}
		}
	}()

	// Create chat
	chatQuery := `
		INSERT INTO chats (type, title, description, avatar, creator_id)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, chatQuery,
		chat.Type, chat.Title, chat.Description, chat.Avatar, chat.CreatorID,
	).Scan(&chat.ID, &chat.CreatedAt, &chat.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create chat: %w", err)
	}

	// Add members
	memberQuery := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = $3
	`
	for _, member := range members {
		member.ChatID = chat.ID
		_, err = tx.ExecContext(ctx, memberQuery, member.ChatID, member.UserID, member.Role)
		if err != nil {
			return fmt.Errorf("add member %s: %w", member.UserID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (s *PostgresStorage) GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error) {
	chat := &models.Chat{}
	query := `
		SELECT id, type, title, description, avatar, creator_id, created_at, updated_at
		FROM chats WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&chat.ID, &chat.Type, &chat.Title, &chat.Description, &chat.Avatar,
		&chat.CreatorID, &chat.CreatedAt, &chat.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return chat, nil
}

func (s *PostgresStorage) UpdateChat(ctx context.Context, chat *models.Chat) error {
	query := `
		UPDATE chats
		SET title = $1, description = $2, avatar = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query, chat.Title, chat.Description, chat.Avatar, chat.ID).Scan(&chat.UpdatedAt)
}

func (s *PostgresStorage) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error) {
	// Get chats excluding archived and soft-deleted
	query := `
		SELECT c.id, c.type, c.title, c.description, c.avatar, c.creator_id, c.created_at, c.updated_at
		FROM chats c
		JOIN chat_members cm ON c.id = cm.chat_id
		WHERE cm.user_id = $1 AND cm.archived = false AND cm.deleted_at IS NULL
		ORDER BY c.updated_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []*models.Chat
	for rows.Next() {
		chat := &models.Chat{}
		err := rows.Scan(
			&chat.ID, &chat.Type, &chat.Title, &chat.Description, &chat.Avatar,
			&chat.CreatorID, &chat.CreatedAt, &chat.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// For direct chats, get other participant info
	for _, chat := range chats {
		if chat.Type == "direct" {
			members, err := s.GetChatMembersWithUsers(ctx, chat.ID)
			if err == nil {
				chat.Members = members
			}
		}
	}

	return chats, nil
}

func (s *PostgresStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error {
	// Add member
	query := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = $3
	`
	_, err := s.db.ExecContext(ctx, query, member.ChatID, member.UserID, member.Role)
	if err != nil {
		return err
	}

	// Update chat's updated_at timestamp
	_, err = s.db.ExecContext(ctx, `UPDATE chats SET updated_at = NOW() WHERE id = $1`, member.ChatID)
	return err
}

func (s *PostgresStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	query := `
		SELECT chat_id, user_id, role, joined_at, last_read_at, muted, pinned, archived, deleted_at
		FROM chat_members WHERE chat_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.ChatMember
	for rows.Next() {
		m := &models.ChatMember{}
		err := rows.Scan(&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt, &m.Muted, &m.Pinned, &m.Archived, &m.DeletedAt)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *PostgresStorage) GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	query := `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at, cm.last_read_at, cm.muted, cm.pinned, cm.archived, cm.deleted_at,
			u.id, u.email, u.first_name, u.last_name, u.middle_name, u.avatar, u.status, u.role, u.created_at, u.updated_at, u.last_online
		FROM chat_members cm
		JOIN users u ON cm.user_id = u.id
		WHERE cm.chat_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*models.ChatMember
	for rows.Next() {
		m := &models.ChatMember{}
		u := &models.User{}
		var middleName, avatar sql.NullString
		err := rows.Scan(
			&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt, &m.Muted, &m.Pinned, &m.Archived, &m.DeletedAt,
			&u.ID, &u.Email, &u.FirstName, &u.LastName, &middleName, &avatar, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt, &u.LastOnline,
		)
		if err != nil {
			return nil, err
		}
		if middleName.Valid {
			u.MiddleName = &middleName.String
		}
		if avatar.Valid {
			u.Avatar = &avatar.String
		}
		m.User = u
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *PostgresStorage) MuteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET muted = true WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET muted = false WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) PinChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET pinned = true WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET pinned = false WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) ArchiveChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET archived = true WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) UnarchiveChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET archived = false WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) SoftDeleteChat(ctx context.Context, chatID, userID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_members SET deleted_at = NOW() WHERE chat_id = $1 AND user_id = $2`,
		chatID, userID)
	return err
}

func (s *PostgresStorage) ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) error {
	// Mark all messages in this chat as deleted for the user
	query := `
		INSERT INTO deleted_messages (user_id, message_id)
		SELECT $1, m.id FROM messages m WHERE m.chat_id = $2
		ON CONFLICT DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, userID, chatID)
	return err
}

func (s *PostgresStorage) MarkChatForReconciliation(ctx context.Context, chatID uuid.UUID, reason string, details map[string]interface{}) error {
	query := `
		INSERT INTO xmpp_reconciliation (chat_id, reason, details)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, reason, resolved_at) 
		DO UPDATE SET 
			details = EXCLUDED.details,
			attempts = EXCLUDED.attempts + 1,
			last_attempt_at = NOW(),
			created_at = EXCLUDED.created_at
		WHERE xmpp_reconciliation.resolved_at IS NULL
	`
	_, err := s.db.ExecContext(ctx, query, chatID, reason, details)
	return err
}

func (s *PostgresStorage) GetChatsNeedingReconciliation(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	query := `
		SELECT id, chat_id, reason, details, created_at, attempts, last_attempt_at
		FROM xmpp_reconciliation
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
		var id, chatID uuid.UUID
		var reason string
		var details map[string]interface{}
		var createdAt, lastAttemptAt time.Time
		var attempts int

		if err := rows.Scan(&id, &chatID, &reason, &details, &createdAt, &attempts, &lastAttemptAt); err != nil {
			return nil, err
		}

		results = append(results, map[string]interface{}{
			"id":           id,
			"chat_id":      chatID,
			"reason":       reason,
			"details":      details,
			"created_at":   createdAt,
			"attempts":     attempts,
			"last_attempt": lastAttemptAt,
		})
	}
	return results, rows.Err()
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

func (s *PostgresStorage) RemoveChatMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	// Remove member and check if any members left
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
				err = fmt.Errorf("original: %w; rollback: %v", err, rbErr)
			}
		}
	}()

	// Delete the member
	deleteQuery := `DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2`
	_, err = tx.ExecContext(ctx, deleteQuery, chatID, userID)
	if err != nil {
		return false, fmt.Errorf("delete member: %w", err)
	}

	// Check remaining member count
	var count int
	countQuery := `SELECT COUNT(*) FROM chat_members WHERE chat_id = $1`
	err = tx.QueryRowContext(ctx, countQuery, chatID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("count members: %w", err)
	}

	// If no members left, delete the chat
	if count == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
		if err != nil {
			return false, fmt.Errorf("delete chat: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}

	return count == 0, nil
}

func (s *PostgresStorage) DeleteChat(ctx context.Context, chatID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chats WHERE id = $1`, chatID)
	return err
}

func (s *PostgresStorage) CreateMessage(ctx context.Context, msg *models.Message) error {
	query := `
		INSERT INTO messages (chat_id, sender_id, type, content, file_url, reply_to, encrypted_content, encrypted_keys, latitude, longitude, location_address, duration, forwarded_from, forwarded_sender_name, scheduled_at, thread_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at
	`
	// Convert encrypted keys map to JSONB
	var encryptedKeysJSON interface{}
	if len(msg.EncryptedKeys) > 0 {
		data, err := json.Marshal(msg.EncryptedKeys)
		if err != nil {
			return fmt.Errorf("marshal encrypted keys: %w", err)
		}
		encryptedKeysJSON = string(data)
	}

	// Extract location values
	var latitude, longitude *float64
	var locationAddress *string
	if msg.Location != nil {
		latitude = &msg.Location.Latitude
		longitude = &msg.Location.Longitude
		if msg.Location.Address != "" {
			locationAddress = &msg.Location.Address
		}
	}

	// Convert forwarded fields to appropriate types
	var forwardedFrom *uuid.UUID
	var forwardedSenderName *string
	if msg.ForwardedFrom != nil {
		forwardedFrom = msg.ForwardedFrom
	}
	if msg.ForwardedSenderName != nil {
		forwardedSenderName = msg.ForwardedSenderName
	}

	log.Printf("[DB] CreateMessage: forwarded_from=%v, forwarded_sender_name=%v",
		forwardedFrom, forwardedSenderName)

	return s.db.QueryRowContext(ctx, query,
		msg.ChatID, msg.SenderID, msg.Type, msg.Content, msg.FileURL, msg.ReplyTo,
		msg.EncryptedContent, encryptedKeysJSON,
		latitude, longitude, locationAddress, msg.Duration,
		forwardedFrom, forwardedSenderName,
		msg.ScheduledAt, msg.ThreadID).Scan(&msg.ID, &msg.CreatedAt, &msg.UpdatedAt)
}

func (s *PostgresStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	msg := &models.Message{}
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE m.id = $1
	`
	err := scanMessageWithEncryption(
		s.db.QueryRowContext(ctx, query, id),
		msg,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

// scanMessageWithEncryption scans a message row including encryption fields
func scanMessageWithEncryption(rows interface{ Scan(...interface{}) error }, m *models.Message) error {
	var replyTo sql.NullString
	var fileURL sql.NullString
	var encryptedContent sql.NullString
	var encryptedKeysJSON sql.NullString
	var latitude sql.NullFloat64
	var longitude sql.NullFloat64
	var locationAddress sql.NullString
	var duration sql.NullFloat64
	var forwardedFrom sql.NullString
	var forwardedSenderName sql.NullString
	var scheduledAt sql.NullTime
	var threadID sql.NullString
	var syncedToXMPP sql.NullBool
	var xmppMessageID sql.NullString
	var replyToContent sql.NullString
	var replyToSenderName sql.NullString
	err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Type, &m.Content, &fileURL,
		&m.CreatedAt, &m.UpdatedAt, &replyTo,
		&encryptedContent, &encryptedKeysJSON,
		&latitude, &longitude, &locationAddress, &duration,
		&forwardedFrom, &forwardedSenderName,
		&scheduledAt, &threadID, &syncedToXMPP, &xmppMessageID,
		&replyToContent, &replyToSenderName)
	if err != nil {
		return err
	}
	if fileURL.Valid {
		m.FileURL = &fileURL.String
	}
	if replyTo.Valid {
		id, err := uuid.Parse(replyTo.String)
		if err == nil {
			m.ReplyTo = &id
		}
	}
	if replyToContent.Valid {
		m.ReplyToContent = &replyToContent.String
	}
	if replyToSenderName.Valid && replyToSenderName.String != "" {
		m.ReplyToSenderName = &replyToSenderName.String
	}
	if encryptedContent.Valid {
		m.EncryptedContent = encryptedContent.String
	}
	if encryptedKeysJSON.Valid && encryptedKeysJSON.String != "" {
		if err := json.Unmarshal([]byte(encryptedKeysJSON.String), &m.EncryptedKeys); err != nil {
			log.Printf("[ERROR] Failed to unmarshal encrypted keys: %v", err)
			// Continue with empty EncryptedKeys, but log the error
		}
	}
	if latitude.Valid && longitude.Valid {
		m.Location = &models.MessageLocation{
			Latitude:  latitude.Float64,
			Longitude: longitude.Float64,
			Address:   locationAddress.String,
		}
	}
	if duration.Valid {
		m.Duration = &duration.Float64
	}
	if forwardedFrom.Valid {
		id, err := uuid.Parse(forwardedFrom.String)
		if err == nil {
			m.ForwardedFrom = &id
		}
	}
	if forwardedSenderName.Valid {
		m.ForwardedSenderName = &forwardedSenderName.String
		log.Printf("[DB] scanMessageWithEncryption: Set forwarded_sender_name=%s", forwardedSenderName.String)
	}
	if scheduledAt.Valid {
		m.ScheduledAt = &scheduledAt.Time
	}
	if threadID.Valid {
		id, err := uuid.Parse(threadID.String)
		if err == nil {
			m.ThreadID = &id
		}
	}
	return nil
}

const messageColumns = `m.id, m.chat_id, m.sender_id, m.type, m.content, m.file_url, m.created_at, m.updated_at, m.reply_to, m.encrypted_content, m.encrypted_keys, m.latitude, m.longitude, m.location_address, m.duration, m.forwarded_from, m.forwarded_sender_name, m.scheduled_at, m.thread_id, m.synced_to_xmpp, m.xmpp_message_id, reply_msg.content as reply_to_content, reply_sender.first_name || ' ' || reply_sender.last_name as reply_to_sender_name`

const messageColumnsBase = `m.id, m.chat_id, m.sender_id, m.type, m.content, m.file_url, m.created_at, m.updated_at, m.reply_to, m.encrypted_content, m.encrypted_keys, m.latitude, m.longitude, m.location_address, m.duration, m.forwarded_from, m.forwarded_sender_name, m.scheduled_at, m.thread_id, m.synced_to_xmpp, m.xmpp_message_id`

func (s *PostgresStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return s.GetMessagesByChatForUser(ctx, chatID, uuid.Nil, limit, offset)
}

func (s *PostgresStorage) GetAllMessagesByChat(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
		LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
		WHERE m.chat_id = $1
		ORDER BY m.created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		if err := scanMessageWithEncryption(rows, &m); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (s *PostgresStorage) GetMessagesByChatForUser(ctx context.Context, chatID, userID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	var query string
	var args []interface{}

	if userID == uuid.Nil {
		// No user filter - get all messages (for backward compatibility)
		query = `
			SELECT ` + messageColumns + `
			FROM messages m
			LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
			LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
			WHERE m.chat_id = $1
			ORDER BY m.created_at ASC
			LIMIT $2 OFFSET $3
		`
		args = []interface{}{chatID, limit, offset}
	} else {
		// Filter out messages deleted by this specific user
		query = `
			SELECT ` + messageColumns + `
			FROM messages m
			LEFT JOIN messages reply_msg ON m.reply_to = reply_msg.id
			LEFT JOIN users reply_sender ON reply_msg.sender_id = reply_sender.id
			WHERE m.chat_id = $1 AND m.id NOT IN (
				SELECT message_id FROM deleted_messages WHERE user_id = $2
			)
			ORDER BY m.created_at ASC
			LIMIT $3 OFFSET $4
		`
		args = []interface{}{chatID, userID, limit, offset}
	}

	log.Printf("[DB] GetMessagesByChatForUser: chatID=%s, userID=%s, limit=%d, offset=%d", chatID, userID, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := scanMessageWithEncryption(rows, m); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	// Load pinned message IDs for this chat
	pinnedQuery := `SELECT message_id FROM pinned_messages WHERE chat_id = $1`
	log.Printf("[DB] Loading pinned messages for chat %s", chatID)
	pinnedRows, err := s.db.QueryContext(ctx, pinnedQuery, chatID)
	if err != nil {
		log.Printf("[DB] Failed to load pinned messages: %v", err)
		// Continue without pinned status
		return messages, rows.Err()
	}
	defer pinnedRows.Close()

	pinnedIDs := make(map[uuid.UUID]bool)
	pinnedCount := 0
	for pinnedRows.Next() {
		var messageID uuid.UUID
		if err := pinnedRows.Scan(&messageID); err != nil {
			log.Printf("[DB] Failed to scan pinned message ID: %v", err)
			continue
		}
		pinnedIDs[messageID] = true
		pinnedCount++
	}
	log.Printf("[DB] Loaded %d pinned messages for chat %s", pinnedCount, chatID)

	// Mark messages as pinned
	markedCount := 0
	for _, m := range messages {
		if pinnedIDs[m.ID] {
			m.Pinned = true
			markedCount++
		}
	}
	log.Printf("[DB] Marked %d messages as pinned out of %d total", markedCount, len(messages))

	return messages, rows.Err()
}

func (s *PostgresStorage) SearchMessages(ctx context.Context, chatID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
	// Validate and sanitize query
	if len(query) > 200 {
		query = query[:200]
	}
	// Escape special ILIKE characters to prevent pattern abuse
	query = strings.ReplaceAll(query, "%", "\\%")
	query = strings.ReplaceAll(query, "_", "\\_")

	sqlQuery := `
		SELECT ` + messageColumns + `
		FROM messages
		WHERE chat_id = $1 AND content ILIKE $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	searchPattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, sqlQuery, chatID, searchPattern, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := scanMessageWithEncryption(rows, m); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *PostgresStorage) UpdateMessage(ctx context.Context, msg *models.Message) error {
	query := `
		UPDATE messages 
		SET content = $1, updated_at = NOW() 
		WHERE id = $2
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query, msg.Content, msg.ID).Scan(&msg.UpdatedAt)
}

func (s *PostgresStorage) UpdateMessageFileURL(ctx context.Context, messageID uuid.UUID, fileURL string) error {
	query := `
		UPDATE messages 
		SET file_url = $1, updated_at = NOW() 
		WHERE id = $2
	`
	_, err := s.db.ExecContext(ctx, query, fileURL, messageID)
	return err
}

func (s *PostgresStorage) DeleteMessage(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM messages WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

func (s *PostgresStorage) MarkMessageAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	query := `
		INSERT INTO message_reads (message_id, user_id, read_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (message_id, user_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, messageID, userID)
	return err
}

func (s *PostgresStorage) MarkMessagesAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	if len(messageIDs) == 0 {
		return nil
	}

	query := `
		INSERT INTO message_reads (message_id, user_id, read_at)
		SELECT unnest($1::uuid[]), $2, NOW()
		ON CONFLICT (message_id, user_id) DO NOTHING
	`
	_, err := s.db.ExecContext(ctx, query, messageIDs, userID)
	return err
}

func (s *PostgresStorage) GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT user_id FROM message_reads WHERE message_id = $1`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}

func (s *PostgresStorage) GetMessagesReadStatus(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	if len(messageIDs) == 0 {
		return make(map[uuid.UUID][]uuid.UUID), nil
	}

	placeholders := make([]string, len(messageIDs))
	args := make([]interface{}, len(messageIDs))
	for i, id := range messageIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT message_id, user_id
		FROM message_reads
		WHERE message_id IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)
	for rows.Next() {
		var messageID, userID uuid.UUID
		if err := rows.Scan(&messageID, &userID); err != nil {
			return nil, err
		}
		result[messageID] = append(result[messageID], userID)
	}
	return result, rows.Err()
}

func (s *PostgresStorage) PinMessage(ctx context.Context, chatID, messageID, userID uuid.UUID) error {
	query := `INSERT INTO pinned_messages (chat_id, message_id, user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, chatID, messageID, userID)
	return err
}

func (s *PostgresStorage) UnpinMessage(ctx context.Context, chatID, messageID uuid.UUID) error {
	query := `DELETE FROM pinned_messages WHERE chat_id = $1 AND message_id = $2`
	_, err := s.db.ExecContext(ctx, query, chatID, messageID)
	return err
}

func (s *PostgresStorage) GetPinnedMessages(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	query := `
		SELECT m.` + messageColumns + `
		FROM pinned_messages pm
		JOIN messages m ON pm.message_id = m.id
		WHERE pm.chat_id = $1
		ORDER BY pm.pinned_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query, chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		if err := scanMessageWithEncryption(rows, &m); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, rows.Err()
}

func (s *PostgresStorage) CreateSession(ctx context.Context, session *models.Session) error {
	query := `
		INSERT INTO sessions (id, user_id, token, device_info, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := s.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.Token, session.DeviceInfo, session.IP, session.ExpiresAt,
	)
	return err
}

func (s *PostgresStorage) GetSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	session := &models.Session{}
	query := `
		SELECT id, user_id, token, device_info, ip, created_at, expires_at
		FROM sessions WHERE token = $1 AND expires_at > NOW()
	`
	err := s.db.QueryRowContext(ctx, query, token).Scan(
		&session.ID, &session.UserID, &session.Token, &session.DeviceInfo,
		&session.IP, &session.CreatedAt, &session.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (s *PostgresStorage) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func (s *PostgresStorage) DeleteOldSessionsForUser(ctx context.Context, userID uuid.UUID, keep int) error {
	// Keep only the most recent 'keep' sessions for the user
	query := `
		DELETE FROM sessions
		WHERE id IN (
			SELECT id FROM sessions
			WHERE user_id = $1
			ORDER BY created_at DESC
			OFFSET $2
		)
	`
	_, err := s.db.ExecContext(ctx, query, userID, keep)
	return err
}

func (s *PostgresStorage) CreateFile(ctx context.Context, file *models.File) error {
	query := `
		INSERT INTO files (id, message_id, name, size, mime_type, url, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.ExecContext(ctx, query,
		file.ID, file.MessageID, file.Name, file.Size, file.MimeType, file.URL, file.UploadedAt,
	)
	return err
}

func (s *PostgresStorage) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	file := &models.File{}
	query := `
		SELECT id, message_id, name, size, mime_type, url, uploaded_at
		FROM files
		WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID, &file.MessageID, &file.Name, &file.Size, &file.MimeType, &file.URL, &file.UploadedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *PostgresStorage) GetFilesByMessage(ctx context.Context, messageID uuid.UUID) ([]*models.File, error) {
	query := `
		SELECT id, message_id, name, size, mime_type, url, uploaded_at
		FROM files
		WHERE message_id = $1
		ORDER BY uploaded_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*models.File
	for rows.Next() {
		file := &models.File{}
		if err := rows.Scan(&file.ID, &file.MessageID, &file.Name, &file.Size, &file.MimeType, &file.URL, &file.UploadedAt); err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// NotificationStorage implementation

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

func (s *PostgresStorage) GetNotificationSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	settings := &models.NotificationSettings{UserID: userID}
	query := `
		SELECT push_enabled, email_enabled, email, 
		       CAST(quiet_hours_start AS TEXT), 
		       CAST(quiet_hours_end AS TEXT), 
		       quiet_hours_enabled, protocol_preference, updated_at
		FROM notification_settings
		WHERE user_id = $1
	`
	var quietStartStr, quietEndStr string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.PushEnabled, &settings.EmailEnabled, &settings.Email,
		&quietStartStr, &quietEndStr, &settings.QuietHoursEnabled, &settings.ProtocolPreference, &settings.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return default settings
			return &models.NotificationSettings{
				UserID:       userID,
				PushEnabled:  true,
				EmailEnabled: true,
			}, nil
		}
		return nil, err
	}
	// Set quiet hours strings if they exist
	if quietStartStr != "" {
		settings.QuietHoursStart = &quietStartStr
	}
	if quietEndStr != "" {
		settings.QuietHoursEnd = &quietEndStr
	}
	return settings, nil
}

func (s *PostgresStorage) UpdateNotificationSettings(ctx context.Context, settings *models.NotificationSettings) error {
	// Convert string times to proper time.Time for database
	var quietStart, quietEnd interface{}
	if settings.QuietHoursStart != nil {
		if t, err := time.Parse("15:04", *settings.QuietHoursStart); err == nil {
			quietStart = t
		} else {
			quietStart = nil
		}
	}
	if settings.QuietHoursEnd != nil {
		if t, err := time.Parse("15:04", *settings.QuietHoursEnd); err == nil {
			quietEnd = t
		} else {
			quietEnd = nil
		}
	}

	// Ensure protocol_preference has a valid value
	protocolPreference := settings.ProtocolPreference
	if protocolPreference == "" {
		protocolPreference = "websocket"
	}

	// Log the values being sent to database
	fmt.Printf("[PostgresStorage] UpdateNotificationSettings: protocol_preference=%q, settings.ProtocolPreference=%q, quiet_hours_enabled=%v\n", protocolPreference, settings.ProtocolPreference, settings.QuietHoursEnabled)

	query := `
		INSERT INTO notification_settings (user_id, push_enabled, email_enabled, email, quiet_hours_start, quiet_hours_end, quiet_hours_enabled, protocol_preference, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			push_enabled = EXCLUDED.push_enabled,
			email_enabled = EXCLUDED.email_enabled,
			email = COALESCE(EXCLUDED.email, notification_settings.email),
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			quiet_hours_enabled = EXCLUDED.quiet_hours_enabled,
			protocol_preference = CASE WHEN EXCLUDED.protocol_preference IS NULL OR EXCLUDED.protocol_preference = '' THEN 'websocket' ELSE EXCLUDED.protocol_preference END,
			updated_at = EXCLUDED.updated_at
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		settings.UserID, settings.PushEnabled, settings.EmailEnabled, settings.Email,
		quietStart, quietEnd, settings.QuietHoursEnabled,
		protocolPreference,
	).Scan(&settings.UpdatedAt)
}

func (s *PostgresStorage) GetChatMember(ctx context.Context, chatID, userID uuid.UUID) (*models.ChatMember, error) {
	query := `
		SELECT chat_id, user_id, role, joined_at, last_read_at, muted, pinned, archived, deleted_at
		FROM chat_members
		WHERE chat_id = $1 AND user_id = $2
	`
	m := &models.ChatMember{}
	var lastRead *time.Time
	err := s.db.QueryRowContext(ctx, query, chatID, userID).Scan(
		&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &lastRead, &m.Muted, &m.Pinned, &m.Archived, &m.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m.LastReadAt = lastRead
	return m, nil
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

func (s *PostgresStorage) AddBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	query := `INSERT INTO bookmarks (user_id, message_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, userID, messageID)
	return err
}

func (s *PostgresStorage) RemoveBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	query := `DELETE FROM bookmarks WHERE user_id = $1 AND message_id = $2`
	_, err := s.db.ExecContext(ctx, query, userID, messageID)
	return err
}

func (s *PostgresStorage) GetBookmarks(ctx context.Context, userID uuid.UUID) ([]*models.Message, error) {
	query := `SELECT ` + messageColumns + ` FROM messages m
		JOIN bookmarks b ON b.message_id = m.id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := scanMessageWithEncryption(rows, m); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *PostgresStorage) BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	query := `INSERT INTO blocked_users (blocker_id, blocked_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, blockerID, blockedID)
	return err
}

func (s *PostgresStorage) UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	query := `DELETE FROM blocked_users WHERE blocker_id = $1 AND blocked_id = $2`
	_, err := s.db.ExecContext(ctx, query, blockerID, blockedID)
	return err
}

func (s *PostgresStorage) GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT blocked_id FROM blocked_users WHERE blocker_id = $1`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blockedIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		blockedIDs = append(blockedIDs, id)
	}
	return blockedIDs, rows.Err()
}

func (s *PostgresStorage) IsUserBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM blocked_users WHERE blocker_id = $1 AND blocked_id = $2)`
	err := s.db.QueryRowContext(ctx, query, blockerID, blockedID).Scan(&exists)
	return exists, err
}

func (s *PostgresStorage) AddMention(ctx context.Context, messageID, mentionedUserID uuid.UUID) error {
	query := `INSERT INTO mentions (message_id, mentioned_user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, messageID, mentionedUserID)
	return err
}

func (s *PostgresStorage) SearchAllMessages(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
	searchQuery := `
		SELECT m.` + messageColumns + `
		FROM messages m
		JOIN chat_members cm ON m.chat_id = cm.chat_id
		WHERE cm.user_id = $1 
			AND cm.archived = false 
			AND cm.deleted_at IS NULL
			AND to_tsvector('russian', m.content) @@ plainto_tsquery('russian', $2)
		ORDER BY m.created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := s.db.QueryContext(ctx, searchQuery, userID, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		if err := scanMessageWithEncryption(rows, &m); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, nil
}

func (s *PostgresStorage) GetUserMentions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Mention, error) {
	query := `
		SELECT id, message_id, mentioned_user_id, created_at
		FROM mentions
		WHERE mentioned_user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mentions []*models.Mention
	for rows.Next() {
		var m models.Mention
		if err := rows.Scan(&m.ID, &m.MessageID, &m.MentionedUserID, &m.CreatedAt); err != nil {
			return nil, err
		}
		mentions = append(mentions, &m)
	}
	return mentions, nil
}

func (s *PostgresStorage) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	query := `INSERT INTO reactions (message_id, user_id, emoji) VALUES ($1, $2, $3) ON CONFLICT (message_id, user_id, emoji) DO NOTHING`
	_, err := s.db.ExecContext(ctx, query, messageID, userID, emoji)
	return err
}

func (s *PostgresStorage) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	query := `DELETE FROM reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`
	_, err := s.db.ExecContext(ctx, query, messageID, userID, emoji)
	return err
}

func (s *PostgresStorage) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]*models.Reaction, error) {
	query := `
		SELECT id, message_id, user_id, emoji, created_at
		FROM reactions
		WHERE message_id = $1
		ORDER BY created_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []*models.Reaction
	for rows.Next() {
		var r models.Reaction
		if err := rows.Scan(&r.ID, &r.MessageID, &r.UserID, &r.Emoji, &r.CreatedAt); err != nil {
			return nil, err
		}
		reactions = append(reactions, &r)
	}
	return reactions, nil
}

func (s *PostgresStorage) GetUserLastOnline(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	// Use last session activity or WebSocket connection
	query := `
		SELECT COALESCE(MAX(last_used_at), '1970-01-01'::timestamp)
		FROM device_tokens
		WHERE user_id = $1
	`
	var lastOnline time.Time
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&lastOnline)
	return lastOnline, err
}

func (s *PostgresStorage) UpdateUserLastOnline(ctx context.Context, userID uuid.UUID) error {
	// Update device token timestamp
	_, err := s.db.ExecContext(ctx, `
		UPDATE device_tokens SET last_used_at = NOW() WHERE user_id = $1
	`, userID)
	return err
}

// AuditStorage implementation
func (s *PostgresStorage) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, user_id, action, resource, resource_id, details, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := s.db.ExecContext(ctx, query, log.ID, log.UserID, log.Action, log.Resource, log.ResourceID, log.Details, log.IPAddress, log.UserAgent, log.CreatedAt)
	return err
}

func (s *PostgresStorage) GetAuditLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, resource, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Resource, &log.ResourceID, &log.Details, &log.IPAddress, &log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}

func (s *PostgresStorage) GetAllAuditLogs(ctx context.Context, limit, offset int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, resource, resource_id, details, ip_address, user_agent, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Resource, &log.ResourceID, &log.Details, &log.IPAddress, &log.UserAgent, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &log)
	}
	return logs, rows.Err()
}

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

// XMPP sync methods
func (s *PostgresStorage) GetUnsyncedMessages(ctx context.Context, limit int) ([]*models.Message, error) {
	// Add time filter to prevent syncing millions of old messages
	// Only sync messages created in the last 30 days
	query := `
		SELECT ` + messageColumns + `
		FROM messages
		WHERE (synced_to_xmpp = false OR synced_to_xmpp IS NULL)
		AND created_at > NOW() - INTERVAL '30 days'
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := scanMessageWithEncryption(rows, m); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *PostgresStorage) MarkMessageAsSyncedToXMPP(ctx context.Context, messageID uuid.UUID, xmppMessageID string) error {
	query := `
		UPDATE messages
		SET synced_to_xmpp = true, xmpp_message_id = $1
		WHERE id = $2
	`
	_, err := s.db.ExecContext(ctx, query, xmppMessageID, messageID)
	return err
}

func (s *PostgresStorage) GetMessageByXMPPID(ctx context.Context, xmppMessageID string) (*models.Message, error) {
	msg := &models.Message{}
	query := `SELECT ` + messageColumns + ` FROM messages m WHERE xmpp_message_id = $1`
	err := scanMessageWithEncryption(
		s.db.QueryRowContext(ctx, query, xmppMessageID),
		msg,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

func (s *PostgresStorage) GetScheduledMessages(ctx context.Context) ([]*models.Message, error) {
	query := `
		SELECT ` + messageColumns + `
		FROM messages m
		WHERE scheduled_at IS NOT NULL AND scheduled_at <= NOW()
		ORDER BY scheduled_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		if err := scanMessageWithEncryption(rows, m); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *PostgresStorage) GetThreadMessages(ctx context.Context, threadID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	query := `
		SELECT ` + messageColumns + `
		FROM messages
		WHERE thread_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, threadID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		var m models.Message
		if err := scanMessageWithEncryption(rows, &m); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, rows.Err()
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
