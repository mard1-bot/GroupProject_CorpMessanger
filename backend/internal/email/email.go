package email

import (
	"crypto/tls"
	"fmt"
	"html"
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

// SendNotification sends an email notification with HTML support
func (s *Service) SendNotification(user *models.User, subject, textBody, htmlBody string) error {
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
	msg := s.buildMultipartMessage(from, to, subject, textBody, htmlBody)

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
			MinVersion: tls.VersionTLS12,
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

	// Plain text version
	textBody := fmt.Sprintf(
		"Привет, %s!\n\n%s отправил(а) вам новое сообщение в чате \"%s\":\n\n%s\n\n"+
			"Откройте приложение, чтобы ответить.",
		user.FirstName, senderName, chatTitle, messagePreview,
	)

	// HTML version
	htmlBody := s.buildNewMessageHTML(user.FirstName, senderName, chatTitle, messagePreview)

	return s.SendNotification(user, subject, textBody, htmlBody)
}

// SendMentionNotification sends notification about mention
func (s *Service) SendMentionNotification(user *models.User, senderName, chatTitle, messagePreview string) error {
	subject := fmt.Sprintf("%s упомянул вас в %s", senderName, chatTitle)

	// Plain text version
	textBody := fmt.Sprintf(
		"Привет, %s!\n\n%s упомянул вас в чате \"%s\":\n\n%s\n\n"+
			"Откройте приложение, чтобы ответить.",
		user.FirstName, senderName, chatTitle, messagePreview,
	)

	// HTML version
	htmlBody := s.buildMentionHTML(user.FirstName, senderName, chatTitle, messagePreview)

	return s.SendNotification(user, subject, textBody, htmlBody)
}

// SendInviteNotification sends notification about chat invite
func (s *Service) SendInviteNotification(user *models.User, inviterName, chatTitle string) error {
	subject := fmt.Sprintf("Приглашение в чат: %s", chatTitle)

	// Plain text version
	textBody := fmt.Sprintf(
		"Привет, %s!\n\n%s пригласил(а) вас в чат \"%s\".\n\n"+
			"Откройте приложение, чтобы принять приглашение.",
		user.FirstName, inviterName, chatTitle,
	)

	// HTML version
	htmlBody := s.buildInviteHTML(user.FirstName, inviterName, chatTitle)

	return s.SendNotification(user, subject, textBody, htmlBody)
}

// buildNewMessageHTML creates HTML template for new message notification
func (s *Service) buildNewMessageHTML(userName, senderName, chatTitle, messagePreview string) string {
	safeSender := html.EscapeString(senderName)
	safeChat := html.EscapeString(chatTitle)
	safeMessage := html.EscapeString(messagePreview)
	safeUser := html.EscapeString(userName)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; border-radius: 10px 10px 0 0; }
		.content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
		.message-box { background: white; padding: 20px; border-left: 4px solid #667eea; margin: 20px 0; }
		.button { display: inline-block; background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
		.footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>Corp Messenger</h1>
		</div>
		<div class="content">
			<p>Привет, %s!</p>
			<p><strong>%s</strong> отправил(а) вам новое сообщение в чате <strong>%s</strong>:</p>
			<div class="message-box">
				<p>%s</p>
			</div>
			<p>Откройте приложение, чтобы ответить.</p>
		</div>
		<div class="footer">
			<p>Это автоматическое уведомление от Corp Messenger.</p>
		</div>
	</div>
</body>
</html>`, safeUser, safeSender, safeChat, safeMessage)
}

// buildMentionHTML creates HTML template for mention notification
func (s *Service) buildMentionHTML(userName, senderName, chatTitle, messagePreview string) string {
	safeSender := html.EscapeString(senderName)
	safeChat := html.EscapeString(chatTitle)
	safeMessage := html.EscapeString(messagePreview)
	safeUser := html.EscapeString(userName)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background: linear-gradient(135deg, #f093fb 0%%, #f5576c 100%%); color: white; padding: 30px; border-radius: 10px 10px 0 0; }
		.content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
		.message-box { background: white; padding: 20px; border-left: 4px solid #f5576c; margin: 20px 0; }
		.button { display: inline-block; background: #f5576c; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
		.footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>@Mention</h1>
		</div>
		<div class="content">
			<p>Привет, %s!</p>
			<p><strong>%s</strong> упомянул вас в чате <strong>%s</strong>:</p>
			<div class="message-box">
				<p>%s</p>
			</div>
			<p>Откройте приложение, чтобы ответить.</p>
		</div>
		<div class="footer">
			<p>Это автоматическое уведомление от Corp Messenger.</p>
		</div>
	</div>
</body>
</html>`, safeUser, safeSender, safeChat, safeMessage)
}

// buildInviteHTML creates HTML template for invite notification
func (s *Service) buildInviteHTML(userName, inviterName, chatTitle string) string {
	safeInviter := html.EscapeString(inviterName)
	safeChat := html.EscapeString(chatTitle)
	safeUser := html.EscapeString(userName)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
		.container { max-width: 600px; margin: 0 auto; padding: 20px; }
		.header { background: linear-gradient(135deg, #4facfe 0%%, #00f2fe 100%%); color: white; padding: 30px; border-radius: 10px 10px 0 0; }
		.content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
		.invite-box { background: white; padding: 20px; border-left: 4px solid #4facfe; margin: 20px 0; }
		.button { display: inline-block; background: #4facfe; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
		.footer { text-align: center; margin-top: 30px; color: #999; font-size: 12px; }
	</style>
</head>
<body>
	<div class="container">
		<div class="header">
			<h1>🎉 Приглашение</h1>
		</div>
		<div class="content">
			<p>Привет, %s!</p>
			<p><strong>%s</strong> пригласил(а) вас в чат <strong>%s</strong>.</p>
			<div class="invite-box">
				<p>Принять приглашение и присоединиться к чату?</p>
			</div>
			<p>Откройте приложение, чтобы принять приглашение.</p>
		</div>
		<div class="footer">
			<p>Это автоматическое уведомление от Corp Messenger.</p>
		</div>
	</div>
</body>
</html>`, safeUser, safeInviter, safeChat)
}

// buildMultipartMessage builds multipart email message with both text and HTML
func (s *Service) buildMultipartMessage(from string, to []string, subject, textBody, htmlBody string) string {
	header := make(textproto.MIMEHeader)
	header.Set("From", from)
	header.Set("To", strings.Join(to, ", "))
	header.Set("Subject", subject)
	header.Set("MIME-Version", "1.0")
	header.Set("Content-Type", "multipart/alternative; boundary=\"boundary\"")

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, strings.Join(v, ", "))
	}
	message += "\r\n"

	// Add text part
	message += "--boundary\r\n"
	message += "Content-Type: text/plain; charset=utf-8\r\n"
	message += "Content-Transfer-Encoding: quoted-printable\r\n"
	message += "\r\n"
	message += textBody + "\r\n"

	// Add HTML part
	message += "--boundary\r\n"
	message += "Content-Type: text/html; charset=utf-8\r\n"
	message += "Content-Transfer-Encoding: quoted-printable\r\n"
	message += "\r\n"
	message += htmlBody + "\r\n"

	message += "--boundary--\r\n"

	return message
}

// buildMessage builds simple text email message (fallback)
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
