#!/usr/bin/env bash
# Generates self-signed TLS certificates for development
# For production, use Let's Encrypt or purchase commercial certificates
# Run: bash scripts/generate-certs.sh

set -euo pipefail

CERTS_DIR="$(cd "$(dirname "$0")/.." && pwd)/certs"
mkdir -p "$CERTS_DIR"
chmod 700 "$CERTS_DIR"

echo "[generate-certs] Generating self-signed certificates..."

# Check if certificates already exist
if [ -f "$CERTS_DIR/server.crt" ] && [ -f "$CERTS_DIR/server.key" ]; then
    echo "⚠  Certificates already exist at $CERTS_DIR"
    echo "   To regenerate, delete them first:"
    echo "   rm $CERTS_DIR/server.crt $CERTS_DIR/server.key"
    exit 1
fi

# Generate private key
openssl genrsa -out "$CERTS_DIR/server.key" 4096
chmod 600 "$CERTS_DIR/server.key"

# Generate certificate signing request
openssl req -new -key "$CERTS_DIR/server.key" -out "$CERTS_DIR/server.csr" \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=CorpMessenger/OU=IT/CN=localhost"

# Generate self-signed certificate (valid for 365 days)
openssl x509 -req -days 365 -in "$CERTS_DIR/server.csr" \
    -signkey "$CERTS_DIR/server.key" -out "$CERTS_DIR/server.crt"

# Clean up CSR
rm "$CERTS_DIR/server.csr"

chmod 644 "$CERTS_DIR/server.crt"

echo "[generate-certs] Done!"
echo "  ✓ Private key: $CERTS_DIR/server.key"
echo "  ✓ Certificate: $CERTS_DIR/server.crt"
echo ""
echo "⚠  WARNING: These are self-signed certificates for development only!"
echo "   For production, use Let's Encrypt or purchase commercial certificates."
echo ""
echo "   To use Let's Encrypt:"
echo "   1. Install certbot: sudo apt-get install certbot"
echo "   2. Generate certificates: sudo certbot certonly --standalone -d yourdomain.com"
echo "   3. Copy to certs/: sudo cp /etc/letsencrypt/live/yourdomain.com/fullchain.pem $CERTS_DIR/server.crt"
echo "   4. Copy to certs/: sudo cp /etc/letsencrypt/live/yourdomain.com/privkey.pem $CERTS_DIR/server.key"
