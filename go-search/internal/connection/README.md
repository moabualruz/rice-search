# Connection Package

The `connection` package provides connection and PC tracking for Rice Search. It tracks unique client machines (ricegrep CLI instances) that access the system, enabling usage analytics, access control, and client management.

## Overview

A **Connection** represents a unique client machine identified by its hardware and system information. Each connection is tracked with:

- **Unique ID**: Deterministically generated from PC info (MAC address, hostname, username)
- **PC Information**: Hostname, OS, architecture, MAC address, username
- **Store Access**: List of stores this connection can access (nil = all stores)
- **Usage Statistics**: Files indexed and searches performed
- **Activity Status**: Active/inactive state for access control
- **Last Seen**: Timestamp and IP address of last activity

## Key Features

### 1. Deterministic ID Generation

Connection IDs are generated deterministically from PC information, ensuring the same machine always gets the same ID:

```go
pcInfo := PCInfo{
    Hostname:   "laptop",
    OS:         "linux",
    Arch:       "amd64",
    MACAddress: "00:11:22:33:44:55",
    Username:   "user",
}

id := GenerateConnectionID(pcInfo)
// Always returns same ID for same PC info
// Format: "conn_" + first 16 chars of SHA256 hash
```

**Priority for ID generation:**
1. MAC address (most unique)
2. Hostname + Username (if no MAC)
3. Hostname only (fallback)

Combined with OS + Arch for additional specificity.

### 2. Store Access Control

Connections have fine-grained access control to stores:

```go
conn := NewConnection("my-laptop", pcInfo)

// Initial state: nil StoreAccess = access to ALL stores
conn.HasStoreAccess("any-store") // true

// Grant explicit access - transitions to explicit mode
conn.GrantStoreAccess("project1")
conn.HasStoreAccess("project1") // true
conn.HasStoreAccess("project2") // false (no longer has access to all)

// Revoke access
conn.RevokeStoreAccess("project1")
conn.HasStoreAccess("project1") // false

// Wildcard access
conn.StoreAccess = []string{"*"}
conn.HasStoreAccess("any-store") // true
```

**Access States:**
- `nil`: Access to all stores (default for new connections)
- `[]string{"store1", "store2"}`: Explicit access to listed stores only
- `[]string{"*"}`: Wildcard access to all stores
- `[]string{}`: No access to any stores

### 3. Usage Tracking

Track connection activity:

```go
// Update last seen
conn.Touch("192.168.1.100")

// Increment statistics
svc.IncrementStats(ctx, connID, indexedFiles, searches)
```

### 4. Event Publishing

The service publishes events via the event bus:

```go
// Published events:
TopicConnectionRegistered = "connection.registered"  // New or updated connection
TopicConnectionSeen       = "connection.seen"        // Last seen updated
TopicConnectionDeleted    = "connection.deleted"     // Connection removed
```

## Usage

### Creating a Service

```go
import (
    "github.com/ricesearch/rice-search/internal/connection"
    "github.com/ricesearch/rice-search/internal/bus"
)

eventBus := bus.NewMemoryBus()
svc, err := connection.NewService(eventBus, connection.ServiceConfig{
    StoragePath: "./data/connections",  // Persist to files
})
```

### Registering Connections

```go
pcInfo := connection.PCInfo{
    Hostname:   "laptop",
    OS:         "linux",
    Arch:       "amd64",
    MACAddress: "00:11:22:33:44:55",
    Username:   "user",
}

conn := connection.NewConnection("my-laptop", pcInfo)
err := svc.RegisterConnection(ctx, conn)

// Or get/create in one call
conn, err := svc.GetOrCreate(ctx, "my-laptop", pcInfo)
```

### Listing Connections

```go
// List all connections
all, err := svc.ListConnections(ctx, connection.ConnectionFilter{})

// Filter by active only
active, err := svc.ListConnections(ctx, connection.ConnectionFilter{
    ActiveOnly: true,
})

// Filter by explicit store access
withAccess, err := svc.ListConnections(ctx, connection.ConnectionFilter{
    Store: "project1",  // Only connections with explicit access
})
```

### Managing Access

```go
// Grant access to a store
err := svc.GrantStoreAccess(ctx, connID, "project1")

// Revoke access
err := svc.RevokeStoreAccess(ctx, connID, "project1")

// Disable a connection
err := svc.SetActive(ctx, connID, false)
```

### Updating Activity

```go
// Update last seen
err := svc.UpdateLastSeen(ctx, connID, "192.168.1.100")

// Increment usage stats
err := svc.IncrementStats(ctx, connID, filesIndexed, searchesPerformed)
```

## Data Models

### Connection

```go
type Connection struct {
    ID           string    // Deterministic ID from PC info
    Name         string    // Human-readable name
    PCInfo       PCInfo    // PC identification info
    StoreAccess  []string  // Stores accessible (nil=all, empty=none)
    CreatedAt    time.Time // First registration
    LastSeenAt   time.Time // Last activity
    LastIP       string    // Last IP address
    IndexedFiles int64     // Total files indexed
    SearchCount  int64     // Total searches performed
    IsActive     bool      // Access enabled/disabled
}
```

### PCInfo

```go
type PCInfo struct {
    Hostname   string // Machine hostname
    OS         string // Operating system (linux, darwin, windows)
    Arch       string // Architecture (amd64, arm64)
    MACAddress string // MAC address (optional)
    Username   string // OS username (optional)
}
```

### ConnectionFilter

```go
type ConnectionFilter struct {
    ActiveOnly bool   // Only return active connections
    Store      string // Only return connections with explicit access to this store
}
```

## Storage

The package supports two storage backends:

### Memory Storage (Testing)

```go
storage := connection.NewMemoryStorage()
```

### File Storage (Production)

Stores connections as JSON files in `{basePath}/{connectionID}.json`:

```go
storage := connection.NewFileStorage("./data/connections")
```

**File format:**
```json
{
  "id": "conn_a1b2c3d4e5f6g7h8",
  "name": "my-laptop",
  "pc_info": {
    "hostname": "laptop",
    "os": "linux",
    "arch": "amd64",
    "mac_address": "00:11:22:33:44:55",
    "username": "user"
  },
  "store_access": ["project1", "project2"],
  "created_at": "2025-01-01T00:00:00Z",
  "last_seen_at": "2025-01-01T12:00:00Z",
  "last_ip": "192.168.1.100",
  "indexed_files": 1500,
  "search_count": 250,
  "is_active": true
}
```

## Validation

### Connection Name Rules

- Must start with alphanumeric character
- Can contain alphanumeric, hyphens, underscores
- Max length: 64 characters
- Examples:
  - ✅ `my-laptop`, `MyLaptop`, `laptop_123`
  - ❌ `-laptop`, `my laptop`, `my.laptop`

### Connection Validation

```go
err := conn.Validate()
// Checks:
// - Name is valid
// - Hostname is not empty
// - OS is not empty
// - Arch is not empty
```

## Thread Safety

All service methods are thread-safe using `sync.RWMutex`:

- Read methods use `RLock()` (concurrent reads allowed)
- Write methods use `Lock()` (exclusive access)
- Storage operations are also protected with mutexes

## Testing

The package includes comprehensive tests covering:

- Name validation
- ID generation (deterministic)
- Store access control (all states)
- Storage backends (memory + file)
- Service operations (CRUD + filters)
- Event publishing
- Concurrent access

Run tests:
```bash
go test -v ./internal/connection/...
```

## Integration

### With ricegrep CLI

The ricegrep CLI should:

1. Collect PC info on startup
2. Register/update connection on first API call
3. Include connection ID in all subsequent requests
4. Update last seen periodically

### With API Server

The API server should:

1. Accept connection info in request headers
2. Update last seen on each request
3. Check store access before operations
4. Increment stats after indexing/searching
5. Subscribe to connection events for logging

### With Event Bus

Other services can subscribe to connection events:

```go
bus.Subscribe(ctx, connection.TopicConnectionRegistered, func(ctx context.Context, event bus.Event) error {
    // Handle new/updated connection
    conn := event.Payload.(*connection.Connection)
    log.Printf("Connection registered: %s (%s)", conn.Name, conn.ID)
    return nil
})
```

## Best Practices

1. **Use GetOrCreate** for client registration - idempotent and handles updates
2. **Filter by explicit access** when auditing - use `HasExplicitStoreAccess`
3. **Disable instead of delete** - preserve historical data by setting `IsActive = false`
4. **Persist connections** - use FileStorage in production for durability
5. **Monitor events** - subscribe to connection events for security auditing
6. **Update last seen** - call on every API request to track active clients

## Future Enhancements

Potential improvements:

- [ ] Connection groups/tags for batch management
- [ ] Rate limiting per connection
- [ ] Connection quotas (max files, searches)
- [ ] Expiration/TTL for inactive connections
- [ ] IP whitelist/blacklist per connection
- [ ] Audit log of all connection actions
- [ ] REST API endpoints for connection management
- [ ] CLI commands for connection administration
