// Package metrics provides time-series history for charting.
package metrics

import (
	"context"
	"sync"
	"time"
)

// DataPoint represents a single time-series data point.
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricHistory stores time-series data with automatic bucketing and retention.
type MetricHistory struct {
	mu          sync.RWMutex
	buckets     []DataPoint
	bucketSize  time.Duration // Duration per bucket (e.g., 5 minutes)
	maxBuckets  int           // Max buckets to retain (e.g., 12 = 1 hour at 5-min buckets)
	accumulator float64       // Current bucket accumulator
	count       int64         // Current bucket count
	lastBucket  time.Time     // Start time of current bucket
	storage     *RedisStorage // Optional Redis backend
	metricName  string        // Metric name for Redis storage
}

// NewMetricHistory creates a new metric history with specified bucket size and retention.
// bucketSize: duration per data point (e.g., 5*time.Minute)
// maxBuckets: number of buckets to retain (e.g., 12 for 1 hour at 5-min buckets)
func NewMetricHistory(bucketSize time.Duration, maxBuckets int) *MetricHistory {
	return &MetricHistory{
		buckets:    make([]DataPoint, 0, maxBuckets),
		bucketSize: bucketSize,
		maxBuckets: maxBuckets,
		lastBucket: time.Now().Truncate(bucketSize),
	}
}

// NewMetricHistoryWithRedis creates a new metric history with Redis persistence.
// If Redis connection fails, falls back to in-memory only (logs warning).
func NewMetricHistoryWithRedis(bucketSize time.Duration, maxBuckets int, storage *RedisStorage, metricName string) *MetricHistory {
	h := &MetricHistory{
		buckets:    make([]DataPoint, 0, maxBuckets),
		bucketSize: bucketSize,
		maxBuckets: maxBuckets,
		lastBucket: time.Now().Truncate(bucketSize),
		storage:    storage,
		metricName: metricName,
	}

	// Try to load existing data from Redis
	if storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		since := time.Now().Add(-time.Duration(maxBuckets) * bucketSize)
		if dataPoints, err := storage.LoadHistory(ctx, metricName, since); err == nil && len(dataPoints) > 0 {
			h.buckets = dataPoints
		}
	}

	return h
}

// Record adds a value to the current bucket.
func (h *MetricHistory) Record(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	currentBucket := now.Truncate(h.bucketSize)

	// Check if we need to finalize the previous bucket
	if currentBucket.After(h.lastBucket) {
		h.finalizeBucket()
		h.lastBucket = currentBucket
	}

	h.accumulator += value
	h.count++
}

// RecordCount increments the count for the current bucket (for rate metrics).
func (h *MetricHistory) RecordCount() {
	h.Record(1)
}

// finalizeBucket saves the current bucket and starts a new one.
// Must be called with lock held.
func (h *MetricHistory) finalizeBucket() {
	if h.count == 0 {
		return
	}

	// Calculate average for the bucket
	avg := h.accumulator / float64(h.count)

	dp := DataPoint{
		Timestamp: h.lastBucket,
		Value:     avg,
	}

	h.buckets = append(h.buckets, dp)

	// Persist to Redis if available (non-blocking)
	if h.storage != nil && h.metricName != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = h.storage.SaveDataPoint(ctx, h.metricName, dp)
		}()
	}

	// Trim to max buckets
	if len(h.buckets) > h.maxBuckets {
		h.buckets = h.buckets[len(h.buckets)-h.maxBuckets:]
	}

	// Reset accumulator
	h.accumulator = 0
	h.count = 0
}

// RecordSum adds to the sum for the current bucket (for count metrics).
func (h *MetricHistory) RecordSum(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	currentBucket := now.Truncate(h.bucketSize)

	// Check if we need to finalize the previous bucket
	if currentBucket.After(h.lastBucket) {
		h.finalizeSumBucket()
		h.lastBucket = currentBucket
	}

	h.accumulator += value
}

// finalizeSumBucket saves the sum for the current bucket.
// Must be called with lock held.
func (h *MetricHistory) finalizeSumBucket() {
	dp := DataPoint{
		Timestamp: h.lastBucket,
		Value:     h.accumulator,
	}

	h.buckets = append(h.buckets, dp)

	// Persist to Redis if available (non-blocking)
	if h.storage != nil && h.metricName != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = h.storage.SaveDataPoint(ctx, h.metricName, dp)
		}()
	}

	// Trim to max buckets
	if len(h.buckets) > h.maxBuckets {
		h.buckets = h.buckets[len(h.buckets)-h.maxBuckets:]
	}

	// Reset accumulator
	h.accumulator = 0
}

// GetHistory returns a copy of the time-series data.
func (h *MetricHistory) GetHistory() []DataPoint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Finalize current bucket for reading
	h.mu.RUnlock()
	h.mu.Lock()
	currentBucket := time.Now().Truncate(h.bucketSize)
	if currentBucket.After(h.lastBucket) && h.count > 0 {
		h.finalizeBucket()
		h.lastBucket = currentBucket
	}
	h.mu.Unlock()
	h.mu.RLock()

	result := make([]DataPoint, len(h.buckets))
	copy(result, h.buckets)
	return result
}

// GetHistoryWithCurrent returns history including any unflushed current bucket data.
func (h *MetricHistory) GetHistoryWithCurrent() []DataPoint {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]DataPoint, len(h.buckets))
	copy(result, h.buckets)

	// Add current bucket if it has data
	if h.count > 0 {
		avg := h.accumulator / float64(h.count)
		result = append(result, DataPoint{
			Timestamp: h.lastBucket,
			Value:     avg,
		})
	}

	return result
}

// GetHistorySince returns data points since the given time.
func (h *MetricHistory) GetHistorySince(since time.Time) []DataPoint {
	all := h.GetHistoryWithCurrent()
	result := make([]DataPoint, 0, len(all))
	for _, dp := range all {
		if !dp.Timestamp.Before(since) {
			result = append(result, dp)
		}
	}
	return result
}

// TimeSeriesData holds multiple time-series for the web UI.
type TimeSeriesData struct {
	SearchRate    *MetricHistory // Searches per 5-minute bucket
	SearchLatency *MetricHistory // Average latency per bucket
	IndexRate     *MetricHistory // Files indexed per bucket
}

// NewTimeSeriesData creates a new time-series data collection.
// Uses 5-minute buckets with 12 buckets (1 hour) retention.
func NewTimeSeriesData() *TimeSeriesData {
	bucketSize := 5 * time.Minute
	maxBuckets := 12 // 1 hour retention

	return &TimeSeriesData{
		SearchRate:    NewMetricHistory(bucketSize, maxBuckets),
		SearchLatency: NewMetricHistory(bucketSize, maxBuckets),
		IndexRate:     NewMetricHistory(bucketSize, maxBuckets),
	}
}

// NewTimeSeriesDataWithRedis creates a new time-series data collection with Redis persistence.
// Falls back to in-memory if Redis connection fails.
func NewTimeSeriesDataWithRedis(storage *RedisStorage) *TimeSeriesData {
	bucketSize := 5 * time.Minute
	maxBuckets := 12 // 1 hour retention

	return &TimeSeriesData{
		SearchRate:    NewMetricHistoryWithRedis(bucketSize, maxBuckets, storage, "search_rate"),
		SearchLatency: NewMetricHistoryWithRedis(bucketSize, maxBuckets, storage, "search_latency"),
		IndexRate:     NewMetricHistoryWithRedis(bucketSize, maxBuckets, storage, "index_rate"),
	}
}

// RecordSearch records a search event for time-series tracking.
func (t *TimeSeriesData) RecordSearch(latencyMs float64) {
	t.SearchRate.RecordCount()
	t.SearchLatency.Record(latencyMs)
}

// RecordIndex records an indexing event.
func (t *TimeSeriesData) RecordIndex(fileCount int) {
	t.IndexRate.RecordSum(float64(fileCount))
}
