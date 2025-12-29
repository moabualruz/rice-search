# Security

> ⚠️ **IMPLEMENTATION STATUS**: The security features documented below are **NOT YET IMPLEMENTED**. Current deployment is suitable for development and internal/trusted networks only.
>
> | Feature | Status |
> |---------|--------|
> | Authentication (API Key, JWT) | ❌ Not implemented |
> | Authorization (store-level, role-based) | ❌ Not implemented |
> | Rate Limiting | ❌ Not implemented |
> | TLS/HTTPS | ❌ Not implemented (use reverse proxy) |
> | Security Headers | ❌ Not implemented |
> | Audit Logging | ❌ Not implemented |
> | Input Validation | ✅ Basic validation implemented |
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

### Configuration

```yaml
rate_limit:
  enabled: true
  
  # Per-client limits
  search: 100/min
  index: 20/min
  ml: 200/min
  default: 300/min
  
  # Global limits
  global_search: 10000/min
  global_index: 500/min
```

### Implementation

```go
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
}

func (rl *RateLimiter) Allow(clientID, operation string) bool {
    key := clientID + ":" + operation
    
    rl.mu.RLock()
    limiter, exists := rl.limiters[key]
    rl.mu.RUnlock()
    
    if !exists {
        limiter = rl.createLimiter(operation)
        rl.mu.Lock()
        rl.limiters[key] = limiter
        rl.mu.Unlock()
    }
    
    return limiter.Allow()
}
```

### Response Headers

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1735430400
Retry-After: 60
```

### Rate Limit Response

```json
{
    "error": {
        "code": "RATE_LIMITED",
        "message": "Rate limit exceeded",
        "details": {
            "limit": 100,
            "reset_at": "2025-12-29T01:01:00Z"
        }
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

## Security Checklist

| Item | Status |
|------|--------|
| Authentication enabled | Required for production |
| Rate limiting enabled | Required for production |
| TLS enabled | Required for production |
| Input validation | Always |
| Security headers | Always |
| Audit logging | Recommended |
| Secrets in env vars | Required |
