package notifications

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"

	"github.com/google/uuid"
)

type Service struct {
	logger          *slog.Logger
	storage         storage.Storage
	fcm             *FCMClient
	smtpHost        string
	smtpPort        int
	smtpUser        string
	smtpPass        string
	vapidPublicKey  string
	vapidPrivateKey string
}

func NewService(logger *slog.Logger, storage storage.Storage) (*Service, error) {
	fcm := NewFCMClient()
	if !fcm.IsConfigured() {
		logger.Warn("FCM not configured, push notifications disabled")
	}

	// Email config from env
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := 587
	if port := os.Getenv("SMTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			smtpPort = p
		}
	}
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASSWORD")

	if smtpHost != "" && smtpUser != "" {
		logger.Info("Email notifications configured", "host", smtpHost)
	} else {
		logger.Warn("Email notifications not configured")
	}

	// VAPID keys for web push
	vapidPublicKey := os.Getenv("VAPID_PUBLIC_KEY")
	vapidPrivateKey := os.Getenv("VAPID_PRIVATE_KEY")

	if vapidPublicKey != "" && vapidPrivateKey != "" {
		logger.Info("Web push notifications configured")
	} else {
		logger.Warn("Web push notifications not configured")
	}

	return &Service{
		logger:          logger,
		storage:         storage,
		fcm:             fcm,
		smtpHost:        smtpHost,
		smtpPort:        smtpPort,
		smtpUser:        smtpUser,
		smtpPass:        smtpPass,
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
	}, nil
}

type NotificationPayload struct {
	Title  string
	Body   string
	ChatID uuid.UUID
	Type   string // "message", "mention", "invite"
	Data   map[string]string
}

func (s *Service) SendToUser(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
	// Get user notification settings
	settings, err := s.storage.GetNotificationSettings(ctx, userID)
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}

	// Check quiet hours
	if settings != nil && settings.QuietHoursEnabled && s.isQuietHours(settings) {
		s.logger.Info("skipping notification, quiet hours", "user_id", userID)
		return nil
	}

	// Get user chat member info for muting
	if payload.ChatID != uuid.Nil {
		member, err := s.storage.GetChatMember(ctx, payload.ChatID, userID)
		if err == nil && member != nil && member.Muted {
			s.logger.Info("skipping notification, chat muted", "user_id", userID, "chat_id", payload.ChatID)
			return nil
		}
	}

	// 1. Send Local/Push notification
	if settings == nil || settings.PushEnabled {
		if err := s.sendPush(ctx, userID, payload); err != nil {
			s.logger.Error("failed to send push", "error", err, "user_id", userID)
		}
	}

	// 2. Send Email if enabled and user offline > 5 min
	if (settings == nil || settings.EmailEnabled) && s.isUserOffline(ctx, userID) {
		if err := s.sendEmail(ctx, userID, payload); err != nil {
			s.logger.Error("failed to send email", "error", err, "user_id", userID)
		}
	}

	return nil
}

func (s *Service) sendPush(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
	// Send FCM push notifications
	if s.fcm != nil && s.fcm.IsConfigured() {
		tokens, err := s.storage.GetDeviceTokens(ctx, userID)
		if err != nil {
			s.logger.Error("get device tokens", "error", err, "user_id", userID)
		} else {
			for _, token := range tokens {
				if err := s.fcm.Send(ctx, token.Token, payload.Title, payload.Body, payload.Data); err != nil {
					s.logger.Error("fcm send failed", "error", err, "token", token.Token)
				}
			}
		}
	}

	// Send Web Push notifications
	// TODO: Uncomment after fixing webpush-go import
	// if s.vapidPublicKey != "" && s.vapidPrivateKey != "" {
	// 	if err := s.sendWebPush(ctx, userID, payload); err != nil {
	// 		s.logger.Error("web push send failed", "error", err, "user_id", userID)
	// 	}
	// }

	return nil
}

// func (s *Service) sendWebPush(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
// 	subs, err := s.storage.GetWebPushSubscriptions(ctx, userID)
// 	if err != nil {
// 		return fmt.Errorf("get web push subscriptions: %w", err)
// 	}

// 	for _, sub := range subs {
// 		// Decode base64 keys
// 		key, err := base64.RawURLEncoding.DecodeString(sub.Key)
// 		if err != nil {
// 			s.logger.Error("decode web push key", "error", err)
// 			continue
// 		}

// 		auth, err := base64.RawURLEncoding.DecodeString(sub.Auth)
// 		if err != nil {
// 			s.logger.Error("decode web push auth", "error", err)
// 			continue
// 		}

// 		// Create webpush subscription
// 		webpushSub := &webpush.Subscription{
// 			Endpoint: sub.Endpoint,
// 			Keys: webpush.Keys{
// 				P256DH: string(key),
// 				Auth:   string(auth),
// 			},
// 		}

// 		// Create notification payload
// 		notificationPayload := &webpush.Message{
// 			Title: payload.Title,
// 			Body:  payload.Body,
// 			Data:  payload.Data,
// 		}

// 		// Send notification
// 		resp, err := webpush.SendNotification(webpushSub, notificationPayload, &webpush.Options{
// 			Subscriber:      s.vapidPublicKey,
// 			VAPIDPrivateKey: s.vapidPrivateKey,
// 			TTL:             3600,
// 		})

// 		if err != nil {
// 			s.logger.Error("webpush send failed", "error", err, "endpoint", sub.Endpoint)
// 			continue
// 		}

// 		if resp.StatusCode() != http.StatusCreated && resp.StatusCode() != http.StatusOK {
// 			s.logger.Warn("webpush returned non-success status", "status", resp.StatusCode())
// 		}
// 	}

// 	return nil
// }

func (s *Service) sendEmail(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
	if s.smtpHost == "" || s.smtpUser == "" {
		return fmt.Errorf("email not configured")
	}

	user, err := s.storage.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	settings, _ := s.storage.GetNotificationSettings(ctx, userID)
	email := user.Email
	if settings != nil && settings.Email != "" {
		email = settings.Email
	}

	// Sanitize user-controlled content to prevent HTML injection
	safeTitle := html.EscapeString(payload.Title)
	safeBody := html.EscapeString(payload.Body)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: [Corp Messenger] %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"\r\n"+
		"<html><body>\r\n"+
		"<p><strong>%s</strong></p>\r\n"+
		"<p>%s</p>\r\n"+
		"<hr><p style='font-size:small;color:gray'>This is an automated notification from Corp Messenger.</p>\r\n"+
		"</body></html>\r\n",
		email, safeTitle, safeTitle, safeBody))

	addr := fmt.Sprintf("%s:%d", s.smtpHost, s.smtpPort)
	auth := smtp.PlainAuth("", s.smtpUser, s.smtpPass, s.smtpHost)

	return smtp.SendMail(addr, auth, s.smtpUser, []string{email}, msg)
}

func (s *Service) isQuietHours(settings *models.NotificationSettings) bool {
	if settings.QuietHoursStart == nil || settings.QuietHoursEnd == nil {
		return false
	}

	now := time.Now().Format("15:04")
	start := *settings.QuietHoursStart
	end := *settings.QuietHoursEnd

	if start < end {
		return now >= start && now <= end
	}
	// Cross-midnight quiet hours
	return now >= start || now <= end
}

func (s *Service) isUserOffline(ctx context.Context, userID uuid.UUID) bool {
	// Check last activity (WebSocket connection)
	// This is a simplified check - in production you'd track last ping
	lastOnline, err := s.storage.GetUserLastOnline(ctx, userID)
	if err != nil {
		return true // Assume offline if error
	}
	return time.Since(lastOnline) > 5*time.Minute
}

// GetTotalUnread returns total unread count for badge
func (s *Service) GetTotalUnread(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.storage.GetTotalUnreadCount(ctx, userID)
}
