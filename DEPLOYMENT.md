# Deploying CorpMessenger to Production

This guide provides step-by-step instructions to deploy CorpMessenger (Backend, DB, Redis, LiveKit, WebRTC TURN Server, and the Web Frontend) to a production Linux server using Docker Compose and Nginx with Let's Encrypt SSL.

## 1. Prerequisites

1. A Linux server (Ubuntu 22.04/24.04 recommended) with at least 4GB RAM.
2. A domain name (e.g., `messenger.yourdomain.com`) with DNS A-records pointing to your server's IP address.
3. Docker and Docker Compose installed on the server.
   ```bash
   sudo apt update
   sudo apt install docker.io docker-compose -y
   ```

## 2. Clone the Repository

Log in to your server and clone the project repository:
```bash
git clone https://github.com/your-org/corpmessenger.git
cd corpmessenger
```

## 3. Environment Variables

Create a `.env` file from the example:
```bash
cp .env.example .env
```
Edit `.env` using `nano` or `vim` and fill in your domain and email:
```dotenv
DOMAIN_NAME=messenger.yourdomain.com
LETSENCRYPT_EMAIL=admin@yourdomain.com
TURN_EXTERNAL_IP=203.0.113.1 # REPLACE WITH YOUR SERVER'S PUBLIC IP
```

## 4. Generate Secrets

The project uses Docker Secrets to securely store passwords and keys (JWT, DB passwords, Coturn secrets, encryption keys, etc).

Run the secret generation script:
```bash
bash scripts/generate-secrets.sh
```

**Important**: After running the script, edit the domain-specific secrets to match your actual domain:
```bash
nano .secrets/cors_origins
# Set to: https://messenger.yourdomain.com

nano .secrets/livekit_url
# Set to: wss://messenger.yourdomain.com
```

## 5. Obtain Let's Encrypt SSL Certificate

Instead of dealing with self-signed certificates, use the provided script to automatically spin up a temporary Nginx server, complete the ACME challenge, and download valid certificates from Let's Encrypt:

```bash
sudo bash scripts/init-letsencrypt.sh
```
If successful, you will see `Let's Encrypt certificates obtained successfully!`.

## 6. Start the Services

Now you are ready to boot up the entire stack. This will build the frontend web app, the backend API, and start Postgres, Redis, Coturn, LiveKit, and Nginx.

```bash
sudo docker-compose -f docker-compose.prod.yml up -d --build
```

### Check Status
Verify that all containers are `Up (healthy)`:
```bash
sudo docker-compose -f docker-compose.prod.yml ps
```

## 7. Next Steps & Administration

- **Access the App**: Navigate to `https://messenger.yourdomain.com` in your browser.
- **Admin Panel**: Register the first user, then optionally elevate them to an administrator via the database or the provided scripts.
- **Logs**: If something goes wrong, view logs with:
  ```bash
  sudo docker-compose -f docker-compose.prod.yml logs -f backend
  sudo docker-compose -f docker-compose.prod.yml logs -f nginx
  ```
- **SSL Auto-Renewal**: The `certbot` container runs in the background and will automatically renew your certificates every 12 hours if they are close to expiring.
