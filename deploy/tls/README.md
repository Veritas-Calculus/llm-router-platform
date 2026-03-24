# TLS Termination Configuration

Production deployment templates for TLS termination in front of the LLM Router.

## Quick Start

### Option A: Caddy (Recommended for simplicity)

```bash
# 1. Install Caddy
sudo apt install -y caddy   # or brew install caddy

# 2. Copy and edit the Caddyfile
cp caddy/Caddyfile /etc/caddy/Caddyfile
# Edit: replace {$DOMAIN} with your domain, or set DOMAIN env var

# 3. Start Caddy (automatic HTTPS via Let's Encrypt)
sudo systemctl enable --now caddy
```

Caddy handles certificate issuance, renewal, and OCSP stapling automatically.

### Option B: Nginx

```bash
# 1. Obtain certificates (e.g., via certbot)
sudo certbot certonly --standalone -d your-domain.com

# 2. Copy and edit the Nginx config
cp nginx/nginx.conf /etc/nginx/sites-available/llm-router
# Edit: replace YOUR_DOMAIN, update certificate paths
ln -s /etc/nginx/sites-available/llm-router /etc/nginx/sites-enabled/

# 3. Test and reload
sudo nginx -t && sudo systemctl reload nginx
```

## Key Configuration Notes

| Setting | Value | Rationale |
|---------|-------|-----------|
| TLS Protocols | 1.2, 1.3 | Modern security baseline |
| Cipher Suite | Mozilla Modern | AEAD-only, forward secrecy |
| HSTS | 2 years + preload | Prevent protocol downgrade |
| `proxy_read_timeout` | 120s | LLM streaming can be long-running |
| `proxy_buffering` | off | Required for SSE/streaming |

## Docker Compose Integration

Add Caddy as a sidecar in `docker-compose.yml`:

```yaml
services:
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./deploy/tls/caddy/Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
    depends_on:
      - server

volumes:
  caddy_data:
```
