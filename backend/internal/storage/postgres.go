package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"corp-messenger/backend/internal/models"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

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

	if err := db.Ping(); err != nil {
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
    token VARCHAR(500) NOT NULL,
    device_info VARCHAR(255),
    ip VARCHAR(45),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_chat_members_user_id ON chat_members(user_id);
CREATE INDEX IF NOT EXISTS idx_chat_members_chat_id ON chat_members(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_chat_id ON messages(chat_id);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
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

func (s *PostgresStorage) CreateUser(ctx context.Context, user *models.User, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

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

	return tx.Commit()
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
		if err == sql.ErrNoRows {
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
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

func (s *PostgresStorage) GetUsers(ctx context.Context) ([]*models.User, error) {
	query := `
		SELECT id, email, phone, first_name, last_name, middle_name,
		       avatar, status, role, created_at, updated_at
		FROM users WHERE status = 'active'
		ORDER BY created_at DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
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
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return chat, nil
}

func (s *PostgresStorage) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error) {
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
	return chats, rows.Err()
}

func (s *PostgresStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error {
	query := `
		INSERT INTO chat_members (chat_id, user_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (chat_id, user_id) DO UPDATE SET role = $3
	`
	_, err := s.db.ExecContext(ctx, query, member.ChatID, member.UserID, member.Role)
	return err
}

func (s *PostgresStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	query := `
		SELECT chat_id, user_id, role, joined_at, last_read_at
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
		err := rows.Scan(&m.ChatID, &m.UserID, &m.Role, &m.JoinedAt, &m.LastReadAt)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *PostgresStorage) CreateMessage(ctx context.Context, msg *models.Message) error {
	query := `
		INSERT INTO messages (chat_id, sender_id, type, content, reply_to)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	return s.db.QueryRowContext(ctx, query,
		msg.ChatID, msg.SenderID, msg.Type, msg.Content, msg.ReplyTo,
	).Scan(&msg.ID, &msg.CreatedAt, &msg.UpdatedAt)
}

func (s *PostgresStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	query := `
		SELECT id, chat_id, sender_id, type, content, created_at, updated_at, reply_to
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := s.db.QueryContext(ctx, query, chatID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.Message
	for rows.Next() {
		m := &models.Message{}
		var replyTo sql.NullString
		err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.Type, &m.Content,
			&m.CreatedAt, &m.UpdatedAt, &replyTo)
		if err != nil {
			return nil, err
		}
		if replyTo.Valid {
			id, _ := uuid.Parse(replyTo.String)
			m.ReplyTo = &id
		}
		messages = append(messages, m)
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
		if err == sql.ErrNoRows {
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
