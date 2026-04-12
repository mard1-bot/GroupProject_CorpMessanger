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
}

type ChatStorage interface {
	CreateChat(ctx context.Context, chat *models.Chat) error
	GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error)
	GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error)
	AddChatMember(ctx context.Context, member *models.ChatMember) error
	GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error)
}

type MessageStorage interface {
	CreateMessage(ctx context.Context, msg *models.Message) error
	GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error)
}

type SessionStorage interface {
	CreateSession(ctx context.Context, session *models.Session) error
	GetSessionByToken(ctx context.Context, token string) (*models.Session, error)
	DeleteSession(ctx context.Context, token string) error
}
