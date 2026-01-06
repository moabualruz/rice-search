# Operations Guide

Day-to-day operations, maintenance, and monitoring for Rice Search.

## Table of Contents

- [Daily Operations](#daily-operations)
- [Monitoring](#monitoring)
- [Log Management](#log-management)
- [Performance Tuning](#performance-tuning)
- [Maintenance Tasks](#maintenance-tasks)
- [Incident Response](#incident-response)
- [Capacity Planning](#capacity-planning)

---

## Daily Operations

### Health Checks

```bash
# Check all services
docker compose -f deploy/docker-compose.yml ps

# Check API health
curl http://localhost:8000/health

# Check Qdrant
curl http://localhost:6333/health

# Check Redis
docker exec deploy-redis-1 redis-cli ping

# Check Ollama models
curl http://localhost:11434/api/tags
```

### Service Management

```bash
# Start all services
docker compose -f deploy/docker-compose.yml up -d

# Stop all services
docker compose -f deploy/docker-compose.yml down

# Restart specific service
docker compose -f deploy/docker-compose.yml restart backend-api

# View service logs
docker compose -f deploy/docker-compose.yml logs -f backend-api
```

---

## Monitoring

### Metrics Endpoints

**Backend Metrics (Prometheus format):**
```bash
curl http://localhost:8000/metrics

# Key metrics:
# - http_requests_total
# - http_request_duration_seconds
# - search_queries_total
# - indexing_tasks_total
# - model_memory_bytes
```

**System Metrics:**
```bash
# Container stats
docker stats

# Disk usage
docker system df

# Volume usage
du -sh data/*
```

### Key Performance Indicators (KPIs)

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| API Response Time (P95) | <500ms | >1s |
| Search QPS | 10+ | <1 |
| Indexing Throughput | 100 files/min | <10 |
| Error Rate | <1% | >5% |
| CPU Usage | <70% | >90% |
| Memory Usage | <80% | >95% |
| Disk Usage | <80% | >90% |

---

## Log Management

### View Logs

```bash
# All services
docker compose -f deploy/docker-compose.yml logs

# Specific service
docker compose -f deploy/docker-compose.yml logs backend-api

# Follow logs (live)
docker compose -f deploy/docker-compose.yml logs -f backend-api

# Last 100 lines
docker compose -f deploy/docker-compose.yml logs --tail=100 backend-api

# Filter by time
docker compose -f deploy/docker-compose.yml logs --since 2023-01-01 backend-api
```

### Log Levels

```yaml
# backend/settings.yaml
app:
  log_level: "INFO"  # DEBUG, INFO, WARNING, ERROR, CRITICAL
```

### Log Rotation

```bash
# Configure Docker log rotation
# /etc/docker/daemon.json
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}

# Restart Docker
sudo systemctl restart docker
```

---

## Performance Tuning

### Database Optimization

**Qdrant:**
```bash
# Optimize collection
curl -X POST http://localhost:6333/collections/rice_chunks/index

# Check index status
curl http://localhost:6333/collections/rice_chunks
```

**Tantivy:**
```bash
# Compact index
docker exec deploy-tantivy-1 /app/compact-index

# Check index size
docker exec deploy-tantivy-1 du -sh /data
```

**Redis:**
```bash
# Get memory info
docker exec deploy-redis-1 redis-cli INFO memory

# Flush cache (careful!)
docker exec deploy-redis-1 redis-cli FLUSHDB
```

### Model Management

```bash
# Check loaded models
curl http://localhost:11434/api/tags

# Unload model
curl -X POST http://localhost:11434/api/unload \
  -d '{"model": "qwen3-embedding:4b"}'

# Monitor GPU
nvidia-smi -l 1
```

---

## Maintenance Tasks

### Weekly Tasks

**1. Check Disk Space**
```bash
df -h
du -sh data/*

# Clean up if needed
docker system prune -a
```

**2. Review Logs**
```bash
# Check for errors
docker compose -f deploy/docker-compose.yml logs | grep ERROR

# Check for warnings
docker compose -f deploy/docker-compose.yml logs | grep WARNING
```

**3. Update Metrics Dashboard**
```bash
# Check Grafana dashboards
# Review P95 latency, error rates, throughput
```

### Monthly Tasks

**1. Update Dependencies**
```bash
# Pull latest images
docker compose -f deploy/docker-compose.yml pull

# Rebuild custom images
docker compose -f deploy/docker-compose.yml build

# Restart with new images
docker compose -f deploy/docker-compose.yml up -d
```

**2. Database Maintenance**
```bash
# Optimize Qdrant
curl -X POST http://localhost:6333/collections/rice_chunks/index

# Compact Redis
docker exec deploy-redis-1 redis-cli BGSAVE
```

**3. Backup Verification**
```bash
# Test restore process
./restore.sh /backup/rice-search-YYYYMMDD
```

### Quarterly Tasks

**1. Security Audit**
```bash
# Scan for vulnerabilities
docker scout cves --only-fixed

# Update SSL certificates (if not auto-renewed)
sudo certbot renew
```

**2. Capacity Review**
- Review storage growth
- Check memory trends
- Plan for scaling if needed

---

## Incident Response

### Runbook

**1. Service Down**
```bash
# Check which service
docker compose -f deploy/docker-compose.yml ps

# View logs
docker compose -f deploy/docker-compose.yml logs <service>

# Restart service
docker compose -f deploy/docker-compose.yml restart <service>

# If still down, rebuild
docker compose -f deploy/docker-compose.yml up -d --build <service>
```

**2. High Latency**
```bash
# Check metrics
curl http://localhost:8000/metrics | grep duration

# Check CPU/memory
docker stats

# Check model loading
curl http://localhost:11434/api/tags

# Restart if needed
docker compose -f deploy/docker-compose.yml restart backend-api
```

**3. Out of Memory**
```bash
# Check memory usage
docker stats

# Enable model auto-unloading
curl -X PUT http://localhost:8000/api/v1/settings/model_management.auto_unload \
  -H "Content-Type: application/json" \
  -d '{"value": true}'

# Reduce TTL
curl -X PUT http://localhost:8000/api/v1/settings/model_management.ttl_seconds \
  -H "Content-Type: application/json" \
  -d '{"value": 60}'

# Restart services
docker compose -f deploy/docker-compose.yml restart
```

**4. Disk Full**
```bash
# Check disk usage
df -h

# Clean Docker
docker system prune -a --volumes

# Check data directories
du -sh data/*

# Archive old backups
tar czf backups-old.tar.gz /backup/rice-search-old*
rm -rf /backup/rice-search-old*
```

---

## Capacity Planning

### Storage Growth

**Estimate storage needs:**
```
Qdrant: ~1KB per chunk
Tantivy: ~500B per chunk
Total: ~1.5KB per chunk

Example:
- 10,000 files
- Average 10 chunks per file
- Total: 100,000 chunks
- Storage: 150MB (Qdrant + Tantivy)
```

**Monitor growth:**
```bash
# Weekly snapshot
du -sh data/* >> storage-growth.log
```

### Memory Requirements

**Baseline:**
- Backend API: 2GB per instance
- Worker: 4GB per worker
- Qdrant: 4GB + (index size × 1.5)
- Redis: 1GB
- Ollama: 8GB (with models loaded)

**Scaling formula:**
```
Total RAM needed = (
  Backend instances × 2GB +
  Worker instances × 4GB +
  Qdrant base + (index size × 1.5) +
  Redis 1GB +
  Ollama 8GB
)
```

### CPU Requirements

**Baseline:**
- Backend: 1 core per instance
- Worker: 2 cores per worker
- Qdrant: 2 cores
- Ollama: 4 cores (or GPU)

---

## Summary

**Daily Checklist:**
- [ ] Check service health (`make logs`)
- [ ] Review error logs
- [ ] Monitor disk usage
- [ ] Check API response times

**Weekly Checklist:**
- [ ] Review metrics dashboards
- [ ] Clean up Docker resources
- [ ] Check backup integrity

**Monthly Checklist:**
- [ ] Update dependencies
- [ ] Database maintenance
- [ ] Review capacity trends

**Key Commands:**
```bash
# Health check
curl http://localhost:8000/health

# View logs
docker compose -f deploy/docker-compose.yml logs -f

# Restart service
docker compose -f deploy/docker-compose.yml restart backend-api

# Clean up
docker system prune -a
```

For more details:
- [Deployment](deployment.md) - Initial setup
- [Monitoring](monitoring.md) - Metrics & dashboards
- [Troubleshooting](troubleshooting.md) - Common issues
- [Configuration](configuration.md) - Settings tuning

---

**[Back to Documentation Index](README.md)**
