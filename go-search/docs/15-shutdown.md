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

When the server receives SIGTERM, SIGINT, or SIGQUIT (Unix only):

### 1. Signal Handling
```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, shutdownSignals...)
<-sigCh
log.Info("Shutdown signal received")
```

### 2. Set Readiness to False
```go
serverReady.Store(false)
log.Info("Setting server to not ready...")
```
The `/readyz` endpoint will now return 503, stopping new traffic from load balancers.

### 3. Stop HTTP Server
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := httpSrv.Shutdown(ctx); err != nil {
    log.Error("HTTP shutdown error", "error", err)
}
```
This stops accepting new connections but waits for existing requests.

### 4. Drain In-Flight Requests
```go
drainInFlight(shutdownTimeout, log)
```
Polls `inFlightCounter` every 5 seconds until zero or timeout.

```go
func drainInFlight(timeout time.Duration, log *logger.Logger) bool {
    deadline := time.Now().Add(timeout)
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        count := atomic.LoadInt64(&inFlightCounter)
        if count == 0 {
            return true
        }
        
        if time.Now().After(deadline) {
            return false
        }
        
        log.Info("Draining in-flight requests", "remaining", count)
        time.Sleep(100 * time.Millisecond)
    }
}
```

### 5. Close Services (in order)
```go
// Stop monitoring goroutines
// (Monitoring service stops automatically via context cancellation)

// Flush settings audit log
if settingsSvc != nil {
    if err := settingsSvc.Close(); err != nil {
        log.Warn("Error closing settings service", "error", err)
    }
}

// Close metrics (flushes to Redis if configured)
if err := metricsSvc.Close(); err != nil {
    log.Warn("Error closing metrics service", "error", err)
}
```

### 6. Stop gRPC Server
```go
grpcSrv.Stop()
```
Uses `GracefulStop()` for in-flight RPC completion.

```go
func (s *Server) Stop() {
    s.log.Info("Stopping gRPC server...")
    s.grpcServer.GracefulStop()
    s.log.Info("gRPC server stopped")
}
```

### 7. Deferred Cleanups
These run automatically via defer statements (LIFO order):
```go
defer func() { _ = mlSvc.Close() }()        // Release ONNX resources
defer func() { _ = qc.Close() }()           // Close Qdrant gRPC connection
defer func() { _ = eventLogger.Close() }()  // Flush event log buffer
defer func() { _ = innerBus.Close() }()     // Drain pending events
```

**Note**: Connection service does not have a Close method. Data is flushed on each update.

---

## Implementation

See `cmd/rice-search-server/main.go:414-460` for the actual shutdown sequence.

### Signal Waiting

```go
// Wait for shutdown signal (platform-specific: Unix includes SIGQUIT, Windows does not)
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, shutdownSignals...)

<-sigCh
log.Info("Shutdown signal received")
```

### Graceful Shutdown

```go
// Graceful shutdown with in-flight request draining
shutdownTimeout := 30 * time.Second
ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
defer cancel()

// Stop accepting new requests
log.Info("Setting server to not ready...")
serverReady.Store(false)
if err := httpSrv.Shutdown(ctx); err != nil {
    log.Error("HTTP shutdown error", "error", err)
}

// Wait for in-flight requests to complete
log.Info("Draining in-flight requests...")
if drainInFlight(shutdownTimeout, log) {
    log.Info("All in-flight requests completed")
} else {
    remaining := atomic.LoadInt64(&inFlightCounter)
    log.Warn("Shutdown timeout reached with pending requests", "remaining", remaining)
}

// Close services that need cleanup
if settingsSvc != nil {
    if err := settingsSvc.Close(); err != nil {
        log.Warn("Error closing settings service", "error", err)
    } else {
        log.Info("Closed settings service")
    }
}

if err := metricsSvc.Close(); err != nil {
    log.Warn("Error closing metrics service", "error", err)
} else {
    log.Info("Closed metrics service")
}

grpcSrv.Stop()

log.Info("Server stopped")
```

### Drain In-Flight Requests

```go
// drainInFlight waits for all in-flight requests to complete or timeout.
// Returns true if all requests completed, false if timeout reached.
func drainInFlight(timeout time.Duration, log *logger.Logger) bool {
    deadline := time.Now().Add(timeout)
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        count := atomic.LoadInt64(&inFlightCounter)
        if count == 0 {
            return true
        }
        
        if time.Now().After(deadline) {
            return false
        }
        
        select {
        case <-ticker.C:
            log.Info("Draining in-flight requests", "remaining", count)
        default:
            time.Sleep(100 * time.Millisecond)
        }
    }
}
```

### HTTP Server Shutdown

Uses standard `http.Server.Shutdown()` with timeout context:

```go
// Create HTTP server with in-flight tracking middleware
httpSrv := &http.Server{
    Addr:         httpAddr,
    Handler:      recoveryMiddleware(corsMiddleware(loggingMiddleware(inFlightMiddleware(mux), log)), log),
    ReadTimeout:  30 * time.Second,
    WriteTimeout: 60 * time.Second,
}

// Shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := httpSrv.Shutdown(ctx); err != nil {
    log.Error("HTTP shutdown error", "error", err)
}
```

### In-Flight Request Tracking

```go
// inFlightMiddleware tracks in-flight HTTP requests for graceful shutdown.
func inFlightMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt64(&inFlightCounter, 1)
        defer atomic.AddInt64(&inFlightCounter, -1)
        next.ServeHTTP(w, r)
    })
}
```

---

## Timeouts

| Phase | Timeout | Action if Exceeded |
|-------|---------|-------------------|
| Total shutdown | 30s | Log warning, exit anyway |
| HTTP server shutdown | 30s | Force close remaining connections |
| In-flight request drain | 30s | Log warning with remaining count |
| gRPC server | None | Waits indefinitely (GracefulStop) |

**Hardcoded timeout:**
```go
shutdownTimeout := 30 * time.Second
ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
```

**Future**: Could be made configurable via environment variable or config file.

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

## Resource Cleanup

### Qdrant Client

Closes the gRPC connection to Qdrant:

```go
// From internal/qdrant/client.go
func (c *Client) Close() error {
    if c.grpcConn != nil {
        return c.grpcConn.Close()
    }
    return nil
}
```

### ML Service

Closes all ONNX sessions (embed, rerank, sparse):

```go
// From internal/ml/service.go
func (s *ServiceImpl) Close() error {
    var errs []error
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // Close embedder
    if s.embedder != nil {
        if err := s.embedder.Close(); err != nil {
            errs = append(errs, fmt.Errorf("embedder: %w", err))
        }
    }
    
    // Close reranker
    if s.reranker != nil {
        if err := s.reranker.Close(); err != nil {
            errs = append(errs, fmt.Errorf("reranker: %w", err))
        }
    }
    
    // Close sparse encoder
    if s.sparseEncoder != nil {
        if err := s.sparseEncoder.Close(); err != nil {
            errs = append(errs, fmt.Errorf("sparse: %w", err))
        }
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("ML service close errors: %v", errs)
    }
    return nil
}
```

### Event Bus

Closes the memory bus (stops accepting new events):

```go
// From internal/bus/memory.go
func (b *MemoryBus) Close() error {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    if b.closed {
        return ErrBusClosed
    }
    
    b.closed = true
    close(b.closeCh)
    
    // Cancel all subscriptions
    for _, cancel := range b.subs {
        cancel()
    }
    
    return nil
}
```

### Event Logger

Flushes the event log buffer to disk:

```go
// From internal/bus/persistence.go
func (l *EventLogger) Close() error {
    l.mu.Lock()
    defer l.mu.Unlock()
    
    if l.closed {
        return nil
    }
    l.closed = true
    
    // Flush remaining events
    if err := l.flush(); err != nil {
        return err
    }
    
    // Close file
    if l.file != nil {
        return l.file.Close()
    }
    
    return nil
}
```

### Settings Service

Flushes the audit log:

```go
// From internal/settings/service.go
func (s *Service) Close() error {
    if s.auditLogger != nil {
        return s.auditLogger.Close()
    }
    return nil
}
```

### Metrics Service

Closes Redis connection if using Redis persistence:

```go
// From internal/metrics/metrics.go
func (m *Metrics) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if m.redisStorage != nil {
        return m.redisStorage.Close()
    }
    return nil
}
```

---

## Handling Long-Running Operations

All HTTP requests are tracked via the `inFlightMiddleware`:

```go
func inFlightMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        atomic.AddInt64(&inFlightCounter, 1)
        defer atomic.AddInt64(&inFlightCounter, -1)
        next.ServeHTTP(w, r)
    })
}
```

The shutdown sequence waits for all in-flight requests to complete before closing services.

### Context Propagation

All service methods accept a `context.Context` which is:
- Canceled when the HTTP request is canceled
- Canceled when the server shuts down (via `httpSrv.Shutdown()`)

Example from search service:
```go
func (s *Service) Search(ctx context.Context, req Request) (*Response, error) {
    // Check context before expensive operations
    if ctx.Err() != nil {
        return nil, ctx.Err()
    }
    
    // Perform search (respects context cancellation)
    results, err := s.hybridSearch(ctx, req)
    // ...
}
```

### Long Operations

Index operations check context during batch processing:
```go
// From internal/index/pipeline.go
for i, doc := range docs {
    // Check context periodically
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }
    
    if err := s.indexOne(ctx, doc); err != nil {
        return err
    }
}
```

**No explicit shutdown flag or checkpoint system** - relies on context cancellation.

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown (always, even with timeout warnings) |
| 1 | Startup error (config, connections, etc.) |
| 137 | SIGKILL (128 + 9) - cannot catch |
| 143 | SIGTERM (128 + 15) - handled gracefully |

**Note**: The current implementation always exits with code 0 after shutdown sequence completes, even if some requests were dropped or timeouts occurred. Future enhancement could return different exit codes based on shutdown status.

---

## Shutdown Checklist

Current implementation status:

- [x] Signal handling (SIGTERM, SIGINT, SIGQUIT on Unix)
- [x] Readiness flag toggle (`serverReady.Store(false)`)
- [x] HTTP graceful shutdown (`httpSrv.Shutdown(ctx)`)
- [x] In-flight request draining (`drainInFlight()`)
- [x] Service cleanup (settings, metrics)
- [x] gRPC graceful stop (`grpcSrv.Stop()`)
- [x] Resource cleanup via defer (ml, qdrant, event logger, event bus)
- [ ] Monitoring service explicit stop (currently relies on context cancellation)
- [ ] Connection service data flush (currently no Close method - data flushed on updates)
- [ ] Configurable shutdown timeout (currently hardcoded to 30s)
- [ ] Exit code based on shutdown status (currently always exits 0)

### What's Missing

1. **Monitoring Service Stop**: The monitoring service is started with `monitoringSvc.Start(ctx)` and relies on context cancellation to stop its goroutines. No explicit `Stop()` method is called. This works but is implicit.

2. **Connection Service Close**: No `Close()` method exists. Connection data is persisted on each update, so no explicit flush is needed during shutdown.

3. **Configurable Timeout**: The 30-second timeout is hardcoded. Could be made configurable via environment variable or config file.

4. **Exit Codes**: Always exits 0. Could return different codes based on whether timeout was exceeded or services failed to close cleanly.

---

## Logging During Shutdown

Actual shutdown log sequence:

```
2025-12-30T01:00:00Z INFO  Shutdown signal received
2025-12-30T01:00:00Z INFO  Setting server to not ready...
2025-12-30T01:00:00Z INFO  Draining in-flight requests...
2025-12-30T01:00:05Z INFO  Draining in-flight requests remaining=15
2025-12-30T01:00:10Z INFO  Draining in-flight requests remaining=3
2025-12-30T01:00:12Z INFO  All in-flight requests completed
2025-12-30T01:00:12Z INFO  Closed settings service
2025-12-30T01:00:12Z INFO  Closed metrics service
2025-12-30T01:00:12Z INFO  Stopping gRPC server...
2025-12-30T01:00:12Z INFO  gRPC server stopped
2025-12-30T01:00:12Z INFO  Server stopped
```

If shutdown timeout is exceeded:
```
2025-12-30T01:00:30Z WARN  Shutdown timeout reached with pending requests remaining=2
2025-12-30T01:00:30Z INFO  Closed settings service
2025-12-30T01:00:30Z INFO  Closed metrics service
2025-12-30T01:00:30Z INFO  Stopping gRPC server...
2025-12-30T01:00:30Z INFO  gRPC server stopped
2025-12-30T01:00:30Z INFO  Server stopped
```

---

## Testing Shutdown

### Manual Testing

```bash
# Start server
./rice-search-server &
PID=$!

# Start a long-running request (simulated with sleep)
curl -X POST http://localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{"query": "test"}' &

# Trigger shutdown (Unix)
kill -TERM $PID

# Watch logs - should see:
# - "Shutdown signal received"
# - "Setting server to not ready..."
# - "Draining in-flight requests..." (with countdown)
# - "All in-flight requests completed"
# - "Closed settings service"
# - "Closed metrics service"
# - "Server stopped"
```

### Testing Timeout Behavior

```bash
# Start many long requests
for i in {1..10}; do
  curl -X POST http://localhost:8080/v1/stores/default/search \
    -H "Content-Type: application/json" \
    -d '{"query": "test"}' &
done

# Shutdown immediately (they won't finish in 30s)
kill -TERM $PID

# Should see timeout warning after 30s
```

### Testing Readiness Endpoint

```bash
# Start server
./rice-search-server &

# Readiness check (should return 200)
curl http://localhost:8080/readyz
# {"status":"ready"}

# Trigger shutdown
kill -TERM $PID &

# Readiness check (should return 503)
sleep 1
curl http://localhost:8080/readyz
# {"status":"not_ready","reason":"shutting_down"}
```

### Unit Test Example

Currently no unit tests for shutdown. Manual testing required.

**Future**: Could add integration test that:
1. Starts server in background
2. Makes long-running request
3. Sends shutdown signal
4. Verifies request completes successfully
5. Verifies all services closed cleanly
