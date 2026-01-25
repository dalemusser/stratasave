# StrataSave Production Deployment Guide

This guide covers deploying StrataSave to a production environment.

---

## Production Checklist

Use this checklist before going live:

### Security

- [ ] **Session key**: Generate a strong, random 32+ character key
  ```bash
  openssl rand -base64 32
  ```
- [ ] **CSRF key**: Generate a strong, random 32+ character key
- [ ] **Environment**: Set `env = "prod"`
- [ ] **HTTPS**: Enable `use_https = true` with valid certificates
- [ ] **Session domain**: Set appropriately for your domain
- [ ] **API key**: If using API access, generate a strong key
- [ ] **Default credentials**: Remove or change any default passwords
- [ ] **Seed admin**: Configure `seed_admin_email` for initial admin user

### Database

- [ ] **MongoDB URI**: Use authenticated connection string
  ```
  mongodb://user:password@host:27017/stratasave?authSource=admin
  ```
- [ ] **Connection pooling**: Tune `mongo_max_pool_size` and `mongo_min_pool_size`
- [ ] **Replica set**: Consider using a replica set for high availability
- [ ] **Backups**: Configure automated MongoDB backups
- [ ] **Indexes**: Verify indexes are created on startup (check logs)

### Email

- [ ] **SMTP credentials**: Configure production SMTP server
- [ ] **From address**: Use a valid, monitored email address
- [ ] **SPF/DKIM/DMARC**: Configure DNS records for email deliverability
- [ ] **Base URL**: Set to your production domain for email links

### File Storage

- [ ] **Storage backend**: Choose `local` or `s3`
- [ ] **Local storage**: Ensure directory exists and has correct permissions
- [ ] **S3 storage**: Configure bucket, region, and IAM credentials
- [ ] **CloudFront**: Optional CDN for S3 with signed URLs
- [ ] **Backup**: Include uploaded files in backup strategy

### OAuth (if using Google OAuth)

- [ ] **Client credentials**: Configure production OAuth app
- [ ] **Redirect URIs**: Add production domain to allowed redirects
- [ ] **Consent screen**: Configure OAuth consent screen for production

### Monitoring

- [ ] **Health endpoint**: Verify `/health` returns 200
- [ ] **Logging**: Set `log_level = "info"` or `"warn"` for production
- [ ] **Audit logging**: Enable `audit_log_auth` and `audit_log_admin`
- [ ] **Error tracking**: Consider integrating error tracking service

### Performance

- [ ] **Compression**: Enable `enable_compression = true`
- [ ] **Timeouts**: Review and adjust HTTP timeouts for your use case
- [ ] **Rate limiting**: Enable and tune rate limiting settings
- [ ] **Idle logout**: Configure if required for compliance

### Infrastructure

- [ ] **Reverse proxy**: Configure nginx/caddy/traefik in front of app
- [ ] **Firewall**: Restrict access to necessary ports only
- [ ] **Docker**: Use non-root user (already configured in Dockerfile)
- [ ] **Resources**: Set appropriate CPU/memory limits
- [ ] **Restart policy**: Configure automatic restarts on failure

---

## Environment Variables

All configuration can be set via environment variables with the `STRATASAVE_` prefix:

```bash
# Required for production
export STRATASAVE_ENV=prod
export STRATASAVE_SESSION_KEY="your-secure-32-char-session-key-here"
export STRATASAVE_CSRF_KEY="your-secure-32-char-csrf-key-here"
export STRATASAVE_MONGO_URI="mongodb://user:pass@host:27017/stratasave"
export STRATASAVE_BASE_URL="https://yourdomain.com"

# HTTPS (Let's Encrypt)
export STRATASAVE_USE_HTTPS=true
export STRATASAVE_USE_LETS_ENCRYPT=true
export STRATASAVE_LETS_ENCRYPT_EMAIL="admin@yourdomain.com"
export STRATASAVE_DOMAIN="yourdomain.com"

# Email
export STRATASAVE_MAIL_SMTP_HOST="smtp.example.com"
export STRATASAVE_MAIL_SMTP_PORT=587
export STRATASAVE_MAIL_SMTP_USER="smtp-user"
export STRATASAVE_MAIL_SMTP_PASS="smtp-password"
export STRATASAVE_MAIL_FROM="noreply@yourdomain.com"

# Optional: Google OAuth
export STRATASAVE_GOOGLE_CLIENT_ID="your-client-id"
export STRATASAVE_GOOGLE_CLIENT_SECRET="your-client-secret"

# Optional: S3 Storage
export STRATASAVE_STORAGE_TYPE=s3
export STRATASAVE_STORAGE_S3_REGION="us-east-1"
export STRATASAVE_STORAGE_S3_BUCKET="your-bucket"

# Optional: Initial admin
export STRATASAVE_SEED_ADMIN_EMAIL="admin@yourdomain.com"
```

---

## Docker Deployment

### Build the Image

```bash
docker build -t stratasave:latest .
```

### Run with Docker

```bash
docker run -d \
  --name stratasave \
  --restart unless-stopped \
  -p 8080:8080 \
  -e STRATASAVE_ENV=prod \
  -e STRATASAVE_SESSION_KEY="your-session-key" \
  -e STRATASAVE_CSRF_KEY="your-csrf-key" \
  -e STRATASAVE_MONGO_URI="mongodb://host:27017/stratasave" \
  -e STRATASAVE_BASE_URL="https://yourdomain.com" \
  -v /path/to/uploads:/app/uploads \
  stratasave:latest
```

### Docker Compose

```yaml
version: '3.8'

services:
  stratasave:
    build: .
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      STRATASAVE_ENV: prod
      STRATASAVE_SESSION_KEY: ${SESSION_KEY}
      STRATASAVE_CSRF_KEY: ${CSRF_KEY}
      STRATASAVE_MONGO_URI: mongodb://mongo:27017/stratasave
      STRATASAVE_BASE_URL: https://yourdomain.com
      STRATASAVE_MAIL_SMTP_HOST: ${SMTP_HOST}
      STRATASAVE_MAIL_SMTP_PORT: ${SMTP_PORT}
      STRATASAVE_MAIL_SMTP_USER: ${SMTP_USER}
      STRATASAVE_MAIL_SMTP_PASS: ${SMTP_PASS}
    volumes:
      - uploads:/app/uploads
    depends_on:
      - mongo
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3

  mongo:
    image: mongo:7
    restart: unless-stopped
    volumes:
      - mongo_data:/data/db
    # In production, add authentication:
    # environment:
    #   MONGO_INITDB_ROOT_USERNAME: admin
    #   MONGO_INITDB_ROOT_PASSWORD: ${MONGO_PASSWORD}

volumes:
  uploads:
  mongo_data:
```

---

## Reverse Proxy Configuration

### Nginx

```nginx
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    location / {
        proxy_pass http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support (if needed)
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Increase max upload size if needed
    client_max_body_size 32M;
}
```

### Caddy

```caddyfile
yourdomain.com {
    reverse_proxy localhost:8080

    header {
        X-Frame-Options "SAMEORIGIN"
        X-Content-Type-Options "nosniff"
        X-XSS-Protection "1; mode=block"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
}
```

---

## MongoDB Setup

### Production Recommendations

1. **Use authentication**:
   ```bash
   mongosh
   use admin
   db.createUser({
     user: "stratasave",
     pwd: "secure-password",
     roles: [{ role: "readWrite", db: "stratasave" }]
   })
   ```

2. **Enable replica set** for high availability (minimum 3 nodes)

3. **Configure backups**:
   ```bash
   # Daily backup script
   mongodump --uri="mongodb://user:pass@host:27017/stratasave" \
     --out=/backups/$(date +%Y%m%d)
   ```

4. **Monitor performance** with MongoDB tools or cloud monitoring

---

## Health Checks

Multiple health check endpoints are available:

| Endpoint | Purpose | Checks |
|----------|---------|--------|
| `/health` | Full health check | MongoDB connectivity, returns service status |
| `/ready` or `/readyz` | Kubernetes readiness probe | MongoDB connectivity |
| `/livez` | Kubernetes liveness probe | Always returns OK (process is alive) |

### Full Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "ok",
  "services": {
    "mongodb": "ok"
  }
}
```

### Kubernetes Probes

```yaml
# Example Kubernetes deployment configuration
livenessProbe:
  httpGet:
    path: /livez
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

Use these for:
- Load balancer health checks (`/health`)
- Kubernetes readiness probes (`/ready` or `/readyz`)
- Kubernetes liveness probes (`/livez`)
- Monitoring systems

---

## Backup Strategy

### What to Back Up

1. **MongoDB database**: All user data, settings, pages, files metadata
2. **Uploaded files**: If using local storage, back up the uploads directory
3. **Configuration**: Keep `config.toml` or environment variables documented

### Backup Schedule

| Data | Frequency | Retention |
|------|-----------|-----------|
| MongoDB | Daily | 30 days |
| MongoDB | Weekly | 1 year |
| Uploaded files | Daily (incremental) | 30 days |
| Configuration | On change | Version controlled |

---

## Troubleshooting

### Common Issues

**App won't start**
- Check MongoDB connectivity: `mongosh $STRATASAVE_MONGO_URI`
- Verify environment variables are set
- Check logs for configuration errors

**Can't login**
- Verify session_key hasn't changed (invalidates existing sessions)
- Check rate limiting isn't blocking (wait for lockout to expire)
- Verify CSRF key is consistent across restarts

**Emails not sending**
- Test SMTP connection independently
- Check spam folders
- Verify SPF/DKIM/DMARC DNS records

**File uploads failing**
- Check storage directory permissions
- Verify max upload size in config and reverse proxy
- Check available disk space

### Logs

View application logs:
```bash
# Docker
docker logs stratasave

# Systemd
journalctl -u stratasave -f
```

---

## Security Hardening

### Additional Recommendations

1. **Firewall**: Only expose ports 80/443
2. **Fail2ban**: Block repeated failed login attempts at firewall level
3. **Updates**: Keep OS, Docker, and MongoDB updated
4. **Secrets management**: Use Docker secrets or Vault for sensitive config
5. **Network isolation**: Run MongoDB on private network
6. **Audit logs**: Review audit logs regularly for suspicious activity
7. **Backup encryption**: Encrypt backups at rest
8. **Session timeout**: Enable idle logout for sensitive deployments
