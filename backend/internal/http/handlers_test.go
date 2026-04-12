package http

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"

	"corp-messenger/backend/internal/models"

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
func (s testStorage) GetUsers(ctx context.Context) ([]*models.User, error) {
	return nil, nil
}
func (s testStorage) UpdateUser(ctx context.Context, user *models.User) error { return nil }
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
func (s testStorage) AddChatMember(ctx context.Context, member *models.ChatMember) error { return nil }
func (s testStorage) GetChatMembers(ctx context.Context, chatID uuid.UUID) ([]*models.ChatMember, error) {
	return nil, nil
}
func (s testStorage) CreateMessage(ctx context.Context, msg *models.Message) error { return nil }
func (s testStorage) GetMessageByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	return nil, nil
}
func (s testStorage) GetMessagesByChat(ctx context.Context, chatID uuid.UUID, limit, offset int) ([]*models.Message, error) {
	return nil, nil
}
func (s testStorage) CreateSession(ctx context.Context, session *models.Session) error { return nil }
func (s testStorage) GetSessionByToken(ctx context.Context, token string) (*models.Session, error) {
	return nil, nil
}
func (s testStorage) DeleteSession(ctx context.Context, token string) error { return nil }

type testEjabberd struct{ err error }

func (e testEjabberd) Ready(context.Context) error { return e.err }
func (e testEjabberd) Close() error                { return nil }
func TestHealth(t *testing.T) {
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{}, "test-secret", []string{"http://localhost:3000"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
func TestReady(t *testing.T) {
	h := NewHandler(slog.Default(), testStorage{}, testEjabberd{}, "test-secret", []string{"http://localhost:3000"})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ready", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
