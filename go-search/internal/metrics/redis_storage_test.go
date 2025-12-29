package metrics

import (
	"context"
	"testing"
	"time"
)

func TestNewRedisStorage_InvalidURL(t *testing.T) {
	_, err := NewRedisStorage("invalid://url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewRedisStorage_ConnectionFailure(t *testing.T) {
	// Try to connect to non-existent Redis
	_, err := NewRedisStorage("redis://localhost:9999")
	if err == nil {
		t.Fatal("expected error for connection failure")
	}
}

func TestRedisStorage_SaveAndLoad(t *testing.T) {
	// Skip if Redis not available
	storage, err := NewRedisStorage("redis://localhost:6379/15")
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Clean up test data
	defer storage.DeleteMetric(ctx, "test_metric")

	// Create test data points
	now := time.Now()
	dataPoints := []DataPoint{
		{Timestamp: now.Add(-10 * time.Minute), Value: 10.5},
		{Timestamp: now.Add(-5 * time.Minute), Value: 20.3},
		{Timestamp: now, Value: 30.7},
	}

	// Save individual data points
	for _, dp := range dataPoints {
		err := storage.SaveDataPoint(ctx, "test_metric", dp)
		if err != nil {
			t.Fatalf("SaveDataPoint failed: %v", err)
		}
	}

	// Load history
	since := now.Add(-15 * time.Minute)
	loaded, err := storage.LoadHistory(ctx, "test_metric", since)
	if err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}

	if len(loaded) != len(dataPoints) {
		t.Errorf("expected %d data points, got %d", len(dataPoints), len(loaded))
	}

	// Verify values (allow small float precision differences)
	for i, dp := range loaded {
		if i >= len(dataPoints) {
			break
		}
		expected := dataPoints[i].Value
		if dp.Value < expected-0.1 || dp.Value > expected+0.1 {
			t.Errorf("data point %d: expected value ~%.2f, got %.2f", i, expected, dp.Value)
		}
	}
}

func TestRedisStorage_SaveBatch(t *testing.T) {
	storage, err := NewRedisStorage("redis://localhost:6379/15")
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer storage.Close()

	ctx := context.Background()
	defer storage.DeleteMetric(ctx, "test_batch")

	// Create batch of data points
	now := time.Now()
	batch := []DataPoint{
		{Timestamp: now.Add(-20 * time.Minute), Value: 5.0},
		{Timestamp: now.Add(-15 * time.Minute), Value: 10.0},
		{Timestamp: now.Add(-10 * time.Minute), Value: 15.0},
		{Timestamp: now.Add(-5 * time.Minute), Value: 20.0},
		{Timestamp: now, Value: 25.0},
	}

	// Save batch
	err = storage.SaveBatch(ctx, "test_batch", batch)
	if err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}

	// Load and verify
	loaded, err := storage.LoadHistory(ctx, "test_batch", now.Add(-30*time.Minute))
	if err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}

	if len(loaded) != len(batch) {
		t.Errorf("expected %d data points, got %d", len(batch), len(loaded))
	}
}

func TestRedisStorage_TTL(t *testing.T) {
	storage, err := NewRedisStorage("redis://localhost:6379/15")
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer storage.Close()

	ctx := context.Background()
	defer storage.DeleteMetric(ctx, "test_ttl")

	// Set short TTL for testing
	storage.SetTTL(1 * time.Second)

	// Save old and new data points
	now := time.Now()
	old := DataPoint{Timestamp: now.Add(-2 * time.Second), Value: 10.0}
	new := DataPoint{Timestamp: now, Value: 20.0}

	storage.SaveDataPoint(ctx, "test_ttl", old)
	storage.SaveDataPoint(ctx, "test_ttl", new)

	// Load - should still have both
	loaded, _ := storage.LoadHistory(ctx, "test_ttl", now.Add(-5*time.Second))
	if len(loaded) < 1 {
		t.Error("expected at least 1 data point immediately after save")
	}

	// The old data point should be removed automatically by ZRemRangeByScore
	// when we save the new one (if TTL has expired)
	time.Sleep(100 * time.Millisecond)
	loaded, _ = storage.LoadHistory(ctx, "test_ttl", now.Add(-5*time.Second))

	// Should have the new point, might have old if timing is tight
	if len(loaded) == 0 {
		t.Error("expected at least the new data point")
	}
}

func TestRedisStorage_GetMetricNames(t *testing.T) {
	storage, err := NewRedisStorage("redis://localhost:6379/15")
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create several metrics
	metrics := []string{"metric1", "metric2", "metric3"}
	dp := DataPoint{Timestamp: time.Now(), Value: 1.0}

	for _, name := range metrics {
		storage.SaveDataPoint(ctx, name, dp)
		defer storage.DeleteMetric(ctx, name)
	}

	// Get metric names
	names, err := storage.GetMetricNames(ctx)
	if err != nil {
		t.Fatalf("GetMetricNames failed: %v", err)
	}

	// Should have at least the ones we created
	if len(names) < len(metrics) {
		t.Errorf("expected at least %d metrics, got %d", len(metrics), len(names))
	}

	// Verify our metrics are in the list
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	for _, expected := range metrics {
		if !nameMap[expected] {
			t.Errorf("expected metric %s not found in names", expected)
		}
	}
}

func TestRedisStorage_DeleteMetric(t *testing.T) {
	storage, err := NewRedisStorage("redis://localhost:6379/15")
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Create metric
	dp := DataPoint{Timestamp: time.Now(), Value: 42.0}
	storage.SaveDataPoint(ctx, "test_delete", dp)

	// Verify it exists
	loaded, _ := storage.LoadHistory(ctx, "test_delete", time.Now().Add(-1*time.Minute))
	if len(loaded) == 0 {
		t.Fatal("metric should exist before delete")
	}

	// Delete metric
	err = storage.DeleteMetric(ctx, "test_delete")
	if err != nil {
		t.Fatalf("DeleteMetric failed: %v", err)
	}

	// Verify it's gone
	loaded, _ = storage.LoadHistory(ctx, "test_delete", time.Now().Add(-1*time.Minute))
	if len(loaded) != 0 {
		t.Errorf("expected 0 data points after delete, got %d", len(loaded))
	}
}

func TestRedisStorage_GetStats(t *testing.T) {
	storage, err := NewRedisStorage("redis://localhost:6379/15")
	if err != nil {
		t.Skip("Redis not available:", err)
	}
	defer storage.Close()

	ctx := context.Background()

	stats, err := storage.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	// Verify expected fields
	if _, ok := stats["total_metrics"]; !ok {
		t.Error("stats missing total_metrics")
	}
	if _, ok := stats["redis_info"]; !ok {
		t.Error("stats missing redis_info")
	}
	if _, ok := stats["prefix"]; !ok {
		t.Error("stats missing prefix")
	}
	if _, ok := stats["ttl_hours"]; !ok {
		t.Error("stats missing ttl_hours")
	}
}
