package query

import (
	"context"
	"math"
	"sync"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

var (
	// ErrModelNotEnabled is returned when model-based understanding is disabled.
	ErrModelNotEnabled = errors.New(errors.CodeMLError, "model-based query understanding not enabled")
)

// IntentEmbedding represents a pre-computed embedding for an intent.
type IntentEmbedding struct {
	Intent    QueryIntent
	Embedding []float32
}

// canonicalIntentQueries maps intents to canonical query patterns.
var canonicalIntentQueries = map[QueryIntent][]string{
	IntentFind: {
		"find function",
		"where is the code",
		"locate implementation",
		"search for method",
	},
	IntentExplain: {
		"how does this work",
		"explain the logic",
		"what is the purpose",
		"describe the implementation",
	},
	IntentList: {
		"list all functions",
		"show all methods",
		"enumerate classes",
		"get all endpoints",
	},
	IntentFix: {
		"fix the bug",
		"debug the error",
		"resolve the issue",
		"repair the problem",
	},
	IntentCompare: {
		"compare implementations",
		"difference between methods",
		"contrast approaches",
		"similarities and differences",
	},
}

// ModelBasedUnderstanding implements ML model-based query understanding.
// Uses embedding similarity to classify intent and combines with heuristic
// keyword extraction for robust query understanding.
type ModelBasedUnderstanding struct {
	mu              sync.RWMutex
	enabled         bool
	mlService       ml.Service
	intentEmbedding []IntentEmbedding
	log             *logger.Logger
}

// NewModelBasedUnderstanding creates a new model-based understanding service.
func NewModelBasedUnderstanding(log *logger.Logger) *ModelBasedUnderstanding {
	return &ModelBasedUnderstanding{
		enabled:         false,
		intentEmbedding: nil,
		log:             log,
	}
}

// Initialize initializes the model-based understanding with ML service.
// This pre-computes embeddings for canonical intent queries.
func (m *ModelBasedUnderstanding) Initialize(ctx context.Context, mlService ml.Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mlService == nil {
		return errors.New(errors.CodeMLError, "ML service is required for model-based understanding")
	}

	m.mlService = mlService
	m.log.Info("Initializing model-based query understanding")

	// Pre-compute embeddings for canonical intent queries
	if err := m.precomputeIntentEmbeddings(ctx); err != nil {
		m.log.Warn("Failed to pre-compute intent embeddings, model will be disabled", "error", err)
		return err
	}

	m.log.Info("Model-based query understanding initialized successfully",
		"intent_embeddings", len(m.intentEmbedding))
	return nil
}

// precomputeIntentEmbeddings pre-computes embeddings for canonical intent queries.
func (m *ModelBasedUnderstanding) precomputeIntentEmbeddings(ctx context.Context) error {
	if m.mlService == nil {
		return errors.New(errors.CodeMLError, "ML service not set")
	}

	var allQueries []string
	var queryToIntent []QueryIntent

	// Collect all canonical queries
	for intent, queries := range canonicalIntentQueries {
		for _, query := range queries {
			allQueries = append(allQueries, query)
			queryToIntent = append(queryToIntent, intent)
		}
	}

	if len(allQueries) == 0 {
		return errors.New(errors.CodeMLError, "no canonical intent queries defined")
	}

	// Generate embeddings for all queries
	embeddings, err := m.mlService.Embed(ctx, allQueries)
	if err != nil {
		return errors.Wrap(errors.CodeMLError, "failed to generate intent embeddings", err)
	}

	// Store embeddings with their intents
	m.intentEmbedding = make([]IntentEmbedding, len(embeddings))
	for i, embedding := range embeddings {
		m.intentEmbedding[i] = IntentEmbedding{
			Intent:    queryToIntent[i],
			Embedding: embedding,
		}
	}

	return nil
}

// Parse analyzes a query using ML model embeddings.
// Uses cosine similarity to canonical intent queries for classification.
// Falls back to heuristic keyword extraction if model fails.
func (m *ModelBasedUnderstanding) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.enabled {
		return nil, ErrModelNotEnabled
	}

	if m.mlService == nil || len(m.intentEmbedding) == 0 {
		m.log.Debug("Model-based understanding not initialized, falling back")
		return nil, ErrModelNotEnabled
	}

	// Generate embedding for the query
	embeddings, err := m.mlService.Embed(ctx, []string{query})
	if err != nil {
		m.log.Warn("Failed to generate query embedding", "error", err)
		return nil, err
	}

	if len(embeddings) == 0 {
		return nil, errors.New(errors.CodeMLError, "no embedding generated for query")
	}

	queryEmbedding := embeddings[0]

	// Classify intent using cosine similarity
	intent, confidence := m.classifyIntent(queryEmbedding)

	// Extract keywords using heuristic method (robust and fast)
	normalized := normalizeQuery(query)
	keywords := extractKeywords(normalized)
	codeTerms := extractCodeTerms(keywords)

	// Detect target type using heuristics (works well)
	targetType := DetectTargetType(normalized)

	// Expand with synonyms
	expanded := expandWithSynonyms(keywords, codeTerms)

	// Build search query
	searchQuery := buildSearchQuery(normalized, keywords, expanded, intent)

	// Detect query type
	queryType := DetectQueryType(query)

	result := &ParsedQuery{
		Original:     query,
		Normalized:   normalized,
		Keywords:     keywords,
		CodeTerms:    codeTerms,
		ActionIntent: intent,
		TargetType:   targetType,
		Expanded:     expanded,
		SearchQuery:  searchQuery,
		Confidence:   confidence,
		UsedModel:    true,
		QueryType:    queryType,
	}

	m.log.Debug("Parsed query with model",
		"original", query,
		"intent", intent,
		"target", targetType,
		"keywords", len(keywords),
		"confidence", confidence,
	)

	return result, nil
}

// classifyIntent classifies query intent using cosine similarity.
func (m *ModelBasedUnderstanding) classifyIntent(queryEmbedding []float32) (QueryIntent, float32) {
	if len(m.intentEmbedding) == 0 {
		return IntentUnknown, 0.0
	}

	// Track best match for each intent type
	intentScores := make(map[QueryIntent]float32)
	intentCounts := make(map[QueryIntent]int)

	for _, ie := range m.intentEmbedding {
		similarity := cosineSimilarity(queryEmbedding, ie.Embedding)

		// Track max similarity for each intent
		if similarity > intentScores[ie.Intent] {
			intentScores[ie.Intent] = similarity
		}
		intentCounts[ie.Intent]++
	}

	// Find best intent
	var bestIntent QueryIntent = IntentUnknown
	var bestScore float32 = 0.0

	for intent, score := range intentScores {
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	// Require minimum confidence threshold (0.65 is reasonable for semantic similarity)
	if bestScore < 0.65 {
		return IntentUnknown, bestScore
	}

	// Convert similarity to confidence (0.65-1.0 maps to 0.7-1.0)
	confidence := 0.7 + (bestScore-0.65)*0.3/0.35
	if confidence > 1.0 {
		confidence = 1.0
	}

	return bestIntent, confidence
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// IsEnabled returns whether model-based understanding is enabled.
func (m *ModelBasedUnderstanding) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled && m.mlService != nil && len(m.intentEmbedding) > 0
}

// SetEnabled enables or disables model-based understanding.
func (m *ModelBasedUnderstanding) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.enabled = enabled
	if enabled {
		if m.mlService == nil {
			m.log.Warn("Model-based query understanding enabled but ML service not initialized")
		} else {
			m.log.Info("Model-based query understanding enabled")
		}
	} else {
		m.log.Info("Model-based query understanding disabled")
	}
}
