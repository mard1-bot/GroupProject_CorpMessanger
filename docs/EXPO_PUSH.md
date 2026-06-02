# Expo Push Notifications Guide

This guide explains how to set up and use Expo Push Notifications for the Corp Messenger React Native app.

## Overview

Expo Push Notifications provide a simple way to send push notifications to React Native apps without needing to configure Firebase Cloud Messaging (FCM) directly. Expo handles the complexity of push notification delivery across iOS and Android platforms.

## Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Client    │────────▶│   Backend   │────────▶│  Expo Push  │
│ (React Native)│ Token  │  (Go API)   │  API Key  │   Service   │
└─────────────┘         └─────────────┘         └─────────────┘
                              │
                              ▼
                        ┌─────────────┐
                        │   Apple     │
                        │  APNs       │
                        └─────────────┘
                              │
                              ▼
                        ┌─────────────┐
                        │   Google    │
                        │   FCM       │
                        └─────────────┘
```

## Setup

### 1. Get Expo Access Token

1. Go to [Expo Dashboard](https://expo.dev)
2. Navigate to your account settings
3. Go to **Access Tokens**
4. Create a new access token with "Push Notifications" scope
5. Copy the token (you won't see it again)

### 2. Configure Backend

Add the Expo Push API key to your backend environment:

```bash
# In backend/.env
EXPO_PUSH_API_KEY=your-expo-push-access-token
```

### 3. Restart Backend

```bash
# Development
cd backend
go run cmd/api/main.go

# Production
docker compose -f docker-compose.prod.yml restart backend
```

## Client Implementation

The client already has Expo Push token registration implemented in `client/services/notifications.ts`:

```typescript
async registerDeviceToken(): Promise<void> {
  const token = await Notifications.getExpoPushTokenAsync({
    projectId: process.env.EXPO_PROJECT_ID || '',
  });

  await api.registerDeviceToken({
    token: token.data,
    platform: Platform.OS,
    device_name: Platform.OS,
  });
}
```

## Backend Implementation

### Expo Push Client

The backend uses a custom Expo Push client in `backend/internal/notifications/expo_push.go`:

```go
type ExpoPushClient struct {
    httpClient *http.Client
    apiKey     string
    logger     *slog.Logger
}

func NewExpoPushClient(apiKey string, logger *slog.Logger) *ExpoPushClient {
    return &ExpoPushClient{
        httpClient: &http.Client{Timeout: 30 * time.Second},
        apiKey:     apiKey,
        logger:     logger,
    }
}
```

### Sending Notifications

The notification service automatically detects Expo tokens and routes them to Expo Push:

```go
func (s *Service) sendPush(ctx context.Context, userID uuid.UUID, payload *NotificationPayload) error {
    // Send Expo Push notifications
    if s.expoPush != nil && s.expoPush.IsConfigured() {
        tokens, err := s.storage.GetDeviceTokens(ctx, userID)
        expoTokens := make([]string, 0)
        for _, token := range tokens {
            if isExpoToken(token.Token) {
                expoTokens = append(expoTokens, token.Token)
            }
        }
        if len(expoTokens) > 0 {
            s.expoPush.SendMulticast(ctx, expoTokens, payload.Title, payload.Body, payload.Data)
        }
    }
}
```

## Token Detection

The backend automatically detects Expo push tokens by their format:

- `ExponentPushToken[xxxxxxxxxxxx]` - Legacy Expo tokens
- `ExpoPushToken[xxxxxxxxxxxx]` - New Expo tokens

Non-Expo tokens are sent via FCM v1 instead.

## Notification Payload

Expo Push notifications support the following fields:

```go
type ExpoPushMessage struct {
    To           string                 // Expo push token
    Title        string                 // Notification title
    Body         string                 // Notification body
    Data         map[string]interface{} // Custom data
    Sound        string                 // Sound to play
    Priority     string                 // "default", "normal", "high"
    Badge        int                    // Badge count
    ChannelID    string                 // Android channel ID
    TTL          *int                   // Time to live in seconds
    Subtitle     string                 // Subtitle (iOS)
    Category     string                 // Category identifier
}
```

## Rate Limiting

Expo Push API has the following limits:
- **100 messages per request** (batch size)
- **No explicit rate limit** but recommended to add delays

Our implementation:
- Sends in batches of 100 tokens
- 100ms delay between batches
- Parallel sending within batches

## Error Handling

### Common Errors

**DeviceNotRegistered**
- The device token is invalid or expired
- Solution: Remove the token from database

**MessageTooBig**
- Notification payload exceeds size limit
- Solution: Reduce payload size (max 4KB)

**InvalidCredentials**
- Invalid API key
- Solution: Check EXPO_PUSH_API_KEY

### Error Response Format

```json
{
  "data": [
    {
      "status": "error",
      "message": "DeviceNotRegistered",
      "details": {
        "error": "DeviceNotRegistered"
      }
    }
  ]
}
```

## Testing

### Test with Expo Push Tool

Use the [Expo Push Tool](https://expo.dev/notifications) to test notifications:

1. Get your device token from the app logs
2. Send a test notification via the Expo dashboard
3. Verify it appears on your device

### Test via Backend API

```bash
# Register a device token first
curl -X POST http://localhost:8080/api/v1/device-tokens \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "token": "ExpoPushToken[xxxxxxxxxxxx]",
    "platform": "ios",
    "device_name": "iPhone"
  }'

# Send a test notification
curl -X POST http://localhost:8080/api/v1/notifications/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Test Notification",
    "body": "This is a test from Expo Push"
  }'
```

## Production Deployment

### Docker Secrets

For production, use Docker secrets:

```yaml
# docker-compose.prod.yml
secrets:
  expo_push_api_key:
    file: .secrets/expo_push_api_key

services:
  backend:
    environment:
      EXPO_PUSH_API_KEY_FILE: /run/secrets/expo_push_api_key
    secrets:
      - expo_push_api_key
```

Create the secret file:

```bash
echo "your-expo-push-access-token" > .secrets/expo_push_api_key
chmod 600 .secrets/expo_push_api_key
```

### Monitoring

Monitor Expo Push delivery:

```bash
# Check backend logs for Expo Push errors
docker logs corp-backend | grep "expo push"

# Check for failed tokens
docker logs corp-backend | grep "DEVICE_NOT_REGISTERED"
```

## Comparison: Expo Push vs FCM

| Feature | Expo Push | FCM v1 |
|---------|-----------|--------|
| Setup Complexity | Low (just API key) | High (service account) |
| Platform Support | iOS + Android | iOS + Android + Web |
| Token Format | Expo tokens | Native tokens |
| Rate Limits | 100/batch | 600k/min |
| Cost | Free | Free tier available |
| Customization | Limited | Extensive |
| Analytics | Basic | Advanced |

## Best Practices

### 1. Token Rotation

Expo tokens can expire. Handle this by:

```go
if err := s.expoPush.Send(ctx, token, title, body, data); err != nil {
    if err.Error() == "DEVICE_NOT_REGISTERED" {
        // Remove invalid token
        s.storage.DeleteDeviceToken(ctx, userID, token)
    }
}
```

### 2. Batch Sending

Always use `SendMulticast` for multiple tokens to avoid rate limiting:

```go
// Good
s.expoPush.SendMulticast(ctx, tokens, title, body, data)

// Bad - individual sends
for _, token := range tokens {
    s.expoPush.Send(ctx, token, title, body, data)
}
```

### 3. Error Logging

Log Expo Push errors for debugging:

```go
if err := s.expoPush.SendMulticast(ctx, tokens, title, body, data); err != nil {
    s.logger.Error("expo push failed", "error", err, "user_id", userID)
}
```

### 4. Fallback to FCM

If Expo Push fails, fallback to FCM for non-Expo tokens:

```go
// Expo tokens go to Expo Push
if isExpoToken(token) {
    s.expoPush.Send(ctx, token, title, body, data)
} else {
    // Native tokens go to FCM
    s.fcm.Send(ctx, token, title, body, data)
}
```

## Troubleshooting

### Notifications Not Arriving

1. Check if EXPO_PUSH_API_KEY is set
2. Verify the device token is valid
3. Check backend logs for errors
4. Test with Expo Push Tool
5. Ensure app has notification permissions

### Invalid Token Errors

1. Token may have expired
2. User may have uninstalled the app
3. Token format may be incorrect
4. Solution: Remove and re-register the token

### Rate Limiting Issues

1. Reduce batch size
2. Add delays between batches
3. Implement exponential backoff

## Resources

- [Expo Push Notifications Documentation](https://docs.expo.dev/push-notifications/overview/)
- [Expo Push API Reference](https://docs.expo.dev/push-notifications/push-api/)
- [Expo Access Tokens](https://docs.expo.dev/accounts/access-tokens/)
- [React Native Notifications](https://docs.expo.dev/versions/latest/sdk/notifications/)

## Migration from FCM

If migrating from FCM to Expo Push:

1. Keep FCM v1 for native builds
2. Use Expo Push for Expo builds
3. Backend automatically routes based on token format
4. No client changes needed
5. Gradually migrate users to Expo builds

## Security

- Never commit EXPO_PUSH_API_KEY to Git
- Use Docker secrets in production
- Rotate access tokens periodically
- Monitor for unauthorized usage
- Use HTTPS for all API calls
