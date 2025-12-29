# Graceful Shutdown

## Overview

Proper shutdown ensures no data loss and clean connection termination.

---

## Shutdown Signals

| Signal | Action |
|--------|--------|
| `SIGTERM` | Graceful shutdown (default k8s) |
| `SIGINT` | Graceful shutdown (Ctrl+C) |
| `SIGQUIT` | Graceful shutdown with stack dump |
| `SIGKILL` | Immediate termination (cannot catch) |

```go
func main() {
    ctx, stop := signal.NotifyContext(context.Background(),
        syscall.SIGTERM,
        syscall.SIGINT,
        syscall.SIGQUIT,
    )
    defer stop()
    
    // Start server
    go server.Start()
    
    // Wait for shutdown signal
    <-ctx.Done()
    
    // Graceful shutdown
    shutdown(context.Background())
}
```

---

## Shutdown Sequence

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          SHUTDOWN SEQUENCE                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. SIGNAL RECEIVED (SIGTERM)                                               │
│     │                                                                       │
│     ▼                                                                       │
│  2. STOP ACCEPTING NEW REQUESTS                                             │
│     ├── Set readiness to false (503 on /readyz)                            │
│     ├── Stop HTTP listener                                                  │
│     └── Kubernetes removes from service endpoints                          │
│     │                                                                       │
│     ▼                                                                       │
│  3. DRAIN IN-FLIGHT REQUESTS (wait up to 30s)                              │
│     ├── Wait for active HTTP requests to complete                          │
│     ├── Wait for active event handlers to complete                         │
│     └── Log progress every 5s                                              │
│     │                                                                       │
│     ▼                                                                       │
│  4. STOP EVENT BUS                                                          │
│     ├── Stop consuming new events                                          │
│     ├── Wait for pending events to process                                 │
│     └── Close connections (Kafka/NATS)                                     │
│     │                                                                       │
│     ▼                                                                       │
│  5. FLUSH CACHES                                                            │
│     └── Persist cache to disk if configured                                │
│     │                                                                       │
│     ▼                                                                       │
│  6. CLOSE CONNECTIONS                                                       │
│     ├── Close Qdrant connection pool                                       │
│     ├── Close Redis connections                                            │
│     └── Release GPU resources                                              │
│     │                                                                       │
│     ▼                                                                       │
│  7. EXIT                                                                    │
│     └── Log final message, exit 0                                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Implementation

### Main Shutdown Function

```go
func shutdown(ctx context.Context) {
    log.Info("Shutdown initiated")
    
    // Create timeout context
    ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
    defer cancel()
    
    // 1. Stop accepting requests
    log.Info("Stopping HTTP server")
    readinessFlag.Store(false)
    if err := httpServer.Shutdown(ctx); err != nil {
        log.Warn("HTTP shutdown error", "error", err)
    }
    
    // 2. Drain in-flight work
    log.Info("Draining in-flight requests")
    if err := drainInFlight(ctx); err != nil {
        log.Warn("Drain error", "error", err)
    }
    
    // 3. Stop event bus
    log.Info("Stopping event bus")
    if err := eventBus.Close(); err != nil {
        log.Warn("Event bus close error", "error", err)
    }
    
    // 4. Flush caches
    log.Info("Flushing caches")
    if err := cache.Flush(); err != nil {
        log.Warn("Cache flush error", "error", err)
    }
    
    // 5. Close connections
    log.Info("Closing connections")
    qdrantClient.Close()
    mlService.Close()
    
    log.Info("Shutdown complete")
}
```

### Drain In-Flight Requests

```go
func drainInFlight(ctx context.Context) error {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            remaining := inFlightCounter.Load()
            if remaining > 0 {
                log.Warn("Shutdown timeout, dropping requests", "remaining", remaining)
            }
            return ctx.Err()
            
        case <-ticker.C:
            remaining := inFlightCounter.Load()
            if remaining == 0 {
                return nil
            }
            log.Info("Waiting for in-flight requests", "remaining", remaining)
        }
    }
}
```

### HTTP Server Shutdown

```go
func (s *Server) Shutdown(ctx context.Context) error {
    // Stop accepting new connections
    s.listener.Close()
    
    // Wait for active requests
    return s.httpServer.Shutdown(ctx)
}
```

---

## Timeouts

| Phase | Timeout | Action if Exceeded |
|-------|---------|-------------------|
| Total shutdown | 30s | Force exit |
| HTTP drain | 15s | Drop remaining requests |
| Event drain | 10s | Drop pending events |
| Connection close | 5s | Force close |

### Configuration

```yaml
shutdown:
  timeout: 30s
  http_drain_timeout: 15s
  event_drain_timeout: 10s
  connection_timeout: 5s
```

---

## Kubernetes Configuration

### Pod Spec

```yaml
spec:
  terminationGracePeriodSeconds: 45  # > shutdown timeout
  containers:
    - name: rice-search
      lifecycle:
        preStop:
          exec:
            command: ["/bin/sh", "-c", "sleep 5"]  # Wait for endpoint removal
```

### Why preStop sleep?

```
1. SIGTERM sent to pod
2. Pod marked as Terminating
3. Endpoint removed from Service (async!)
4. preStop runs (sleep 5s)
5. App shutdown starts
6. No more traffic arrives (endpoints already removed)
```

Without sleep, requests may arrive during shutdown.

---

## Connection Draining

### Qdrant

```go
func (c *QdrantClient) Close() error {
    // Cancel in-flight requests
    c.cancelFunc()
    
    // Wait for pending requests (with timeout)
    c.wg.Wait()
    
    // Close HTTP client
    c.httpClient.CloseIdleConnections()
    
    return nil
}
```

### Event Bus (Kafka)

```go
func (b *KafkaBus) Close() error {
    // Stop consuming
    b.consumer.Close()
    
    // Flush producer
    b.producer.Flush(10000) // 10s timeout
    
    // Close producer
    b.producer.Close()
    
    return nil
}
```

---

## Handling Long-Running Operations

### Index Operation

```go
func (s *IndexService) Index(ctx context.Context, req IndexRequest) error {
    // Check if shutting down
    if shuttingDown.Load() {
        return ErrShuttingDown
    }
    
    // Track in-flight
    inFlightCounter.Add(1)
    defer inFlightCounter.Add(-1)
    
    // Use shutdown-aware context
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()
    
    // Watch for shutdown
    go func() {
        <-shutdownChan
        cancel()
    }()
    
    // Do work
    return s.doIndex(ctx, req)
}
```

### Checkpoint Long Operations

```go
func (s *IndexService) indexBatch(ctx context.Context, docs []Document) error {
    for i, doc := range docs {
        // Check context periodically
        if ctx.Err() != nil {
            // Save checkpoint
            s.saveCheckpoint(i)
            return ctx.Err()
        }
        
        if err := s.indexOne(ctx, doc); err != nil {
            return err
        }
    }
    return nil
}
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown |
| 1 | Shutdown error |
| 2 | Forced shutdown (timeout) |
| 137 | SIGKILL (128 + 9) |
| 143 | SIGTERM (128 + 15) |

---

## Logging During Shutdown

```
2025-12-29T01:00:00Z INFO  Received SIGTERM, initiating shutdown
2025-12-29T01:00:00Z INFO  Stopping HTTP server, readiness=false
2025-12-29T01:00:01Z INFO  Draining in-flight requests remaining=15
2025-12-29T01:00:06Z INFO  Draining in-flight requests remaining=3
2025-12-29T01:00:08Z INFO  All requests drained
2025-12-29T01:00:08Z INFO  Stopping event bus
2025-12-29T01:00:09Z INFO  Flushing caches embed_entries=50000
2025-12-29T01:00:10Z INFO  Closing Qdrant connections
2025-12-29T01:00:10Z INFO  Releasing GPU resources
2025-12-29T01:00:10Z INFO  Shutdown complete, exiting
```

---

## Testing Shutdown

```bash
# Start server
./rice-search serve &
PID=$!

# Send requests
curl -X POST http://localhost:8080/v1/search -d '{"query": "test"}' &

# Trigger shutdown
kill -TERM $PID

# Watch logs
# Should see graceful drain
```

### Integration Test

```go
func TestGracefulShutdown(t *testing.T) {
    server := startServer()
    
    // Start long request
    go func() {
        resp, _ := http.Post(searchURL, "application/json", slowQuery)
        assert.Equal(t, 200, resp.StatusCode)
    }()
    
    time.Sleep(100 * time.Millisecond)
    
    // Trigger shutdown
    server.Shutdown()
    
    // Should complete without error
    assert.NoError(t, server.Wait())
}
```
