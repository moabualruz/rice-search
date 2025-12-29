package qdrant

import (
	"context"
	"fmt"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// UpsertPoints inserts or updates points in a collection.
func (c *Client) UpsertPoints(ctx context.Context, collection string, points []Point) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	if len(points) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	qdrantPoints := make([]*qdrant.PointStruct, 0, len(points))
	for _, p := range points {
		point, err := pointToQdrant(p)
		if err != nil {
			return fmt.Errorf("failed to convert point %s: %w", p.ID, err)
		}
		qdrantPoints = append(qdrantPoints, point)
	}

	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName(collection),
		Points:         qdrantPoints,
		Wait:           qdrant.PtrOf(true), // Wait for indexing
	})
	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	return nil
}

// UpsertPointsBatch upserts points in batches to avoid memory issues.
func (c *Client) UpsertPointsBatch(ctx context.Context, collection string, points []Point, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 100 // Default batch size
	}

	for i := 0; i < len(points); i += batchSize {
		end := i + batchSize
		if end > len(points) {
			end = len(points)
		}

		batch := points[i:end]
		if err := c.UpsertPoints(ctx, collection, batch); err != nil {
			return fmt.Errorf("failed to upsert batch %d-%d: %w", i, end, err)
		}
	}

	return nil
}

// DeletePoints deletes points based on filter criteria.
func (c *Client) DeletePoints(ctx context.Context, collection string, filter DeleteFilter) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	name := collectionName(collection)

	// Delete by IDs
	if len(filter.IDs) > 0 {
		pointIDs := make([]*qdrant.PointId, len(filter.IDs))
		for i, id := range filter.IDs {
			pointIDs[i] = qdrant.NewIDUUID(id)
		}

		_, err := c.client.Delete(ctx, &qdrant.DeletePoints{
			CollectionName: name,
			Points: &qdrant.PointsSelector{
				PointsSelectorOneOf: &qdrant.PointsSelector_Points{
					Points: &qdrant.PointsIdsList{
						Ids: pointIDs,
					},
				},
			},
			Wait: qdrant.PtrOf(true),
		})
		if err != nil {
			return fmt.Errorf("failed to delete by IDs: %w", err)
		}
		return nil
	}

	// Delete by filter
	qdrantFilter := buildDeleteFilter(filter)
	if qdrantFilter == nil {
		return fmt.Errorf("no valid delete criteria specified")
	}

	_, err := c.client.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: name,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Filter{
				Filter: qdrantFilter,
			},
		},
		Wait: qdrant.PtrOf(true),
	})
	if err != nil {
		return fmt.Errorf("failed to delete by filter: %w", err)
	}

	return nil
}

// CountPoints returns the number of points matching the filter.
func (c *Client) CountPoints(ctx context.Context, collection string, filter *SearchFilter) (uint64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return 0, fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	countReq := &qdrant.CountPoints{
		CollectionName: collectionName(collection),
		Exact:          qdrant.PtrOf(true),
	}

	if filter != nil {
		countReq.Filter = buildSearchFilter(filter)
	}

	count, err := c.client.Count(ctx, countReq)
	if err != nil {
		return 0, fmt.Errorf("failed to count points: %w", err)
	}

	return count, nil
}

// pointToQdrant converts a Point to a Qdrant PointStruct.
func pointToQdrant(p Point) (*qdrant.PointStruct, error) {
	// Build payload map
	payload := map[string]any{
		"store":         p.Payload.Store,
		"path":          p.Payload.Path,
		"language":      p.Payload.Language,
		"content":       p.Payload.Content,
		"symbols":       p.Payload.Symbols,
		"start_line":    p.Payload.StartLine,
		"end_line":      p.Payload.EndLine,
		"document_hash": p.Payload.DocumentHash,
		"chunk_hash":    p.Payload.ChunkHash,
		"indexed_at":    p.Payload.IndexedAt.Format(time.RFC3339),
	}

	// Build named vectors
	vectors := &qdrant.Vectors{
		VectorsOptions: &qdrant.Vectors_Vectors{
			Vectors: &qdrant.NamedVectors{
				Vectors: map[string]*qdrant.Vector{
					"dense": {
						Data: p.DenseVector,
					},
					"sparse": {
						Data:    p.SparseValues,
						Indices: &qdrant.SparseIndices{Data: p.SparseIndices},
					},
				},
			},
		},
	}

	return &qdrant.PointStruct{
		Id:      qdrant.NewIDUUID(p.ID),
		Vectors: vectors,
		Payload: qdrant.NewValueMap(payload),
	}, nil
}

// buildDeleteFilter builds a Qdrant filter from DeleteFilter.
func buildDeleteFilter(f DeleteFilter) *qdrant.Filter {
	var conditions []*qdrant.Condition

	if f.Path != "" {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "path",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{
							Keyword: f.Path,
						},
					},
				},
			},
		})
	}

	if f.PathPrefix != "" {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "path",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Text{
							Text: f.PathPrefix,
						},
					},
				},
			},
		})
	}

	if f.DocumentHash != "" {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "document_hash",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{
							Keyword: f.DocumentHash,
						},
					},
				},
			},
		})
	}

	if len(conditions) == 0 {
		return nil
	}

	return &qdrant.Filter{
		Must: conditions,
	}
}
