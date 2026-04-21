package notifications

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"corp-messenger/backend/internal/models"
	"corp-messenger/backend/internal/storage"

	"github.com/google/uuid"
)

type Service struct {
	logger   *slog.Logger
	storage  storage.Storage
	fcm      *FCMClient
	smtpHost string
	smtpPort int
	smtpUser string
	smtpPass string
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

	return &Service{
		logger:   logger,
		storage:  storage,
		fcm:      fcm,
		smtpHost: smtpHost,
		smtpPort: smtpPort,
		smtpUser: smtpUser,
		smtpPass: smtpPass,
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
	if s.fcm == nil || !s.fcm.IsConfigured() {
		return fmt.Errorf("fcm not initialized")
	}

	tokens, err := s.storage.GetDeviceTokens(ctx, userID)
	if err != nil {
		return fmt.Errorf("get device tokens: %w", err)
	}

	for _, token := range tokens {
		err := s.fcm.Send(ctx, token.Token, payload.Title, payload.Body, payload.Data)
		if err != nil {
			s.logger.Error("failed to send notification", "error", err, "user_id", token.UserID, "platform", token.Platform)
			// Delete invalid token
			if strings.Contains(err.Error(), "INVALID_TOKEN") {
				if delErr := s.storage.DeleteDeviceToken(ctx, token.Token); delErr != nil {
					s.logger.Error("failed to delete invalid token", "error", delErr, "user_id", token.UserID)
				}
			}
		}
	}

	return nil
}

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
