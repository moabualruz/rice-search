package connection

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
)

func TestValidateConnectionName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"my-laptop", true},
		{"MyLaptop", true},
		{"my_laptop", true},
		{"laptop123", true},
		{"123laptop", true},
		{"a", true},
		{"", false},          // Empty
		{"-laptop", false},   // Starts with hyphen
		{"_laptop", false},   // Starts with underscore
		{"my laptop", false}, // Space
		{"my.laptop", false}, // Dot
		{"my@laptop", false}, // Special char
		{"my/laptop", false}, // Slash
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConnectionName(tt.name)
			if tt.valid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", tt.name, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected %q to be invalid, got no error", tt.name)
			}
		})
	}
}

func TestGenerateConnectionID(t *testing.T) {
	// Test deterministic ID generation
	pcInfo1 := PCInfo{
		Hostname:   "laptop",
		OS:         "linux",
		Arch:       "amd64",
		MACAddress: "00:11:22:33:44:55",
		Username:   "user",
	}

	id1 := GenerateConnectionID(pcInfo1)
	id2 := GenerateConnectionID(pcInfo1)

	if id1 != id2 {
		t.Error("expected same PCInfo to generate same ID")
	}

	if !contains(id1, "conn_") {
		t.Error("expected ID to start with 'conn_' prefix")
	}

	// Different MAC = different ID
	pcInfo2 := pcInfo1
	pcInfo2.MACAddress = "55:44:33:22:11:00"

	id3 := GenerateConnectionID(pcInfo2)
	if id1 == id3 {
		t.Error("expected different MAC to generate different ID")
	}

	// No MAC address - use hostname + username
	pcInfo3 := PCInfo{
		Hostname: "laptop",
		OS:       "darwin",
		Arch:     "arm64",
		Username: "user",
	}

	id4 := GenerateConnectionID(pcInfo3)
	if !contains(id4, "conn_") {
		t.Error("expected ID to start with 'conn_' prefix")
	}

	// Different username = different ID
	pcInfo4 := pcInfo3
	pcInfo4.Username = "admin"

	id5 := GenerateConnectionID(pcInfo4)
	if id4 == id5 {
		t.Error("expected different username to generate different ID")
	}
}

func TestNewConnection(t *testing.T) {
	pcInfo := PCInfo{
		Hostname:   "laptop",
		OS:         "linux",
		Arch:       "amd64",
		MACAddress: "00:11:22:33:44:55",
		Username:   "user",
	}

	conn := NewConnection("my-laptop", pcInfo)

	if conn.Name != "my-laptop" {
		t.Errorf("expected name 'my-laptop', got %s", conn.Name)
	}

	if conn.PCInfo.Hostname != "laptop" {
		t.Errorf("expected hostname 'laptop', got %s", conn.PCInfo.Hostname)
	}

	if conn.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	if conn.LastSeenAt.IsZero() {
		t.Error("expected LastSeenAt to be set")
	}

	if !conn.IsActive {
		t.Error("expected new connection to be active")
	}

	if conn.StoreAccess != nil {
		t.Error("expected new connection to have nil StoreAccess (all stores)")
	}
}

func TestConnectionValidate(t *testing.T) {
	pcInfo := PCInfo{
		Hostname: "laptop",
		OS:       "linux",
		Arch:     "amd64",
	}

	// Valid connection
	conn := NewConnection("my-laptop", pcInfo)
	if err := conn.Validate(); err != nil {
		t.Errorf("expected valid connection, got error: %v", err)
	}

	// Invalid name
	conn.Name = ""
	if err := conn.Validate(); err == nil {
		t.Error("expected error for empty name")
	}
	conn.Name = "my-laptop"

	// Missing hostname
	conn.PCInfo.Hostname = ""
	if err := conn.Validate(); err == nil {
		t.Error("expected error for empty hostname")
	}
	conn.PCInfo.Hostname = "laptop"

	// Missing OS
	conn.PCInfo.OS = ""
	if err := conn.Validate(); err == nil {
		t.Error("expected error for empty OS")
	}
	conn.PCInfo.OS = "linux"

	// Missing Arch
	conn.PCInfo.Arch = ""
	if err := conn.Validate(); err == nil {
		t.Error("expected error for empty Arch")
	}
}

func TestConnectionTouch(t *testing.T) {
	pcInfo := PCInfo{Hostname: "laptop", OS: "linux", Arch: "amd64"}
	conn := NewConnection("my-laptop", pcInfo)

	originalLastSeen := conn.LastSeenAt
	time.Sleep(time.Millisecond * 10)

	conn.Touch("192.168.1.1")

	if !conn.LastSeenAt.After(originalLastSeen) {
		t.Error("expected LastSeenAt to be updated")
	}

	if conn.LastIP != "192.168.1.1" {
		t.Errorf("expected LastIP '192.168.1.1', got %s", conn.LastIP)
	}

	// Touch without IP
	conn.Touch("")
	if conn.LastIP != "192.168.1.1" {
		t.Error("expected LastIP to remain unchanged when empty IP provided")
	}
}

func TestConnectionStoreAccess(t *testing.T) {
	pcInfo := PCInfo{Hostname: "laptop", OS: "linux", Arch: "amd64"}
	conn := NewConnection("my-laptop", pcInfo)

	// nil list = access all stores
	if !conn.HasStoreAccess("any-store") {
		t.Error("expected access to all stores with nil list")
	}

	// Grant access to specific store
	conn.GrantStoreAccess("project1")
	if !conn.HasStoreAccess("project1") {
		t.Error("expected access to project1")
	}

	if conn.HasStoreAccess("project2") {
		t.Error("expected no access to project2")
	}

	// Grant access to another store
	conn.GrantStoreAccess("project2")
	if !conn.HasStoreAccess("project2") {
		t.Error("expected access to project2")
	}

	// Grant duplicate - should not add
	conn.GrantStoreAccess("project1")
	count := 0
	for _, s := range conn.StoreAccess {
		if s == "project1" {
			count++
		}
	}
	if count != 1 {
		t.Error("expected only one entry for project1")
	}

	// Wildcard access
	conn.StoreAccess = []string{"*"}
	if !conn.HasStoreAccess("any-store") {
		t.Error("expected wildcard to grant access to all")
	}

	// Revoke access
	conn.StoreAccess = []string{"project1", "project2", "project3"}
	conn.RevokeStoreAccess("project2")

	if conn.HasStoreAccess("project2") {
		t.Error("expected access to project2 to be revoked")
	}

	if !conn.HasStoreAccess("project1") {
		t.Error("expected access to project1 to remain")
	}

	if len(conn.StoreAccess) != 2 {
		t.Errorf("expected 2 stores in access list, got %d", len(conn.StoreAccess))
	}
}

func TestMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	pcInfo := PCInfo{Hostname: "laptop", OS: "linux", Arch: "amd64"}

	// Test save and load
	conn := NewConnection("my-laptop", pcInfo)
	if err := storage.Save(conn); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := storage.Load(conn.ID)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.ID != conn.ID {
		t.Errorf("expected ID %s, got %s", conn.ID, loaded.ID)
	}

	if loaded.Name != conn.Name {
		t.Errorf("expected name %s, got %s", conn.Name, loaded.Name)
	}

	// Test exists
	if !storage.Exists(conn.ID) {
		t.Error("expected connection to exist")
	}

	if storage.Exists("nonexistent") {
		t.Error("expected nonexistent connection to not exist")
	}

	// Test load all
	pcInfo2 := PCInfo{Hostname: "desktop", OS: "windows", Arch: "amd64"}
	conn2 := NewConnection("my-desktop", pcInfo2)
	if err := storage.Save(conn2); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	all, err := storage.LoadAll()
	if err != nil {
		t.Fatalf("failed to load all: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("expected 2 connections, got %d", len(all))
	}

	// Test delete
	if err := storage.Delete(conn.ID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if storage.Exists(conn.ID) {
		t.Error("expected connection to be deleted")
	}

	_ = ctx // Unused but shows it's available for context-based operations
}

func TestFileStorage(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "connection_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage := NewFileStorage(tempDir)
	pcInfo := PCInfo{Hostname: "laptop", OS: "linux", Arch: "amd64", Username: "user"}

	// Test save and load
	conn := NewConnection("my-laptop", pcInfo)
	if err := storage.Save(conn); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Check file exists
	filePath := filepath.Join(tempDir, conn.ID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected connection file to exist")
	}

	loaded, err := storage.Load(conn.ID)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.ID != conn.ID {
		t.Errorf("expected ID %s, got %s", conn.ID, loaded.ID)
	}

	if loaded.PCInfo.Username != conn.PCInfo.Username {
		t.Errorf("expected username %s, got %s", conn.PCInfo.Username, loaded.PCInfo.Username)
	}

	// Test exists
	if !storage.Exists(conn.ID) {
		t.Error("expected connection to exist")
	}

	// Test load nonexistent
	_, err = storage.Load("nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent connection")
	}

	// Test load all
	pcInfo2 := PCInfo{Hostname: "desktop", OS: "windows", Arch: "amd64"}
	conn2 := NewConnection("my-desktop", pcInfo2)
	if err := storage.Save(conn2); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	all, err := storage.LoadAll()
	if err != nil {
		t.Fatalf("failed to load all: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("expected 2 connections, got %d", len(all))
	}

	// Test delete
	if err := storage.Delete(conn.ID); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if storage.Exists(conn.ID) {
		t.Error("expected connection to be deleted")
	}
}

func TestServiceWithMemoryStorage(t *testing.T) {
	ctx := context.Background()
	eventBus := bus.NewMemoryBus()

	svc, err := NewService(eventBus, ServiceConfig{})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	pcInfo := PCInfo{
		Hostname:   "laptop",
		OS:         "linux",
		Arch:       "amd64",
		MACAddress: "00:11:22:33:44:55",
		Username:   "user",
	}

	// Test register connection
	conn := NewConnection("my-laptop", pcInfo)
	if err := svc.RegisterConnection(ctx, conn); err != nil {
		t.Fatalf("failed to register connection: %v", err)
	}

	// Test duplicate registration updates
	conn.LastIP = "192.168.1.100"
	if err := svc.RegisterConnection(ctx, conn); err != nil {
		t.Fatalf("failed to update connection: %v", err)
	}

	// Test get connection
	retrieved, err := svc.GetConnection(ctx, conn.ID)
	if err != nil {
		t.Fatalf("failed to get connection: %v", err)
	}

	if retrieved.ID != conn.ID {
		t.Errorf("expected ID %s, got %s", conn.ID, retrieved.ID)
	}

	// Test connection exists
	if !svc.ConnectionExists(ctx, conn.ID) {
		t.Error("expected connection to exist")
	}

	if svc.ConnectionExists(ctx, "nonexistent") {
		t.Error("expected nonexistent connection to not exist")
	}

	// Test list connections
	pcInfo2 := PCInfo{Hostname: "desktop", OS: "windows", Arch: "amd64"}
	conn2 := NewConnection("my-desktop", pcInfo2)
	if err := svc.RegisterConnection(ctx, conn2); err != nil {
		t.Fatalf("failed to register second connection: %v", err)
	}

	connections, err := svc.ListConnections(ctx, ConnectionFilter{})
	if err != nil {
		t.Fatalf("failed to list connections: %v", err)
	}

	if len(connections) != 2 {
		t.Errorf("expected 2 connections, got %d", len(connections))
	}

	// Test filter by active only
	_ = svc.SetActive(ctx, conn2.ID, false)
	activeOnly, err := svc.ListConnections(ctx, ConnectionFilter{ActiveOnly: true})
	if err != nil {
		t.Fatalf("failed to list active connections: %v", err)
	}

	if len(activeOnly) != 1 {
		t.Errorf("expected 1 active connection, got %d", len(activeOnly))
	}

	// Test update last seen
	if err := svc.UpdateLastSeen(ctx, conn.ID, "192.168.1.200"); err != nil {
		t.Fatalf("failed to update last seen: %v", err)
	}

	updated, _ := svc.GetConnection(ctx, conn.ID)
	if updated.LastIP != "192.168.1.200" {
		t.Errorf("expected LastIP '192.168.1.200', got %s", updated.LastIP)
	}

	// Test increment stats
	if err := svc.IncrementStats(ctx, conn.ID, 100, 50); err != nil {
		t.Fatalf("failed to increment stats: %v", err)
	}

	statsUpdated, _ := svc.GetConnection(ctx, conn.ID)
	if statsUpdated.IndexedFiles != 100 {
		t.Errorf("expected IndexedFiles 100, got %d", statsUpdated.IndexedFiles)
	}

	if statsUpdated.SearchCount != 50 {
		t.Errorf("expected SearchCount 50, got %d", statsUpdated.SearchCount)
	}

	// Increment again - should add
	if err := svc.IncrementStats(ctx, conn.ID, 50, 25); err != nil {
		t.Fatalf("failed to increment stats again: %v", err)
	}

	statsUpdated2, _ := svc.GetConnection(ctx, conn.ID)
	if statsUpdated2.IndexedFiles != 150 {
		t.Errorf("expected IndexedFiles 150, got %d", statsUpdated2.IndexedFiles)
	}

	// Test store access
	if err := svc.GrantStoreAccess(ctx, conn.ID, "project1"); err != nil {
		t.Fatalf("failed to grant store access: %v", err)
	}

	withAccess, _ := svc.GetConnection(ctx, conn.ID)
	if !withAccess.HasStoreAccess("project1") {
		t.Error("expected access to project1")
	}

	// Filter by store access
	storeFiltered, err := svc.ListConnections(ctx, ConnectionFilter{Store: "project1"})
	if err != nil {
		t.Fatalf("failed to list connections by store: %v", err)
	}

	if len(storeFiltered) != 1 {
		t.Errorf("expected 1 connection with access, got %d", len(storeFiltered))
	}

	if err := svc.RevokeStoreAccess(ctx, conn.ID, "project1"); err != nil {
		t.Fatalf("failed to revoke store access: %v", err)
	}

	withoutAccess, _ := svc.GetConnection(ctx, conn.ID)
	if withoutAccess.HasStoreAccess("project1") {
		t.Error("expected no access to project1")
	}

	// Test delete connection
	if err := svc.DeleteConnection(ctx, conn.ID); err != nil {
		t.Fatalf("failed to delete connection: %v", err)
	}

	if svc.ConnectionExists(ctx, conn.ID) {
		t.Error("expected connection to be deleted")
	}
}

func TestServiceGetOrCreate(t *testing.T) {
	ctx := context.Background()
	eventBus := bus.NewMemoryBus()

	svc, err := NewService(eventBus, ServiceConfig{})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	pcInfo := PCInfo{
		Hostname:   "laptop",
		OS:         "linux",
		Arch:       "amd64",
		MACAddress: "00:11:22:33:44:55",
	}

	// First call should create
	conn1, err := svc.GetOrCreate(ctx, "my-laptop", pcInfo)
	if err != nil {
		t.Fatalf("failed to get or create: %v", err)
	}

	if conn1.Name != "my-laptop" {
		t.Errorf("expected name 'my-laptop', got %s", conn1.Name)
	}

	// Second call with same PCInfo should return existing
	conn2, err := svc.GetOrCreate(ctx, "my-laptop-renamed", pcInfo)
	if err != nil {
		t.Fatalf("failed to get or create again: %v", err)
	}

	if conn1.ID != conn2.ID {
		t.Error("expected same connection ID for same PCInfo")
	}

	// Should only have 1 connection
	all, _ := svc.ListConnections(ctx, ConnectionFilter{})
	if len(all) != 1 {
		t.Errorf("expected 1 connection, got %d", len(all))
	}
}

func TestEventTopics(t *testing.T) {
	// Test that event topics are defined
	if TopicConnectionRegistered == "" {
		t.Error("TopicConnectionRegistered should not be empty")
	}

	if TopicConnectionSeen == "" {
		t.Error("TopicConnectionSeen should not be empty")
	}

	if TopicConnectionDeleted == "" {
		t.Error("TopicConnectionDeleted should not be empty")
	}

	// Ensure topics are unique
	topics := map[string]bool{
		TopicConnectionRegistered: true,
		TopicConnectionSeen:       true,
		TopicConnectionDeleted:    true,
	}

	if len(topics) != 3 {
		t.Error("expected 3 unique event topics")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
