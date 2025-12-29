# Health Checks

## Overview

Health checks for Kubernetes probes and load balancer integration.

---

## Endpoints

| Endpoint | Purpose | Returns |
|----------|---------|---------|
| `GET /healthz` | Liveness probe | Is process alive? |
| `GET /readyz` | Readiness probe | Can handle requests? |
| `GET /v1/health` | Detailed health | Full status + dependencies |

---

## Liveness Probe (/healthz)

### Purpose

Kubernetes uses this to know if the container should be restarted.

### Logic

Returns 200 if the HTTP server is responding.

```go
func LivenessHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

### Response

**Healthy (200):**
```json
{"status": "ok"}
```

### Kubernetes Config

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

---

## Readiness Probe (/readyz)

### Purpose

Kubernetes uses this to know if the pod should receive traffic.

### Logic

Returns 200 only if ALL of these are true:
1. Qdrant is reachable
2. ML models are loaded
3. Event bus is connected (if distributed)

```go
func ReadinessHandler(w http.ResponseWriter, r *http.Request) {
    checks := runReadinessChecks()
    
    allReady := true
    for _, check := range checks {
        if check.Status != "ok" {
            allReady = false
            break
        }
    }
    
    status := http.StatusOK
    if !allReady {
        status = http.StatusServiceUnavailable
    }
    
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(ReadinessResponse{
        Status: statusString(allReady),
        Checks: checks,
    })
}
```

### Response

**Ready (200):**
```json
{
    "status": "ready",
    "checks": {
        "qdrant": {"status": "ok", "latency_ms": 5},
        "models": {"status": "ok"},
        "event_bus": {"status": "ok"}
    }
}
```

**Not Ready (503):**
```json
{
    "status": "not_ready",
    "checks": {
        "qdrant": {"status": "ok", "latency_ms": 5},
        "models": {"status": "loading", "progress": "2/3"},
        "event_bus": {"status": "ok"}
    }
}
```

### Kubernetes Config

```yaml
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 3
```

---

## Detailed Health (/v1/health)

### Purpose

Detailed health information for debugging and monitoring.

### Response

```json
{
    "status": "healthy",
    "version": "1.0.0",
    "git_commit": "abc123",
    "uptime_seconds": 3600,
    "timestamp": "2025-12-29T01:00:00Z",
    
    "checks": {
        "qdrant": {
            "status": "ok",
            "url": "http://localhost:6333",
            "version": "1.12.4",
            "latency_ms": 5,
            "collections": 2
        },
        
        "models": {
            "status": "ok",
            "device": "cuda:0",
            "load_mode": "all",
            "embed": {
                "loaded": true,
                "name": "jina-embeddings-v3",
                "memory_mb": 600
            },
            "sparse": {
                "loaded": true,
                "name": "splade-pp-en-v1",
                "memory_mb": 250
            },
            "rerank": {
                "loaded": true,
                "name": "jina-reranker-v2",
                "memory_mb": 500
            }
        },
        
        "event_bus": {
            "status": "ok",
            "type": "memory",
            "topics": 6,
            "pending_events": 12
        },
        
        "cache": {
            "status": "ok",
            "type": "memory",
            "embed_entries": 50000,
            "embed_hit_rate": 0.72,
            "sparse_entries": 50000,
            "sparse_hit_rate": 0.68
        },
        
        "stores": {
            "status": "ok",
            "count": 2,
            "total_chunks": 45000
        }
    },
    
    "system": {
        "goroutines": 150,
        "heap_mb": 512,
        "cpu_percent": 25,
        "open_files": 50
    }
}
```

---

## Health Check Logic

### Qdrant Check

```go
func checkQdrant() HealthCheck {
    start := time.Now()
    
    // Try to list collections
    _, err := qdrantClient.ListCollections(ctx)
    
    latency := time.Since(start)
    
    if err != nil {
        return HealthCheck{
            Status:  "error",
            Error:   err.Error(),
            Latency: latency,
        }
    }
    
    return HealthCheck{
        Status:  "ok",
        Latency: latency,
    }
}
```

### Models Check

```go
func checkModels() HealthCheck {
    embedLoaded := mlService.IsModelLoaded("embed")
    sparseLoaded := mlService.IsModelLoaded("sparse")
    rerankLoaded := mlService.IsModelLoaded("rerank")
    
    if embedLoaded && sparseLoaded && rerankLoaded {
        return HealthCheck{Status: "ok"}
    }
    
    // Return loading status
    loaded := 0
    if embedLoaded { loaded++ }
    if sparseLoaded { loaded++ }
    if rerankLoaded { loaded++ }
    
    return HealthCheck{
        Status:   "loading",
        Progress: fmt.Sprintf("%d/3", loaded),
    }
}
```

### Event Bus Check

```go
func checkEventBus() HealthCheck {
    // For memory bus, always ok if initialized
    if bus.Type() == "memory" {
        return HealthCheck{Status: "ok"}
    }
    
    // For Kafka/NATS, check connection
    err := bus.Ping(ctx)
    if err != nil {
        return HealthCheck{
            Status: "error",
            Error:  err.Error(),
        }
    }
    
    return HealthCheck{Status: "ok"}
}
```

---

## Startup Sequence

```
1. HTTP server starts (healthz returns 200)
2. readyz returns 503 (not ready)
3. Connect to Qdrant
4. Load ML models (may take 30-60s)
5. Connect to event bus
6. readyz returns 200 (ready)
7. Start accepting traffic
```

### Startup Probe (Kubernetes)

For slow model loading:

```yaml
startupProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 10
  failureThreshold: 30  # 5 minutes max startup
```

---

## Dependency Health

### Required Dependencies

| Dependency | Required for Ready | Check Method |
|------------|-------------------|--------------|
| Qdrant | Yes | List collections |
| ML Models | Yes | Check loaded flag |
| Event Bus | Yes (if distributed) | Ping |

### Optional Dependencies

| Dependency | Impact if Down | Check Method |
|------------|---------------|--------------|
| Cache (Redis) | Degraded perf | Ping |
| Metrics endpoint | No metrics | N/A |

---

## Health Status Codes

| Status | HTTP Code | Meaning |
|--------|-----------|---------|
| `healthy` | 200 | All systems operational |
| `degraded` | 200 | Operating with reduced capability |
| `not_ready` | 503 | Cannot accept traffic |
| `unhealthy` | 503 | Critical failure |

### Degraded Example

```json
{
    "status": "degraded",
    "checks": {
        "qdrant": {"status": "ok"},
        "models": {"status": "ok"},
        "cache": {"status": "error", "error": "Redis connection refused"}
    },
    "message": "Cache unavailable, operating without caching"
}
```

---

## Configuration

```yaml
health:
  # Check intervals
  qdrant_check_interval: 10s
  model_check_interval: 30s
  
  # Timeouts
  qdrant_timeout: 5s
  
  # Thresholds
  qdrant_latency_warn_ms: 100
  qdrant_latency_error_ms: 1000
```

---

## Load Balancer Integration

### AWS ALB

```yaml
TargetGroup:
  HealthCheckPath: /readyz
  HealthCheckIntervalSeconds: 10
  HealthyThresholdCount: 2
  UnhealthyThresholdCount: 3
```

### nginx

```nginx
upstream rice-search {
    server backend1:8080;
    server backend2:8080;
}

server {
    location /health {
        proxy_pass http://rice-search/readyz;
    }
}
```

---

## Graceful Degradation

When dependencies fail:

| Failure | Behavior |
|---------|----------|
| Qdrant down | Return 503, queue requests |
| Model OOM | Evict LRU, retry |
| Cache down | Continue without cache |
| Event bus down | Fall back to local calls |
