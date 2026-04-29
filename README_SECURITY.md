# Security Configuration Guide

## Docker Secrets Setup

This project uses Docker secrets for managing sensitive data in production.

### Files Created:
- `docker-compose.prod.yml` - Production configuration with Docker secrets
- `.secrets/` directory - Contains secret files for Docker secrets

### Secret Files:
- `.secrets/postgres_password` - PostgreSQL password
- `.secrets/database_url` - Full database connection string
- `.secrets/jwt_secret` - JWT signing secret (minimum 32 characters)
- `.secrets/ejabberd_password` - XMPP admin password
- `.secrets/livekit_url` - LiveKit server URL
- `.secrets/livekit_api_key` - LiveKit API key
- `.secrets/livekit_api_secret` - LiveKit API secret (minimum 32 characters)
- `.secrets/livekit_keys` - LiveKit keys in format `key:secret`
- `.secrets/encryption_master_key` - Master encryption key
- `.secrets/cors_origins` - Allowed CORS origins
- `.secrets/tls_cert_path` - TLS certificate path
- `.secrets/tls_key_path` - TLS private key path

### Usage:

#### Development:
```bash
docker-compose up -d
```

#### Production:
```bash
docker-compose -f docker-compose.prod.yml up -d
```

### Security Features Implemented:

1. **Container Security:**
   - `no-new-privileges:true` - Prevents privilege escalation
   - `read_only:true` - Containers run in read-only mode
   - `tmpfs` for temporary directories - Prevents disk persistence
   - Minimal container privileges

2. **Secrets Management:**
   - All sensitive data stored in Docker secrets
   - No hardcoded passwords in docker-compose files
   - Secrets mounted as files in `/run/secrets/`

3. **Network Security:**
   - Internal services communicate through Docker network
   - Only necessary ports exposed
   - WebSocket CORS validation with IP logging

4. **Rate Limiting:**
   - Redis-based rate limiting for API endpoints
   - Database-based login attempt tracking
   - Account lockout after failed attempts

### Production Setup Checklist:

1. **Update Secrets:**
   ```bash
   # Change all default passwords and secrets
   echo "your-secure-password" > .secrets/postgres_password
   echo "your-32-character-jwt-secret" > .secrets/jwt_secret
   echo "your-32-character-livekit-secret" > .secrets/livekit_api_secret
   ```

2. **TLS Certificates:**
   ```bash
   # Place certificates in /certs directory
   mkdir -p certs
   cp your-server.crt certs/server.crt
   cp your-server.key certs/server.key
   ```

3. **CORS Origins:**
   ```bash
   # Update allowed origins
   echo "https://yourdomain.com,https://app.yourdomain.com" > .secrets/cors_origins
   ```

4. **File Permissions:**
   ```bash
   # Restrict secret file permissions
   chmod 600 .secrets/*
   chmod 700 .secrets
   ```

### Monitoring:
- All rejected WebSocket origins are logged with IP addresses
- Failed login attempts tracked in database
- Rate limit violations logged

### Backup Strategy:
- Database data persisted in `./data/postgres/`
- Redis data persisted in `./data/redis/`
- Upload files persisted in `./data/uploads/`
- ejabberd data persisted in `./data/ejabberd/`

### Additional Recommendations:
1. Use external secret management (HashiCorp Vault, AWS Secrets Manager) in production
2. Enable HTTPS with proper certificates
3. Set up monitoring and alerting
4. Regular security audits
5. Implement network segmentation
6. Use WAF (Web Application Firewall)
