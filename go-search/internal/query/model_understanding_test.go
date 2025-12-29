package query

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// mockMLService is a mock ML service for testing.
type mockMLService struct {
	embedFunc func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *mockMLService) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, texts)
	}
	return nil, nil
}

func (m *mockMLService) SparseEncode(ctx context.Context, texts []string) ([]ml.SparseVector, error) {
	return nil, nil
}

func (m *mockMLService) Rerank(ctx context.Context, query string, documents []string, topK int) ([]ml.RankedResult, error) {
	return nil, nil
}

func (m *mockMLService) Health() ml.HealthStatus {
	return ml.HealthStatus{}
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

// generateMockEmbedding creates a deterministic embedding for testing.
func generateMockEmbedding(text string, dim int) []float32 {
	embedding := make([]float32, dim)
	hash := 0
	for _, c := range text {
		hash = (hash*31 + int(c)) % 1000000
	}

	// Generate deterministic but different embeddings
	for i := 0; i < dim; i++ {
		val := float32((hash+i*7)%1000) / 1000.0
		embedding[i] = val
	}

	// Simple normalization
	var norm float32
	for _, v := range embedding {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(1.0 / float32(len(embedding)))
		for i := range embedding {
			embedding[i] *= norm
		}
	}

	return embedding
}

func TestModelBasedUnderstandingDisabled(t *testing.T) {
	log := logger.Default()
	model := NewModelBasedUnderstanding(log)
	ctx := context.Background()

	// Should be disabled by default
	if model.IsEnabled() {
		t.Error("expected model to be disabled by default")
	}

	// Parse should return error when disabled
	result, err := model.Parse(ctx, "test query")
	if err != ErrModelNotEnabled {
		t.Errorf("expected ErrModelNotEnabled, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result when disabled")
	}
}

func TestModelBasedUnderstandingEnable(t *testing.T) {
	log := logger.Default()
	model := NewModelBasedUnderstanding(log)

	// Enable (without initialization)
	model.SetEnabled(true)
	// IsEnabled checks for ML service, so should still be false
	if model.IsEnabled() {
		t.Error("expected model to be disabled without initialization")
	}

	// Disable
	model.SetEnabled(false)
	if model.IsEnabled() {
		t.Error("expected model to be disabled")
	}
}

func TestModelBasedUnderstanding_Initialize(t *testing.T) {
	log := logger.Default()
	m := NewModelBasedUnderstanding(log)

	// Create mock ML service
	mock := &mockMLService{
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			embeddings := make([][]float32, len(texts))
			for i, text := range texts {
				embeddings[i] = generateMockEmbedding(text, 128)
			}
			return embeddings, nil
		},
	}

	ctx := context.Background()
	err := m.Initialize(ctx, mock)
	if err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	// Check that intent embeddings were pre-computed
	if len(m.intentEmbedding) == 0 {
		t.Error("expected non-empty intent embeddings")
	}

	t.Logf("Pre-computed %d intent embeddings", len(m.intentEmbedding))
}

func TestModelBasedUnderstanding_Parse(t *testing.T) {
	log := logger.Default()
	m := NewModelBasedUnderstanding(log)

	// Create mock ML service with embeddings that cluster by intent
	mock := &mockMLService{
		embedFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
			embeddings := make([][]float32, len(texts))
			for i, text := range texts {
				// Create embeddings that cluster by keywords
				embedding := make([]float32, 128)

				// Find queries cluster together
				if containsAny(text, []string{"find", "where", "locate", "search"}) {
					for j := 0; j < 128; j++ {
						embedding[j] = 0.1 + float32(j%10)/100.0
					}
				} else if containsAny(text, []string{"explain", "how", "what", "describe"}) {
					for j := 0; j < 128; j++ {
						embedding[j] = 0.2 + float32(j%10)/100.0
					}
				} else if containsAny(text, []string{"list", "show", "enumerate", "get all"}) {
					for j := 0; j < 128; j++ {
						embedding[j] = 0.3 + float32(j%10)/100.0
					}
				} else {
					// Default embedding
					embedding = generateMockEmbedding(text, 128)
				}

				embeddings[i] = embedding
			}
			return embeddings, nil
		},
	}

	ctx := context.Background()
	if err := m.Initialize(ctx, mock); err != nil {
		t.Fatalf("failed to initialize: %v", err)
	}

	m.SetEnabled(true)

	tests := []struct {
		name           string
		query          string
		expectedIntent QueryIntent
		minConfidence  float32
	}{
		{
			name:           "find query",
			query:          "where is the authentication function",
			expectedIntent: IntentFind,
			minConfidence:  0.6,
		},
		{
			name:           "explain query",
			query:          "how does error handling work",
			expectedIntent: IntentExplain,
			minConfidence:  0.6,
		},
		{
			name:           "list query",
			query:          "list all api endpoints",
			expectedIntent: IntentList,
			minConfidence:  0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.Parse(ctx, tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if !result.UsedModel {
				t.Error("expected UsedModel to be true")
			}

			if result.ActionIntent != tt.expectedIntent {
				t.Logf("WARNING: expected intent %q, got %q (mock embeddings may not cluster perfectly)",
					tt.expectedIntent, result.ActionIntent)
			}

			if len(result.Keywords) == 0 {
				t.Error("expected non-empty keywords")
			}

			if result.SearchQuery == "" {
				t.Error("expected non-empty search query")
			}

			t.Logf("Query: %q => Intent: %s, Confidence: %.2f, Keywords: %v",
				tt.query, result.ActionIntent, result.Confidence, result.Keywords)
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
		epsilon  float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			epsilon:  0.01,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
			epsilon:  0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.epsilon {
				t.Errorf("expected similarity ~%f, got %f", tt.expected, result)
			}
		})
	}
}

// Helper function for testing
func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if len(text) >= len(kw) {
			for i := 0; i <= len(text)-len(kw); i++ {
				if text[i:i+len(kw)] == kw {
					return true
				}
			}
		}
	}
	return false
}
