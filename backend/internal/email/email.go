package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"net/textproto"
	"strings"

	"corp-messenger/backend/internal/config"
	"corp-messenger/backend/internal/models"
)

// Service handles email notifications
type Service struct {
	config config.Config
}

// NewService creates a new email service
func NewService(cfg config.Config) *Service {
	return &Service{
		config: cfg,
	}
}

// SendNotification sends an email notification
func (s *Service) SendNotification(user *models.User, subject, body string) error {
	if s.config.SMTPHost == "" || s.config.SMTPUser == "" {
		return fmt.Errorf("SMTP not configured")
	}

	// Get user's email from notification settings or user email
	email := user.Email
	if email == "" {
		return fmt.Errorf("user has no email")
	}

	// Build email
	from := s.config.SMTPFrom
	if from == "" {
		from = s.config.SMTPUser
	}

	to := []string{email}
	msg := s.buildMessage(from, to, subject, body)

	// Send email with TLS support
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	auth := smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPassword, s.config.SMTPHost)

	// Connect to SMTP server with TLS
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Start TLS if available
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: s.config.SMTPHost,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	// Authenticate
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Set sender and recipients
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", addr, err)
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send data: %w", err)
	}
	defer w.Close()

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// SendNewMessageNotification sends notification about new message
func (s *Service) SendNewMessageNotification(user *models.User, senderName, chatTitle, messagePreview string) error {
	subject := fmt.Sprintf("Новое сообщение в %s", chatTitle)
	body := fmt.Sprintf(
		"Привет, %s!\n\n%s отправил(а) вам новое сообщение в чате \"%s\":\n\n%s\n\n"+
			"Откройте приложение, чтобы ответить.",
		user.FirstName, senderName, chatTitle, messagePreview,
	)
	return s.SendNotification(user, subject, body)
}

// SendMentionNotification sends notification about mention
func (s *Service) SendMentionNotification(user *models.User, senderName, chatTitle, messagePreview string) error {
	subject := fmt.Sprintf("%s упомянул вас в %s", senderName, chatTitle)
	body := fmt.Sprintf(
		"Привет, %s!\n\n%s упомянул вас в чате \"%s\":\n\n%s\n\n"+
			"Откройте приложение, чтобы ответить.",
		user.FirstName, senderName, chatTitle, messagePreview,
	)
	return s.SendNotification(user, subject, body)
}

// buildMessage builds email message with proper headers
func (s *Service) buildMessage(from string, to []string, subject, body string) string {
	header := make(textproto.MIMEHeader)
	header.Set("From", from)
	header.Set("To", strings.Join(to, ", "))
	header.Set("Subject", subject)
	header.Set("MIME-Version", "1.0")
	header.Set("Content-Type", "text/plain; charset=utf-8")

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, ", "))
	}
	message += "\r\n" + body

	return message
}
