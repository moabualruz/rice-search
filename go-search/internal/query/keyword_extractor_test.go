package query

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestKeywordExtractorParse(t *testing.T) {
	log := logger.Default()
	extractor := NewKeywordExtractor(log)
	ctx := context.Background()

	tests := []struct {
		name           string
		query          string
		expectedIntent QueryIntent
		expectedTarget string
		minKeywords    int
		minConfidence  float32
	}{
		{
			name:           "find function",
			query:          "where is the authenticate function",
			expectedIntent: IntentFind,
			expectedTarget: TargetFunction,
			minKeywords:    1,
			minConfidence:  0.7,
		},
		{
			name:           "explain how",
			query:          "how does the authentication handler work",
			expectedIntent: IntentExplain,
			expectedTarget: TargetAuth,
			minKeywords:    2,
			minConfidence:  0.7,
		},
		{
			name:           "list all tests",
			query:          "list all unit tests",
			expectedIntent: IntentList,
			expectedTarget: TargetTest,
			minKeywords:    1,
			minConfidence:  0.7,
		},
		{
			name:           "fix error",
			query:          "fix database connection error",
			expectedIntent: IntentFix,
			expectedTarget: TargetError,
			minKeywords:    2,
			minConfidence:  0.7,
		},
		{
			name:           "compare implementations",
			query:          "compare redis and memory cache implementations",
			expectedIntent: IntentCompare,
			expectedTarget: TargetUnknown,
			minKeywords:    3,
			minConfidence:  0.6,
		},
		{
			name:           "simple search",
			query:          "authentication",
			expectedIntent: IntentUnknown,
			expectedTarget: TargetAuth,
			minKeywords:    1,
			minConfidence:  0.5,
		},
		{
			name:           "code term search",
			query:          "configuration settings",
			expectedIntent: IntentUnknown,
			expectedTarget: TargetConfig,
			minKeywords:    2,
			minConfidence:  0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractor.Parse(ctx, tt.query)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Original != tt.query {
				t.Errorf("expected original %q, got %q", tt.query, result.Original)
			}

			if result.ActionIntent != tt.expectedIntent {
				t.Errorf("expected intent %q, got %q", tt.expectedIntent, result.ActionIntent)
			}

			if result.TargetType != tt.expectedTarget {
				t.Errorf("expected target %q, got %q", tt.expectedTarget, result.TargetType)
			}

			if len(result.Keywords) < tt.minKeywords {
				t.Errorf("expected at least %d keywords, got %d", tt.minKeywords, len(result.Keywords))
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("expected confidence >= %f, got %f", tt.minConfidence, result.Confidence)
			}

			if result.UsedModel {
				t.Error("expected UsedModel to be false")
			}

			if result.SearchQuery == "" {
				t.Error("expected non-empty search query")
			}
		})
	}
}

func TestKeywordExtractorEmptyQuery(t *testing.T) {
	log := logger.Default()
	extractor := NewKeywordExtractor(log)
	ctx := context.Background()

	result, err := extractor.Parse(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != nil {
		t.Error("expected nil result for empty query")
	}
}

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello   world  ", "hello world"},
		{"Hello World", "hello world"},
		{"where  is  the  function", "where is the function"},
		{"Query\nWith\nNewlines", "query with newlines"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeQuery(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		query    string
		expected []string
		minCount int
	}{
		{
			query:    "find the authentication function",
			expected: []string{"find", "authentication", "function"},
			minCount: 3,
		},
		{
			query:    "a simple test",
			expected: []string{"simple", "test"},
			minCount: 2,
		},
		{
			query:    "how does it work",
			expected: []string{"how", "does", "work"},
			minCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := extractKeywords(tt.query)
			if len(result) < tt.minCount {
				t.Errorf("expected at least %d keywords, got %d: %v",
					tt.minCount, len(result), result)
			}
		})
	}
}

func TestExtractCodeTerms(t *testing.T) {
	tests := []struct {
		keywords []string
		minTerms int
	}{
		{
			keywords: []string{"function", "error", "handler"},
			minTerms: 2, // function and error
		},
		{
			keywords: []string{"class", "method", "variable"},
			minTerms: 2, // class and variable (method is synonym of function)
		},
		{
			keywords: []string{"test", "config", "api"},
			minTerms: 3, // all three are code terms
		},
		{
			keywords: []string{"normal", "words"},
			minTerms: 0, // no code terms
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := extractCodeTerms(tt.keywords)
			if len(result) < tt.minTerms {
				t.Errorf("expected at least %d code terms, got %d: %v", tt.minTerms, len(result), result)
			}
		})
	}
}

func TestExpandWithSynonyms(t *testing.T) {
	keywords := []string{"function", "error"}
	codeTerms := []string{"function", "error"}

	result := expandWithSynonyms(keywords, codeTerms)

	// Should include originals
	hasFunction := false
	hasError := false
	for _, term := range result {
		if term == "function" {
			hasFunction = true
		}
		if term == "error" {
			hasError = true
		}
	}

	if !hasFunction {
		t.Error("expected 'function' in expanded terms")
	}
	if !hasError {
		t.Error("expected 'error' in expanded terms")
	}

	// Should have more terms than input due to synonyms
	if len(result) <= len(keywords) {
		t.Errorf("expected expansion, got %d terms (input: %d)", len(result), len(keywords))
	}
}

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name       string
		normalized string
		keywords   []string
		expanded   []string
		intent     QueryIntent
		minLength  int
	}{
		{
			name:       "find intent strips question",
			normalized: "where is the authenticate function",
			keywords:   []string{"authenticate", "function"},
			expanded:   []string{"authenticate", "function", "func", "method"},
			intent:     IntentFind,
			minLength:  10,
		},
		{
			name:       "explain intent keeps context",
			normalized: "how does authentication work",
			keywords:   []string{"authentication", "work"},
			expanded:   []string{"authentication", "work"},
			intent:     IntentExplain,
			minLength:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSearchQuery(tt.normalized, tt.keywords, tt.expanded, tt.intent)
			if len(result) < tt.minLength {
				t.Errorf("expected search query length >= %d, got %d: %q",
					tt.minLength, len(result), result)
			}
		})
	}
}

func TestCalculateConfidence(t *testing.T) {
	tests := []struct {
		intent       QueryIntent
		targetType   string
		keywordCount int
		minConf      float32
		maxConf      float32
	}{
		{IntentFind, TargetFunction, 3, 0.8, 1.0},
		{IntentUnknown, TargetUnknown, 1, 0.4, 0.7},
		{IntentExplain, TargetClass, 4, 0.8, 1.0},
		{IntentUnknown, TargetFunction, 3, 0.6, 0.9},
	}

	for _, tt := range tests {
		conf := calculateConfidence(tt.intent, tt.targetType, tt.keywordCount)
		if conf < tt.minConf || conf > tt.maxConf {
			t.Errorf("expected confidence between %f and %f, got %f",
				tt.minConf, tt.maxConf, conf)
		}
	}
}
