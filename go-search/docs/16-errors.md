# Error Handling

## Error Types

### Base Error

```go
type Error struct {
    Code       string         `json:"code"`
    Message    string         `json:"message"`
    Details    map[string]any `json:"details,omitempty"`
    Cause      error          `json:"-"`
    StatusCode int            `json:"-"`
}

func (e *Error) Error() string {
    return e.Message
}

func (e *Error) Unwrap() error {
    return e.Cause
}
```

---

## Error Codes

### Client Errors (4xx)

| Code | HTTP | Description |
|------|------|-------------|
| `INVALID_REQUEST` | 400 | Malformed JSON, missing body |
| `VALIDATION_ERROR` | 400 | Field validation failed |
| `UNAUTHORIZED` | 401 | Missing or invalid auth |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found (store, document, etc.) |
| `ALREADY_EXISTS` | 409 | Resource already exists (store, etc.) |
| `RATE_LIMITED` | 429 | Too many requests |

### Server Errors (5xx)

| Code | HTTP | Description |
|------|------|-------------|
| `INTERNAL_ERROR` | 500 | Unexpected server error |
| `ML_ERROR` | 500 | ML inference failed |
| `QDRANT_ERROR` | 500 | Qdrant operation failed |
| `INDEXING_ERROR` | 500 | Indexing failed |
| `SERVICE_UNAVAILABLE` | 503 | Dependency unavailable |
| `TIMEOUT` | 504 | Operation timed out |

---

## Error Response Format

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Request validation failed",
        "details": {
            "field": "query",
            "reason": "cannot be empty"
        }
    },
    "meta": {
        "request_id": "req_abc123"
    }
}
```

### Multiple Validation Errors

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Multiple validation errors",
        "details": {
            "errors": [
                {"field": "query", "reason": "cannot be empty"},
                {"field": "top_k", "reason": "must be positive"}
            ]
        }
    }
}
```

---

## Error Constructors

```go
// Client errors
func ErrInvalidRequest(msg string) *Error
func ErrValidation(field, reason string) *Error
func ErrUnauthorized() *Error
func ErrStoreNotFound(name string) *Error
func ErrRateLimited(retryAfter time.Duration) *Error

// Server errors
func ErrInternal(cause error) *Error
func ErrMLFailed(cause error) *Error
func ErrQdrantFailed(cause error) *Error
func ErrTimeout(operation string) *Error
func ErrServiceUnavailable(service string) *Error
```

---

## Validation Errors

### Field Validation

| Field | Validations |
|-------|-------------|
| `query` | Required, max 10000 chars |
| `top_k` | 1-1000 |
| `store` | Required, alphanumeric + hyphen, max 64 chars |
| `path` | Required, max 1024 chars |
| `content` | Required, max 10MB |
| `sparse_weight` | 0.0-1.0 |
| `dense_weight` | 0.0-1.0 |

### Validation Response

```json
{
    "error": {
        "code": "VALIDATION_ERROR",
        "message": "Field 'top_k' must be between 1 and 1000",
        "details": {
            "field": "top_k",
            "value": 5000,
            "constraint": "max:1000"
        }
    }
}
```

---

## Retry Strategy

### Retryable Errors

| Error | Retryable | Strategy |
|-------|-----------|----------|
| `RATE_LIMITED` | Yes | Wait for `Retry-After` |
| `TIMEOUT` | Yes | Exponential backoff |
| `SERVICE_UNAVAILABLE` | Yes | Exponential backoff |
| `QDRANT_ERROR` | Sometimes | Retry connection errors |
| `ML_ERROR` | Sometimes | Retry OOM errors |
| `VALIDATION_ERROR` | No | Fix request |
| `NOT_FOUND` | No | Create resource first |

### Retry Headers

```http
Retry-After: 60
X-RateLimit-Reset: 1735430400
```

### Exponential Backoff

```
Attempt 1: Wait 100ms
Attempt 2: Wait 200ms
Attempt 3: Wait 400ms
Max retries: 3
Max wait: 5s
```

---

## Error Logging

### Log Format

```json
{
    "level": "error",
    "time": "2025-12-29T01:00:00Z",
    "request_id": "req_abc123",
    "error_code": "QDRANT_ERROR",
    "message": "Failed to search Qdrant",
    "cause": "connection refused",
    "stack": "..."
}
```

### What to Log

| Level | When |
|-------|------|
| `error` | 5xx errors, unexpected failures |
| `warn` | 4xx errors, retryable failures |
| `info` | Successful operations |
| `debug` | Request/response details |

### Sensitive Data

Never log:
- Full request body with content
- API keys
- Embeddings (too large)
- Full file contents

---

## Error Propagation

### Event Bus Errors

```go
// Event response includes error
type EmbedResponse struct {
    CorrelationID string      `json:"correlation_id"`
    Embeddings    [][]float32 `json:"embeddings,omitempty"`
    Error         *Error      `json:"error,omitempty"`
}

// Check for error
if resp.Error != nil {
    return nil, resp.Error
}
```

### HTTP → Event → HTTP

```
1. HTTP request arrives
2. Publish event to bus
3. Service processes, fails
4. Publish error response
5. API receives error response
6. Convert to HTTP error
7. Return to client
```

---

## Panic Recovery

### HTTP Middleware

```go
func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Error("panic recovered", "error", err, "stack", debug.Stack())
                WriteError(w, ErrInternal(fmt.Errorf("panic: %v", err)))
            }
        }()
        next.ServeHTTP(w, r)
    })
}
```

### Event Handler

```go
func SafeHandler(handler Handler) Handler {
    return func(ctx context.Context, event any) (err error) {
        defer func() {
            if r := recover(); r != nil {
                err = fmt.Errorf("panic in handler: %v", r)
            }
        }()
        return handler(ctx, event)
    }
}
```

---

## Circuit Breaker

> ⚠️ **NOT IMPLEMENTED** - Circuit breaker is documented but not yet implemented in the codebase.

For external dependencies (Qdrant, ML models).

### States

| State | Description |
|-------|-------------|
| Closed | Normal operation |
| Open | Failing, reject requests |
| Half-Open | Testing recovery |

### Configuration

```go
type CircuitBreaker struct {
    FailureThreshold int           // Failures before open
    SuccessThreshold int           // Successes to close
    Timeout          time.Duration // Time in open state
}
```

### Usage

```go
cb := NewCircuitBreaker(5, 2, 30*time.Second)

result, err := cb.Execute(func() (any, error) {
    return qdrant.Search(query)
})

if errors.Is(err, ErrCircuitOpen) {
    return ErrServiceUnavailable("qdrant")
}
```
