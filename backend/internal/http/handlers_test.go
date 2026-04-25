package http

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/websocket"

	"github.com/google/uuid"
)

type testStorage struct{ err error }

func (s testStorage) Ready(context.Context) error { return s.err }
func (s testStorage) Close() error                { return nil }
func (s testStorage) CreateUser(ctx context.Context, user *models.User, passwordHash string) error {
	return nil
}
func (s testStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	return nil, "", nil
}
func (s testStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return nil, nil
}
func (s testStorage) GetUsers(ctx context.Context, search string, excludeUserID string, limit int) ([]*models.User, error) {
	return nil, nil
}
func (s testStorage) UpdateUser(ctx context.Context, user *models.User) error { return nil }
func (s testStorage) UpdateUserStatus(ctx context.Context, userID uuid.UUID, status, customStatus string) error {
	return nil
}
func (s testStorage) UsersShareChat(ctx context.Context, userID1, userID2 uuid.UUID) (bool, error) {
	return false, nil
}
func (s testStorage) CreateChat(ctx context.Context, chat *models.Chat) error { return nil }
func (s testStorage) CreateChatWithMembers(ctx context.Context, chat *models.Chat, members []*models.ChatMember) error {
	return nil
}
func (s testStorage) GetChatByID(ctx context.Context, id uuid.UUID) (*models.Chat, error) {
	return nil, nil
}
func (s testStorage) GetUserChats(ctx context.Context, userID uuid.UUID) ([]*models.Chat, error) {
	return nil, nil
}
func (s testStorage) UpdateChat(ctx context.Context, chat *models.Chat) error {
	return nil
}
func (s testStorage) DeleteChat(ctx context.Context, chatID uuid.UUID) error             { return nil }
func (s testStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error { return nil }
func (s testStorage) RemoveChatMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	return false, nil
}
func (s testStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	return nil, nil
}
func (s testStorage) GetChatMembersWithUsers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	return nil, nil
}
func (s testStorage) MuteChat(ctx context.Context, chatID, userID uuid.UUID) error       { return nil }
func (s testStorage) UnmuteChat(ctx context.Context, chatID, userID uuid.UUID) error     { return nil }
func (s testStorage) PinChat(ctx context.Context, chatID, userID uuid.UUID) error        { return nil }
func (s testStorage) UnpinChat(ctx context.Context, chatID, userID uuid.UUID) error      { return nil }
func (s testStorage) ArchiveChat(ctx context.Context, chatID, userID uuid.UUID) error    { return nil }
func (s testStorage) UnarchiveChat(ctx context.Context, chatID, userID uuid.UUID) error  { return nil }
func (s testStorage) SoftDeleteChat(ctx context.Context, chatID, userID uuid.UUID) error { return nil }
func (s testStorage) ClearChatHistory(ctx context.Context, chatID, userID uuid.UUID) error {
	return nil
}
func (s testStorage) CreateMessage(ctx context.Context, msg *models.Message) error { return nil }
func (s testStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	return nil, nil
}
func (s testStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) GetMessagesByChatForUser(ctx context.Context, chatID, userID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) GetAllMessagesByChat(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) SearchMessages(ctx context.Context, chatID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) SearchAllMessages(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) UpdateMessage(ctx context.Context, msg *models.Message) error { return nil }
func (s testStorage) UpdateMessageFileURL(ctx context.Context, messageID uuid.UUID, fileURL string) error {
	return nil
}
func (s testStorage) DeleteMessage(ctx context.Context, id uuid.UUID) error { return nil }
func (s testStorage) MarkMessageAsRead(ctx context.Context, messageID, userID uuid.UUID) error {
	return nil
}
func (s testStorage) MarkMessagesAsRead(ctx context.Context, messageIDs []uuid.UUID, userID uuid.UUID) error {
	return nil
}
func (s testStorage) GetMessageReadStatus(ctx context.Context, messageID uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (s testStorage) GetMessagesReadStatus(ctx context.Context, messageIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	return make(map[uuid.UUID][]uuid.UUID), nil
}
func (s testStorage) PinMessage(ctx context.Context, chatID, messageID, userID uuid.UUID) error {
	return nil
}
func (s testStorage) UnpinMessage(ctx context.Context, chatID, messageID uuid.UUID) error {
	return nil
}
func (s testStorage) GetPinnedMessages(ctx context.Context, chatID uuid.UUID) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) CreateSession(ctx context.Context, session *models.Session) error { return nil }
func (s testStorage) GetSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	return nil, nil
}
func (s testStorage) DeleteSession(ctx context.Context, token string) error { return nil }
func (s testStorage) DeleteOldSessionsForUser(ctx context.Context, userID uuid.UUID, keep int) error {
	return nil
}
func (s testStorage) CreateFile(ctx context.Context, file *models.File) error { return nil }
func (s testStorage) GetFileByID(ctx context.Context, id uuid.UUID) (*models.File, error) {
	return nil, nil
}
func (s testStorage) GetFilesByMessage(ctx context.Context, messageID uuid.UUID) ([]*models.File, error) {
	return nil, nil
}
func (s testStorage) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	return nil
}
func (s testStorage) GetAuditLogs(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.AuditLog, error) {
	return nil, nil
}
func (s testStorage) GetAllAuditLogs(ctx context.Context, limit, offset int) ([]*models.AuditLog, error) {
	return nil, nil
}
func (s testStorage) CreateDeviceToken(ctx context.Context, token *models.DeviceToken) error {
	return nil
}
func (s testStorage) GetDeviceTokens(ctx context.Context, userID uuid.UUID) ([]*models.DeviceToken, error) {
	return nil, nil
}
func (s testStorage) DeleteDeviceToken(ctx context.Context, token string) error { return nil }
func (s testStorage) GetNotificationSettings(ctx context.Context, userID uuid.UUID) (*models.NotificationSettings, error) {
	return nil, nil
}
func (s testStorage) UpdateNotificationSettings(ctx context.Context, settings *models.NotificationSettings) error {
	return nil
}
func (s testStorage) GetChatMember(ctx context.Context, chatID, userID uuid.UUID) (*models.ChatMember, error) {
	return nil, nil
}
func (s testStorage) GetTotalUnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return 0, nil
}
func (s testStorage) GetUserLastOnline(ctx context.Context, userID uuid.UUID) (time.Time, error) {
	return time.Time{}, nil
}
func (s testStorage) UpdateUserLastOnline(ctx context.Context, userID uuid.UUID) error { return nil }
func (s testStorage) GetUserPublicKey(ctx context.Context, userID uuid.UUID) (*models.EncryptionKey, error) {
	return nil, nil
}
func (s testStorage) GetUsersPublicKeys(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]*models.EncryptionKey, error) {
	return make(map[uuid.UUID]*models.EncryptionKey), nil
}
func (s testStorage) SaveUserPublicKey(ctx context.Context, key *models.EncryptionKey) error {
	return nil
}
func (s testStorage) AddBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	return nil
}
func (s testStorage) RemoveBookmark(ctx context.Context, userID, messageID uuid.UUID) error {
	return nil
}
func (s testStorage) GetBookmarks(ctx context.Context, userID uuid.UUID) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) BlockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	return nil
}
func (s testStorage) UnblockUser(ctx context.Context, blockerID, blockedID uuid.UUID) error {
	return nil
}
func (s testStorage) GetBlockedUsers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (s testStorage) IsUserBlocked(ctx context.Context, blockerID, blockedID uuid.UUID) (bool, error) {
	return false, nil
}
func (s testStorage) AddMention(ctx context.Context, messageID, mentionedUserID uuid.UUID) error {
	return nil
}
func (s testStorage) GetUserMentions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Mention, error) {
	return nil, nil
}
func (s testStorage) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return nil
}
func (s testStorage) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	return nil
}
func (s testStorage) GetMessageReactions(ctx context.Context, messageID uuid.UUID) ([]*models.Reaction, error) {
	return nil, nil
}

type testEjabberd struct{ err error }

func (e testEjabberd) Ready(context.Context) error { return e.err }
func (e testEjabberd) Close() error                { return nil }
func TestHealth(t *testing.T) {
	hub := websocket.NewHub(testStorage{})
	go hub.Run()
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{}, "test-secret", []string{"http://localhost:3000"}, 168*time.Hour, hub, nil, "http://localhost:8080", 20, 60, 10000)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
func TestReady(t *testing.T) {
	hub := websocket.NewHub(testStorage{})
	go hub.Run()
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{}, "test-secret", []string{"http://localhost:3000"}, 168*time.Hour, hub, nil, "http://localhost:8080", 20, 60, 10000)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ready", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
