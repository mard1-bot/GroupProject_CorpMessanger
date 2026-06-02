#!/usr/bin/env bash
# Generates cryptographically random production secrets into .secrets/
# Run once before first deploy: bash scripts/generate-secrets.sh
# NEVER commit the .secrets/ directory.

set -euo pipefail

SECRETS_DIR="$(cd "$(dirname "$0")/.." && pwd)/.secrets"
mkdir -p "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"

rand_base64() { openssl rand -base64 "$1" | tr -d '\n='; }

echo "[generate-secrets] Writing secrets to $SECRETS_DIR ..."

# PostgreSQL
if [ ! -f "$SECRETS_DIR/postgres_password" ] || [ "${FORCE:-0}" = "1" ]; then
  rand_base64 32 > "$SECRETS_DIR/postgres_password"
  echo "  ✓ postgres_password"
fi

# Database URL — uses the generated postgres_password
PG_PASS=$(cat "$SECRETS_DIR/postgres_password")
echo "postgres://corpmessenger:${PG_PASS}@postgres:5432/corpmessenger?sslmode=require" \
  > "$SECRETS_DIR/database_url"
echo "  ✓ database_url"

# JWT secret (min 64 chars for HS256)
if [ ! -f "$SECRETS_DIR/jwt_secret" ] || [ "${FORCE:-0}" = "1" ]; then
  rand_base64 48 > "$SECRETS_DIR/jwt_secret"
  echo "  ✓ jwt_secret"
fi

# Encryption master key (AES-256 = 32 bytes → base64)
if [ ! -f "$SECRETS_DIR/encryption_master_key" ] || [ "${FORCE:-0}" = "1" ]; then
  openssl rand -base64 32 | tr -d '\n' > "$SECRETS_DIR/encryption_master_key"
  echo "  ✓ encryption_master_key"
fi

# LiveKit
if [ ! -f "$SECRETS_DIR/livekit_api_key" ] || [ "${FORCE:-0}" = "1" ]; then
  rand_base64 12 > "$SECRETS_DIR/livekit_api_key"
  echo "  ✓ livekit_api_key"
fi
if [ ! -f "$SECRETS_DIR/livekit_api_secret" ] || [ "${FORCE:-0}" = "1" ]; then
  rand_base64 40 > "$SECRETS_DIR/livekit_api_secret"
  echo "  ✓ livekit_api_secret"
fi

LK_KEY=$(cat "$SECRETS_DIR/livekit_api_key")
LK_SECRET=$(cat "$SECRETS_DIR/livekit_api_secret")
echo "${LK_KEY}: ${LK_SECRET}" > "$SECRETS_DIR/livekit_keys"
echo "  ✓ livekit_keys"

# ejabberd
if [ ! -f "$SECRETS_DIR/ejabberd_password" ] || [ "${FORCE:-0}" = "1" ]; then
  rand_base64 24 > "$SECRETS_DIR/ejabberd_password"
  echo "  ✓ ejabberd_password"
fi

# TURN server
if [ ! -f "$SECRETS_DIR/turn_password" ] || [ "${FORCE:-0}" = "1" ]; then
  rand_base64 32 > "$SECRETS_DIR/turn_password"
  echo "  ✓ turn_password"
fi

# CORS origins — must be set manually to your real domain
if [ ! -f "$SECRETS_DIR/cors_origins" ]; then
  echo "https://your-domain.com" > "$SECRETS_DIR/cors_origins"
  echo "  ✓ cors_origins  ⚠  Edit .secrets/cors_origins with your real domain!"
fi

# LiveKit URL — must be set manually
if [ ! -f "$SECRETS_DIR/livekit_url" ]; then
  echo "wss://livekit.your-domain.com" > "$SECRETS_DIR/livekit_url"
  echo "  ✓ livekit_url  ⚠  Edit .secrets/livekit_url with your real LiveKit URL!"
fi

chmod 600 "$SECRETS_DIR"/*
echo ""
echo "[generate-secrets] Done. Secrets written to $SECRETS_DIR/"
echo ""
echo "⚠  Next steps:"
echo "   1. Edit .secrets/cors_origins — set your real domain(s)"
echo "   2. Edit .secrets/livekit_url  — set your LiveKit server URL"
echo "   3. Place TLS certificate at ./certs/server.crt and ./certs/server.key"
echo "   4. NEVER commit the .secrets/ directory"
