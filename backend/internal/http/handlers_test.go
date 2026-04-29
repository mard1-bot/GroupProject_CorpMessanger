package http

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"corp-messenger/backend/internal/livekit"
	"corp-messenger/backend/internal/websocket"

	"corp-messenger/backend/internal/models"

	"github.com/google/uuid"
)

type testStorage struct{ err error }

func (s testStorage) Ready(context.Context) error { return s.err }
func (s testStorage) Close() error                { return nil }
func (s testStorage) CreateUser(ctx context.Context, user *models.User, passwordHash string) error {
	return nil
}
func (s testStorage) UpdateUserPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	return nil
}
func (s testStorage) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	return nil
}
func (s testStorage) GetUserByEmail(ctx context.Context, email string) (*models.User, string, error) {
	return nil, "", nil
}
func (s testStorage) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return nil, nil
}
func (s testStorage) GetUserByIDWithPassword(ctx context.Context, id uuid.UUID) (*models.User, string, error) {
	return nil, "", nil
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
func (s testStorage) MarkChatForReconciliation(ctx context.Context, chatID uuid.UUID, reason string, details map[string]interface{}) error {
	return nil
}
func (s testStorage) GetChatsNeedingReconciliation(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s testStorage) ResolveReconciliation(ctx context.Context, chatID uuid.UUID, reason string) error {
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
func (s testStorage) CreateWebPushSubscription(ctx context.Context, sub *models.WebPushSubscription) error {
	return nil
}
func (s testStorage) GetWebPushSubscriptions(ctx context.Context, userID uuid.UUID) ([]*models.WebPushSubscription, error) {
	return nil, nil
}
func (s testStorage) DeleteWebPushSubscription(ctx context.Context, userID uuid.UUID, endpoint string) error {
	return nil
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
func (s testStorage) GetUnsyncedMessages(ctx context.Context, limit int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) MarkMessageAsSyncedToXMPP(ctx context.Context, messageID uuid.UUID, xmppMessageID string) error {
	return nil
}
func (s testStorage) GetMessageByXMPPID(ctx context.Context, xmppMessageID string) (*models.Message, error) {
	return nil, nil
}
func (s testStorage) GetScheduledMessages(ctx context.Context) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) GetThreadMessages(ctx context.Context, threadID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) AddToXMPPSyncDeadLetter(ctx context.Context, messageID, chatID uuid.UUID, errorMessage string) error {
	return nil
}
func (s testStorage) GetXMPPSyncDeadLetters(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s testStorage) ResolveXMPPSyncDeadLetter(ctx context.Context, messageID uuid.UUID) error {
	return nil
}
func (s testStorage) AddToSchedulerFailures(ctx context.Context, messageID, chatID uuid.UUID, errorMessage string) error {
	return nil
}
func (s testStorage) GetSchedulerFailures(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	return nil, nil
}
func (s testStorage) ResolveSchedulerFailure(ctx context.Context, messageID uuid.UUID) error {
	return nil
}
func (s testStorage) RecordLoginAttempt(ctx context.Context, email, ipAddress string, userID *uuid.UUID, success bool) error {
	return nil
}
func (s testStorage) GetFailedLoginAttempts(ctx context.Context, email string, since time.Time) (int, error) {
	return 0, nil
}
func (s testStorage) CleanupOldLoginAttempts(ctx context.Context, olderThan time.Time) error {
	return nil
}

type testEjabberd struct{ err error }

func (e testEjabberd) Ready(context.Context) error                        { return e.err }
func (e testEjabberd) Close() error                                       { return nil }
func (e testEjabberd) GetHost() string                                    { return "localhost" }
func (e testEjabberd) CreateUser(uuid.UUID, string) error                 { return nil }
func (e testEjabberd) DeleteUser(uuid.UUID) error                         { return nil }
func (e testEjabberd) UpdateUserPassword(uuid.UUID, string) error         { return nil }
func (e testEjabberd) CreateChatRoom(uuid.UUID, string, uuid.UUID) error  { return nil }
func (e testEjabberd) UpdateChatRoomTitle(uuid.UUID, string) error        { return nil }
func (e testEjabberd) DestroyChatRoom(uuid.UUID) error                    { return nil }
func (e testEjabberd) AddMemberToRoom(uuid.UUID, uuid.UUID, string) error { return nil }
func (e testEjabberd) RemoveMemberFromRoom(uuid.UUID, uuid.UUID) error    { return nil }
func (e testEjabberd) SendMessage(string, string, string) error           { return nil }
func (e testEjabberd) SendRoomMessage(uuid.UUID, string, string) error    { return nil }
func (e testEjabberd) GetRoomMessages(uuid.UUID, int) ([]map[string]interface{}, error) {
	return nil, nil
}
func TestHealth(t *testing.T) {
	hub := websocket.NewHub(testStorage{}, livekit.NewService("", "", ""), "")
	go hub.Run(context.Background())
	lk := livekit.NewService("", "", "")
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{}, "test-secret", []string{"http://localhost:3000"}, 168*time.Hour, hub, nil, "http://localhost:8080", 20, 60, 10000, nil, "", "", "", lk)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
func TestReady(t *testing.T) {
	hub := websocket.NewHub(testStorage{}, livekit.NewService("", "", ""), "")
	go hub.Run(context.Background())
	lk := livekit.NewService("", "", "")
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{}, "test-secret", []string{"http://localhost:3000"}, 168*time.Hour, hub, nil, "http://localhost:8080", 20, 60, 10000, nil, "", "", "", lk)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ready", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"Normal string", "hello world", "hello world"},
		{"With null bytes", "hello\x00world", "helloworld"},
		{"With whitespace", "  hello  ", "hello"},
		{"Long string", string(make([]byte, 15000)), ""}, // Should be truncated to 10000
		{"Empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeInput(tt.input)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("sanitizeInput() = %v, want to contain %v", result, tt.contains)
			}
			if tt.contains == "" && len(result) > 0 && tt.name == "Long string" {
				t.Errorf("sanitizeInput() for long string should be truncated, got length %d", len(result))
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected string
	}{
		{"X-Forwarded-For", map[string]string{"X-Forwarded-For": "192.168.1.1, 10.0.0.1"}, "192.168.1.1"},
		{"RemoteAddr", map[string]string{}, "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = "127.0.0.1:12345"
			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("getClientIP() = %v, want %v", ip, tt.expected)
			}
		})
	}
}

func TestTrackFailedLoginAttempt(t *testing.T) {
	// Test tracking failed login attempts
	trackFailedLoginAttempt("test@example.com", "192.168.1.1")

	if !isAccountLocked("test@example.com") {
		// Should not be locked after 1 attempt
	}

	// Add more attempts
	for i := 0; i < 5; i++ {
		trackFailedLoginAttempt("test@example.com", "192.168.1.1")
	}

	if !isAccountLocked("test@example.com") {
		t.Error("expected account to be locked after 5 failed attempts")
	}
}
