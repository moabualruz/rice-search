# Deployment Guide

Production deployment guide for Rice Search.

## Table of Contents

- [Deployment Overview](#deployment-overview)
- [Prerequisites](#prerequisites)
- [Docker Compose Deployment](#docker-compose-deployment)
- [Configuration for Production](#configuration-for-production)
- [SSL/TLS Setup](#ssltls-setup)
- [Reverse Proxy (Nginx)](#reverse-proxy-nginx)
- [Monitoring & Logging](#monitoring--logging)
- [Backup & Recovery](#backup--recovery)
- [Scaling](#scaling)
- [Security Hardening](#security-hardening)

---

## Deployment Overview

Rice Search is designed for **self-hosted deployment** using Docker Compose.

**Deployment Models:**

1. **Single Server** - All services on one machine (recommended for <10k files)
2. **Multi-Server** - Distributed services (for >10k files or high concurrency)
3. **Kubernetes** - Future support (currently Docker Compose only)

---

## Prerequisites

**Server Requirements:**

| Component | Minimum | Recommended |
| :--- | :--- | :--- |
| CPU | 4 cores | 8+ cores |
| RAM | 8GB | 16GB+ |
| Disk | 50GB SSD | 100GB+ SSD |
| GPU | None (CPU mode) | NVIDIA GPU (8GB+ VRAM) |
| OS | Ubuntu 22.04+ | Ubuntu 22.04 LTS |

**Software:**

- Docker 20.10+
- Docker Compose 2.x
- Git (for cloning)
- Nginx (for reverse proxy)

**Network:**

- Public IP or domain name
- Ports 80, 443 open (HTTP/HTTPS)
- Internal ports: 3000, 8000, 6333, 6379, 9000, 11434

---

## Docker Compose Deployment

### 1. Clone Repository

```bash
# SSH (recommended)
git clone git@github.com:yourusername/rice-search.git
cd rice-search

# HTTPS
git clone https://github.com/yourusername/rice-search.git
cd rice-search
```

### 2. Configure Environment

```bash
# Copy environment template
cp deploy/.env.example deploy/.env

# Edit configuration
nano deploy/.env
```

**Production .env:**

```bash
# Models
EMBEDDING_MODEL=jinaai/jina-embeddings-v3
EMBEDDING_DIM=1024
LLM_MODEL=google/codegemma-7b-it
RERANK_MODEL=BAAI/bge-reranker-base
RERANK_MODE=local

# Infrastructure
QDRANT_URL=http://qdrant:6333
REDIS_URL=redis://redis:6379/0
OLLAMA_BASE_URL=http://ollama:11434
TANTIVY_URL=http://tantivy:3002

# Security
AUTH_ENABLED=true
CORS_ORIGINS=["https://your-domain.com"]

# Performance
MODEL_TTL_SECONDS=600
MODEL_AUTO_UNLOAD=true
FORCE_GPU=false  # Set true if GPU available
```

### 3. Update settings.yaml

```bash
nano backend/settings.yaml
```

**Production settings:**

```yaml
app:
  debug: false  # IMPORTANT: Disable debug mode

server:
  cors_origins:
    - "https://your-domain.com"
    - "https://www.your-domain.com"

auth:
  enabled: true  # Enable authentication

model_management:
  force_gpu: false  # or true if GPU available
  ttl_seconds: 600  # 10 minutes (longer for production)
```

### 4. Pull and Build Images

```bash
# Pull latest images
docker compose -f deploy/docker-compose.yml pull

# Build custom images
docker compose -f deploy/docker-compose.yml build

# Or build specific services
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
```

### 5. Start Services

```bash
# Start all services
docker compose -f deploy/docker-compose.yml up -d

# Check status
docker compose -f deploy/docker-compose.yml ps

# View logs
docker compose -f deploy/docker-compose.yml logs -f
```

### 6. Verify Deployment

```bash
# Check health
curl http://localhost:8000/health

# Expected response:
# {"status": "ok", "components": {...}}

# Check frontend
curl http://localhost:3000

# Check all services are running
docker compose -f deploy/docker-compose.yml ps
# All services should show "Up" or "Up (healthy)"
```

---

## Configuration for Production

### Security Settings

```yaml
# backend/settings.yaml

auth:
  enabled: true  # Require authentication

server:
  cors_origins:
    - "https://your-domain.com"  # Only your domain

app:
  debug: false  # No debug info in responses
```

### Performance Tuning

```yaml
# Model Management
model_management:
  ttl_seconds: 600  # Keep models loaded longer
  auto_unload: true # Free memory when idle

# Indexing
indexing:
  batch_size: 200  # Larger batches for GPU

# SPLADE
models:
  sparse:
    device: "cuda"  # Use GPU if available
    precision: "fp16"  # Faster inference
    batch_size: 64
```

### Resource Limits

```yaml
# deploy/docker-compose.yml

services:
  backend-api:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 4G
        reservations:
          cpus: '1.0'
          memory: 2G

  ollama:
    deploy:
      resources:
        limits:
          memory: 8G
        reservations:
          devices:
            - driver: nvidia
              count: 1
              capabilities: [gpu]
```

---

## SSL/TLS Setup

### Option 1: Let's Encrypt (Recommended)

Use Certbot to get free SSL certificates:

```bash
# Install Certbot
sudo apt update
sudo apt install certbot python3-certbot-nginx

# Get certificate
sudo certbot --nginx -d your-domain.com -d www.your-domain.com

# Auto-renewal (cron job created automatically)
sudo certbot renew --dry-run
```

### Option 2: Self-Signed Certificate (Development)

```bash
# Generate self-signed cert
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/ssl/private/nginx-selfsigned.key \
  -out /etc/ssl/certs/nginx-selfsigned.crt

# Use in Nginx config
```

---

## Reverse Proxy (Nginx)

### Install Nginx

```bash
sudo apt update
sudo apt install nginx
```

### Configure Nginx

```nginx
# /etc/nginx/sites-available/rice-search

# Redirect HTTP to HTTPS
server {
    listen 80;
    server_name your-domain.com www.your-domain.com;
    return 301 https://$server_name$request_uri;
}

# HTTPS server
server {
    listen 443 ssl http2;
    server_name your-domain.com www.your-domain.com;

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # Frontend (Next.js)
    location / {
        proxy_pass http://localhost:3000;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
    }

    # Backend API
    location /api/ {
        proxy_pass http://localhost:8000/api/;
        proxy_http_version 1.1;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Host $host;

        # Increase timeouts for long-running requests
        proxy_connect_timeout 300;
        proxy_send_timeout 300;
        proxy_read_timeout 300;
    }

    # Health check
    location /health {
        proxy_pass http://localhost:8000/health;
        access_log off;
    }

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "no-referrer-when-downgrade" always;
}
```

### Enable Site

```bash
# Create symlink
sudo ln -s /etc/nginx/sites-available/rice-search /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload Nginx
sudo systemctl reload nginx

# Enable on boot
sudo systemctl enable nginx
```

---

## Monitoring & Logging

### Prometheus Metrics

Rice Search exposes Prometheus metrics at `/metrics`:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'rice-search'
    static_configs:
      - targets: ['localhost:8000']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

### Grafana Dashboard

Import pre-built dashboard (future):

- Request latency
- Search QPS
- Indexing throughput
- Model memory usage

### Centralized Logging

#### Option 1: Docker logs

```bash
# View logs
docker compose -f deploy/docker-compose.yml logs -f

# Export logs
docker compose -f deploy/docker-compose.yml logs > logs.txt
```

#### Option 2: Loki + Promtail

```yaml
# docker-compose.yml (add)
services:
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"

  promtail:
    image: grafana/promtail:latest
    volumes:
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
```

---

## Backup & Recovery

### What to Back Up

1. **Qdrant data** - Vector database
2. **Tantivy index** - BM25 search index
3. **MinIO objects** - Original files (optional)
4. **Redis data** - Settings cache
5. **Configuration files** - settings.yaml, .env

### Backup Script

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR=/backup/rice-search-$(date +%Y%m%d)
mkdir -p $BACKUP_DIR

# Stop services (optional, for consistency)
docker compose -f deploy/docker-compose.yml stop

# Backup volumes
docker run --rm -v deploy_qdrant_data:/data \
  -v $BACKUP_DIR:/backup \
  alpine tar czf /backup/qdrant.tar.gz /data

docker run --rm -v deploy_tantivy_data:/data \
  -v $BACKUP_DIR:/backup \
  alpine tar czf /backup/tantivy.tar.gz /data

# Backup config
cp backend/settings.yaml $BACKUP_DIR/
cp deploy/.env $BACKUP_DIR/

# Restart services
docker compose -f deploy/docker-compose.yml start

# Upload to S3 (optional)
aws s3 sync $BACKUP_DIR s3://your-backup-bucket/rice-search/
```

### Restore Script

```bash
#!/bin/bash
# restore.sh

BACKUP_DIR=/backup/rice-search-20260106

# Stop services
docker compose -f deploy/docker-compose.yml down

# Restore volumes
docker run --rm -v deploy_qdrant_data:/data \
  -v $BACKUP_DIR:/backup \
  alpine tar xzf /backup/qdrant.tar.gz -C /

docker run --rm -v deploy_tantivy_data:/data \
  -v $BACKUP_DIR:/backup \
  alpine tar xzf /backup/tantivy.tar.gz -C /

# Restore config
cp $BACKUP_DIR/settings.yaml backend/
cp $BACKUP_DIR/.env deploy/

# Start services
docker compose -f deploy/docker-compose.yml up -d
```

### Automated Backups

```bash
# Add to cron (daily at 2 AM)
crontab -e

0 2 * * * /path/to/backup.sh >> /var/log/rice-backup.log 2>&1
```

---

## Scaling

### Horizontal Scaling

**Scale Stateless Services:**

```bash
# Scale API instances
docker compose -f deploy/docker-compose.yml up -d --scale backend-api=3

# Scale workers
docker compose -f deploy/docker-compose.yml up -d --scale backend-worker=5

# Update Nginx upstream
upstream backend_api {
    server localhost:8000;
    server localhost:8001;
    server localhost:8002;
}
```

### Vertical Scaling

**Increase Resources:**

```yaml
# docker-compose.yml
services:
  backend-api:
    deploy:
      resources:
        limits:
          cpus: '4.0'
          memory: 8G
```

### Database Scaling

**Qdrant Cluster:**

```yaml
services:
  qdrant-1:
    image: qdrant/qdrant:latest
    environment:
      QDRANT__CLUSTER__ENABLED: "true"

  qdrant-2:
    image: qdrant/qdrant:latest
    environment:
      QDRANT__CLUSTER__ENABLED: "true"
```

---

## Security Hardening

### Firewall Rules

```bash
# UFW (Ubuntu)
sudo ufw allow 22    # SSH
sudo ufw allow 80    # HTTP
sudo ufw allow 443   # HTTPS
sudo ufw enable

# Block direct access to internal services
sudo ufw deny 6333  # Qdrant
sudo ufw deny 6379  # Redis
sudo ufw deny 11434 # Ollama
```

### Docker Security

```yaml
# docker-compose.yml
services:
  backend-api:
    security_opt:
      - no-new-privileges:true
    read_only: true
    tmpfs:
      - /tmp
```

### Secrets Management

```bash
# Use Docker secrets (Swarm mode)
echo "your-secret" | docker secret create redis_password -

# Reference in compose
services:
  redis:
    secrets:
      - redis_password
```

---

## Summary

**Production Deployment Checklist:**

- [ ] Update settings.yaml (debug: false, auth: true)
- [ ] Configure .env with production values
- [ ] Set up SSL/TLS certificates
- [ ] Configure Nginx reverse proxy
- [ ] Enable firewall rules
- [ ] Set up monitoring (Prometheus + Grafana)
- [ ] Configure automated backups
- [ ] Test health endpoints
- [ ] Load test before launch
- [ ] Document runbook for team

**Quick Deploy:**

```bash
# 1. Clone repo
git clone <repo> && cd rice-search

# 2. Configure
cp deploy/.env.example deploy/.env
# Edit deploy/.env and backend/settings.yaml

# 3. Start services
docker compose -f deploy/docker-compose.yml up -d

# 4. Set up Nginx + SSL
sudo apt install nginx certbot
sudo certbot --nginx -d your-domain.com

# 5. Verify
curl https://your-domain.com/health
```

For more details:
- [Configuration](configuration.md) - Settings reference
- [Operations](operations.md) - Day-to-day operations
- [Security](security.md) - Security best practices
- [Troubleshooting](troubleshooting.md) - Common issues

---

**[Back to Documentation Index](README.md)**
