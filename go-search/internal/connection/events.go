package connection

// Event topics for connection lifecycle.
const (
	// TopicConnectionRegistered is published when a connection is registered (new or updated).
	TopicConnectionRegistered = "connection.registered"

	// TopicConnectionSeen is published when a connection's last seen time is updated.
	TopicConnectionSeen = "connection.seen"

	// TopicConnectionDeleted is published when a connection is deleted.
	TopicConnectionDeleted = "connection.deleted"
)

// Event payload structures for connection events.

// ConnectionRegisteredPayload is the payload for connection.registered events.
type ConnectionRegisteredPayload struct {
	Connection *Connection `json:"connection"`
	IsNew      bool        `json:"is_new"` // true if newly created, false if updated
}

// ConnectionSeenPayload is the payload for connection.seen events.
type ConnectionSeenPayload struct {
	ConnectionID string `json:"connection_id"`
	IP           string `json:"ip"`
	Timestamp    int64  `json:"timestamp"`
}

// ConnectionDeletedPayload is the payload for connection.deleted events.
type ConnectionDeletedPayload struct {
	ConnectionID string `json:"connection_id"`
	Name         string `json:"name"`
}
