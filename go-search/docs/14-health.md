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

Returns 200 if service is healthy or degraded (can handle requests).  
Returns 503 only if unhealthy (critical failure).

Checks:
1. Qdrant connectivity
2. ML models (embedder + sparse required)

```go
func (h *HealthHandler) HandleReady(w http.ResponseWriter, r *http.Request) {
    status := h.checker.Check(ctx)
    
    if status.Status == "healthy" {
        w.WriteHeader(http.StatusOK)
    } else if status.Status == "degraded" {
        w.WriteHeader(http.StatusOK) // Still OK but with warnings
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(status)
}
```

### Response

**Ready (200):**
```json
{
    "status": "healthy",
    "timestamp": "2025-12-30T01:00:00Z",
    "version": "1.0.0",
    "uptime": "1h30m0s",
    "components": {
        "ml": {
            "status": "healthy",
            "message": "all models loaded"
        },
        "qdrant": {
            "status": "healthy",
            "message": "connected",
            "latency_ms": 5
        }
    }
}
```

**Degraded (200):**
```json
{
    "status": "degraded",
    "timestamp": "2025-12-30T01:00:00Z",
    "version": "1.0.0",
    "uptime": "1h30m0s",
    "components": {
        "ml": {
            "status": "degraded",
            "message": "some models not loaded"
        },
        "qdrant": {
            "status": "healthy",
            "message": "connected",
            "latency_ms": 5
        }
    }
}
```

**Not Ready (503):**
```json
{
    "status": "unhealthy",
    "timestamp": "2025-12-30T01:00:00Z",
    "version": "1.0.0",
    "uptime": "1h30m0s",
    "components": {
        "ml": {
            "status": "healthy",
            "message": "all models loaded"
        },
        "qdrant": {
            "status": "unhealthy",
            "message": "connection refused",
            "latency_ms": 5000
        }
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

### Current Implementation

Currently returns the same simplified response as `/readyz`:

```json
{
    "status": "healthy",
    "timestamp": "2025-12-30T01:00:00Z",
    "version": "1.0.0",
    "uptime": "1h30m0s",
    "components": {
        "ml": {
            "status": "healthy",
            "message": "all models loaded"
        },
        "qdrant": {
            "status": "healthy",
            "message": "connected",
            "latency_ms": 5
        }
    }
}
```

**Note:** A more comprehensive `DetailedHealthChecker` exists in `internal/search/health_detailed.go` but is not currently wired up to this endpoint. See "Future Enhancement" section below for the full detailed response format.

---

## Future Enhancement: Detailed Health Response

The codebase includes a comprehensive health checker (`DetailedHealthChecker` in `internal/search/health_detailed.go`) that can provide much richer health information. This is not currently exposed but is available for future use.

### Planned Detailed Response Format

```json
{
    "status": "healthy",
    "version": "1.0.0",
    "git_commit": "abc123",
    "uptime_seconds": 3600,
    "timestamp": "2025-12-29T01:00:00Z",
    
    "checks": {
        "qdrant": {
            "status": "healthy",
            "url": "http://localhost:6333",
            "version": "1.12.4",
            "latency_ms": 5,
            "collections": 2
        },
        
        "models": {
            "status": "healthy",
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
            },
            "query": {
                "loaded": true,
                "name": "codebert-base",
                "memory_mb": 438
            }
        },
        
        "event_bus": {
            "status": "healthy",
            "type": "memory",
            "topics": 6,
            "pending_events": 12
        },
        
        "cache": {
            "status": "healthy",
            "type": "memory",
            "embed_entries": 50000,
            "embed_hit_rate": 0.72,
            "sparse_entries": 50000,
            "sparse_hit_rate": 0.68
        },
        
        "stores": {
            "status": "healthy",
            "count": 2,
            "total_chunks": 45000
        }
    },
    
    "system": {
        "goroutines": 150,
        "heap_mb": 512,
        "alloc_mb": 520,
        "sys_mb": 800,
        "num_gc": 42,
        "goos": "linux",
        "goarch": "amd64",
        "num_cpu": 8
    }
}
```

To enable this detailed response, wire up `DetailedHealthChecker` to the `/v1/health` endpoint in the server initialization.

---

## Health Check Logic

### Current Implementation

The simplified health checker (`HealthChecker` in `internal/search/health.go`) performs basic checks:

#### Qdrant Check

```go
func (h *HealthChecker) checkQdrant(ctx context.Context) Component {
    if h.qdrant == nil {
        return Component{
            Status:  "unhealthy",
            Message: "Qdrant client not configured",
        }
    }
    
    start := time.Now()
    err := h.qdrant.HealthCheck(ctx)
    latency := time.Since(start).Milliseconds()
    
    if err != nil {
        return Component{
            Status:  "unhealthy",
            Message: err.Error(),
            Latency: latency,
        }
    }
    
    return Component{
        Status:  "healthy",
        Message: "connected",
        Latency: latency,
    }
}
```

#### Models Check

```go
func (h *HealthChecker) checkML() Component {
    if h.ml == nil {
        return Component{
            Status:  "unhealthy",
            Message: "ML service not configured",
        }
    }
    
    health := h.ml.Health()
    if !health.Healthy {
        return Component{
            Status:  "unhealthy",
            Message: health.Error,
        }
    }
    
    // Check which models are loaded (embedder + sparse required)
    allLoaded := true
    for model, loaded := range health.ModelsLoaded {
        if !loaded && (model == "embedder" || model == "sparse") {
            allLoaded = false
            break
        }
    }
    
    if !allLoaded {
        return Component{
            Status:  "degraded",
            Message: "some models not loaded",
        }
    }
    
    return Component{
        Status:  "healthy",
        Message: "all models loaded",
    }
}
```

### Future: Detailed Health Checks

The `DetailedHealthChecker` includes additional checks for event bus, cache, stores, and system metrics. See `internal/search/health_detailed.go` for implementation.

---

## Startup Sequence

```
1. HTTP server starts (healthz returns 200)
2. readyz returns unhealthy (503)
3. Connect to Qdrant
4. Load ML models (may take 30-60s)
5. readyz returns healthy (200)
6. Start accepting traffic
```

**Note:** The current implementation checks Qdrant + ML only. Future event bus checks would add another step.

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

### Required Dependencies (Current)

| Dependency | Required for Ready | Check Method |
|------------|-------------------|--------------|
| Qdrant | Yes | `HealthCheck()` |
| ML Models (embedder + sparse) | Yes | Check loaded flags |

### Optional Dependencies (Future)

The `DetailedHealthChecker` includes checks for:

| Dependency | Impact if Down | Check Method |
|------------|---------------|--------------|
| Event Bus | Degraded | Ping (memory bus always healthy) |
| Cache | Degraded perf | Stats from ML service |
| Stores | Degraded | List stores + count chunks |
| Reranker Model | Degraded search quality | Check loaded flag |
| Query Model | No query expansion | Check loaded flag |

---

## Health Status Codes

| Status | HTTP Code | Meaning |
|--------|-----------|---------|
| `healthy` | 200 | All critical systems operational |
| `degraded` | 200 | Operating with reduced capability (e.g., some models not loaded) |
| `unhealthy` | 503 | Critical failure (Qdrant down or ML service failed) |

### Degraded Example

```json
{
    "status": "degraded",
    "timestamp": "2025-12-30T01:00:00Z",
    "version": "1.0.0",
    "uptime": "5m0s",
    "components": {
        "ml": {
            "status": "degraded",
            "message": "some models not loaded"
        },
        "qdrant": {
            "status": "healthy",
            "message": "connected",
            "latency_ms": 3
        }
    }
}
```

**Note:** Degraded state still returns HTTP 200, indicating the service can handle traffic (albeit with reduced functionality).

---

## Configuration

Health checks use context timeouts configured in the handlers:

```go
// Readiness check timeout
ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)

// Detailed health check timeout  
ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
```

**Note:** Advanced configuration (check intervals, thresholds, etc.) would be added when wiring up the `DetailedHealthChecker`.

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

| Failure | Current Behavior | Status Code |
|---------|------------------|-------------|
| Qdrant down | Return unhealthy | 503 (not ready) |
| ML service failed | Return unhealthy | 503 (not ready) |
| Some models not loaded | Return degraded | 200 (still ready) |

**Future Behavior** (with `DetailedHealthChecker`):

| Failure | Behavior | Status Code |
|---------|----------|-------------|
| Qdrant down | Return unhealthy | 503 |
| ML service failed | Return unhealthy | 503 |
| Some models not loaded | Return degraded | 200 |
| Event bus down | Return degraded (fall back to local) | 200 |
| Cache down | Return degraded (continue without cache) | 200 |
| Stores service error | Return degraded | 200 |
