# Security

> ⚠️ **IMPLEMENTATION STATUS**: Security features are partially implemented. Current deployment is suitable for development and trusted networks.
>
> | Feature | Status |
> |---------|--------|
> | Authentication (API Key, JWT) | ❌ Not implemented |
> | Authorization (store-level, role-based) | ❌ Not implemented |
> | Rate Limiting | ✅ **Implemented** (per-client, configurable) |
> | TLS/HTTPS | ❌ Not implemented (use reverse proxy) |
> | Security Headers | ❌ Not implemented |
> | Audit Logging | ✅ **Fully implemented** (connection + settings) |
> | Input Validation | ✅ **Comprehensive** (12+ validators) |
> | Path Security | ✅ **Advanced** (traversal, null bytes, reserved names) |
> | Log Sanitization | ✅ **Implemented** (masking, injection prevention) |
>
> For production deployments, use a reverse proxy (nginx, Traefik) for TLS and authentication.

## Overview

Security features for authentication, authorization, and input validation.

---

## Authentication Modes

| Mode | Use Case | Configuration |
|------|----------|---------------|
| `none` | Development, internal | Default |
| `api-key` | Simple auth, CLI tools | `AUTH_MODE=api-key` |
| `jwt` | Users, fine-grained access | `AUTH_MODE=jwt` |

---

## API Key Authentication

### Configuration

```yaml
auth:
  mode: api-key
  api_keys:
    - key: "sk_live_abc123"
      name: "production"
      stores: ["*"]  # Access all stores
    - key: "sk_dev_xyz789"
      name: "development"
      stores: ["dev-*"]  # Only dev stores
```

Or via environment:

```bash
AUTH_MODE=api-key
API_KEYS=sk_live_abc123,sk_dev_xyz789
```

### Request Header

```http
X-API-Key: sk_live_abc123
```

### Implementation

```go
func APIKeyMiddleware(keys map[string]APIKey) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            key := c.Request().Header.Get("X-API-Key")
            if key == "" {
                return ErrUnauthorized()
            }
            
            apiKey, exists := keys[key]
            if !exists {
                return ErrUnauthorized()
            }
            
            c.Set("api_key", apiKey)
            return next(c)
        }
    }
}
```

---

## JWT Authentication

### Configuration

```yaml
auth:
  mode: jwt
  jwt:
    secret: "your-256-bit-secret"
    issuer: "rice-search"
    audience: "rice-api"
    expiry: 24h
```

### Request Header

```http
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

### JWT Claims

```json
{
    "sub": "user_123",
    "iss": "rice-search",
    "aud": "rice-api",
    "exp": 1735430400,
    "iat": 1735344000,
    "stores": ["default", "my-project"],
    "role": "user"
}
```

### Roles

| Role | Permissions |
|------|-------------|
| `admin` | All operations, all stores |
| `user` | Search, index own stores |
| `readonly` | Search only |

---

## Authorization

### Store-Level Access

```go
func authorizeStore(c echo.Context, store string) error {
    apiKey := c.Get("api_key").(APIKey)
    
    // Check if wildcard access
    if contains(apiKey.Stores, "*") {
        return nil
    }
    
    // Check specific store access
    for _, pattern := range apiKey.Stores {
        if matchPattern(pattern, store) {
            return nil
        }
    }
    
    return ErrForbidden("no access to store: " + store)
}
```

### Operation-Level Access

| Operation | Required Permission |
|-----------|-------------------|
| Search | `read` |
| Index | `write` |
| Delete | `write` |
| Create store | `admin` |
| Delete store | `admin` |

---

## Rate Limiting

**Status: ✅ IMPLEMENTED**

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `RICE_RATE_LIMIT` | `0` | Requests per second per client (0 = disabled) |

Environment variable:
```bash
RICE_RATE_LIMIT=100  # 100 requests/sec per client
```

### Implementation

Located in `internal/pkg/middleware/ratelimit.go`:

- **Per-client rate limiting** using `golang.org/x/time/rate`
- **Client identification** via X-Forwarded-For, X-Real-IP, or RemoteAddr
- **Automatic cleanup** of stale client entries (5-minute idle timeout)
- **Token bucket algorithm** with configurable burst allowance (2x rate)
- Returns **429 Too Many Requests** when rate limit exceeded

```go
// Example: 100 req/sec per client
type RateLimiter struct {
    mu       sync.RWMutex
    clients  map[string]*rate.Limiter
    rate     rate.Limit
    burst    int
    lastSeen map[string]time.Time
}

func (rl *RateLimiter) Allow(clientIP string) bool {
    return rl.getLimiter(clientIP).Allow()
}
```

### Usage in Server

```go
// Enabled when RICE_RATE_LIMIT > 0 in config
if appCfg.Security.RateLimit > 0 {
    rateLimiter := middleware.NewRateLimiter(middleware.RateLimiterConfig{
        RequestsPerSecond: float64(appCfg.Security.RateLimit),
        Burst:             appCfg.Security.RateLimit * 2,
        CleanupInterval:   time.Minute,
    })
    mux.Use(rateLimiter.Middleware)
}
```

### Rate Limit Response

Status: `429 Too Many Requests`

```json
{
    "error": "rate limit exceeded",
    "code": "RATE_LIMITED",
    "details": {
        "retry_after": "1"
    }
}
```

---

## Input Validation

### Request Size Limits

| Limit | Value | Configuration |
|-------|-------|---------------|
| Request body | 10MB | `MAX_REQUEST_SIZE` |
| Query string | 10KB | Hardcoded |
| Header size | 8KB | Hardcoded |
| File content | 10MB | `MAX_FILE_SIZE` |

### Field Validation

| Field | Validation |
|-------|------------|
| `query` | Required, 1-10000 chars, UTF-8 |
| `store` | Required, alphanumeric + hyphen, 1-64 chars |
| `path` | Required, 1-1024 chars, no null bytes |
| `top_k` | 1-1000 |
| `sparse_weight` | 0.0-1.0 |
| `dense_weight` | 0.0-1.0 |

### Path Traversal Prevention

```go
func validatePath(path string) error {
    // No null bytes
    if strings.Contains(path, "\x00") {
        return ErrInvalidPath("null byte in path")
    }
    
    // No path traversal
    cleaned := filepath.Clean(path)
    if strings.HasPrefix(cleaned, "..") {
        return ErrInvalidPath("path traversal detected")
    }
    
    // No absolute paths
    if filepath.IsAbs(path) {
        return ErrInvalidPath("absolute path not allowed")
    }
    
    return nil
}
```

### Content Validation

```go
func validateContent(content string) error {
    // Size check
    if len(content) > maxFileSize {
        return ErrValidation("content", "exceeds max size")
    }
    
    // UTF-8 check
    if !utf8.ValidString(content) {
        return ErrValidation("content", "invalid UTF-8")
    }
    
    // Binary detection (optional)
    if isBinary(content) {
        return ErrValidation("content", "binary content not allowed")
    }
    
    return nil
}
```

---

## Injection Prevention

### Query Sanitization

```go
func sanitizeQuery(query string) string {
    // Remove control characters
    query = strings.Map(func(r rune) rune {
        if unicode.IsControl(r) && r != '\n' && r != '\t' {
            return -1
        }
        return r
    }, query)
    
    return strings.TrimSpace(query)
}
```

### Log Injection Prevention

```go
func sanitizeForLog(s string) string {
    // Replace newlines to prevent log injection
    s = strings.ReplaceAll(s, "\n", "\\n")
    s = strings.ReplaceAll(s, "\r", "\\r")
    
    // Truncate
    if len(s) > 200 {
        s = s[:200] + "..."
    }
    
    return s
}
```

---

## TLS/HTTPS

### Configuration

```yaml
tls:
  enabled: true
  cert_file: /certs/server.crt
  key_file: /certs/server.key
  min_version: TLS1.2
```

### Implementation

```go
func configureTLS(certFile, keyFile string) *tls.Config {
    return &tls.Config{
        MinVersion: tls.VersionTLS12,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
    }
}
```

---

## Security Headers

```go
func SecurityHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        h := c.Response().Header()
        
        h.Set("X-Content-Type-Options", "nosniff")
        h.Set("X-Frame-Options", "DENY")
        h.Set("X-XSS-Protection", "1; mode=block")
        h.Set("Content-Security-Policy", "default-src 'self'")
        h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
        
        return next(c)
    }
}
```

---

## Secrets Management

### What's Sensitive

| Secret | Storage | Access |
|--------|---------|--------|
| API keys | Environment/Vault | Runtime only |
| JWT secret | Environment/Vault | Runtime only |
| Qdrant API key | Environment | Runtime only |
| HuggingFace token | Environment | Model download |

### Never Log

```go
var sensitiveFields = []string{
    "api_key",
    "authorization",
    "x-api-key",
    "password",
    "secret",
    "token",
}

func maskSensitive(headers http.Header) http.Header {
    masked := make(http.Header)
    for k, v := range headers {
        if isSensitive(k) {
            masked[k] = []string{"[REDACTED]"}
        } else {
            masked[k] = v
        }
    }
    return masked
}
```

---

## Audit Logging

### What to Log

| Event | Fields |
|-------|--------|
| Authentication success | client_id, method, ip |
| Authentication failure | method, ip, reason |
| Store created | user, store_name |
| Store deleted | user, store_name |
| Bulk index | user, store, doc_count |

### Audit Log Format

```json
{
    "time": "2025-12-29T01:00:00Z",
    "type": "audit",
    "event": "store.created",
    "user": "user_123",
    "ip": "10.0.0.1",
    "details": {
        "store": "my-project"
    }
}
```

---

## Implementation Status

| Feature | Status | Notes |
|---------|--------|-------|
| Input Validation | ✅ Implemented | 12+ validators for all input types |
| Path Security | ✅ Implemented | Traversal prevention, null bytes, reserved names |
| Log Sanitization | ✅ Implemented | Injection prevention, sensitive data masking |
| Audit Logging | ✅ Implemented | Connection tracking + settings changes |
| CORS | ✅ Implemented | Configurable via `RICE_CORS_ORIGINS` |
| Rate Limiting | ✅ Implemented | Per-client, configurable via `RICE_RATE_LIMIT` |
| Authentication | ❌ Not Implemented | API key + JWT planned |
| Authorization | ❌ Not Implemented | Store/operation-level planned |
| TLS/HTTPS | ❌ Not Implemented | Use reverse proxy (nginx, Traefik) |
| Security Headers | ❌ Not Implemented | CSP, X-Frame-Options planned |

**Security Score: 6/10 implemented** (suitable for development and trusted networks)

---

## Security Checklist

| Item | Status |
|------|--------|
| Authentication enabled | Required for production |
| Rate limiting enabled | ✅ Available (`RICE_RATE_LIMIT`) |
| TLS enabled | Required for production (use reverse proxy) |
| Input validation | ✅ Always enabled |
| Security headers | Recommended (use reverse proxy) |
| Audit logging | ✅ Enabled by default |
| Secrets in env vars | Required |
