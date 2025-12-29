// Package qdrant provides a wrapper around the Qdrant Go client
// with simplified APIs for Rice Search operations.
package qdrant

import (
	"time"
)

// CollectionConfig defines the configuration for creating a Qdrant collection.
type CollectionConfig struct {
	// Name is the collection name (will be prefixed with "rice_").
	Name string

	// DenseVectorSize is the dimension of dense vectors (e.g., 1536 for Jina).
	DenseVectorSize uint64

	// OnDiskPayload stores payload on disk to save RAM.
	OnDiskPayload bool

	// IndexingThreshold is the number of vectors before HNSW index is built.
	IndexingThreshold uint64

	// MemmapThreshold is the number of vectors before memory-mapping is used.
	MemmapThreshold uint64
}

// DefaultCollectionConfig returns sensible defaults for a code search collection.
func DefaultCollectionConfig(name string) CollectionConfig {
	return CollectionConfig{
		Name:              name,
		DenseVectorSize:   1536, // Jina embeddings
		OnDiskPayload:     true,
		IndexingThreshold: 20000,
		MemmapThreshold:   50000,
	}
}

// Point represents a point to upsert into Qdrant.
type Point struct {
	// ID is the unique point identifier.
	ID string

	// DenseVector is the semantic embedding vector.
	DenseVector []float32

	// SparseIndices are the token IDs for sparse vector.
	SparseIndices []uint32

	// SparseValues are the token weights for sparse vector.
	SparseValues []float32

	// Payload is the metadata associated with this point.
	Payload PointPayload
}

// PointPayload contains the searchable metadata for a chunk.
type PointPayload struct {
	Store        string    `json:"store"`
	Path         string    `json:"path"`
	Language     string    `json:"language"`
	Content      string    `json:"content"`
	Symbols      []string  `json:"symbols"`
	StartLine    int       `json:"start_line"`
	EndLine      int       `json:"end_line"`
	DocumentHash string    `json:"document_hash"`
	ChunkHash    string    `json:"chunk_hash"`
	IndexedAt    time.Time `json:"indexed_at"`
	ConnectionID string    `json:"connection_id,omitempty"` // Originating connection (optional)
}

// SearchRequest defines parameters for a hybrid search.
type SearchRequest struct {
	// Query for dense vector search.
	DenseVector []float32

	// SparseIndices for sparse vector search.
	SparseIndices []uint32

	// SparseValues for sparse vector search.
	SparseValues []float32

	// Limit is the maximum number of results to return.
	Limit uint64

	// PrefetchLimit is the number of candidates to retrieve from each retriever.
	PrefetchLimit uint64

	// Filter constrains the search to matching documents.
	Filter *SearchFilter

	// WithPayload includes payload in results.
	WithPayload bool

	// WithVectors includes dense vectors in results (needed for postrank dedup/diversity).
	WithVectors bool

	// ScoreThreshold filters results below this score.
	ScoreThreshold *float32
}

// SearchFilter defines filter conditions for search.
type SearchFilter struct {
	// PathPrefix filters by path prefix (e.g., "src/auth/").
	PathPrefix string

	// Languages filters by programming language.
	Languages []string

	// DocumentHash filters by document hash.
	DocumentHash string

	// ConnectionID filters by connection.
	ConnectionID string
}

// SearchResult represents a single search result.
type SearchResult struct {
	// ID is the point identifier.
	ID string

	// Score is the relevance score.
	Score float32

	// Payload contains the point metadata.
	Payload PointPayload

	// DenseVector is the dense embedding (only populated if WithVectors=true).
	DenseVector []float32
}

// DeleteFilter defines conditions for deleting points.
type DeleteFilter struct {
	// IDs deletes specific point IDs.
	IDs []string

	// Path deletes all points with this exact path.
	Path string

	// PathPrefix deletes all points with paths starting with this prefix.
	PathPrefix string

	// DocumentHash deletes all points with this document hash.
	DocumentHash string
}

// CollectionInfo contains information about a collection.
type CollectionInfo struct {
	// Name is the collection name (without prefix).
	Name string

	// PointsCount is the total number of points.
	PointsCount uint64

	// VectorsCount is the total number of vectors.
	VectorsCount uint64

	// Status is the collection health status.
	Status string

	// SegmentsCount is the number of segments.
	SegmentsCount uint64
}
