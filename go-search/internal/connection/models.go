// Package connection provides connection and PC tracking for Rice Search.
// Connections represent unique client machines (ricegrep CLI instances) accessing the system.
package connection

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"time"
)

// Connection represents a unique client machine accessing Rice Search.
type Connection struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	PCInfo       PCInfo    `json:"pc_info"`
	StoreAccess  []string  `json:"store_access,omitempty"` // stores this connection can access (nil = all, empty = none)
	CreatedAt    time.Time `json:"created_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	LastIP       string    `json:"last_ip"`
	IndexedFiles int64     `json:"indexed_files"` // total files indexed by this connection
	SearchCount  int64     `json:"search_count"`  // total searches performed
	IsActive     bool      `json:"is_active"`     // false = manually disabled
}

// PCInfo contains identifying information about a client machine.
type PCInfo struct {
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`   // e.g., "linux", "darwin", "windows"
	Arch       string `json:"arch"` // e.g., "amd64", "arm64"
	MACAddress string `json:"mac_address,omitempty"`
	Username   string `json:"username,omitempty"`
}

// Connection name validation rules
var (
	// connectionNameRegex validates connection names: alphanumeric + hyphens/underscores
	connectionNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

	// MaxConnectionNameLength is the maximum length of a connection name
	MaxConnectionNameLength = 64
)

// ValidateConnectionName validates a connection name.
func ValidateConnectionName(name string) error {
	if name == "" {
		return fmt.Errorf("connection name cannot be empty")
	}

	if len(name) > MaxConnectionNameLength {
		return fmt.Errorf("connection name cannot exceed %d characters", MaxConnectionNameLength)
	}

	if !connectionNameRegex.MatchString(name) {
		return fmt.Errorf("connection name must be alphanumeric with hyphens/underscores, starting with alphanumeric")
	}

	return nil
}

// Validate validates the connection data.
func (c *Connection) Validate() error {
	if err := ValidateConnectionName(c.Name); err != nil {
		return err
	}

	if c.PCInfo.Hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	if c.PCInfo.OS == "" {
		return fmt.Errorf("os cannot be empty")
	}

	if c.PCInfo.Arch == "" {
		return fmt.Errorf("arch cannot be empty")
	}

	return nil
}

// Touch updates the LastSeenAt timestamp and IP address.
func (c *Connection) Touch(ip string) {
	c.LastSeenAt = time.Now()
	if ip != "" {
		c.LastIP = ip
	}
}

// HasStoreAccess checks if this connection has access to a store.
func (c *Connection) HasStoreAccess(store string) bool {
	// nil means access to all stores (initial state)
	// empty list means explicit access to nothing
	if c.StoreAccess == nil {
		return true
	}

	for _, s := range c.StoreAccess {
		if s == store || s == "*" {
			return true
		}
	}
	return false
}

// HasExplicitStoreAccess checks if the connection has explicit (non-wildcard) access to a store.
// Returns false for connections with nil access (all stores) or wildcard access.
func (c *Connection) HasExplicitStoreAccess(store string) bool {
	if c.StoreAccess == nil {
		return false // nil = all access, not explicit
	}

	for _, s := range c.StoreAccess {
		if s == store {
			return true
		}
	}
	return false
}

// GrantStoreAccess grants access to a store (if not already granted).
func (c *Connection) GrantStoreAccess(store string) {
	// Initialize to empty list if nil (transition from "all" to "explicit")
	if c.StoreAccess == nil {
		c.StoreAccess = []string{}
	}

	// Check if already in list
	for _, s := range c.StoreAccess {
		if s == store || s == "*" {
			return
		}
	}
	c.StoreAccess = append(c.StoreAccess, store)
}

// RevokeStoreAccess revokes access to a store.
func (c *Connection) RevokeStoreAccess(store string) {
	if c.StoreAccess == nil {
		// Transition from "all access" to "no access to this one store"
		// This is a bit tricky - we can't enumerate all stores to exclude one
		// So we just initialize to empty list (no access)
		c.StoreAccess = []string{}
		return
	}

	filtered := make([]string, 0, len(c.StoreAccess))
	for _, s := range c.StoreAccess {
		if s != store {
			filtered = append(filtered, s)
		}
	}
	c.StoreAccess = filtered
}

// NewConnection creates a new connection with auto-generated ID.
func NewConnection(name string, pcInfo PCInfo) *Connection {
	now := time.Now()
	return &Connection{
		ID:           GenerateConnectionID(pcInfo),
		Name:         name,
		PCInfo:       pcInfo,
		StoreAccess:  nil, // nil = access all stores
		CreatedAt:    now,
		LastSeenAt:   now,
		LastIP:       "",
		IndexedFiles: 0,
		SearchCount:  0,
		IsActive:     true,
	}
}

// GenerateConnectionID generates a deterministic connection ID from PCInfo.
// Priority: MAC address > hostname+username > hostname
func GenerateConnectionID(pcInfo PCInfo) string {
	var input string

	// Prefer MAC address for uniqueness
	if pcInfo.MACAddress != "" {
		input = pcInfo.MACAddress
	} else if pcInfo.Username != "" {
		input = fmt.Sprintf("%s@%s", pcInfo.Username, pcInfo.Hostname)
	} else {
		input = pcInfo.Hostname
	}

	// Add OS+Arch to make ID more specific
	input = fmt.Sprintf("%s|%s|%s", input, pcInfo.OS, pcInfo.Arch)

	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("conn_%x", hash[:8]) // 16-char hex prefix with "conn_" prefix
}

// ConnectionFilter holds filter criteria for listing connections.
type ConnectionFilter struct {
	ActiveOnly bool   // only return active connections
	Store      string // only return connections with access to this store
}
