package storage

import (
	"context"
	"time"

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
	FileStorage
	NotificationStorage
}

type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUsers(ctx context.Context, search string, excludeUserID string, limit int) ([]*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
}

type ChatStorage interface {
	CreateChat(ctx context.Context, chat *models.Chat) error
	CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error
	GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error)
	GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error)
	DeleteChat(ctx context.Context, chatID uuid.UUID) error
	AddChatMember(ctx context.Context, member *models.ChatMember) error
	RemoveChatMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
	GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error)
	GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error)
	MuteChat(ctx context.Context, chatID, userID uuid.UUID) error
	UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error
	PinChat(ctx context.Context, chatID, userID uuid.UUID) error
	UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error
}

type MessageStorage interface {
	CreateMessage(ctx context.Context, msg *models.Message) error
	GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error)
	GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error)
	SearchMessages(ctx context.Context, chatID uuid.UUID, query string, limit, offset int) ([]*models.Message, error)
	UpdateMessage(ctx context.Context, msg *models.Message) error
	UpdateMessageFileURL(ctx context.Context, messageID uuid.UUID, fileURL string) error
	DeleteMessage(ctx context.Context, id uuid.UUID) error
	MarkMessageAsRead(ctx context.Context, messageID, userID uuid.UUID) error
	GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error)
}

type SessionStorage interface {
	CreateSession(ctx context.Context, session *models.Session) error
	GetSessionByToken(ctx context.Context, token string) (*models.Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteOldSessionsForUser(ctx context.Context, userID uuid.UUID, keep int) error
}

type FileStorage interface {
	CreateFile(ctx context.Context, file *models.File) error
	GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error)
	GetFilesByMessage(ctx context.Context, messageID uuid.UUID) ([]*models.File, error)
}

type NotificationStorage interface {
	CreateDeviceToken(ctx context.Context, token *models.DeviceToken) error
	GetDeviceTokens(ctx context.Context, userID uuid.UUID) ([]*models.DeviceToken, error)
	DeleteDeviceToken(ctx context.Context, token string) error
	GetNotificationSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error)
	UpdateNotificationSettings(ctx context.Context, settings *models.NotificationSettings) error
	GetChatMember(ctx context.Context, chatID, userID uuid.UUID) (*models.ChatMember, error)
	GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
	GetUserLastOnline(ctx context.Context, userID uuid.UUID) (time.Time, error)
	UpdateUserLastOnline(ctx context.Context, userID uuid.UUID) error
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
func (s *StubStorage) GetUsers(ctx context.Context, search string, excludeUserID string, limit int) ([]*models.User, error) {
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
func (s *StubStorage) DeleteChat(ctx context.Context, chatID uuid.UUID) error             { return nil }
func (s *StubStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error { return nil }
func (s *StubStorage) RemoveChatMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	return false, nil
}
func (s *StubStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	return nil, nil
}
func (s *StubStorage) GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	return nil, nil
}
func (s *StubStorage) MuteChat(ctx context.Context, chatID, userID uuid.UUID) error   { return nil }
func (s *StubStorage) UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error { return nil }
func (s *StubStorage) PinChat(ctx context.Context, chatID, userID uuid.UUID) error    { return nil }
func (s *StubStorage) UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error  { return nil }
func (s *StubStorage) CreateMessage(ctx context.Context, msg *models.Message) error   { return nil }
func (s *StubStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) SearchMessages(ctx context.Context, chatID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) UpdateMessage(ctx context.Context, msg *models.Message) error { return nil }
func (s *StubStorage) UpdateMessageFileURL(ctx context.Context, messageID uuid.UUID, fileURL string) error {
	return nil
}
func (s *StubStorage) DeleteMessage(ctx context.Context, id uuid.UUID) error { return nil }
func (s *StubStorage) MarkMessageAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	return nil
}
func (s *StubStorage) GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error) {
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
func (s *StubStorage) CreateFile(ctx context.Context, file *models.File) error { return nil }
func (s *StubStorage) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	return nil, nil
}
func (s *StubStorage) GetFilesByMessage(ctx context.Context, messageID uuid.UUID) ([]*models.File, error) {
	return nil, nil
}

// NotificationStorage stubs
func (s *StubStorage) CreateDeviceToken(ctx context.Context, token *models.DeviceToken) error {
	return nil
}
func (s *StubStorage) GetDeviceTokens(ctx context.Context, userID uuid.UUID) ([]*models.DeviceToken, error) {
	return nil, nil
}
func (s *StubStorage) DeleteDeviceToken(ctx context.Context, token string) error { return nil }
func (s *StubStorage) GetNotificationSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	return nil, nil
}
func (s *StubStorage) UpdateNotificationSettings(ctx context.Context, settings *models.NotificationSettings) error {
	return nil
}
func (s *StubStorage) GetChatMember(ctx context.Context, chatID, userID uuid.UUID) (*models.ChatMember, error) {
	return nil, nil
}
func (s *StubStorage) GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return 0, nil
}
func (s *StubStorage) GetUserLastOnline(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	return time.Time{}, nil
}
func (s *StubStorage) UpdateUserLastOnline(ctx context.Context, userID uuid.UUID) error { return nil }
