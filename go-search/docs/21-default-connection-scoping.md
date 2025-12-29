# Default Connection Scoping

## Overview

Search requests now automatically scope to the client's connection by default when the `X-Connection-ID` header is present. This provides better isolation and more relevant results without requiring explicit filtering.

## Behavior

### Automatic Scoping

When a client sends a search request with the `X-Connection-ID` header:

```http
POST /v1/stores/default/search
X-Connection-ID: conn-abc123
Content-Type: application/json

{
  "query": "authentication handler"
}
```

The search automatically filters to files indexed by that connection, equivalent to:

```json
{
  "query": "authentication handler",
  "filter": {
    "connection_id": "conn-abc123"
  }
}
```

### Explicit Override

Users can override the default by explicitly setting `connection_id` in the filter:

```json
{
  "query": "authentication handler",
  "filter": {
    "connection_id": "conn-xyz789"
  }
}
```

This searches files from `conn-xyz789` even if the header contains `conn-abc123`.

### Opt-Out (Search All Connections)

To search across all connections, set `connection_id` to `"*"` or `"all"`:

```json
{
  "query": "authentication handler",
  "filter": {
    "connection_id": "*"
  }
}
```

Or:

```json
{
  "query": "authentication handler",
  "filter": {
    "connection_id": "all"
  }
}
```

## Implementation

### Handler Changes

The `applyDefaultConnectionScope` method in `internal/search/handlers.go`:

1. Extracts `X-Connection-ID` from request headers
2. If no explicit `connection_id` filter is set, uses the header value
3. If filter is `"*"` or `"all"`, clears it (opt-out)
4. Otherwise, respects explicit filter values

### API Contract

The API remains fully backward compatible:

- **No header + No filter** → Search all connections (previous behavior)
- **Header + No filter** → Search header's connection (new default)
- **Header + Explicit filter** → Search explicit connection (user override)
- **Header + `"*"` or `"all"`** → Search all connections (opt-out)

## Testing

Comprehensive tests in `internal/search/search_test.go` cover:

- No header, no filter
- Header present, no filter (auto-scoping)
- Header present, explicit filter (override)
- Header present, `"*"` filter (opt-out)
- Header present, `"all"` filter (opt-out)
- No header, explicit filter

All tests pass. Run with:

```bash
go test ./internal/search -v -run TestApplyDefaultConnectionScope
```

## Client Integration

### Go Client

The `internal/client/client.go` already sends `X-Connection-ID` on all requests:

```go
req.Header.Set("X-Connection-ID", c.connectionID)
```

No changes needed - clients automatically benefit from scoped search.

### ricegrep CLI

The ricegrep CLI (when it integrates with go-search) will:

1. Send its connection ID in the header (already implemented)
2. By default, see only its own files
3. Can opt-out with `--all-connections` flag (maps to `connection_id: "*"`)

### Web UI (Future)

The Web UI can:

1. Extract connection ID from auth/session
2. Add to search requests in the header
3. Provide a checkbox "Search all connections" that sets `connection_id: "*"`

## Logging

When default scoping is applied, a debug log entry is written (commented out in production):

```go
// h.log.Debug("Applied default connection scoping", "connection_id", headerConnID)
```

Uncomment for debugging connection-scoped searches.

## Migration Notes

This is a **non-breaking change**:

- Existing clients without `X-Connection-ID` header: No change in behavior
- Existing clients with explicit filters: No change in behavior
- New clients with header: Automatically get scoped results (better defaults)

No migration or database changes required.
