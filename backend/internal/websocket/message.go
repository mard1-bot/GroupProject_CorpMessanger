package websocket

import (
	"time"

	"github.com/google/uuid"
)

// MessagePayload represents the data structure of a chat message.
type MessagePayload struct {
	ID                  uuid.UUID  `json:"id"`
	ChatID              uuid.UUID  `json:"chat_id"`
	SenderID            uuid.UUID  `json:"sender_id"`
	Type                string     `json:"type"`
	Content             string     `json:"content"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
	ReplyTo             *uuid.UUID `json:"reply_to,omitempty"`
	ReplyToContent      *string    `json:"reply_to_content,omitempty"`
	ReplyToSenderName   *string    `json:"reply_to_sender_name,omitempty"`
	ForwardedFrom       *uuid.UUID `json:"forwarded_from,omitempty"`
	ForwardedSenderName *string    `json:"forwarded_sender_name,omitempty"`
}

// TypingPayload represents a typing indicator.
type TypingPayload struct {
	ChatID uuid.UUID `json:"chat_id"`
	UserID uuid.UUID `json:"user_id"`
}

// ReadReceiptPayload represents a read receipt.
type ReadReceiptPayload struct {
	ChatID    uuid.UUID `json:"chat_id"`
	UserID    uuid.UUID `json:"user_id"`
	MessageID uuid.UUID `json:"message_id"`
	ReadAt    time.Time `json:"read_at"`
}

// PresencePayload represents user online status change.
type PresencePayload struct {
	UserID   uuid.UUID `json:"user_id"`
	Status   string    `json:"status"` // online, offline
	LastSeen time.Time `json:"last_seen,omitempty"`
}

// Event types for WebSocket messages.
const (
	EventNewMessage     = "new_message"
	EventMessageUpdated = "message_updated"
	EventMessageDeleted = "message_deleted"
	EventTyping         = "typing"
	EventReadReceipt    = "read_receipt"
	EventPresence       = "presence"
	EventUserJoined     = "user_joined"
	EventUserLeft       = "user_left"
	EventChatCreated    = "chat_created"
	EventChatDeleted    = "chat_deleted"
	EventChatUpdated    = "chat_updated"
	EventError          = "error"
)
