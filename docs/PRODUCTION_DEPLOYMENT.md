# Production Deployment Guide

This guide covers deploying CorpMessenger to production servers.

## Prerequisites

- Ubuntu 20.04+ or similar Linux server
- Docker and Docker Compose installed
- Domain name with DNS configured
- SSL certificate (Let's Encrypt or commercial)
- At least 4GB RAM, 2 CPU cores
- 50GB+ storage for database and uploads

## 1. Server Preparation

### Install Docker and Docker Compose

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER

# Install Docker Compose
sudo apt install docker-compose-plugin -y

# Verify installation
docker --version
docker compose version
```

### Configure Firewall

```bash
# Allow SSH
sudo ufw allow 22/tcp

# Allow HTTP/HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# Allow TURN server ports
sudo ufw allow 3478/tcp
sudo ufw allow 3478/udp
sudo ufw allow 5349/tcp
sudo ufw allow 5349/udp

# Allow LiveKit ports
sudo ufw allow 7880/tcp
sudo ufw allow 7881/tcp
sudo ufw allow 7882/udp

# Enable firewall
sudo ufw enable
```

## 2. Deploy Application

### Clone Repository

```bash
cd /opt
sudo git clone https://github.com/mard1-bot/GroupProject_CorpMessanger.git
cd GroupProject_CorpMessanger
```

### Generate Secrets

```bash
# Generate all production secrets
bash scripts/generate-secrets.sh

# Edit critical secrets
nano .secrets/cors_origins  # Set to your domain: https://yourdomain.com
nano .secrets/livekit_url   # Set to your LiveKit URL: wss://livekit.yourdomain.com
```

### Generate TLS Certificates

#### Option A: Let's Encrypt (Recommended)

```bash
# Install certbot
sudo apt install certbot -y

# Generate certificates
sudo certbot certonly --standalone -d yourdomain.com -d www.yourdomain.com

# Copy to project directory
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem certs/server.crt
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem certs/server.key
sudo chown $USER:$USER certs/server.crt certs/server.key
```

#### Option B: Self-Signed (Development Only)

```bash
bash scripts/generate-certs.sh
```

### Configure TURN Server

```bash
# Set your public IP
export TURN_EXTERNAL_IP=$(curl -s ifconfig.me)

# Add to .env or docker-compose.prod.yml
echo "TURN_EXTERNAL_IP=$TURN_EXTERNAL_IP" >> .env
```

### Start Services

```bash
# Start all services
docker compose -f docker-compose.prod.yml up -d

# Check status
docker compose -f docker-compose.prod.yml ps

# View logs
docker compose -f docker-compose.prod.yml logs -f
```

## 3. Initial Setup

### Set Admin Password

```bash
docker compose -f docker-compose.prod.yml exec backend /app/api setadminpass "YourSecurePassword123!"
```

### Verify Health

```bash
curl https://yourdomain.com/health
```

Expected response: `{"status":"ok"}`

## 4. Configure Backups

### Setup Cron Job

```bash
# Edit crontab
crontab -e

# Add daily backup at 2 AM
0 2 * * * /opt/GroupProject_CorpMessanger/scripts/backup-postgres.sh >> /var/log/postgres-backup.log 2>&1
```

### Manual Backup Test

```bash
bash scripts/backup-postgres.sh
```

Check backups in `backups/postgres/`.

## 5. Configure Mobile Apps

### Build with EAS

```bash
cd client

# Install EAS CLI
npm install -g eas-cli

# Login to Expo
eas login

# Configure build
eas build:configure

# Build for iOS
eas build --platform ios --profile production

# Build for Android
eas build --platform android --profile production
```

### Update App Configuration

Edit `client/.env`:

```bash
EXPO_PUBLIC_BACKEND_IP=yourdomain.com
EXPO_PUBLIC_BACKEND_PORT=443
EXPO_PUBLIC_TURN_SERVER_URI=turn:yourdomain.com:3478?transport=tcp
EXPO_PUBLIC_TURN_USERNAME=turnuser
EXPO_PUBLIC_TURN_PASSWORD=your-turn-password
```

## 6. Monitoring

### View Logs

```bash
# All services
docker compose -f docker-compose.prod.yml logs -f

# Specific service
docker compose -f docker-compose.prod.yml logs -f backend
docker compose -f docker-compose.prod.yml logs -f nginx
```

### Check Disk Space

```bash
df -h
```

### Check Resource Usage

```bash
docker stats
```

## 7. Updates and Maintenance

### Update Application

```bash
cd /opt/GroupProject_CorpMessanger

# Pull latest changes
git pull origin main

# Rebuild and restart
docker compose -f docker-compose.prod.yml up -d --build

# Run migrations
docker compose -f docker-compose.prod.yml exec backend /app/api migrate
```

### Update TLS Certificates (Let's Encrypt)

```bash
# Renew certificates
sudo certbot renew

# Copy to project
sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem certs/server.crt
sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem certs/server.key
sudo chown $USER:$USER certs/server.crt certs/server.key

# Restart nginx
docker compose -f docker-compose.prod.yml restart nginx
```

### Cleanup Old Backups

```bash
# Keep last 7 daily backups
find backups/postgres -name "corpmessenger_*.sql.gz" -type f -mtime +7 -delete

# Keep last 4 weekly backups
find backups/postgres/weekly -name "corpmessenger_weekly_*.sql.gz" -type f -mtime +28 -delete

# Keep last 12 monthly backups
find backups/postgres/monthly -name "corpmessenger_monthly_*.sql.gz" -type f -mtime +365 -delete
```

## 8. Troubleshooting

### Service Won't Start

```bash
# Check logs
docker compose -f docker-compose.prod.yml logs service-name

# Check disk space
df -h

# Check memory
free -h
```

### Database Connection Issues

```bash
# Check postgres health
docker compose -f docker-compose.prod.yml exec postgres pg_isready -U corpmessenger

# Check database logs
docker compose -f docker-compose.prod.yml logs postgres
```

### TURN Server Issues

```bash
# Test TURN server
docker compose -f docker-compose.prod.yml logs coturn

# Verify external IP
curl ifconfig.me
```

### Nginx Issues

```bash
# Test nginx configuration
docker compose -f docker-compose.prod.yml exec nginx nginx -t

# Reload nginx
docker compose -f docker-compose.prod.yml exec nginx nginx -s reload
```

## 9. Security Checklist

- [ ] All secrets generated and stored securely
- [ ] TLS certificates installed and valid
- [ ] Firewall configured correctly
- [ ] Database backups automated
- [ ] Admin password set to strong value
- [ ] TURN server configured with public IP
- [ ] CORS origins set to production domain
- [ ] Security headers configured in nginx
- [ ] Rate limiting enabled
- [ ] Docker secrets used for sensitive data
- [ ] File permissions set correctly (600 for secrets)
- [ ] Regular security updates applied

## 10. Performance Tuning

### PostgreSQL Tuning

Edit `docker-compose.prod.yml` postgres service:

```yaml
postgres:
  command:
    - postgres
    - -c
    - shared_buffers=256MB
    - -c
    - max_connections=200
    - -c
    - work_mem=4MB
```

### Redis Tuning

```yaml
redis:
  command: redis-server --maxmemory 512mb --maxmemory-policy allkeys-lru
```

### Nginx Tuning

Edit `nginx/nginx.conf`:

```nginx
worker_processes auto;
worker_connections 2048;
```

## 11. Disaster Recovery

### Restore from Backup

```bash
# Extract backup
gunzip backups/postgres/corpmessenger_YYYYMMDD_HHMMSS.sql.gz

# Restore to database
docker compose -f docker-compose.prod.yml exec -T postgres psql -U corpmessenger -d corpmessenger < backups/postgres/corpmessenger_YYYYMMDD_HHMMSS.sql
```

### Full Server Recovery

1. Restore from backup
2. Re-run deployment steps 1-3
3. Restore secrets from secure backup
4. Restart services

## 12. Support

For issues:
- Check logs: `docker compose -f docker-compose.prod.yml logs -f`
- Review documentation in `docs/`
- Check GitHub issues: https://github.com/mard1-bot/GroupProject_CorpMessanger/issues
