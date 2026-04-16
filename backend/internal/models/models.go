package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Email      string    `json:"email" db:"email"`
	Phone      string    `json:"phone,omitempty" db:"phone"`
	FirstName  string    `json:"first_name" db:"first_name"`
	LastName   string    `json:"last_name" db:"last_name"`
	MiddleName string    `json:"middle_name,omitempty" db:"middle_name"`
	Avatar     string    `json:"avatar,omitempty" db:"avatar"`
	Status     string    `json:"status" db:"status"`
	Role       string    `json:"role" db:"role"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type UserCredentials struct {
	UserID       uuid.UUID `json:"user_id" db:"user_id"`
	PasswordHash string    `json:"-" db:"password_hash"`
}

type Chat struct {
	ID          uuid.UUID     `json:"id" db:"id"`
	Type        string        `json:"type" db:"type"`
	Title       string        `json:"title,omitempty" db:"title"`
	Description string        `json:"description,omitempty" db:"description"`
	Avatar      string        `json:"avatar,omitempty" db:"avatar"`
	CreatorID   uuid.UUID     `json:"creator_id" db:"creator_id"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
	Members     []*ChatMember `json:"members,omitempty" db:"-"`
}

type ChatMember struct {
	ChatID     uuid.UUID  `json:"chat_id" db:"chat_id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	Role       string     `json:"role" db:"role"`
	JoinedAt   time.Time  `json:"joined_at" db:"joined_at"`
	LastReadAt *time.Time `json:"last_read_at,omitempty" db:"last_read_at"`
	User       *User      `json:"user,omitempty" db:"-"`
}

type Message struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	ChatID    uuid.UUID   `json:"chat_id" db:"chat_id"`
	SenderID  uuid.UUID   `json:"sender_id" db:"sender_id"`
	Type      string      `json:"type" db:"type"`
	Content   string      `json:"content" db:"content"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
	ReplyTo   *uuid.UUID  `json:"reply_to,omitempty" db:"reply_to"`
	ReadBy    []uuid.UUID `json:"read_by,omitempty" db:"-"`
}

type File struct {
	ID         uuid.UUID `json:"id" db:"id"`
	MessageID  uuid.UUID `json:"message_id" db:"message_id"`
	Name       string    `json:"name" db:"name"`
	Size       int64     `json:"size" db:"size"`
	MimeType   string    `json:"mime_type" db:"mime_type"`
	URL        string    `json:"url" db:"url"`
	UploadedAt time.Time `json:"uploaded_at" db:"uploaded_at"`
}

type Session struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Token      string    `json:"token" db:"token"`
	DeviceInfo string    `json:"device_info,omitempty" db:"device_info"`
	IP         string    `json:"ip,omitempty" db:"ip"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
}

const (
	UserStatusActive   = "active"
	UserStatusInactive = "inactive"
	UserStatusBlocked  = "blocked"

	UserRoleUser      = "user"
	UserRoleAdmin     = "admin"
	UserRoleModerator = "moderator"

	ChatTypeDirect  = "direct"
	ChatTypeGroup   = "group"
	ChatTypeChannel = "channel"

	ChatRoleOwner  = "owner"
	ChatRoleAdmin  = "admin"
	ChatRoleMember = "member"

	MessageTypeText  = "text"
	MessageTypeImage = "image"
	MessageTypeFile  = "file"
	MessageTypeVoice = "voice"
	MessageTypeVideo = "video"
)
