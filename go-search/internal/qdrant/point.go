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

	// Add connection_id if present
	if p.Payload.ConnectionID != "" {
		payload["connection_id"] = p.Payload.ConnectionID
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

// GetChunksByPath retrieves all chunks for a specific file path.
func (c *Client) GetChunksByPath(ctx context.Context, collection, path string) ([]SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Build filter for exact path match
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "path",
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Keyword{
								Keyword: path,
							},
						},
					},
				},
			},
		},
	}

	// Use scroll to get all matching points
	var results []SearchResult
	var offset *qdrant.PointId
	const batchSize = 100

	for {
		scrollReq := &qdrant.ScrollPoints{
			CollectionName: collectionName(collection),
			Filter:         filter,
			Limit:          qdrant.PtrOf(uint32(batchSize)),
			WithPayload:    qdrant.NewWithPayload(true),
			Offset:         offset,
		}

		// Scroll returns []*qdrant.RetrievedPoint directly
		points, err := c.client.Scroll(ctx, scrollReq)
		if err != nil {
			return nil, fmt.Errorf("failed to scroll points: %w", err)
		}

		for _, p := range points {
			result := SearchResult{
				Payload: extractPayload(p.Payload),
			}
			switch v := p.Id.PointIdOptions.(type) {
			case *qdrant.PointId_Uuid:
				result.ID = v.Uuid
			case *qdrant.PointId_Num:
				result.ID = fmt.Sprintf("%d", v.Num)
			}
			results = append(results, result)
		}

		// If we got fewer than batchSize, we've reached the end
		if len(points) < batchSize {
			break
		}

		// Use last point's ID as offset for next page
		if len(points) > 0 {
			offset = points[len(points)-1].Id
		}
	}

	return results, nil
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
