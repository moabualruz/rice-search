// Package context provides context utilities for Rice Search.
package context

import (
	"context"
)

type contextKey string

const (
	// ConnectionIDKey is the context key for storing connection ID
	ConnectionIDKey contextKey = "connection_id"
)

// WithConnectionID adds a connection ID to the context.
func WithConnectionID(ctx context.Context, connectionID string) context.Context {
	return context.WithValue(ctx, ConnectionIDKey, connectionID)
}

// GetConnectionID retrieves the connection ID from context.
// Returns empty string if not found.
func GetConnectionID(ctx context.Context) string {
	if connectionID, ok := ctx.Value(ConnectionIDKey).(string); ok {
		return connectionID
	}
	return ""
}
