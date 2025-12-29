package metrics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStorage provides Redis-backed persistence for metrics history.
type RedisStorage struct {
	client *redis.Client
	prefix string
	ttl    time.Duration // Time to live for data points
}

// NewRedisStorage creates a new Redis storage backend.
// Returns error if connection fails.
func NewRedisStorage(url string) (*RedisStorage, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return &RedisStorage{
		client: client,
		prefix: "rice:metrics:",
		ttl:    24 * time.Hour, // Keep 24 hours of data by default
	}, nil
}

// SaveDataPoint saves a single data point to Redis.
// Uses sorted set with timestamp as score for efficient range queries.
func (rs *RedisStorage) SaveDataPoint(ctx context.Context, metric string, dp DataPoint) error {
	key := rs.prefix + metric
	score := float64(dp.Timestamp.Unix())
	member := fmt.Sprintf("%.2f", dp.Value)

	// Use pipeline for atomic operation
	pipe := rs.client.Pipeline()

	// Add data point
	pipe.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: member,
	})

	// Remove old data points (older than TTL)
	minScore := time.Now().Add(-rs.ttl).Unix()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", minScore))

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("saving data point: %w", err)
	}

	return nil
}

// LoadHistory loads historical data points since the given time.
func (rs *RedisStorage) LoadHistory(ctx context.Context, metric string, since time.Time) ([]DataPoint, error) {
	key := rs.prefix + metric

	// Query sorted set by score range
	results, err := rs.client.ZRangeByScoreWithScores(ctx, key, &redis.ZRangeBy{
		Min: fmt.Sprintf("%d", since.Unix()),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("loading history: %w", err)
	}

	// Convert results to DataPoints
	dataPoints := make([]DataPoint, 0, len(results))
	for _, z := range results {
		value, err := strconv.ParseFloat(z.Member.(string), 64)
		if err != nil {
			// Skip invalid entries
			continue
		}

		dataPoints = append(dataPoints, DataPoint{
			Timestamp: time.Unix(int64(z.Score), 0),
			Value:     value,
		})
	}

	return dataPoints, nil
}

// SaveBatch saves multiple data points in a single operation.
// More efficient than multiple SaveDataPoint calls.
func (rs *RedisStorage) SaveBatch(ctx context.Context, metric string, dataPoints []DataPoint) error {
	if len(dataPoints) == 0 {
		return nil
	}

	key := rs.prefix + metric

	// Build pipeline
	pipe := rs.client.Pipeline()

	// Add all data points
	members := make([]redis.Z, len(dataPoints))
	for i, dp := range dataPoints {
		members[i] = redis.Z{
			Score:  float64(dp.Timestamp.Unix()),
			Member: fmt.Sprintf("%.2f", dp.Value),
		}
	}
	pipe.ZAdd(ctx, key, members...)

	// Remove old data points
	minScore := time.Now().Add(-rs.ttl).Unix()
	pipe.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", minScore))

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("saving batch: %w", err)
	}

	return nil
}

// GetMetricNames returns all metric names stored in Redis.
func (rs *RedisStorage) GetMetricNames(ctx context.Context) ([]string, error) {
	pattern := rs.prefix + "*"
	keys, err := rs.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("getting metric names: %w", err)
	}

	// Strip prefix from keys
	names := make([]string, len(keys))
	prefixLen := len(rs.prefix)
	for i, key := range keys {
		names[i] = key[prefixLen:]
	}

	return names, nil
}

// DeleteMetric deletes all data for a specific metric.
func (rs *RedisStorage) DeleteMetric(ctx context.Context, metric string) error {
	key := rs.prefix + metric
	err := rs.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("deleting metric: %w", err)
	}
	return nil
}

// Close closes the Redis connection.
func (rs *RedisStorage) Close() error {
	return rs.client.Close()
}

// SetTTL sets the time-to-live for data points.
func (rs *RedisStorage) SetTTL(ttl time.Duration) {
	rs.ttl = ttl
}

// GetStats returns storage statistics.
func (rs *RedisStorage) GetStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := rs.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("getting redis stats: %w", err)
	}

	// Count total metrics
	keys, err := rs.client.Keys(ctx, rs.prefix+"*").Result()
	if err != nil {
		return nil, fmt.Errorf("counting metrics: %w", err)
	}

	return map[string]interface{}{
		"total_metrics": len(keys),
		"redis_info":    info,
		"prefix":        rs.prefix,
		"ttl_hours":     rs.ttl.Hours(),
	}, nil
}
