package ejabberd

import (
	"context"

	"github.com/google/uuid"
)

type Client interface {
	Ready(ctx context.Context) error
	Close() error
	GetHost() string
	CreateUser(userID uuid.UUID, password string) error
	DeleteUser(userID uuid.UUID) error
	UpdateUserPassword(userID uuid.UUID, password string) error
	CreateChatRoom(roomID uuid.UUID, title string, ownerID uuid.UUID) error
	UpdateChatRoomTitle(roomID uuid.UUID, title string) error
	DestroyChatRoom(roomID uuid.UUID) error
	AddMemberToRoom(roomID uuid.UUID, userID uuid.UUID, role string) error
	RemoveMemberFromRoom(roomID uuid.UUID, userID uuid.UUID) error
	SendMessage(from, to, body string) error
	SendRoomMessage(roomID uuid.UUID, from, body string) error
	GetRoomMessages(roomID uuid.UUID, limit int) ([]map[string]interface{}, error)
}

type Stub struct{}

func NewStub() *Stub                                               { return &Stub{} }
func (s *Stub) Ready(context.Context) error                        { return nil }
func (s *Stub) Close() error                                       { return nil }
func (s *Stub) GetHost() string                                    { return "localhost" }
func (s *Stub) CreateUser(uuid.UUID, string) error                 { return nil }
func (s *Stub) DeleteUser(uuid.UUID) error                         { return nil }
func (s *Stub) UpdateUserPassword(uuid.UUID, string) error         { return nil }
func (s *Stub) CreateChatRoom(uuid.UUID, string, uuid.UUID) error  { return nil }
func (s *Stub) UpdateChatRoomTitle(uuid.UUID, string) error        { return nil }
func (s *Stub) DestroyChatRoom(uuid.UUID) error                    { return nil }
func (s *Stub) AddMemberToRoom(uuid.UUID, uuid.UUID, string) error { return nil }
func (s *Stub) RemoveMemberFromRoom(uuid.UUID, uuid.UUID) error    { return nil }
func (s *Stub) SendMessage(string, string, string) error           { return nil }
func (s *Stub) SendRoomMessage(uuid.UUID, string, string) error    { return nil }
func (s *Stub) GetRoomMessages(uuid.UUID, int) ([]map[string]interface{}, error) {
	return nil, nil
}
