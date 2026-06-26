package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/notifications"
	"corp-messenger/backend/internal/storage"
	"corp-messenger/backend/internal/websocket"
	"corp-messenger/backend/internal/xmppsync"

	"github.com/google/uuid"
)

type SendMessageInput struct {
	ChatID           uuid.UUID
	SenderID         uuid.UUID
	Type             string
	Content          string
	EncryptedContent string
	EncryptedKeys    map[string]string
	IV               string
	Location         *models.MessageLocation
	ScheduledAt      *time.Time
	Duration         *float64
	ReplyTo          string
}

type MessageService interface {
	SendMessage(ctx context.Context, input SendMessageInput, members []*models.ChatMember) (*models.Message, error)
}

type messageService struct {
	logger          *slog.Logger
	storage         storage.Storage
	hub             *websocket.Hub
	notificationSvc *notifications.Service
	syncService     *xmppsync.SyncService
}

func NewMessageService(
	logger *slog.Logger,
	stg storage.Storage,
	hub *websocket.Hub,
	notifSvc *notifications.Service,
	syncSvc *xmppsync.SyncService,
) MessageService {
	return &messageService{
		logger:          logger,
		storage:         stg,
		hub:             hub,
		notificationSvc: notifSvc,
		syncService:     syncSvc,
	}
}

var (
	ErrBlocked         = errors.New("you are blocked by a member of this chat")
	ErrInvalidType     = errors.New("invalid message type")
	ErrContentTooLarge = errors.New("message content exceeds maximum size")
	ErrInvalidLocation = errors.New("invalid location parameters")
	ErrInvalidReply    = errors.New("invalid reply-to message")
)

func (s *messageService) SendMessage(ctx context.Context, input SendMessageInput, members []*models.ChatMember) (*models.Message, error) {
	// Check if sender is blocked by any chat member
	for _, m := range members {
		if m.UserID != input.SenderID {
			blocked, err := s.storage.IsUserBlocked(ctx, m.UserID, input.SenderID)
			if err != nil {
				return nil, fmt.Errorf("failed to check blocking status: %w", err)
			}
			if blocked {
				return nil, ErrBlocked
			}
		}
	}

	msgType := input.Type
	if msgType == "" {
		msgType = models.MessageTypeText
	}

	msg := &models.Message{
		ChatID:           input.ChatID,
		SenderID:         input.SenderID,
		Type:             msgType,
		Content:          input.Content,
		EncryptedContent: input.EncryptedContent,
		EncryptedKeys:    input.EncryptedKeys,
		Location:         input.Location,
		ScheduledAt:      input.ScheduledAt,
		Duration:         input.Duration,
	}

	if input.ReplyTo != "" {
		replyToID, err := uuid.Parse(input.ReplyTo)
		if err != nil {
			return nil, ErrInvalidReply
		}
		replyMsg, err := s.storage.GetMessageByID(ctx, replyToID)
		if err != nil || replyMsg == nil || replyMsg.ChatID != input.ChatID {
			return nil, ErrInvalidReply
		}
		msg.ReplyTo = &replyToID
		msg.ReplyToContent = &replyMsg.Content

		replySender, err := s.storage.GetUserByID(ctx, replyMsg.SenderID)
		if err == nil && replySender != nil {
			name := replySender.FirstName
			if replySender.LastName != "" {
				name += " " + replySender.LastName
			}
			msg.ReplyToSenderName = &name
		}
	}

	if err := s.storage.CreateMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to create message in db: %w", err)
	}

	// XMPP Sync
	if s.syncService != nil {
		sender, err := s.storage.GetUserByID(ctx, input.SenderID)
		if err == nil && sender != nil && sender.Username != nil {
			go func() {
				_ = s.syncService.SyncMessageToXMPP(context.Background(), msg, *sender.Username)
			}()
		}
	}

	// Notifications
	if s.notificationSvc != nil {
		sender, err := s.storage.GetUserByID(ctx, input.SenderID)
		senderName := "Unknown user"
		if err == nil && sender != nil {
			senderName = sender.FirstName
			if sender.LastName != "" {
				senderName += " " + sender.LastName
			}
		}

		go func() {
			s.logger.Info("[Notifications] goroutine started", "chat_id", msg.ChatID, "sender_id", input.SenderID, "member_count", len(members))
			for _, member := range members {
				if member.UserID == input.SenderID {
					continue
				}

				content := msg.Content
				if len(content) > 100 {
					content = content[:100] + "..."
				}

				payload := &notifications.NotificationPayload{
					Title:  senderName,
					Body:   content,
					ChatID: msg.ChatID,
					Type:   "message",
					Data: map[string]string{
						"chat_id":    msg.ChatID.String(),
						"message_id": msg.ID.String(),
						"type":       "new_message",
					},
				}

				s.logger.Info("[Notifications] calling SendToUser", "user_id", member.UserID)
				if err := s.notificationSvc.SendToUser(context.Background(), member.UserID, payload); err != nil {
					s.logger.Error("[Notifications] SendToUser failed", "error", err, "user_id", member.UserID)
				}
			}
		}()
	}

	// WebSocket broadcast
	if s.hub != nil {
		s.hub.BroadcastToChat(&websocket.BroadcastMessage{
			Type:   websocket.EventNewMessage,
			ChatID: input.ChatID,
			Payload: websocket.MessagePayload{
				ID:                msg.ID,
				ChatID:            msg.ChatID,
				SenderID:          msg.SenderID,
				Type:              msg.Type,
				Content:           msg.Content,
				CreatedAt:         msg.CreatedAt,
				ReplyTo:           msg.ReplyTo,
				ReplyToContent:    msg.ReplyToContent,
				ReplyToSenderName: msg.ReplyToSenderName,
			},
			ExcludeSender: &input.SenderID,
		})

		for _, member := range members {
			s.hub.BroadcastToUser(member.UserID, &websocket.BroadcastMessage{
				Type:   "chat_updated",
				ChatID: input.ChatID,
				Payload: map[string]interface{}{
					"chat_id": input.ChatID,
					"last_message": map[string]interface{}{
						"id":         msg.ID,
						"content":    msg.Content,
						"sender_id":  msg.SenderID,
						"type":       msg.Type,
						"created_at": msg.CreatedAt,
					},
					"updated_at": msg.CreatedAt,
				},
			})
		}
	}

	return msg, nil
}
