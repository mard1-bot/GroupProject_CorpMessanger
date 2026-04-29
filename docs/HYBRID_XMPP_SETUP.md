# Hybrid WebSocket + XMPP Architecture

## Overview

This messenger supports a hybrid architecture where both WebSocket clients and XMPP clients can communicate seamlessly. Messages are synchronized bidirectionally between PostgreSQL (WebSocket clients) and XMPP (ejabberd).

## Architecture

```
WebSocket Clients ←→ PostgreSQL ←→ Sync Service ←→ ejabberd API ←→ XMPP Clients
```

## Configuration

### Environment Variables

Add these to your `.env` file:

```bash
# Enable hybrid mode (WebSocket + XMPP)
HYBRID_MODE=true

# Enable XMPP synchronization
XMPP_SYNC_ENABLED=true

# ejabberd configuration
EJABBERD_HOST=localhost
EJABBERD_PORT=5280
EJABBERD_API_SECRET=your_api_secret
```

### Database Migration

Run the migration to add XMPP sync fields:

```bash
psql -U your_user -d your_database -f migrations/025_xmpp_sync_fields.sql
```

This adds:
- `synced_to_xmpp`: Boolean flag to track if message was synced to XMPP
- `xmpp_message_id`: ID of the corresponding XMPP message

## ejabberd Setup

### 1. Install ejabberd

```bash
# Ubuntu/Debian
sudo apt-get install ejabberd

# macOS
brew install ejabberd

# Docker
docker run -d --name ejabberd -p 5222:5222 -p 5280:5280 -p 5269:5269 ejabberd/ecs
```

### 2. Configure ejabberd

Edit `/etc/ejabberd/ejabberd.yml`:

```yaml
hosts:
  - "localhost"

listen:
  -
    port: 5222
    module: ejabberd_c2s
    max_stanza_size: 65536
    shaper: c2s_shaper
    access: c2s
  -
    port: 5269
    module: ejabberd_s2s_in
    max_stanza_size: 131072
  -
    port: 5280
    module: ejabberd_http
    request_handlers:
      "/api": ejabberd_http_api

api_permissions:
  "api access":
    from:
      - ejabberd_http
    who:
      - admin
    what:
      - "status"
      - "register"
      - "unregister"
      - "change_password"
      - "create_room"
      - "destroy_room"
      - "change_room_option"
      - "set_room_affiliation"
      - "send_message"
```

### 3. Enable HTTP API

Add to `ejabberd.yml`:

```yaml
modules:
  mod_http_api:
    api_access: [admin]
```

### 4. Set API Secret

Generate a secure API secret:

```bash
openssl rand -base64 32
```

Add to `ejabberd.yml`:

```yaml
listen:
  -
    port: 5280
    module: ejabberd_http
    http_bind: 0.0.0.0
    web_admin: true
    request_handlers:
      "/api": ejabberd_http_api

ejabberd_http_api:
  api_key: "your_generated_secret"
```

### 5. Enable MUC (Multi-User Chat)

```yaml
modules:
  mod_muc:
    access:
      - muc
    access_create: muc_create
    access_persistent: muc_create
    access_admin: muc_admin
    default_room_options:
      persistent: true
      public: false
```

### 6. Restart ejabberd

```bash
sudo systemctl restart ejabberd
# or
sudo ejabberdctl restart
```

## How It Works

### PostgreSQL → XMPP Sync

When a WebSocket client sends a message:
1. Message is stored in PostgreSQL
2. Sync service detects new messages (every 5 seconds)
3. Message is sent to XMPP via HTTP API
4. `synced_to_xmpp` flag is set to true

### XMPP → PostgreSQL Sync

When an XMPP client sends a message:
1. Message is stored in ejabberd
2. Sync service polls for new XMPP messages (every 10 seconds)
3. Message is converted and stored in PostgreSQL
4. WebSocket clients receive the message via WebSocket broadcast

## Testing

### Test XMPP Connection

```bash
curl -X GET http://localhost:5280/api/status \
  -H "Authorization: Bearer your_api_secret"
```

Expected response:
```json
{"status":"ok"}
```

### Test XMPP Message Sending

```bash
curl -X POST http://localhost:5280/api/send_message \
  -H "Authorization: Bearer your_api_secret" \
  -H "Content-Type: application/json" \
  -d '{
    "from": "user1@localhost",
    "to": "user2@localhost",
    "body": "Hello from XMPP",
    "type": "chat"
  }'
```

### Test Room Creation

```bash
curl -X POST http://localhost:5280/api/create_room \
  -H "Authorization: Bearer your_api_secret" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "room1",
    "service": "conference.localhost",
    "host": "localhost"
  }'
```

## Troubleshooting

### ejabberd API Not Responding

Check ejabberd logs:
```bash
sudo tail -f /var/log/ejabberd/ejabberd.log
```

Verify API is enabled in configuration.

### Sync Service Not Working

Check logs:
```bash
# Backend logs
tail -f backend/logs/app.log

# Look for:
# [XMPPSync] Starting sync service
# [XMPPSync] Synced message to XMPP
```

### Messages Not Syncing

1. Verify `HYBRID_MODE=true` and `XMPP_SYNC_ENABLED=true`
2. Check ejabberd API secret matches configuration
3. Verify ejabberd is running and accessible
4. Check network connectivity between backend and ejabberd

## XMPP Clients

You can connect with any XMPP client:

- **Gajim** (Desktop)
- **Psi** (Desktop)
- **Conversations** (Android)
- **Swift** (iOS)
- **Pidgin** (Cross-platform)

Connection settings:
- Server: `localhost` (or your ejabberd host)
- Port: `5222`
- Username: Your user UUID
- Password: Your user password

## Limitations

1. **Polling-based sync**: Current implementation uses polling instead of real-time XMPP listeners
2. **No XMPP library**: Uses HTTP API instead of direct XMPP protocol
3. **Message deduplication**: Requires database flags to prevent duplicate messages
4. **MUC only**: Currently supports group chats (MUC), not direct messages

## Future Improvements

1. Use XMPP library for real-time message receiving
2. Implement XMPP presence handling
3. Add support for direct messages (1:1)
4. Implement message deduplication using XMPP stanza IDs
5. Add XMPP file transfer support
