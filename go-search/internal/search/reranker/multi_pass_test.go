package reranker

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/search"
)

// Mock ML Service for testing
type mockMLService struct {
	rerankFunc func(ctx context.Context, query string, documents []string, topK int) ([]ml.RankedResult, error)
}

func (m *mockMLService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, nil
}

func (m *mockMLService) SparseEncode(ctx context.Context, texts []string) ([]ml.SparseVector, error) {
	return nil, nil
}

func (m *mockMLService) Rerank(ctx context.Context, query string, documents []string, topK int) ([]ml.RankedResult, error) {
	if m.rerankFunc != nil {
		return m.rerankFunc(ctx, query, documents, topK)
	}
	// Default: return documents in reverse order with decreasing scores
	results := make([]ml.RankedResult, len(documents))
	for i := range documents {
		results[i] = ml.RankedResult{
			Index: len(documents) - 1 - i,
			Score: float32(100 - i*10),
		}
	}
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (m *mockMLService) Health() ml.HealthStatus {
	return ml.HealthStatus{Healthy: true}
}

func (m *mockMLService) ReloadModels() error {
	return nil
}

func (m *mockMLService) ReloadModelsWithConfig(cfg config.MLConfig) error {
	return nil
}

func (m *mockMLService) Close() error {
	return nil
}

func TestMultiPassReranker_EarlyExit_InsufficientResults(t *testing.T) {
	log := logger.New("debug", "text")
	mock := &mockMLService{}
	reranker := NewMultiPassReranker(mock, log)

	ctx := context.Background()
	results := []search.Result{
		{ID: "1", Content: "test", Score: 0.9},
	}

	result, err := reranker.Rerank(ctx, "test query", results)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.EarlyExit {
		t.Error("expected early exit for insufficient results")
	}

	if result.EarlyExitReason != "insufficient_results" {
		t.Errorf("expected reason 'insufficient_results', got %s", result.EarlyExitReason)
	}
}

func TestMultiPassReranker_EarlyExit_PeakedDistribution(t *testing.T) {
	log := logger.New("debug", "text")

	// Mock reranker that returns peaked distribution
	mock := &mockMLService{
		rerankFunc: func(ctx context.Context, query string, documents []string, topK int) ([]ml.RankedResult, error) {
			return []ml.RankedResult{
				{Index: 0, Score: 0.95}, // Clear winner
				{Index: 1, Score: 0.50}, // Much lower
				{Index: 2, Score: 0.45},
			}, nil
		},
	}

	reranker := NewMultiPassReranker(mock, log)

	ctx := context.Background()
	results := []search.Result{
		{ID: "1", Content: "highly relevant", Score: 0.8},
		{ID: "2", Content: "less relevant", Score: 0.6},
		{ID: "3", Content: "not relevant", Score: 0.4},
	}

	result, err := reranker.Rerank(ctx, "test query", results)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.Pass1Applied {
		t.Error("expected pass 1 to be applied")
	}

	if !result.EarlyExit {
		t.Error("expected early exit for peaked distribution")
	}

	if result.EarlyExitReason != "peaked_distribution" {
		t.Errorf("expected reason 'peaked_distribution', got %s", result.EarlyExitReason)
	}

	if result.Pass2Applied {
		t.Error("expected pass 2 to be skipped on early exit")
	}
}

func TestMultiPassReranker_NoEarlyExit_FlatDistribution(t *testing.T) {
	log := logger.New("debug", "text")

	// Mock reranker that returns flat distribution
	mock := &mockMLService{
		rerankFunc: func(ctx context.Context, query string, documents []string, topK int) ([]ml.RankedResult, error) {
			// All scores are very similar (flat distribution)
			return []ml.RankedResult{
				{Index: 0, Score: 0.70},
				{Index: 1, Score: 0.68},
				{Index: 2, Score: 0.67},
				{Index: 3, Score: 0.66},
			}, nil
		},
	}

	reranker := NewMultiPassReranker(mock, log)

	ctx := context.Background()
	results := make([]search.Result, 4)
	for i := range results {
		results[i] = search.Result{
			ID:      string(rune('1' + i)),
			Content: "content " + string(rune('1'+i)),
			Score:   0.7,
		}
	}

	result, err := reranker.Rerank(ctx, "test query", results)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !result.Pass1Applied {
		t.Error("expected pass 1 to be applied")
	}

	// For flat distribution, we should NOT exit early
	if result.EarlyExit {
		t.Error("expected no early exit for flat distribution")
	}

	// Pass 2 should be applied
	if !result.Pass2Applied {
		t.Error("expected pass 2 to be applied for flat distribution")
	}
}

func TestMultiPassReranker_ConfigUpdate(t *testing.T) {
	log := logger.New("debug", "text")
	mock := &mockMLService{}
	reranker := NewMultiPassReranker(mock, log)

	// Update config
	cfg := Config{
		Pass1Candidates: 50,
		Pass2Candidates: 200,
		Pass1Timeout:    100,
		Pass2Timeout:    200,
		EarlyExitThresh: 0.9,
		EarlyExitGap:    0.4,
	}
	reranker.SetConfig(cfg)

	// Verify config was updated
	if reranker.pass1Candidates != 50 {
		t.Errorf("expected pass1Candidates=50, got %d", reranker.pass1Candidates)
	}
	if reranker.pass2Candidates != 200 {
		t.Errorf("expected pass2Candidates=200, got %d", reranker.pass2Candidates)
	}
	if reranker.pass1Timeout != 100 {
		t.Errorf("expected pass1Timeout=100, got %d", reranker.pass1Timeout)
	}
	if reranker.pass2Timeout != 200 {
		t.Errorf("expected pass2Timeout=200, got %d", reranker.pass2Timeout)
	}
	if reranker.earlyExitThresh != 0.9 {
		t.Errorf("expected earlyExitThresh=0.9, got %f", reranker.earlyExitThresh)
	}
	if reranker.earlyExitGap != 0.4 {
		t.Errorf("expected earlyExitGap=0.4, got %f", reranker.earlyExitGap)
	}
}

func TestMultiPassReranker_AnalyzeDistribution(t *testing.T) {
	log := logger.New("debug", "text")
	mock := &mockMLService{}
	reranker := NewMultiPassReranker(mock, log)

	tests := []struct {
		name          string
		scores        []float32
		expectedShape DistributionShape
	}{
		{
			name:          "peaked distribution",
			scores:        []float32{0.95, 0.50, 0.45, 0.40},
			expectedShape: ShapePeaked,
		},
		{
			name:          "flat distribution",
			scores:        []float32{0.70, 0.69, 0.68, 0.67},
			expectedShape: ShapeFlat,
		},
		{
			name:          "bimodal distribution",
			scores:        []float32{0.90, 0.85, 0.50, 0.45},
			expectedShape: ShapeBimodal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := make([]search.Result, len(tt.scores))
			for i, score := range tt.scores {
				results[i] = search.Result{
					ID:    string(rune('1' + i)),
					Score: score,
				}
			}

			signals := reranker.analyzeDistribution(results)
			if signals.DistributionShape != tt.expectedShape {
				t.Errorf("expected shape %s, got %s", tt.expectedShape, signals.DistributionShape)
			}
		})
	}
}
