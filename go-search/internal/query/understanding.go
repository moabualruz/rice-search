package query

import "context"

// Understanding defines the interface for query understanding.
type Understanding interface {
	// Parse analyzes a query and returns structured understanding.
	Parse(ctx context.Context, query string) (*ParsedQuery, error)

	// IsModelEnabled returns true if ML model-based understanding is enabled.
	IsModelEnabled() bool

	// SetModelEnabled enables or disables model-based understanding.
	SetModelEnabled(enabled bool)
}
