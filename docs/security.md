# Security Guide

Security best practices and authentication setup for Rice Search.

## Table of Contents

- [Security Overview](#security-overview)
- [Authentication & Authorization](#authentication--authorization)
- [Network Security](#network-security)
- [Data Security](#data-security)
- [Secrets Management](#secrets-management)
- [Security Hardening](#security-hardening)
- [Compliance](#compliance)
- [Security Checklist](#security-checklist)

---

## Security Overview

Rice Search security model:
- **Authentication** - Optional, disabled by default
- **Authorization** - Role-based access control (RBAC)
- **Encryption** - TLS/SSL for external traffic
- **Isolation** - Docker network isolation
- **Self-hosted** - All data stays local

**Security Layers:**
```
External Traffic → TLS/SSL (Nginx)
                 → CORS (FastAPI)
                 → Authentication (Optional)
                 → Authorization (RBAC)
                 → Application Logic
                 → Database Access
                 → Docker Network
```

---

## Authentication & Authorization

### Default Configuration (No Auth)

By default, authentication is **disabled**:

```yaml
# backend/settings.yaml
auth:
  enabled: false  # No authentication required
```

**Behavior:**
- All requests are accepted
- Default `org_id`: `"public"`
- All users have admin access

**Use Case:** Internal networks, development, personal use

### Enabling Authentication

```yaml
# backend/settings.yaml
auth:
  enabled: true  # Require authentication
```

```bash
# Or via environment variable
export AUTH_ENABLED=true
```

### Future Authentication Methods (Planned)

**1. API Keys**
```bash
# Generate API key
curl -X POST http://localhost:8000/api/v1/auth/keys \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"name": "My App", "org_id": "myorg"}'

# Use API key
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "X-API-Key: <api-key>" \
  -d '{"query": "test"}'
```

**2. OAuth 2.0 / OIDC (Future)**
- Keycloak integration
- Google/GitHub login
- SAML support

**3. JWT Tokens (Future)**
```bash
# Login
curl -X POST http://localhost:8000/api/v1/auth/login \
  -d '{"username": "user", "password": "pass"}'

# Returns JWT
{"access_token": "eyJ...", "token_type": "bearer"}

# Use token
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Authorization: Bearer eyJ..." \
  -d '{"query": "test"}'
```

### Role-Based Access Control (RBAC)

**Roles:**
- `admin` - Full access (settings, ingest, search)
- `user` - Read-only (search, files)
- `indexer` - Ingest files
- `viewer` - Search only

**Example:**
```python
# Endpoint with role requirement
@router.post("/ingest/file", dependencies=[Depends(requires_role("indexer"))])
async def upload_file(...):
    pass
```

---

## Network Security

### CORS Configuration

```yaml
# backend/settings.yaml
server:
  cors_origins:
    - "http://localhost:3000"     # Frontend (dev)
    - "https://your-domain.com"   # Production
```

**Production CORS:**
```yaml
server:
  cors_origins:
    - "https://your-domain.com"
    - "https://www.your-domain.com"
  # Do NOT use "*" in production!
```

### Firewall Rules

**External Ports (open):**
- 80 (HTTP) - Redirect to HTTPS
- 443 (HTTPS) - Public access

**Internal Ports (block external access):**
- 3000 (Frontend) - Behind Nginx
- 8000 (Backend API) - Behind Nginx
- 6333 (Qdrant) - Internal only
- 6379 (Redis) - Internal only
- 9000 (MinIO) - Internal only
- 11434 (Ollama) - Internal only
- 3002 (Tantivy) - Internal only

**UFW Configuration:**
```bash
sudo ufw allow 22     # SSH
sudo ufw allow 80     # HTTP
sudo ufw allow 443    # HTTPS
sudo ufw deny 6333    # Qdrant
sudo ufw deny 6379    # Redis
sudo ufw deny 11434   # Ollama
sudo ufw enable
```

### Docker Network Isolation

```yaml
# docker-compose.yml
networks:
  rice-net:
    driver: bridge
    internal: false  # Set true for complete isolation
```

**Service communication:**
- Services communicate via Docker network
- No direct external access to internal services

---

## Data Security

### Data at Rest

**Encryption (Optional):**
- Qdrant: Supports encryption at rest
- MinIO: Supports SSE (Server-Side Encryption)
- Redis: Transparent Data Encryption (TDE)

**Example (Qdrant with encryption):**
```yaml
services:
  qdrant:
    environment:
      QDRANT__STORAGE__ENCRYPTION_KEY: ${QDRANT_ENCRYPTION_KEY}
```

### Data in Transit

**TLS/SSL:**
- Nginx terminates SSL
- Internal traffic over Docker network (not encrypted)

**For end-to-end encryption:**
```yaml
# Encrypt internal traffic (advanced)
services:
  qdrant:
    environment:
      QDRANT__SERVICE__ENABLE_TLS: "true"
```

### Sensitive Data Handling

**Do NOT index:**
- Secrets files (.env, credentials.json)
- API keys (config.yaml with keys)
- Private keys (.pem, .key)

**Recommended .riceignore:**
```
.env
.env.*
**/secrets/
**/*.key
**/*.pem
**/credentials.json
**/token.json
```

---

## Secrets Management

### Environment Variables

**DO NOT commit secrets to Git:**
```bash
# .gitignore (already included)
.env
.env.local
.env.production
deploy/.env
```

**Use environment variables:**
```bash
# deploy/.env
REDIS_PASSWORD=<random-password>
MINIO_ACCESS_KEY=<random-key>
MINIO_SECRET_KEY=<random-secret>
```

### Docker Secrets (Swarm Mode)

```bash
# Create secret
echo "my-secret-password" | docker secret create redis_password -

# Use in compose
services:
  redis:
    secrets:
      - redis_password
    command: redis-server --requirepass /run/secrets/redis_password

secrets:
  redis_password:
    external: true
```

### Vault Integration (Future)

```yaml
# Use HashiCorp Vault for secrets
services:
  backend-api:
    environment:
      VAULT_ADDR: https://vault.example.com
      VAULT_TOKEN: ${VAULT_TOKEN}
```

---

## Security Hardening

### Docker Security

**1. Run as non-root user:**
```dockerfile
# Dockerfile
RUN useradd -m -u 1000 appuser
USER appuser
```

**2. Read-only filesystem:**
```yaml
# docker-compose.yml
services:
  backend-api:
    read_only: true
    tmpfs:
      - /tmp
```

**3. Drop capabilities:**
```yaml
services:
  backend-api:
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE  # Only if needed
```

**4. Security options:**
```yaml
services:
  backend-api:
    security_opt:
      - no-new-privileges:true
      - apparmor=docker-default
```

### Application Security

**1. Disable debug mode:**
```yaml
app:
  debug: false  # CRITICAL for production
```

**2. Set secure headers:**
```python
# Nginx adds security headers
add_header X-Frame-Options "SAMEORIGIN";
add_header X-Content-Type-Options "nosniff";
add_header X-XSS-Protection "1; mode=block";
add_header Strict-Transport-Security "max-age=31536000";
```

**3. Input validation:**
- All API endpoints use Pydantic validation
- Query length limits enforced
- File size limits enforced

**4. Rate limiting (Future):**
```python
# Planned: Rate limiting per IP/user
from slowapi import Limiter

limiter = Limiter(key_func=get_remote_address)

@limiter.limit("10/minute")
@router.post("/search/query")
async def search(...):
    pass
```

### Database Security

**Qdrant:**
```yaml
services:
  qdrant:
    environment:
      QDRANT__SERVICE__API_KEY: ${QDRANT_API_KEY}
```

**Redis:**
```yaml
services:
  redis:
    command: redis-server --requirepass ${REDIS_PASSWORD}
```

**MinIO:**
```yaml
services:
  minio:
    environment:
      MINIO_ROOT_USER: ${MINIO_ACCESS_KEY}
      MINIO_ROOT_PASSWORD: ${MINIO_SECRET_KEY}
```

---

## Compliance

### GDPR Compliance

**Data Processing:**
- User data is organization-scoped (`org_id`)
- Files can be deleted on request
- No tracking or analytics by default

**Data Deletion:**
```bash
# Delete all data for an organization
curl -X DELETE http://localhost:8000/api/v1/admin/org/myorg
```

**Data Export:**
```bash
# Export indexed files
curl http://localhost:8000/api/v1/files/list?org_id=myorg > files.json
```

### Audit Logging (Future)

```python
# Planned: Audit all sensitive operations
@audit_log(action="file_upload", resource="file")
async def upload_file(...):
    pass
```

### Data Residency

- All data stored locally (self-hosted)
- No external API calls (fully local inference)
- Choose your deployment region

---

## Security Checklist

### Pre-Deployment

- [ ] Review all secrets in `.env` and `settings.yaml`
- [ ] Disable debug mode (`app.debug: false`)
- [ ] Enable authentication (`auth.enabled: true`)
- [ ] Configure CORS for production domain
- [ ] Set up SSL/TLS certificates
- [ ] Configure firewall rules
- [ ] Review `.riceignore` to exclude secrets
- [ ] Set strong passwords for Redis, MinIO
- [ ] Use read-only Docker filesystems
- [ ] Drop unnecessary Docker capabilities

### Post-Deployment

- [ ] Verify SSL certificate validity
- [ ] Test authentication endpoints
- [ ] Check firewall rules are active
- [ ] Scan for vulnerabilities (`docker scout`)
- [ ] Review access logs for anomalies
- [ ] Set up security monitoring
- [ ] Configure automated backups
- [ ] Document incident response plan

### Regular Audits

- [ ] Monthly: Review access logs
- [ ] Monthly: Update Docker images
- [ ] Quarterly: Security vulnerability scan
- [ ] Quarterly: Review user permissions
- [ ] Annually: Full security audit

---

## Summary

**Security Best Practices:**

1. **Enable Authentication** in production
2. **Use HTTPS** (SSL/TLS via Nginx)
3. **Restrict CORS** to your domain only
4. **Firewall** internal services
5. **Encrypt secrets** (use `.env`, not hardcode)
6. **Disable debug** mode
7. **Run containers** as non-root
8. **Update dependencies** regularly
9. **Monitor logs** for suspicious activity
10. **Backup data** securely

**Quick Security Checklist:**
```bash
# 1. Check debug mode is OFF
grep "debug:" backend/settings.yaml  # Should be "false"

# 2. Check auth is enabled (if needed)
grep "enabled:" backend/settings.yaml | grep auth  # Should be "true"

# 3. Check CORS configuration
grep "cors_origins:" backend/settings.yaml

# 4. Verify SSL is working
curl -I https://your-domain.com  # Should return 200

# 5. Check firewall
sudo ufw status  # Should show only 22, 80, 443 open
```

For more details:
- [Deployment](deployment.md) - Production setup
- [Operations](operations.md) - Security monitoring
- [Configuration](configuration.md) - Auth settings
- [API Reference](api.md) - Authentication endpoints

---

**[Back to Documentation Index](README.md)**
