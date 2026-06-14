package notifications

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"corp-messenger/backend/internal/config"
	"corp-messenger/backend/internal/email"
	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
)

type Service struct {
	logger          *slog.Logger
	storage         storage.Storage
	fcm             *FCMClientV1
	expoPush        *ExpoPushClient
	emailService    *email.Service
	vapidPublicKey  string
	vapidPrivateKey string
}

func NewService(logger *slog.Logger, storage storage.Storage) (*Service, error) {
	logger.Info("Initializing FCM v1 client")
	fcm, err := NewFCMClientV1()
	if err != nil {
		logger.Warn("FCM v1 not configured, push notifications disabled", "error", err)
		fcm = nil
		// Don't return error - FCM is optional
	} else {
		logger.Info("FCM v1 configured successfully")
	}

	// Initialize Expo Push client
	expoAPIKey := os.Getenv("EXPO_PUSH_API_KEY")
	var expoPush *ExpoPushClient
	if expoAPIKey != "" {
		expoPush = NewExpoPushClient(expoAPIKey, logger)
		logger.Info("Expo Push API configured")
	} else {
		logger.Warn("Expo Push API not configured")
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
	smtpFrom := os.Getenv("SMTP_FROM")

	var emailService *email.Service
	if smtpHost != "" && smtpUser != "" {
		cfg := config.Config{
			SMTPHost:     smtpHost,
			SMTPPort:     smtpPort,
			SMTPUser:     smtpUser,
			SMTPPassword: smtpPass,
			SMTPFrom:     smtpFrom,
		}
		emailService = email.NewService(cfg)
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
		expoPush:        expoPush,
		emailService:    emailService,
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
				// Skip Expo tokens when using FCM
				if isExpoToken(token.Token) {
					continue
				}
				if err := s.fcm.Send(ctx, token.Token, payload.Title, payload.Body, payload.Data); err != nil {
					s.logger.Error("fcm send failed", "error", err, "token", token.Token)
				}
			}
		}
	}

	// Send Expo Push notifications
	if s.expoPush != nil && s.expoPush.IsConfigured() {
		tokens, err := s.storage.GetDeviceTokens(ctx, userID)
		if err != nil {
			s.logger.Error("get device tokens for Expo", "error", err, "user_id", userID)
		} else {
			expoTokens := make([]string, 0)
			for _, token := range tokens {
				if isExpoToken(token.Token) {
					expoTokens = append(expoTokens, token.Token)
				}
			}
			if len(expoTokens) > 0 {
				if err := s.expoPush.SendMulticast(ctx, expoTokens, payload.Title, payload.Body, payload.Data); err != nil {
					s.logger.Error("expo push send failed", "error", err, "user_id", userID)
				}
			}
		}
	}

	// Send Web Push notifications
	if s.vapidPublicKey != "" && s.vapidPrivateKey != "" {
		if err := s.sendWebPush(ctx, userID, payload); err != nil {
			s.logger.Error("web push send failed", "error", err, "user_id", userID)
		}
	}

	return nil
}

// isExpoToken checks if a token is an Expo push token
func isExpoToken(token string) bool {
	return len(token) > 20 && (token[:18] == "ExponentPushToken[" || token[:14] == "ExpoPushToken[")
}

func (s *Service) sendWebPush(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
	subs, err := s.storage.GetWebPushSubscriptions(ctx, userID)
	if err != nil {
		return fmt.Errorf("get web push subscriptions: %w", err)
	}

	for _, sub := range subs {
		// Decode base64 keys (client sends standard base64 with btoa())
		key, err := base64.StdEncoding.DecodeString(sub.Key)
		if err != nil {
			s.logger.Error("decode web push key", "error", err)
			continue
		}

		auth, err := base64.StdEncoding.DecodeString(sub.Auth)
		if err != nil {
			s.logger.Error("decode web push auth", "error", err)
			continue
		}

		// Create webpush subscription
		webpushSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: string(key),
				Auth:   string(auth),
			},
		}

		// Create notification payload
		notificationPayload := map[string]interface{}{
			"title": payload.Title,
			"body":  payload.Body,
			"data":  payload.Data,
		}

		// Convert to JSON bytes
		messageBytes, err := json.Marshal(notificationPayload)
		if err != nil {
			s.logger.Error("failed to marshal webpush payload", "error", err)
			continue
		}

		// Send notification
		resp, err := webpush.SendNotification(messageBytes, webpushSub, &webpush.Options{
			Subscriber:      s.vapidPublicKey,
			VAPIDPrivateKey: s.vapidPrivateKey,
			TTL:             3600,
		})

		if err != nil {
			s.logger.Error("webpush send failed", "error", err, "endpoint", sub.Endpoint)
			continue
		}

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			s.logger.Warn("webpush returned non-success status", "status", resp.StatusCode)
		}
	}

	return nil
}

func (s *Service) sendEmail(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
	if s.emailService == nil {
		return fmt.Errorf("email not configured")
	}

	user, err := s.storage.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}

	// Update user email from notification settings if set
	settings, _ := s.storage.GetNotificationSettings(ctx, userID)
	if settings != nil && settings.Email != "" {
		user.Email = settings.Email
	}

	// Use email service for sending with HTML templates
	// Determine notification type and call appropriate method
	switch payload.Type {
	case "mention":
		senderName := payload.Data["sender_name"]
		chatTitle := payload.Data["chat_title"]
		messagePreview := payload.Body
		return s.emailService.SendMentionNotification(user, senderName, chatTitle, messagePreview)
	case "invite":
		inviterName := payload.Data["inviter_name"]
		chatTitle := payload.Data["chat_title"]
		return s.emailService.SendInviteNotification(user, inviterName, chatTitle)
	default: // "message"
		senderName := payload.Data["sender_name"]
		chatTitle := payload.Data["chat_title"]
		messagePreview := payload.Body
		return s.emailService.SendNewMessageNotification(user, senderName, chatTitle, messagePreview)
	}
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
