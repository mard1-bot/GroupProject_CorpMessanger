# FCM HTTP v1 API Migration Guide

## Overview

Google is deprecating the legacy FCM HTTP API in favor of the HTTP v1 API. This guide explains how to migrate from the legacy API to v1.

## Why Migrate?

- **Legacy API deprecation**: Google will eventually shut down the legacy API
- **Better security**: OAuth2 instead of static server keys
- **More features**: Enhanced message targeting and analytics
- **Better error handling**: More detailed error responses

## Prerequisites

### 1. Firebase Service Account

1. Go to [Firebase Console](https://console.firebase.google.com/)
2. Select your project
3. Go to **Project Settings** → **Service Accounts**
4. Click **Generate New Private Key**
5. Save the JSON file securely

### 2. Required Dependencies

Add to `go.mod`:
```bash
cd backend
go get golang.org/x/oauth2
go get golang.org/x/oauth2/google
```

## Configuration

### Environment Variables

Add these to your `.env` file:

```bash
# FCM v1 API Configuration
FIREBASE_PROJECT_ID=your-project-id
FIREBASE_CREDENTIALS_PATH=/path/to/service-account.json

# Alternative: Inline credentials (for Docker/K8s secrets)
# FIREBASE_CREDENTIALS_JSON='{"type":"service_account",...}'

# Legacy API (will be removed)
# FIREBASE_SERVER_KEY=your-legacy-server-key
```

### Docker Secrets

For production, use Docker secrets:

```bash
# Create secret file
echo '{"type":"service_account",...}' > .secrets/firebase_credentials

# Update docker-compose.prod.yml
secrets:
  firebase_credentials:
    file: .secrets/firebase_credentials
```

## Code Changes

### Before (Legacy API)

```go
fcmClient := notifications.NewFCMClient()
err := fcmClient.Send(ctx, deviceToken, title, body, data)
```

### After (v1 API)

```go
fcmClient, err := notifications.NewFCMClientV1()
if err != nil {
    log.Fatalf("Failed to create FCM v1 client: %v", err)
}
err = fcmClient.Send(ctx, deviceToken, title, body, data)
```

## Migration Steps

### 1. Update Service Initialization

In `internal/notifications/service.go`:

```go
// Old
fcmClient := NewFCMClient()

// New
fcmClient, err := NewFCMClientV1()
if err != nil {
    logger.Warn("FCM v1 not configured, falling back to legacy API", "error", err)
    fcmClient = NewFCMClient() // Fallback to legacy
}
```

### 2. Update Environment Variables

```bash
# Add to .env
FIREBASE_PROJECT_ID=corp-messenger-prod
FIREBASE_CREDENTIALS_PATH=./secrets/firebase-service-account.json
```

### 3. Test Migration

```bash
# Run tests
go test ./internal/notifications/...

# Test with real device token
curl -X POST http://localhost:8080/api/v1/test-fcm \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"device_token":"your-test-token"}'
```

### 4. Deploy

```bash
# Update production secrets
echo "$FIREBASE_CREDENTIALS" > .secrets/firebase_credentials

# Deploy
docker-compose -f docker-compose.prod.yml up -d
```

## API Differences

### Message Format

**Legacy API:**
```json
{
  "to": "device-token",
  "notification": {
    "title": "New Message",
    "body": "Hello!"
  }
}
```

**v1 API:**
```json
{
  "message": {
    "token": "device-token",
    "notification": {
      "title": "New Message",
      "body": "Hello!"
    }
  }
}
```

### Error Handling

**Legacy API:**
```json
{
  "results": [{
    "error": "NotRegistered"
  }]
}
```

**v1 API:**
```json
{
  "error": {
    "code": 404,
    "message": "Requested entity was not found.",
    "status": "NOT_FOUND",
    "details": [{
      "errorCode": "UNREGISTERED"
    }]
  }
}
```

## Features Comparison

| Feature | Legacy API | v1 API |
|---------|-----------|--------|
| Authentication | Server Key | OAuth2 |
| Token Refresh | Manual | Automatic |
| Batch Sending | Limited | Enhanced |
| Error Details | Basic | Detailed |
| Analytics | Basic | Advanced |
| Topic Messaging | Yes | Yes |
| Condition Targeting | No | Yes |

## Troubleshooting

### Error: "Failed to get OAuth2 token"

**Cause**: Invalid service account credentials

**Solution**:
```bash
# Verify credentials file
cat .secrets/firebase_credentials | jq .

# Check permissions
chmod 600 .secrets/firebase_credentials
```

### Error: "Project not found"

**Cause**: Incorrect project ID

**Solution**:
```bash
# Get project ID from Firebase Console
# Update FIREBASE_PROJECT_ID in .env
```

### Error: "Permission denied"

**Cause**: Service account lacks permissions

**Solution**:
1. Go to Firebase Console → Project Settings → Service Accounts
2. Ensure service account has "Firebase Cloud Messaging Admin" role

## Performance Considerations

### Rate Limiting

v1 API has the following limits:
- **600,000 requests/minute** per project
- **1,000 requests/second** per device

Our implementation includes:
- Batch processing (100 tokens per batch)
- 100ms delay between batches
- Parallel sending within batches

### Token Caching

OAuth2 tokens are cached automatically by the `oauth2` library:
- Tokens are valid for 1 hour
- Automatic refresh before expiration
- Thread-safe token source

## Rollback Plan

If issues occur, rollback to legacy API:

```bash
# 1. Revert environment variables
unset FIREBASE_PROJECT_ID
unset FIREBASE_CREDENTIALS_PATH
export FIREBASE_SERVER_KEY=your-legacy-key

# 2. Restart service
docker-compose restart backend
```

## Testing Checklist

- [ ] Service account credentials configured
- [ ] OAuth2 token generation works
- [ ] Single device notification sends successfully
- [ ] Multicast to multiple devices works
- [ ] Invalid token errors handled correctly
- [ ] Rate limiting doesn't cause failures
- [ ] Production deployment tested

## Resources

- [FCM HTTP v1 API Reference](https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages)
- [Migration Guide](https://firebase.google.com/docs/cloud-messaging/migrate-v1)
- [Service Account Setup](https://firebase.google.com/docs/admin/setup)
- [OAuth2 for Server to Server](https://developers.google.com/identity/protocols/oauth2/service-account)

## Support

For issues or questions:
1. Check Firebase Console logs
2. Review application logs: `docker logs corp-backend`
3. Test with FCM API Explorer: https://firebase.google.com/docs/cloud-messaging/send-message

## Timeline

- **Phase 1** (Week 1): Setup service account and test in development
- **Phase 2** (Week 2): Deploy to staging and test with real devices
- **Phase 3** (Week 3): Deploy to production with monitoring
- **Phase 4** (Week 4): Remove legacy API code after verification
