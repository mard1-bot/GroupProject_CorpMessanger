package storage

import (
	"context"
	"database/sql"
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
	schema := `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone VARCHAR(20),
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    middle_name VARCHAR(100),
    avatar VARCHAR(500),
    status VARCHAR(20) DEFAULT 'active',
    role VARCHAR(20) DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

CREATE TABLE IF NOT EXISTS user_credentials (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(20) NOT NULL,
    title VARCHAR(200),
    description TEXT,
    avatar VARCHAR(500),
    creator_id UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chat_members (
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) DEFAULT 'member',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_read_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    muted BOOLEAN DEFAULT FALSE,
    pinned BOOLEAN DEFAULT FALSE,
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID REFERENCES chats(id) ON DELETE CASCADE,
    sender_id UUID REFERENCES users(id),
    type VARCHAR(20) DEFAULT 'text',
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    reply_to UUID REFERENCES messages(id)
);

CREATE TABLE IF NOT EXISTS files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id UUID REFERENCES messages(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    url VARCHAR(500) NOT NULL,
    uploaded_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(500) NOT NULL UNIQUE,
    device_info VARCHAR(255),
    ip VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_members_chat_id ON chat_members(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_sender_id ON messages(sender_id);
CREATE INDEX IF NOT EXISTS idx_messages_reply_to ON messages(reply_to);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
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
	err = tx.QueryRowContext(ctx, query,
		user.Email, user.Phone, user.FirstName, user.LastName, user.MiddleName,
		user.Avatar, user.Status, user.Role,
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

func (s *PostgresStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	user := &models.User{}
	var passwordHash string

	query := `
		SELECT u.id, u.email, u.phone, u.first_name, u.last_name, u.middle_name,
		       u.avatar, u.status, u.role, u.created_at, u.updated_at, uc.password_hash
		FROM users u
		JOIN user_credentials uc ON u.id = uc.user_id
		WHERE u.email = $1
	`
	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Phone, &user.FirstName, &user.LastName,
		&user.MiddleName, &user.Avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt, &passwordHash,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}
	return user, passwordHash, nil
}

func (s *PostgresStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user := &models.User{}

	query := `
		SELECT id, email, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at
		FROM users WHERE id = $1
	`
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Phone, &user.FirstName, &user.LastName,
		&user.MiddleName, &user.Avatar, &user.Status, &user.Role,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
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
	err := s.db.QueryRowContext(ctx, query,
		user.Phone, user.FirstName, user.LastName,
		user.MiddleName, user.Avatar,
		user.ID,
	).Scan(&user.UpdatedAt)
	return err
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
		whereClause += fmt.Sprintf(" AND (LOWER(email) LIKE $%d OR LOWER(first_name) LIKE $%d OR LOWER(last_name) LIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, searchPattern)
		argIdx++
	}

	// Add limit
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT id, email, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at
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
		err := rows.Scan(
			&user.ID, &user.Email, &user.Phone, &user.FirstName, &user.LastName,
			&user.MiddleName, &user.Avatar, &user.Status, &user.Role,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
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

	// Load members for direct chats
	if chat.Type == "direct" {
		members, err := s.GetChatMembersWithUsers(ctx, chat.ID)
		if err != nil {
			// Log error but don't fail
			fmt.Printf("ERROR: GetChatMembersWithUsers failed: %v\n", err)
		} else {
			chat.Members = members
		}
	}

	return chat, nil
}

func (s *PostgresStorage) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error) {
	// First get basic chat info
	query := `
		SELECT c.id, c.type, c.title, c.description, c.avatar, c.creator_id, c.created_at, c.updated_at
		FROM chats c
		JOIN chat_members cm ON c.id = cm.chat_id
		WHERE cm.user_id = $1
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
		SELECT chat_id, user_id, role, joined_at, last_read_at, muted, pinned
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
		err := rows.Scan(&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt, &m.Muted, &m.Pinned)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *PostgresStorage) GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	query := `
		SELECT cm.chat_id, cm.user_id, cm.role, cm.joined_at, cm.last_read_at, cm.muted, cm.pinned,
			u.id, u.email, u.first_name, u.last_name, u.middle_name, u.avatar, u.status, u.role, u.created_at, u.updated_at
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
		err := rows.Scan(
			&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt, &m.Muted, &m.Pinned,
			&u.ID, &u.Email, &u.FirstName, &u.LastName, &u.MiddleName, &u.Avatar, &u.Status, &u.Role, &u.CreatedAt, &u.UpdatedAt,
		)
		if err != nil {
			return nil, err
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
		INSERT INTO messages (chat_id, sender_id, type, content, file_url, reply_to)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		msg.ChatID, msg.SenderID, msg.Type, msg.Content, msg.FileURL, msg.ReplyTo,
	).Scan(&msg.ID, &msg.CreatedAt, &msg.UpdatedAt)
}

func (s *PostgresStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	msg := &models.Message{}
	query := `
		SELECT id, chat_id, sender_id, type, content, file_url, created_at, updated_at, reply_to
		FROM messages WHERE id = $1
	`
	var replyTo sql.NullString
	var fileURL sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Type, &msg.Content, &fileURL,
		&msg.CreatedAt, &msg.UpdatedAt, &replyTo,
	)
	if fileURL.Valid {
		msg.FileURL = &fileURL.String
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if replyTo.Valid {
		replyID, err := uuid.Parse(replyTo.String)
		if err == nil {
			msg.ReplyTo = &replyID
		}
	}
	return msg, nil
}

func (s *PostgresStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	query := `
		SELECT id, chat_id, sender_id, type, content, file_url, created_at, updated_at, reply_to
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`
	log.Printf("[DB] GetMessagesByChat: chatID=%s, limit=%d, offset=%d", chatID, limit, offset)
	rows, err := s.db.QueryContext(ctx, query, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		var replyTo sql.NullString
		var fileURL sql.NullString
		err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Type, &m.Content, &fileURL,
			&m.CreatedAt, &m.UpdatedAt, &replyTo)
		if err != nil {
			return nil, err
		}
		if fileURL.Valid {
			m.FileURL = &fileURL.String
		}
		if replyTo.Valid {
			id, err := uuid.Parse(replyTo.String)
			if err == nil {
				m.ReplyTo = &id
			}
			// If parse fails, ReplyTo remains nil (invalid UUID in DB)
		}
		messages = append(messages, m)
	}
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
		SELECT id, chat_id, sender_id, type, content, file_url, created_at, updated_at, reply_to
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
		var replyTo sql.NullString
		var fileURL sql.NullString
		err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Type, &m.Content, &fileURL,
			&m.CreatedAt, &m.UpdatedAt, &replyTo)
		if err != nil {
			return nil, err
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

func (s *PostgresStorage) GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error) {
	query := `SELECT user_id FROM message_reads WHERE message_id = $1`
	rows, err := s.db.QueryContext(ctx, query, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []uuid.UUID
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		users = append(users, userID)
	}
	return users, rows.Err()
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
		SELECT push_enabled, email_enabled, email, quiet_hours_start, quiet_hours_end, quiet_hours_enabled, updated_at
		FROM notification_settings
		WHERE user_id = $1
	`
	var quietStart, quietEnd *string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&settings.PushEnabled, &settings.EmailEnabled, &settings.Email,
		&quietStart, &quietEnd, &settings.QuietHoursEnabled, &settings.UpdatedAt,
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
	settings.QuietHoursStart = quietStart
	settings.QuietHoursEnd = quietEnd
	return settings, nil
}

func (s *PostgresStorage) UpdateNotificationSettings(ctx context.Context, settings *models.NotificationSettings) error {
	query := `
		INSERT INTO notification_settings (user_id, push_enabled, email_enabled, email, quiet_hours_start, quiet_hours_end, quiet_hours_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			push_enabled = EXCLUDED.push_enabled,
			email_enabled = EXCLUDED.email_enabled,
			email = EXCLUDED.email,
			quiet_hours_start = EXCLUDED.quiet_hours_start,
			quiet_hours_end = EXCLUDED.quiet_hours_end,
			quiet_hours_enabled = EXCLUDED.quiet_hours_enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		settings.UserID, settings.PushEnabled, settings.EmailEnabled, settings.Email,
		settings.QuietHoursStart, settings.QuietHoursEnd, settings.QuietHoursEnabled,
	).Scan(&settings.UpdatedAt)
}

func (s *PostgresStorage) GetChatMember(ctx context.Context, chatID, userID uuid.UUID) (*models.ChatMember, error) {
	query := `
		SELECT chat_id, user_id, role, joined_at, last_read_at, muted, pinned
		FROM chat_members
		WHERE chat_id = $1 AND user_id = $2
	`
	m := &models.ChatMember{}
	var lastRead *time.Time
	err := s.db.QueryRowContext(ctx, query, chatID, userID).Scan(
		&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &lastRead, &m.Muted, &m.Pinned,
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
