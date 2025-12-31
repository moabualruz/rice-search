package query

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestServiceParse(t *testing.T) {
	log := logger.Default()
	service := NewService(log)
	ctx := context.Background()

	tests := []struct {
		name           string
		query          string
		expectedIntent QueryIntent
		shouldUseModel bool
	}{
		{
			name:           "basic find query",
			query:          "where is the authentication function",
			expectedIntent: IntentFind,
			shouldUseModel: false,
		},
		{
			name:           "explain query",
			query:          "how does error handling work",
			expectedIntent: IntentExplain,
			shouldUseModel: false,
		},
		{
			name:           "list query",
			query:          "list all api endpoints",
			expectedIntent: IntentList,
			shouldUseModel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.Parse(ctx, tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.ActionIntent != tt.expectedIntent {
				t.Errorf("expected intent %q, got %q", tt.expectedIntent, result.ActionIntent)
			}

			if result.UsedModel != tt.shouldUseModel {
				t.Errorf("expected UsedModel=%v, got %v", tt.shouldUseModel, result.UsedModel)
			}

			if result.SearchQuery == "" {
				t.Error("expected non-empty search query")
			}
		})
	}
}

func TestServiceEmptyQuery(t *testing.T) {
	log := logger.Default()
	service := NewService(log)
	ctx := context.Background()

	result, err := service.Parse(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result for empty query")
	}

	if result.Original != "" {
		t.Error("expected empty original query")
	}

	if result.ActionIntent != IntentUnknown {
		t.Errorf("expected IntentUnknown, got %q", result.ActionIntent)
	}

	if result.UsedModel {
		t.Error("expected UsedModel to be false")
	}
}

func TestServiceModelToggle(t *testing.T) {
	log := logger.Default()
	service := NewService(log)

	// Initially disabled
	if service.IsModelEnabled() {
		t.Error("expected model to be disabled initially")
	}

	// Enable model without initialization
	service.SetModelEnabled(true)
	// Should still be false because it requires ML service
	if service.IsModelEnabled() {
		t.Error("expected model to remain disabled (IsModelEnabled=false) if ML service not initialized")
	}

	// Disable model
	service.SetModelEnabled(false)
	if service.IsModelEnabled() {
		t.Error("expected model to be disabled after SetModelEnabled(false)")
	}
}

func TestServiceFallbackToKeywords(t *testing.T) {
	log := logger.Default()
	service := NewService(log)
	ctx := context.Background()

	// Enable model (but it's not implemented, so should fall back)
	service.SetModelEnabled(true)

	query := "find authentication handler"
	result, err := service.Parse(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have fallen back to keyword extraction
	if result.UsedModel {
		t.Error("expected UsedModel to be false (fallback)")
	}

	if result.ActionIntent != IntentFind {
		t.Errorf("expected IntentFind, got %q", result.ActionIntent)
	}

	if len(result.Keywords) == 0 {
		t.Error("expected non-empty keywords from fallback")
	}
}

func TestServiceConsistency(t *testing.T) {
	log := logger.Default()
	service := NewService(log)
	ctx := context.Background()

	queries := []string{
		"where is the function",
		"how does error handling work",
		"list all tests",
	}

	// Test that parsing the same query multiple times gives same results
	for _, query := range queries {
		t.Run(query, func(t *testing.T) {
			result1, err1 := service.Parse(ctx, query)
			result2, err2 := service.Parse(ctx, query)

			if err1 != nil || err2 != nil {
				t.Fatalf("unexpected errors: %v, %v", err1, err2)
			}

			// Results should be identical
			if result1.ActionIntent != result2.ActionIntent {
				t.Errorf("inconsistent intent: %q vs %q", result1.ActionIntent, result2.ActionIntent)
			}

			if result1.TargetType != result2.TargetType {
				t.Errorf("inconsistent target: %q vs %q", result1.TargetType, result2.TargetType)
			}

			if len(result1.Keywords) != len(result2.Keywords) {
				t.Errorf("inconsistent keyword count: %d vs %d",
					len(result1.Keywords), len(result2.Keywords))
			}

			if result1.SearchQuery != result2.SearchQuery {
				t.Errorf("inconsistent search query: %q vs %q",
					result1.SearchQuery, result2.SearchQuery)
			}
		})
	}
}

func TestServiceComplexQueries(t *testing.T) {
	log := logger.Default()
	service := NewService(log)
	ctx := context.Background()

	tests := []struct {
		name        string
		query       string
		minKeywords int
		minConf     float32
	}{
		{
			name:        "multi-word technical query",
			query:       "how does the jwt token validation middleware work",
			minKeywords: 3,
			minConf:     0.6,
		},
		{
			name:        "compound query",
			query:       "find all error handlers and exception middleware in api routes",
			minKeywords: 4,
			minConf:     0.6,
		},
		{
			name:        "specific code location",
			query:       "where is the authenticate function in the auth module",
			minKeywords: 3,
			minConf:     0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.Parse(ctx, tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(result.Keywords) < tt.minKeywords {
				t.Errorf("expected at least %d keywords, got %d",
					tt.minKeywords, len(result.Keywords))
			}

			if result.Confidence < tt.minConf {
				t.Errorf("expected confidence >= %f, got %f",
					tt.minConf, result.Confidence)
			}
		})
	}
}

func TestServiceConcurrency(t *testing.T) {
	log := logger.Default()
	service := NewService(log)
	ctx := context.Background()

	queries := []string{
		"find authentication",
		"how does parsing work",
		"list all tests",
		"fix database error",
		"compare implementations",
	}

	// Parse queries concurrently
	results := make(chan *ParsedQuery, len(queries))
	errors := make(chan error, len(queries))

	for _, q := range queries {
		go func(query string) {
			result, err := service.Parse(ctx, query)
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(q)
	}

	// Collect results
	for i := 0; i < len(queries); i++ {
		select {
		case err := <-errors:
			t.Fatalf("unexpected error in concurrent parse: %v", err)
		case result := <-results:
			if result == nil {
				t.Error("received nil result")
			}
		}
	}
}
