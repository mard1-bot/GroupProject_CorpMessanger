package storage

import (
	"context"

	"corp-messenger/backend/internal/models"

	"github.com/google/uuid"
)

type Storage interface {
	Ready(ctx context.Context) error
	Close() error

	UserStorage
	ChatStorage
	MessageStorage
	SessionStorage
}

type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUsers(ctx context.Context) ([]*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
}

type ChatStorage interface {
	CreateChat(ctx context.Context, chat *models.Chat) error
	CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error
	GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error)
	GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error)
	AddChatMember(ctx context.Context, member *models.ChatMember) error
	GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error)
}

type MessageStorage interface {
	CreateMessage(ctx context.Context, msg *models.Message) error
	GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error)
	GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error)
}

type SessionStorage interface {
	CreateSession(ctx context.Context, session *models.Session) error
	GetSessionByToken(ctx context.Context, token string) (*models.Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteOldSessionsForUser(ctx context.Context, userID uuid.UUID, keep int) error
}

// StubStorage is a stub implementation of Storage for testing
type StubStorage struct{}

func NewStub() *StubStorage {
	return &StubStorage{}
}

func (s *StubStorage) Ready(ctx context.Context) error { return nil }
func (s *StubStorage) Close() error                    { return nil }
func (s *StubStorage) CreateUser(ctx context.Context, user *models.User, passwordHash string) error {
	return nil
}
func (s *StubStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	return nil, "", nil
}
func (s *StubStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return nil, nil
}
func (s *StubStorage) GetUsers(ctx context.Context) ([]*models.User, error) {
	return nil, nil
}
func (s *StubStorage) UpdateUser(ctx context.Context, user *models.User) error { return nil }
func (s *StubStorage) CreateChat(ctx context.Context, chat *models.Chat) error { return nil }
func (s *StubStorage) CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error {
	return nil
}
func (s *StubStorage) GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error) {
	return nil, nil
}
func (s *StubStorage) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error) {
	return nil, nil
}
func (s *StubStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error { return nil }
func (s *StubStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	return nil, nil
}
func (s *StubStorage) CreateMessage(ctx context.Context, msg *models.Message) error { return nil }
func (s *StubStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) CreateSession(ctx context.Context, session *models.Session) error { return nil }
func (s *StubStorage) GetSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	return nil, nil
}
func (s *StubStorage) DeleteSession(ctx context.Context, token string) error { return nil }
func (s *StubStorage) DeleteOldSessionsForUser(ctx context.Context, userID uuid.UUID, keep int) error {
	return nil
}
