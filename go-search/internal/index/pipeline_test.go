package index

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTracker(t *testing.T) {
	tracker := NewTracker()

	// Test SetHash and HasHash
	tracker.SetHash("default", "main.go", "abc123")
	if !tracker.HasHash("default", "main.go", "abc123") {
		t.Error("expected hash to exist")
	}
	if tracker.HasHash("default", "main.go", "different") {
		t.Error("expected different hash to not match")
	}
	if tracker.HasHash("default", "other.go", "abc123") {
		t.Error("expected different path to not exist")
	}
	if tracker.HasHash("other", "main.go", "abc123") {
		t.Error("expected different store to not exist")
	}

	// Test GetPaths
	tracker.SetHash("default", "utils.go", "def456")
	paths := tracker.GetPaths("default")
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}

	// Test RemovePath
	tracker.RemovePath("default", "main.go")
	if tracker.HasHash("default", "main.go", "abc123") {
		t.Error("expected hash to be removed")
	}

	// Test RemoveByPrefix
	tracker.SetHash("default", "src/main.go", "hash1")
	tracker.SetHash("default", "src/utils.go", "hash2")
	tracker.SetHash("default", "pkg/other.go", "hash3")
	tracker.RemoveByPrefix("default", "src/")

	paths = tracker.GetPaths("default")
	for _, p := range paths {
		if p == "src/main.go" || p == "src/utils.go" {
			t.Errorf("expected src/ paths to be removed, found %s", p)
		}
	}

	// Test ClearStore
	tracker.ClearStore("default")
	paths = tracker.GetPaths("default")
	if len(paths) != 0 {
		t.Errorf("expected 0 paths after clear, got %d", len(paths))
	}
}

func TestTrackerPersistence(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create tracker and add data
	tracker1 := NewTracker()
	tracker1.SetHash("store1", "file1.go", "hash1")
	tracker1.SetHash("store1", "file2.go", "hash2")
	tracker1.SetHash("store2", "other.go", "hash3")

	// Save
	if err := tracker1.Save(tmpDir); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify files exist
	if _, err := os.Stat(filepath.Join(tmpDir, "store1.json")); os.IsNotExist(err) {
		t.Error("expected store1.json to exist")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "store2.json")); os.IsNotExist(err) {
		t.Error("expected store2.json to exist")
	}

	// Load into new tracker
	tracker2 := NewTracker()
	if err := tracker2.Load(tmpDir); err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	// Verify data
	if !tracker2.HasHash("store1", "file1.go", "hash1") {
		t.Error("expected hash1 to exist after load")
	}
	if !tracker2.HasHash("store1", "file2.go", "hash2") {
		t.Error("expected hash2 to exist after load")
	}
	if !tracker2.HasHash("store2", "other.go", "hash3") {
		t.Error("expected hash3 to exist after load")
	}
}

func TestTrackerChanged(t *testing.T) {
	tracker := NewTracker()
	tracker.SetHash("default", "file1.go", "hash1")
	tracker.SetHash("default", "file2.go", "hash2")
	tracker.SetHash("default", "file3.go", "hash3")

	// Test changed detection
	newFiles := map[string]string{
		"file1.go": "hash1",   // unchanged
		"file2.go": "newhash", // changed
		"file4.go": "hash4",   // new
	}

	changed := tracker.Changed("default", newFiles)

	hasFile2, hasFile4 := false, false
	for _, path := range changed {
		if path == "file2.go" {
			hasFile2 = true
		}
		if path == "file4.go" {
			hasFile4 = true
		}
		if path == "file1.go" {
			t.Error("file1.go should not be in changed list")
		}
	}

	if !hasFile2 {
		t.Error("file2.go should be in changed list")
	}
	if !hasFile4 {
		t.Error("file4.go should be in changed list")
	}
}

func TestTrackerRemoved(t *testing.T) {
	tracker := NewTracker()
	tracker.SetHash("default", "file1.go", "hash1")
	tracker.SetHash("default", "file2.go", "hash2")
	tracker.SetHash("default", "file3.go", "hash3")

	// file2.go is no longer present
	currentPaths := []string{"file1.go", "file3.go"}
	removed := tracker.Removed("default", currentPaths)

	if len(removed) != 1 || removed[0] != "file2.go" {
		t.Errorf("expected [file2.go] to be removed, got %v", removed)
	}
}

func TestTrackerListFiles(t *testing.T) {
	tracker := NewTracker()
	for i := 0; i < 100; i++ {
		// Use unique filenames with numbers to ensure 100 distinct paths
		tracker.SetHash("default", filepath.Join("src", fmt.Sprintf("file%d.go", i)), fmt.Sprintf("hash%d", i))
	}

	// Test pagination
	page1, total := tracker.ListFiles("default", 1, 25)
	if total != 100 {
		t.Errorf("expected total 100, got %d", total)
	}
	if len(page1) != 25 {
		t.Errorf("expected 25 items on page 1, got %d", len(page1))
	}

	page4, _ := tracker.ListFiles("default", 4, 25)
	if len(page4) != 25 {
		t.Errorf("expected 25 items on page 4, got %d", len(page4))
	}

	// Test beyond last page
	page10, _ := tracker.ListFiles("default", 10, 25)
	if len(page10) != 0 {
		t.Errorf("expected 0 items beyond last page, got %d", len(page10))
	}
}

func TestBatchProcessor(t *testing.T) {
	// Simple processor that doubles each number
	processor := NewBatchProcessor(BatchConfig{
		Size:    3,
		Workers: 1,
	}, func(ctx context.Context, batch []int) ([]int, error) {
		result := make([]int, len(batch))
		for i, v := range batch {
			result[i] = v * 2
		}
		return result, nil
	})

	items := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	results, err := processor.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(items) {
		t.Errorf("expected %d results, got %d", len(items), len(results))
	}

	for i, r := range results {
		expected := items[i] * 2
		if r != expected {
			t.Errorf("result[%d] = %d, expected %d", i, r, expected)
		}
	}
}

func TestBatchProcessorParallel(t *testing.T) {
	// Processor with multiple workers
	processor := NewBatchProcessor(BatchConfig{
		Size:    5,
		Workers: 4,
	}, func(ctx context.Context, batch []int) ([]int, error) {
		result := make([]int, len(batch))
		for i, v := range batch {
			result[i] = v * 2
		}
		return result, nil
	})

	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	results, err := processor.Process(context.Background(), items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != len(items) {
		t.Errorf("expected %d results, got %d", len(items), len(results))
	}

	// Results should be in order
	for i, r := range results {
		expected := items[i] * 2
		if r != expected {
			t.Errorf("result[%d] = %d, expected %d", i, r, expected)
		}
	}
}

func TestSplitIntoBatches(t *testing.T) {
	tests := []struct {
		name        string
		items       []int
		size        int
		wantBatches int
	}{
		{"exact fit", []int{1, 2, 3, 4, 5, 6}, 3, 2},
		{"with remainder", []int{1, 2, 3, 4, 5}, 3, 2},
		{"single batch", []int{1, 2}, 5, 1},
		{"empty", []int{}, 3, 0},
		{"size 1", []int{1, 2, 3}, 1, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := splitIntoBatches(tt.items, tt.size)
			if len(batches) != tt.wantBatches {
				t.Errorf("got %d batches, want %d", len(batches), tt.wantBatches)
			}
		})
	}
}

func TestBatchChunks(t *testing.T) {
	chunks := make([]*Chunk, 10)
	for i := range chunks {
		chunks[i] = &Chunk{
			ID:      ComputeChunkID("test", "file.go", i, i+10),
			Content: "content",
		}
	}

	batches := BatchChunks(chunks, 3)
	if len(batches) != 4 {
		t.Errorf("expected 4 batches, got %d", len(batches))
	}

	// First batch should have 3 items
	if len(batches[0].Chunks) != 3 {
		t.Errorf("expected 3 chunks in first batch, got %d", len(batches[0].Chunks))
	}
	if len(batches[0].Contents) != 3 {
		t.Errorf("expected 3 contents in first batch, got %d", len(batches[0].Contents))
	}

	// Last batch should have 1 item
	if len(batches[3].Chunks) != 1 {
		t.Errorf("expected 1 chunk in last batch, got %d", len(batches[3].Chunks))
	}
}

func TestProgressTracker(t *testing.T) {
	var updates []Progress
	tracker := NewProgressTracker(func(p Progress) {
		updates = append(updates, p)
	})

	tracker.ChunkStage(1, 10, "main.go")
	tracker.EmbedStage(5, 10)
	tracker.UpsertStage(10, 10)
	tracker.Complete(8, 1, 1)

	if len(updates) != 4 {
		t.Errorf("expected 4 updates, got %d", len(updates))
	}

	// Check stages
	if updates[0].Stage != "chunking" {
		t.Errorf("expected stage 'chunking', got %s", updates[0].Stage)
	}
	if updates[1].Stage != "embedding" {
		t.Errorf("expected stage 'embedding', got %s", updates[1].Stage)
	}
	if updates[2].Stage != "upserting" {
		t.Errorf("expected stage 'upserting', got %s", updates[2].Stage)
	}
	if updates[3].Stage != "complete" {
		t.Errorf("expected stage 'complete', got %s", updates[3].Stage)
	}

	// Check percent calculation
	if updates[1].Percent != 50 {
		t.Errorf("expected 50%%, got %.1f%%", updates[1].Percent)
	}
}

func TestDefaultConfigs(t *testing.T) {
	// Test DefaultPipelineConfig
	pCfg := DefaultPipelineConfig()
	if pCfg.EmbedBatchSize <= 0 {
		t.Error("EmbedBatchSize should be positive")
	}
	if pCfg.UpsertBatchSize <= 0 {
		t.Error("UpsertBatchSize should be positive")
	}
	if pCfg.Workers <= 0 {
		t.Error("Workers should be positive")
	}

	// Test DefaultBatchConfig
	bCfg := DefaultBatchConfig()
	if bCfg.Size <= 0 {
		t.Error("Size should be positive")
	}
	if bCfg.Workers <= 0 {
		t.Error("Workers should be positive")
	}
}
