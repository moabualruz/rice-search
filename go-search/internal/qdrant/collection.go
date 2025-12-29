package qdrant

import (
	"context"
	"fmt"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

// CreateCollection creates a new collection with both dense and sparse vector support.
func (c *Client) CreateCollection(ctx context.Context, cfg CollectionConfig) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	name := collectionName(cfg.Name)

	// Check if collection already exists
	exists, err := c.collectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}
	if exists {
		return nil // Collection already exists
	}

	// Create collection with named vectors for dense and sparse
	err = c.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: qdrant.NewVectorsConfigMap(map[string]*qdrant.VectorParams{
			"dense": {
				Size:     cfg.DenseVectorSize,
				Distance: qdrant.Distance_Cosine,
				OnDisk:   qdrant.PtrOf(false), // Keep dense vectors in memory for speed
			},
		}),
		SparseVectorsConfig: &qdrant.SparseVectorConfig{
			Map: map[string]*qdrant.SparseVectorParams{
				"sparse": {
					Index: &qdrant.SparseIndexConfig{
						OnDisk:            qdrant.PtrOf(false),
						FullScanThreshold: qdrant.PtrOf(uint64(10000)),
					},
				},
			},
		},
		OnDiskPayload: qdrant.PtrOf(cfg.OnDiskPayload),
		OptimizersConfig: &qdrant.OptimizersConfigDiff{
			IndexingThreshold: qdrant.PtrOf(cfg.IndexingThreshold),
			MemmapThreshold:   qdrant.PtrOf(cfg.MemmapThreshold),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", name, err)
	}

	// Create payload indexes for efficient filtering
	if err := c.createPayloadIndexes(ctx, name); err != nil {
		return fmt.Errorf("failed to create payload indexes: %w", err)
	}

	return nil
}

// createPayloadIndexes creates indexes on payload fields for efficient filtering.
func (c *Client) createPayloadIndexes(ctx context.Context, collectionName string) error {
	indexes := []struct {
		field  string
		schema qdrant.FieldType
	}{
		{"path", qdrant.FieldType_FieldTypeText},
		{"language", qdrant.FieldType_FieldTypeKeyword},
		{"symbols", qdrant.FieldType_FieldTypeKeyword},
		{"document_hash", qdrant.FieldType_FieldTypeKeyword},
		{"store", qdrant.FieldType_FieldTypeKeyword},
	}

	for _, idx := range indexes {
		_, err := c.client.CreateFieldIndex(ctx, &qdrant.CreateFieldIndexCollection{
			CollectionName: collectionName,
			FieldName:      idx.field,
			FieldType:      qdrant.PtrOf(idx.schema),
		})
		if err != nil {
			// Index might already exist, which is fine
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("failed to create index on %s: %w", idx.field, err)
			}
		}
	}

	return nil
}

// DeleteCollection deletes a collection.
func (c *Client) DeleteCollection(ctx context.Context, name string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	err := c.client.DeleteCollection(ctx, collectionName(name))
	if err != nil {
		return fmt.Errorf("failed to delete collection %s: %w", name, err)
	}

	return nil
}

// ListCollections returns all Rice Search collections (without prefix).
func (c *Client) ListCollections(ctx context.Context) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	collections, err := c.client.ListCollections(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	var result []string
	for _, col := range collections {
		if strings.HasPrefix(col, CollectionPrefix) {
			result = append(result, strings.TrimPrefix(col, CollectionPrefix))
		}
	}

	return result, nil
}

// GetCollectionInfo returns information about a collection.
func (c *Client) GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	fullName := collectionName(name)
	info, err := c.client.GetCollectionInfo(ctx, fullName)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info for %s: %w", name, err)
	}

	statusStr := "unknown"
	switch info.Status {
	case qdrant.CollectionStatus_Green:
		statusStr = "green"
	case qdrant.CollectionStatus_Yellow:
		statusStr = "yellow"
	case qdrant.CollectionStatus_Red:
		statusStr = "red"
	}

	// Safely dereference optional pointer fields
	var pointsCount uint64
	if info.PointsCount != nil {
		pointsCount = *info.PointsCount
	}

	return &CollectionInfo{
		Name:          name,
		PointsCount:   pointsCount,
		VectorsCount:  pointsCount, // Use points count as vectors count approximation
		Status:        statusStr,
		SegmentsCount: uint64(info.SegmentsCount),
	}, nil
}

// CollectionExists checks if a collection exists.
func (c *Client) CollectionExists(ctx context.Context, name string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return false, fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	return c.collectionExists(ctx, collectionName(name))
}

// collectionExists is the internal helper (expects full collection name).
func (c *Client) collectionExists(ctx context.Context, fullName string) (bool, error) {
	collections, err := c.client.ListCollections(ctx)
	if err != nil {
		return false, err
	}

	for _, col := range collections {
		if col == fullName {
			return true, nil
		}
	}

	return false, nil
}
