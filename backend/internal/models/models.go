package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Email        string     `json:"email" db:"email"`
	Phone        string     `json:"phone,omitempty" db:"phone"`
	FirstName    string     `json:"first_name" db:"first_name"`
	LastName     string     `json:"last_name" db:"last_name"`
	MiddleName   string     `json:"middle_name,omitempty" db:"middle_name"`
	Avatar       string     `json:"avatar,omitempty" db:"avatar"`
	Status       string     `json:"status" db:"status"`
	CustomStatus *string    `json:"custom_status,omitempty" db:"custom_status"`
	Role         string     `json:"role" db:"role"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	LastOnline   *time.Time `json:"last_online,omitempty" db:"last_online"`
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
	Muted      bool       `json:"muted" db:"muted"`
	Pinned     bool       `json:"pinned" db:"pinned"`
	Archived   bool       `json:"archived" db:"archived"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
	User       *User      `json:"user,omitempty" db:"-"`
}

type Message struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	ChatID      uuid.UUID   `json:"chat_id" db:"chat_id"`
	SenderID    uuid.UUID   `json:"sender_id" db:"sender_id"`
	Type        string      `json:"type" db:"type"`
	Content     string      `json:"content" db:"content"`
	FileURL     *string     `json:"file_url,omitempty" db:"file_url"`
	ReplyTo     *uuid.UUID  `json:"reply_to,omitempty" db:"reply_to"`
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" db:"updated_at"`
	ReadBy      []uuid.UUID `json:"read_by,omitempty" db:"-"`
	Duration    *float64    `json:"duration,omitempty" db:"duration"`         // For audio messages in seconds
	ScheduledAt *time.Time  `json:"scheduled_at,omitempty" db:"scheduled_at"` // For scheduled messages
	ThreadID    *uuid.UUID  `json:"thread_id,omitempty" db:"thread_id"`       // For message threads
	// E2E Encryption fields
	EncryptedContent string            `json:"encrypted_content,omitempty" db:"encrypted_content"`
	EncryptionKeyID  *uuid.UUID        `json:"encryption_key_id,omitempty" db:"encryption_key_id"`
	EncryptedKeys    map[string]string `json:"encrypted_keys,omitempty" db:"encrypted_keys"` // user_id -> encrypted session key
	Location         *MessageLocation  `json:"location,omitempty" db:"-"`
}

type MessageLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address,omitempty"`
}

type EncryptionKey struct {
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	PublicKey  string    `json:"public_key" db:"public_key"`             // X25519 public key (base64)
	PrivateKey string    `json:"private_key,omitempty" db:"private_key"` // Encrypted private key (optional, for backup)
	KeyVersion int       `json:"key_version" db:"key_version"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type Mention struct {
	ID              uuid.UUID `json:"id" db:"id"`
	MessageID       uuid.UUID `json:"message_id" db:"message_id"`
	MentionedUserID uuid.UUID `json:"mentioned_user_id" db:"mentioned_user_id"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
}

type File struct {
	ID           uuid.UUID `json:"id" db:"id"`
	MessageID    uuid.UUID `json:"message_id" db:"message_id"`
	Name         string    `json:"name" db:"name"`
	Size         int64     `json:"size" db:"size"`
	MimeType     string    `json:"mime_type" db:"mime_type"`
	URL          string    `json:"url" db:"url"`
	ThumbnailURL *string   `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	UploadedAt   time.Time `json:"uploaded_at" db:"uploaded_at"`
}

type DeviceToken struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Token      string    `json:"token" db:"token"`
	Platform   string    `json:"platform" db:"platform"`
	DeviceName string    `json:"device_name,omitempty" db:"device_name"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	LastUsedAt time.Time `json:"last_used_at" db:"last_used_at"`
}

type Session struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Token      string    `json:"token" db:"token"`
	DeviceInfo string    `json:"device_info" db:"device_info"`
	IP         string    `json:"ip" db:"ip"`
	ExpiresAt  time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type Reaction struct {
	ID        uuid.UUID `json:"id" db:"id"`
	MessageID uuid.UUID `json:"message_id" db:"message_id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Emoji     string    `json:"emoji" db:"emoji"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type NotificationSettings struct {
	UserID            uuid.UUID `json:"user_id" db:"user_id"`
	PushEnabled       bool      `json:"push_enabled" db:"push_enabled"`
	EmailEnabled      bool      `json:"email_enabled" db:"email_enabled"`
	Email             string    `json:"email,omitempty" db:"email"`
	QuietHoursStart   *string   `json:"quiet_hours_start,omitempty" db:"quiet_hours_start"`
	QuietHoursEnd     *string   `json:"quiet_hours_end,omitempty" db:"quiet_hours_end"`
	QuietHoursEnabled bool      `json:"quiet_hours_enabled" db:"quiet_hours_enabled"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type UnreadCount struct {
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	ChatID    uuid.UUID `json:"chat_id" db:"chat_id"`
	Count     int       `json:"count" db:"count"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type AuditLog struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Action     string    `json:"action" db:"action"`             // e.g., "message_sent", "user_blocked", "chat_created"
	Resource   string    `json:"resource" db:"resource"`         // e.g., "message", "user", "chat"
	ResourceID string    `json:"resource_id" db:"resource_id"`   // ID of the affected resource
	Details    string    `json:"details,omitempty" db:"details"` // Additional JSON details
	IPAddress  string    `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent  string    `json:"user_agent,omitempty" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
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
