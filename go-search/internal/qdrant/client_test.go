package qdrant

import (
	"testing"
	"time"
)

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()

	if cfg.Host != DefaultHost {
		t.Errorf("expected host %s, got %s", DefaultHost, cfg.Host)
	}

	if cfg.Port != DefaultPort {
		t.Errorf("expected port %d, got %d", DefaultPort, cfg.Port)
	}

	if cfg.Timeout != DefaultTimeout {
		t.Errorf("expected timeout %v, got %v", DefaultTimeout, cfg.Timeout)
	}
}

func TestDefaultCollectionConfig(t *testing.T) {
	cfg := DefaultCollectionConfig("test")

	if cfg.Name != "test" {
		t.Errorf("expected name 'test', got %s", cfg.Name)
	}

	if cfg.DenseVectorSize != 1536 {
		t.Errorf("expected dense vector size 1536, got %d", cfg.DenseVectorSize)
	}

	if !cfg.OnDiskPayload {
		t.Error("expected OnDiskPayload to be true")
	}
}

func TestCollectionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"default", "rice_default"},
		{"myproject", "rice_myproject"},
		{"test-store", "rice_test-store"},
	}

	for _, tt := range tests {
		result := collectionName(tt.input)
		if result != tt.expected {
			t.Errorf("collectionName(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}

func TestPointPayload(t *testing.T) {
	now := time.Now()
	payload := PointPayload{
		Store:        "default",
		Path:         "src/main.go",
		Language:     "go",
		Content:      "package main",
		Symbols:      []string{"main", "init"},
		StartLine:    1,
		EndLine:      10,
		DocumentHash: "abc123",
		ChunkHash:    "def456",
		IndexedAt:    now,
	}

	if payload.Store != "default" {
		t.Errorf("expected store 'default', got %s", payload.Store)
	}

	if len(payload.Symbols) != 2 {
		t.Errorf("expected 2 symbols, got %d", len(payload.Symbols))
	}
}

func TestPoint(t *testing.T) {
	point := Point{
		ID:            "chunk_abc123",
		DenseVector:   make([]float32, 1536),
		SparseIndices: []uint32{1, 2, 3},
		SparseValues:  []float32{0.1, 0.2, 0.3},
		Payload: PointPayload{
			Store:    "default",
			Path:     "test.go",
			Language: "go",
		},
	}

	if point.ID != "chunk_abc123" {
		t.Errorf("expected ID 'chunk_abc123', got %s", point.ID)
	}

	if len(point.DenseVector) != 1536 {
		t.Errorf("expected dense vector of size 1536, got %d", len(point.DenseVector))
	}

	if len(point.SparseIndices) != len(point.SparseValues) {
		t.Error("sparse indices and values should have same length")
	}
}

func TestSearchRequest(t *testing.T) {
	req := SearchRequest{
		DenseVector:   make([]float32, 1536),
		SparseIndices: []uint32{1, 2, 3},
		SparseValues:  []float32{0.1, 0.2, 0.3},
		Limit:         20,
		PrefetchLimit: 100,
		WithPayload:   true,
		Filter: &SearchFilter{
			PathPrefix: "src/",
			Languages:  []string{"go", "typescript"},
		},
	}

	if req.Limit != 20 {
		t.Errorf("expected limit 20, got %d", req.Limit)
	}

	if req.Filter == nil {
		t.Error("expected filter to be set")
	}

	if req.Filter.PathPrefix != "src/" {
		t.Errorf("expected path prefix 'src/', got %s", req.Filter.PathPrefix)
	}

	if len(req.Filter.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(req.Filter.Languages))
	}
}

func TestDeleteFilter(t *testing.T) {
	// Test by IDs
	filterByIDs := DeleteFilter{
		IDs: []string{"id1", "id2"},
	}
	if len(filterByIDs.IDs) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(filterByIDs.IDs))
	}

	// Test by path
	filterByPath := DeleteFilter{
		Path: "src/main.go",
	}
	if filterByPath.Path != "src/main.go" {
		t.Errorf("expected path 'src/main.go', got %s", filterByPath.Path)
	}

	// Test by path prefix
	filterByPrefix := DeleteFilter{
		PathPrefix: "src/deprecated/",
	}
	if filterByPrefix.PathPrefix != "src/deprecated/" {
		t.Errorf("expected path prefix 'src/deprecated/', got %s", filterByPrefix.PathPrefix)
	}
}

func TestCollectionInfo(t *testing.T) {
	info := CollectionInfo{
		Name:          "default",
		PointsCount:   1000,
		VectorsCount:  1000,
		Status:        "green",
		SegmentsCount: 4,
	}

	if info.Name != "default" {
		t.Errorf("expected name 'default', got %s", info.Name)
	}

	if info.PointsCount != 1000 {
		t.Errorf("expected points count 1000, got %d", info.PointsCount)
	}

	if info.Status != "green" {
		t.Errorf("expected status 'green', got %s", info.Status)
	}
}

func TestBuildDeleteFilter(t *testing.T) {
	// Empty filter should return nil
	emptyFilter := DeleteFilter{}
	result := buildDeleteFilter(emptyFilter)
	if result != nil {
		t.Error("expected nil for empty filter")
	}

	// Filter with path
	pathFilter := DeleteFilter{Path: "src/main.go"}
	result = buildDeleteFilter(pathFilter)
	if result == nil {
		t.Error("expected non-nil for path filter")
	}
	if len(result.Must) != 1 {
		t.Errorf("expected 1 condition, got %d", len(result.Must))
	}

	// Filter with document hash
	hashFilter := DeleteFilter{DocumentHash: "abc123"}
	result = buildDeleteFilter(hashFilter)
	if result == nil {
		t.Error("expected non-nil for hash filter")
	}
}

func TestBuildSearchFilter(t *testing.T) {
	// Nil filter should return nil
	result := buildSearchFilter(nil)
	if result != nil {
		t.Error("expected nil for nil filter")
	}

	// Empty filter should return nil
	emptyFilter := &SearchFilter{}
	result = buildSearchFilter(emptyFilter)
	if result != nil {
		t.Error("expected nil for empty filter")
	}

	// Filter with path prefix
	pathFilter := &SearchFilter{PathPrefix: "src/"}
	result = buildSearchFilter(pathFilter)
	if result == nil {
		t.Error("expected non-nil for path prefix filter")
	}

	// Filter with languages
	langFilter := &SearchFilter{Languages: []string{"go", "python"}}
	result = buildSearchFilter(langFilter)
	if result == nil {
		t.Error("expected non-nil for languages filter")
	}

	// Combined filter
	combinedFilter := &SearchFilter{
		PathPrefix: "src/",
		Languages:  []string{"go"},
	}
	result = buildSearchFilter(combinedFilter)
	if result == nil {
		t.Error("expected non-nil for combined filter")
	}
	if len(result.Must) != 2 {
		t.Errorf("expected 2 conditions, got %d", len(result.Must))
	}
}
