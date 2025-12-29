# Redis-Based Metrics Persistence

This document describes the Redis-based metrics persistence feature for Rice Search.

## Overview

By default, Rice Search stores metrics history in-memory. With Redis persistence enabled, metrics are automatically saved to Redis, allowing:

- **Persistence across restarts** - Metrics history survives server restarts
- **Multi-instance support** - Multiple server instances can share metrics
- **Automatic cleanup** - Old data is automatically removed based on TTL

## Configuration

### Environment Variables

```bash
# Enable Redis persistence for metrics
RICE_METRICS_PERSISTENCE=redis

# Redis connection URL
RICE_METRICS_REDIS_URL=redis://localhost:6379/0
```

### YAML Configuration

```yaml
metrics:
  persistence: redis  # "memory" (default) or "redis"
  redis_url: redis://localhost:6379/0
```

## Behavior

### In-Memory Mode (Default)

```bash
# No Redis required
RICE_METRICS_PERSISTENCE=memory
```

- Metrics stored in memory only
- Lost on restart
- No external dependencies
- Fast and simple

### Redis Mode

```bash
# Requires Redis connection
RICE_METRICS_PERSISTENCE=redis
RICE_METRICS_REDIS_URL=redis://localhost:6379/0
```

- Metrics saved to Redis automatically
- Non-blocking asynchronous saves
- **Graceful fallback**: If Redis connection fails at startup, falls back to in-memory mode with a warning
- **Automatic recovery**: If Redis becomes available later, metrics will start persisting
- Data retained for 24 hours (configurable)

## Persisted Metrics

The following time-series metrics are persisted to Redis:

- **Search Rate** - Searches per 5-minute bucket
- **Search Latency** - Average latency per bucket
- **Index Rate** - Files indexed per bucket

## Redis Data Structure

Metrics are stored as sorted sets:

```
Key: rice:metrics:{metric_name}
Score: Unix timestamp
Member: Metric value (float)
```

Example:
```redis
# Search rate metric
ZRANGE rice:metrics:search_rate 0 -1 WITHSCORES
```

## TTL and Cleanup

- Default TTL: **24 hours**
- Old data points automatically removed on new writes
- No manual cleanup required

## Usage Examples

### Starting with Redis

```bash
# 1. Start Redis (if not already running)
docker run -d -p 6379:6379 redis:alpine

# 2. Configure Rice Search
export RICE_METRICS_PERSISTENCE=redis
export RICE_METRICS_REDIS_URL=redis://localhost:6379/0

# 3. Start server
./rice-search serve
```

### Programmatic Usage

```go
import (
	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/metrics"
)

// Load configuration
cfg, _ := config.Load("")

// Create metrics instance based on config
var m *metrics.Metrics
if cfg.Metrics.Persistence == "redis" && cfg.Metrics.RedisURL != "" {
	m = metrics.NewWithRedis(cfg.Metrics.RedisURL)
} else {
	m = metrics.New() // In-memory
}

// Use metrics normally
m.RecordSearch(latencyMs, resultCount, nil)

// Close when done (important for Redis mode)
defer m.Close()
```

### Checking Persistence Status

```go
if m.IsRedisPersisted() {
	fmt.Println("Metrics are persisted to Redis")
} else {
	fmt.Println("Metrics are in-memory only")
}
```

## Fallback Behavior

If Redis connection fails:

1. **At startup**: Warning logged, falls back to in-memory mode
2. **During operation**: Writes fail silently, metrics continue in-memory

Example warning:
```
WARNING: Failed to connect to Redis for metrics persistence: dial tcp :6379: connect: connection refused
         Falling back to in-memory metrics
```

## Performance

- **Non-blocking writes**: Redis saves happen asynchronously in goroutines
- **Minimal overhead**: ~2ms timeout per save (non-blocking)
- **No impact on search performance**: Metrics writes never block requests

## Testing

Tests automatically skip if Redis is not available:

```bash
# Run tests (skips Redis tests if not available)
go test ./internal/metrics/

# Run with Redis
docker run -d -p 6379:6379 redis:alpine
go test ./internal/metrics/ -v
```

## Troubleshooting

### Connection Refused

```
Error: dial tcp :6379: connect: connection refused
```

**Solution**: Start Redis server
```bash
docker run -d -p 6379:6379 redis:alpine
```

### Wrong Database

```yaml
# Use database 1 instead of 0
metrics:
  redis_url: redis://localhost:6379/1
```

### Authentication Required

```yaml
# Include password in URL
metrics:
  redis_url: redis://:password@localhost:6379/0
```

### Custom TTL

Currently hardcoded to 24 hours. To customize:

```go
storage, _ := metrics.NewRedisStorage(redisURL)
storage.SetTTL(48 * time.Hour)  // 48 hours
```

## Migration

### From In-Memory to Redis

1. Update configuration
2. Restart server
3. Historical data will be lost (in-memory is ephemeral)
4. New data will be persisted to Redis

### From Redis to In-Memory

1. Update configuration to `persistence: memory`
2. Restart server
3. Historical data in Redis will be ignored
4. New data will be in-memory only

## Monitoring

### Redis Memory Usage

```bash
# Check memory usage
redis-cli INFO memory

# View all metric keys
redis-cli KEYS "rice:metrics:*"

# Check specific metric
redis-cli ZRANGE rice:metrics:search_rate 0 -1 WITHSCORES
```

### Metric Stats

```go
// Get storage statistics
stats, _ := storage.GetStats(ctx)
fmt.Printf("Total metrics: %d\n", stats["total_metrics"])
```

## Best Practices

1. **Use Redis for production** - Metrics history survives restarts
2. **Use in-memory for development** - Simpler, no dependencies
3. **Monitor Redis memory** - Set appropriate TTL
4. **Always call Close()** - Properly close Redis connections
5. **Handle fallback gracefully** - Application continues if Redis fails

## Related

- [Metrics Package](../internal/metrics/README.md)
- [Configuration](09-config.md)
- [Observability](13-observability.md)
