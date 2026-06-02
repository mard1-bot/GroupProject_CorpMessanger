# Error Tracking with Glitchtip (Sentry-compatible)

This project uses **Glitchtip** as a self-hosted, Sentry-compatible error tracking solution. Glitchtip provides the same API as Sentry but is open-source and can be self-hosted, giving you full control over your error data.

## What is Glitchtip?

Glitchtip is an open-source error tracking platform that is compatible with the Sentry SDK. It allows you to:
- Track errors and exceptions in real-time
- Monitor performance issues
- Get alerts for critical errors
- Analyze error trends and patterns
- Keep your error data private (self-hosted)

## Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Client    │────────▶│   Backend   │────────▶│  Glitchtip  │
│ (React Native)│ Sentry SDK  │ (Go) Sentry │  (Server)   │
└─────────────┘         └─────────────┘         └─────────────┘
                              │
                              ▼
                        ┌─────────────┐
                        │ PostgreSQL  │
                        │ (Database)  │
                        └─────────────┘
```

## Setup

### 1. Start Glitchtip

Glitchtip is included in the production Docker Compose configuration:

```bash
# Generate secrets
bash scripts/generate-secrets.sh

# Start all services including Glitchtip
docker compose -f docker-compose.prod.yml up -d
```

Glitchtip will be available at `http://your-server:8000`

### 2. Initialize Glitchtip

1. Open `http://your-server:8000` in your browser
2. Create an admin account
3. Create a new project (e.g., "CorpMessenger Backend" and "CorpMessenger Client")
4. Copy the DSN (Data Source Name) for each project

### 3. Configure Sentry DSN

Edit the generated secrets file:

```bash
nano .secrets/sentry_dsn
```

Replace the placeholder with your actual Glitchtip DSN:
```
https://your-dsn@glitchtip.yourdomain.com/project-id
```

### 4. Restart Backend

```bash
docker compose -f docker-compose.prod.yml restart backend
```

## Backend Integration (Go)

The backend uses the official Sentry Go SDK (`github.com/getsentry/sentry-go`).

### Configuration

Set the `SENTRY_DSN` environment variable in `backend/.env`:

```bash
SENTRY_DSN=https://your-dsn@glitchtip.yourdomain.com/project-id
```

### Initialization

Sentry is automatically initialized in `backend/internal/app/app.go`:

```go
if cfg.SentryDSN != "" {
    err := sentry.Init(sentry.ClientOptions{
        Dsn:              cfg.SentryDSN,
        Environment:      cfg.AppEnv,
        Release:          "1.0.0",
        TracesSampleRate: 1.0,
    })
    if err != nil {
        logg.Warn("failed to initialize Sentry", "error", err)
    } else {
        logg.Info("Sentry initialized", "dsn", cfg.SentryDSN)
    }
}
```

### Manual Error Reporting

To manually capture errors:

```go
import "github.com/getsentry/sentry-go"

// Capture an exception
sentry.CaptureException(err)

// Capture a message
sentry.CaptureMessage("Something went wrong")

// Add breadcrumbs
sentry.AddBreadcrumb(&sentry.Breadcrumb{
    Category: "user",
    Message:  "User performed action",
    Level:    sentry.LevelInfo,
})
```

## Client Integration (React Native)

The client uses the Sentry React Native SDK (`@sentry/react-native`).

### Configuration

Set the `EXPO_PUBLIC_SENTRY_DSN` environment variable in `client/.env`:

```bash
EXPO_PUBLIC_SENTRY_DSN=https://your-dsn@glitchtip.yourdomain.com/project-id
```

### Initialization

Sentry is automatically initialized in `client/app/_layout.tsx`:

```typescript
const SENTRY_DSN = process.env.EXPO_PUBLIC_SENTRY_DSN || '';
if (SENTRY_DSN) {
  Sentry.init({
    dsn: SENTRY_DSN,
    debug: __DEV__,
    environment: __DEV__ ? 'development' : 'production',
    tracesSampleRate: 1.0,
  });
}
```

### Manual Error Reporting

To manually capture errors:

```typescript
import * as Sentry from '@sentry/react-native';

// Capture an exception
try {
  // Your code
} catch (error) {
  Sentry.captureException(error);
}

// Capture a message
Sentry.captureMessage('Something went wrong');

// Add breadcrumbs
Sentry.addBreadcrumb({
  category: 'user',
  message: 'User performed action',
  level: 'info',
});
```

## Using Glitchtip Dashboard

### Viewing Errors

1. Navigate to your project in Glitchtip
2. Click on "Issues" to see all errors
3. Filter by environment, level, or time range

### Error Details

Each error includes:
- Stack trace
- Request data (for backend errors)
- User information (if available)
- Breadcrumbs (context leading to the error)
- Tags (environment, release, etc.)

### Alerts

Set up alerts to be notified of critical errors:

1. Go to Project Settings → Alerts
2. Create new alert rules
3. Configure notification channels (email, Slack, etc.)

## Best Practices

### 1. Don't Track Sensitive Data

Glitchtip automatically filters common sensitive fields, but be careful:

```go
// Bad - includes password
sentry.CaptureException(err, sentry.WithScope(func(scope *sentry.Scope) {
    scope.SetExtra("password", userPassword)
}))

// Good - excludes sensitive data
sentry.CaptureException(err, sentry.WithScope(func(scope *sentry.Scope) {
    scope.SetExtra("user_id", userID)
}))
```

### 2. Use Breadcrumbs for Context

Add breadcrumbs to provide context for errors:

```go
sentry.AddBreadcrumb(&sentry.Breadcrumb{
    Category: "auth",
    Message:  "User logged in",
    Level:    sentry.LevelInfo,
    Data: map[string]interface{}{
        "user_id": userID,
    },
})
```

### 3. Set Release Information

Track errors by release:

```go
sentry.Init(sentry.ClientOptions{
    Release: "1.0.0", // or use git commit hash
})
```

### 4. Use Environment Tags

Separate development and production errors:

```go
sentry.Init(sentry.ClientOptions{
    Environment: cfg.AppEnv, // "dev", "prod", etc.
})
```

### 5. Don't Swallow Errors

Always capture errors before handling them:

```go
// Bad
if err != nil {
    log.Error("error occurred", "error", err)
    return err
}

// Good
if err != nil {
    sentry.CaptureException(err)
    log.Error("error occurred", "error", err)
    return err
}
```

## Troubleshooting

### Errors Not Appearing in Glitchtip

1. Check that the DSN is correct
2. Verify the environment variable is set
3. Check backend logs for Sentry initialization errors
4. Ensure Glitchtip is running: `docker compose -f docker-compose.prod.yml ps glitchtip`

### Too Many Errors

1. Adjust the sampling rate in Sentry initialization
2. Filter out expected errors in Glitchtip settings
3. Use error grouping to reduce noise

### Performance Impact

Sentry has minimal performance impact:
- Asynchronous error reporting
- Sampling for performance monitoring
- No blocking operations

## Privacy and Security

- Glitchtip is self-hosted, so your error data stays on your servers
- All secrets are stored in Docker secrets (`.secrets/` directory)
- The `.secrets/` directory is excluded from Git
- Use HTTPS in production for secure communication

## Alternatives

If you prefer not to self-host, you can use:
- **Sentry Cloud** (https://sentry.io) - Commercial hosted solution
- **Rollbar** (https://rollbar.com) - Alternative error tracking service

## Resources

- Glitchtip Documentation: https://glitchtip.com/documentation
- Sentry Go SDK: https://docs.sentry.io/platforms/go/
- Sentry React Native SDK: https://docs.sentry.io/platforms/react-native/
- Sentry Best Practices: https://docs.sentry.io/platforms/go/best-practices/
