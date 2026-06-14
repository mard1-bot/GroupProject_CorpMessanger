# Email Notifications Guide

This guide explains how to configure and use email notifications in Corp Messenger.

## Overview

Email notifications are sent to users when they are offline for more than 5 minutes and have email notifications enabled. The system supports multiple SMTP providers and sends both plain text and HTML email templates.

## Supported SMTP Providers

### 1. Gmail

**Best for:** Development and small projects

**Configuration:**
```bash
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=noreply@yourdomain.com
```

**Setup:**
1. Enable 2-Factor Authentication on your Google Account
2. Go to [Google Account Settings](https://myaccount.google.com/security)
3. Navigate to **Security** → **2-Step Verification** → **App passwords**
4. Generate a new app password for "Corp Messenger"
5. Use the 16-character app password as `SMTP_PASSWORD`

**Limitations:**
- 500 emails/day for free accounts
- May require additional verification for production use
- Not recommended for high-volume applications

### 2. SendGrid

**Best for:** Production applications with moderate volume

**Configuration:**
```bash
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USER=apikey
SMTP_PASSWORD=SG.your-sendgrid-api-key
SMTP_FROM=noreply@yourdomain.com
```

**Setup:**
1. Sign up at [SendGrid](https://sendgrid.com)
2. Create an API key with "Mail Send" permissions
3. Verify your sender domain
4. Use the API key as `SMTP_PASSWORD`

**Pricing:**
- Free tier: 100 emails/day
- Paid plans start at $19.95/month for 50,000 emails

### 3. Mailgun

**Best for:** Applications requiring advanced features

**Configuration:**
```bash
SMTP_HOST=smtp.mailgun.org
SMTP_PORT=587
SMTP_USER=postmaster@your-domain.mailgun.org
SMTP_PASSWORD=your-mailgun-api-key
SMTP_FROM=noreply@yourdomain.com
```

**Setup:**
1. Sign up at [Mailgun](https://www.mailgun.com)
2. Verify your domain
3. Get SMTP credentials from the dashboard
4. Use the API key as `SMTP_PASSWORD`

**Pricing:**
- Free trial: 5,000 emails/month
- Paid plans start at $35/month for 50,000 emails

### 4. Amazon SES

**Best for:** High-volume applications on AWS

**Configuration:**
```bash
SMTP_HOST=email-smtp.us-east-1.amazonaws.com
SMTP_PORT=587
SMTP_USER=your-aws-access-key-id
SMTP_PASSWORD=your-aws-secret-access-key
SMTP_FROM=noreply@yourdomain.com
```

**Setup:**
1. Create an AWS account
2. Verify your domain in SES console
3. Create SMTP credentials
4. Use IAM credentials as `SMTP_USER` and `SMTP_PASSWORD`

**Pricing:**
- $0.10 per 1,000 emails (first 62,000 emails/month free in Free Tier)
- No additional sending fee

### 5. Postmark

**Best for:** Transactional emails with high deliverability

**Configuration:**
```bash
SMTP_HOST=smtp.postmarkapp.com
SMTP_PORT=587
SMTP_USER=your-postmark-api-token
SMTP_PASSWORD=your-postmark-api-token
SMTP_FROM=noreply@yourdomain.com
```

**Setup:**
1. Sign up at [Postmark](https://postmarkapp.com)
2. Verify your sender signature
3. Get API token from the dashboard
4. Use the API token for both username and password

**Pricing:**
- Free trial: 1,000 emails
- Paid plans start at $15/month for 10,000 emails

## Email Templates

The system includes three email templates:

### 1. New Message Notification

**Subject:** "Новое сообщение в {chat_title}"

**Content:** Notifies user about a new message in a chat

**Template features:**
- Purple gradient header
- Message preview in highlighted box
- Mobile-responsive design

### 2. Mention Notification

**Subject:** "{sender_name} упомянул вас в {chat_title}"

**Content:** Notifies user when mentioned in a message

**Template features:**
- Pink/red gradient header
- @Mention branding
- Highlighted mention box

### 3. Invite Notification

**Subject:** "Приглашение в чат: {chat_title}"

**Content:** Notifies user about chat invitation

**Template features:**
- Blue gradient header
- 🎉 emoji for celebration
- Invite acceptance prompt

## Configuration

### Backend Environment Variables

Add to `backend/.env`:

```bash
# SMTP Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=noreply@yourdomain.com
```

### Docker Secrets (Production)

For production, use Docker secrets:

```yaml
# docker-compose.prod.yml
secrets:
  smtp_user:
    file: .secrets/smtp_user
  smtp_password:
    file: .secrets/smtp_password

services:
  backend:
    environment:
      SMTP_HOST: smtp.gmail.com
      SMTP_PORT: 587
      SMTP_USER_FILE: /run/secrets/smtp_user
      SMTP_PASSWORD_FILE: /run/secrets/smtp_password
      SMTP_FROM: noreply@yourdomain.com
    secrets:
      - smtp_user
      - smtp_password
```

Create secret files:

```bash
echo "your-email@gmail.com" > .secrets/smtp_user
echo "your-app-password" > .secrets/smtp_password
chmod 600 .secrets/smtp_*
```

## User Notification Settings

Users can control email notifications through the app:

- **Enable/Disable:** Turn email notifications on/off
- **Email Address:** Use a different email than account email
- **Quiet Hours:** Set time range to suppress notifications
- **Chat Muting:** Mute specific chats

## When Emails Are Sent

Emails are sent when:

1. User is offline for more than 5 minutes
2. User has email notifications enabled
3. User is not in quiet hours
4. Chat is not muted

## Testing

### Test Email Configuration

```bash
# Test SMTP connection
telnet smtp.gmail.com 587

# Test with backend API
curl -X POST http://localhost:8080/api/v1/notifications/test-email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "test@example.com",
    "subject": "Test Email",
    "body": "This is a test email"
  }'
```

### Check Email Logs

```bash
# Backend logs
docker logs corp-backend | grep "email"

# Check for SMTP errors
docker logs corp-backend | grep "SMTP"
```

## Troubleshooting

### Authentication Failed

**Cause:** Invalid credentials or 2FA not enabled

**Solution:**
- For Gmail: Use App Password, not regular password
- For SendGrid: Verify API key has "Mail Send" permission
- For Mailgun: Verify domain is verified

### TLS Handshake Failed

**Cause:** TLS version mismatch or certificate issue

**Solution:**
- Ensure SMTP server supports TLS 1.2+
- Check firewall allows outbound TLS connections
- Verify SMTP host is correct

### Rate Limiting

**Cause:** Sending too many emails too quickly

**Solution:**
- Upgrade to paid plan for higher limits
- Implement queue system for bulk sends
- Use provider's bulk send API instead of SMTP

### Emails Going to Spam

**Cause:** Poor sender reputation or missing SPF/DKIM

**Solution:**
- Verify your domain with the provider
- Set up SPF, DKIM, and DMARC records
- Use a dedicated sending domain
- Monitor sender reputation

## Best Practices

### 1. Use Dedicated Email Address

Use a dedicated email like `noreply@yourdomain.com` instead of personal email.

### 2. Verify Your Domain

Always verify your sending domain with the email provider to improve deliverability.

### 3. Monitor Bounce Rates

Track bounce rates and remove invalid email addresses from your database.

### 4. Implement Rate Limiting

Don't send emails too quickly. Implement delays between sends.

### 5. Use HTML Fallback

Always include both HTML and plain text versions for compatibility.

### 6. Test Before Production

Test email templates with different email clients (Gmail, Outlook, Apple Mail).

### 7. Keep Credentials Secure

Never commit SMTP credentials to Git. Use environment variables or secrets.

## Security

### TLS Configuration

The email service uses TLS 1.2+ for secure connections:

```go
tlsConfig := &tls.Config{
    ServerName: s.config.SMTPHost,
    MinVersion: tls.VersionTLS12,
}
```

### Credential Storage

- Never commit credentials to Git
- Use environment variables for development
- Use Docker secrets for production
- Rotate credentials periodically

### Content Sanitization

All user-generated content is sanitized using `html.EscapeString` to prevent XSS attacks.

## Performance Considerations

### Sending Speed

- Single email: ~1-2 seconds
- Batch emails: Add delays between sends
- Consider using queue for high volume

### Memory Usage

- Email templates are generated on-demand
- No template caching (minimal memory impact)
- Multipart messages increase size slightly

### Database Impact

- Email addresses stored in user table
- Notification settings stored separately
- No additional indexes needed

## Comparison of Providers

| Provider | Free Tier | Paid Plans | Deliverability | Features |
|----------|-----------|------------|----------------|----------|
| Gmail | 500/day | N/A | Medium | Basic |
| SendGrid | 100/day | $19.95/mo | High | Advanced |
| Mailgun | 5,000/mo | $35/mo | High | Advanced |
| SES | 62,000/mo | $0.10/1k | High | Basic |
| Postmark | 1,000 | $15/mo | Very High | Advanced |

## Resources

- [Gmail App Passwords](https://support.google.com/accounts/answer/185833)
- [SendGrid Documentation](https://docs.sendgrid.com)
- [Mailgun Documentation](https://documentation.mailgun.com)
- [Amazon SES Documentation](https://docs.aws.amazon.com/ses)
- [Postmark Documentation](https://postmarkapp.com/support)

## Migration Guide

### Migrating from One Provider to Another

1. Get credentials from new provider
2. Update environment variables
3. Test with a single email
4. Monitor deliverability
5. Update DNS records if needed

### Migrating from Legacy Email Service

If migrating from a different email implementation:

1. Update `backend/internal/email/email.go`
2. Update notification service integration
3. Test all email templates
4. Update user notification settings if needed
