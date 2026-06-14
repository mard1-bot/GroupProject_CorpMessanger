#!/bin/bash
# init-letsencrypt.sh
# Automates the initial acquisition of Let's Encrypt certificates for the Nginx proxy.

if ! [ -x "$(command -v docker-compose)" ]; then
  echo 'Error: docker-compose is not installed.' >&2
  exit 1
fi

# Load variables from .env
if [ -f .env ]; then
  export $(cat .env | grep -v '^#' | xargs)
else
  echo "Error: .env file not found. Please create one from .env.example."
  exit 1
fi

DOMAIN=${DOMAIN_NAME:-""}
EMAIL=${LETSENCRYPT_EMAIL:-""}

if [ -z "$DOMAIN" ]; then
  echo "Error: DOMAIN_NAME is not set in .env"
  exit 1
fi

if [ -z "$EMAIL" ]; then
  echo "Error: LETSENCRYPT_EMAIL is not set in .env"
  exit 1
fi

data_path="./certs"
rsa_key_size=4096

echo "### Requesting Let's Encrypt certificate for $DOMAIN ..."
# Join $DOMAIN to -d args
domain_args="-d $DOMAIN"

# Enable staging mode if needed
# staging_arg="--staging"
staging_arg=""

# Stop nginx if it's running
docker-compose -f docker-compose.prod.yml stop nginx
docker-compose -f docker-compose.prod.yml stop certbot

echo "### Starting nginx in HTTP-only mode to handle challenge..."
docker-compose -f docker-compose.prod.yml up -d nginx

echo "### Deleting dummy certificate for $DOMAIN ..."
docker-compose -f docker-compose.prod.yml run --rm --entrypoint "\
  rm -Rf /etc/letsencrypt/live/$DOMAIN && \
  rm -Rf /etc/letsencrypt/archive/$DOMAIN && \
  rm -Rf /etc/letsencrypt/renewal/$DOMAIN.conf" certbot
echo

echo "### Requesting Let's Encrypt certificate for $DOMAIN ..."
docker-compose -f docker-compose.prod.yml run --rm --entrypoint "\
  certbot certonly --webroot -w /var/www/certbot \
    $staging_arg \
    $domain_args \
    --email $EMAIL \
    --rsa-key-size $rsa_key_size \
    --agree-tos \
    --force-renewal" certbot

echo "### Copying certs to root of ./certs for Nginx configuration compatibility..."
# Nginx expects /certs/server.crt and /certs/server.key
docker-compose -f docker-compose.prod.yml run --rm --entrypoint "\
  cp /etc/letsencrypt/live/$DOMAIN/fullchain.pem /etc/letsencrypt/server.crt && \
  cp /etc/letsencrypt/live/$DOMAIN/privkey.pem /etc/letsencrypt/server.key" certbot

echo "### Restarting Nginx ..."
docker-compose -f docker-compose.prod.yml exec nginx nginx -s reload

echo "Let's Encrypt certificates obtained successfully!"
