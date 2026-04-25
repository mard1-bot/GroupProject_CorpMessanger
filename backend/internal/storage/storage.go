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
	EncryptionStorage
	BookmarkStorage
	BlockStorage
	MentionStorage
	ReactionStorage
	AuditStorage
}

type UserStorage interface {
	CreateUser(ctx context.Context, user *models.User, passwordHash string) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUsers(ctx context.Context, search string, excludeUserID string, limit int) ([]*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	UpdateUserStatus(ctx context.Context, userID uuid.UUID, status, customStatus string) error
	UsersShareChat(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error)
}

type ChatStorage interface {
	CreateChat(ctx context.Context, chat *models.Chat) error
	CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error
	GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error)
	GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error)
	UpdateChat(ctx context.Context, chat *models.Chat) error
	DeleteChat(ctx context.Context, chatID uuid.UUID) error
	AddChatMember(ctx context.Context, member *models.ChatMember) error
	RemoveChatMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error)
	GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error)
	GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error)
	MuteChat(ctx context.Context, chatID, userID uuid.UUID) error
	UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error
	PinChat(ctx context.Context, chatID, userID uuid.UUID) error
	UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error
	ArchiveChat(ctx context.Context, chatID, userID uuid.UUID) error
	UnarchiveChat(ctx context.Context, chatID, userID uuid.UUID) error
	SoftDeleteChat(ctx context.Context, chatID, userID uuid.UUID) error
	ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) error
}

type MessageStorage interface {
	CreateMessage(ctx context.Context, msg *models.Message) error
	GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error)
	GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error)
	GetMessagesByChatForUser(ctx context.Context, chatID, userID uuid.UUID, limit, offset int) ([]*models.Message, error)
	GetAllMessagesByChat(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error)
	SearchMessages(ctx context.Context, chatID uuid.UUID, query string, limit, offset int) ([]*models.Message, error)
	SearchAllMessages(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]*models.Message, error)
	UpdateMessage(ctx context.Context, msg *models.Message) error
	UpdateMessageFileURL(ctx context.Context, messageID uuid.UUID, fileURL string) error
	DeleteMessage(ctx context.Context, messageID uuid.UUID) error
	MarkMessageAsRead(ctx context.Context, messageID, userID uuid.UUID) error
	MarkMessagesAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error
	GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error)
	GetMessagesReadStatus(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	PinMessage(ctx context.Context, chatID, messageID, userID uuid.UUID) error
	UnpinMessage(ctx context.Context, chatID, messageID uuid.UUID) error
	GetPinnedMessages(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error)
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

type AuditStorage interface {
	CreateAuditLog(ctx context.Context, log *models.AuditLog) error
	GetAuditLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error)
	GetAllAuditLogs(ctx context.Context, limit, offset int) ([]*models.AuditLog, error) // Admin only
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

type EncryptionStorage interface {
	GetUserPublicKey(ctx context.Context, userID uuid.UUID) (*models.EncryptionKey, error)
	GetUsersPublicKeys(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*models.EncryptionKey, error)
	SaveUserPublicKey(ctx context.Context, key *models.EncryptionKey) error
}

type BookmarkStorage interface {
	AddBookmark(ctx context.Context, userID, messageID uuid.UUID) error
	RemoveBookmark(ctx context.Context, userID, messageID uuid.UUID) error
	GetBookmarks(ctx context.Context, userID uuid.UUID) ([]*models.Message, error)
}

type BlockStorage interface {
	BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error
	UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error
	GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	IsUserBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error)
}

type MentionStorage interface {
	AddMention(ctx context.Context, messageID, userID uuid.UUID) error
	GetUserMentions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Mention, error)
}

type ReactionStorage interface {
	AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error
	RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error
	GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]*models.Reaction, error)
}

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
func (s *StubStorage) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status, customStatus string) error {
	return nil
}
func (s *StubStorage) UsersShareChat(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	return false, nil
}
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
func (s *StubStorage) UpdateChat(ctx context.Context, chat *models.Chat) error            { return nil }
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
func (s *StubStorage) MuteChat(ctx context.Context, chatID, userID uuid.UUID) error       { return nil }
func (s *StubStorage) UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error     { return nil }
func (s *StubStorage) PinChat(ctx context.Context, chatID, userID uuid.UUID) error        { return nil }
func (s *StubStorage) UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error      { return nil }
func (s *StubStorage) ArchiveChat(ctx context.Context, chatID, userID uuid.UUID) error    { return nil }
func (s *StubStorage) UnarchiveChat(ctx context.Context, chatID, userID uuid.UUID) error  { return nil }
func (s *StubStorage) SoftDeleteChat(ctx context.Context, chatID, userID uuid.UUID) error { return nil }
func (s *StubStorage) ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) error {
	return nil
}
func (s *StubStorage) CreateMessage(ctx context.Context, msg *models.Message) error { return nil }
func (s *StubStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) GetMessagesByChatForUser(ctx context.Context, chatID, userID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) GetAllMessagesByChat(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) SearchMessages(ctx context.Context, chatID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s *StubStorage) SearchAllMessages(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
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
func (s *StubStorage) MarkMessagesAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	return nil
}
func (s *StubStorage) GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (s *StubStorage) GetMessagesReadStatus(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	return make(map[uuid.UUID][]uuid.UUID), nil
}
func (s *StubStorage) PinMessage(ctx context.Context, chatID, messageID, userID uuid.UUID) error {
	return nil
}
func (s *StubStorage) UnpinMessage(ctx context.Context, chatID, messageID uuid.UUID) error {
	return nil
}
func (s *StubStorage) GetPinnedMessages(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
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
func (s *StubStorage) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	return nil
}
func (s *StubStorage) GetAuditLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	return nil, nil
}
func (s *StubStorage) GetAllAuditLogs(ctx context.Context, limit, offset int) ([]*models.AuditLog, error) {
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

// EncryptionStorage stubs
func (s *StubStorage) GetUserPublicKey(ctx context.Context, userID uuid.UUID) (*models.EncryptionKey, error) {
	return nil, nil
}
func (s *StubStorage) GetUsersPublicKeys(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*models.EncryptionKey, error) {
	return make(map[uuid.UUID]*models.EncryptionKey), nil
}
func (s *StubStorage) SaveUserPublicKey(ctx context.Context, key *models.EncryptionKey) error {
	return nil
}

// BookmarkStorage stubs
func (s *StubStorage) AddBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	return nil
}
func (s *StubStorage) RemoveBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	return nil
}
func (s *StubStorage) GetBookmarks(ctx context.Context, userID uuid.UUID) ([]*models.Message, error) {
	return nil, nil
}

// BlockStorage stubs
func (s *StubStorage) BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	return nil
}
func (s *StubStorage) UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	return nil
}
func (s *StubStorage) GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (s *StubStorage) IsUserBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	return false, nil
}

func (s *StubStorage) AddMention(ctx context.Context, messageID, mentionedUserID uuid.UUID) error {
	return nil
}
func (s *StubStorage) GetUserMentions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Mention, error) {
	return nil, nil
}
func (s *StubStorage) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return nil
}
func (s *StubStorage) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return nil
}
func (s *StubStorage) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]*models.Reaction, error) {
	return nil, nil
}

type testEjabberd struct{ err error }
