package query

import (
	"context"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// Service combines keyword extraction and model-based understanding.
// It uses the model if enabled, falling back to keyword extraction.
type Service struct {
	keywordExtractor *KeywordExtractor
	modelBased       *ModelBasedUnderstanding
	useModel         bool
	log              *logger.Logger
}

// NewService creates a new query understanding service.
func NewService(log *logger.Logger) *Service {
	return &Service{
		keywordExtractor: NewKeywordExtractor(log),
		modelBased:       NewModelBasedUnderstanding(log),
		useModel:         false, // Disabled by default
		log:              log,
	}
}

// InitializeWithMLService initializes model-based understanding with ML service.
// This should be called after the ML service is ready.
func (s *Service) InitializeWithMLService(ctx context.Context, mlService ml.Service) error {
	if s.modelBased == nil {
		s.log.Warn("Model-based understanding not available")
		return nil
	}

	if err := s.modelBased.Initialize(ctx, mlService); err != nil {
		s.log.Warn("Failed to initialize model-based understanding", "error", err)
		return err
	}

	s.log.Info("Query understanding service initialized with ML model")
	return nil
}

// Parse analyzes a query and returns structured understanding.
// If model-based understanding is enabled, tries that first.
// Always falls back to keyword extraction on error or if model is disabled.
func (s *Service) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
	if query == "" {
		return &ParsedQuery{
			Original:     "",
			Normalized:   "",
			Keywords:     []string{},
			CodeTerms:    []string{},
			ActionIntent: IntentUnknown,
			TargetType:   TargetUnknown,
			Expanded:     []string{},
			SearchQuery:  "",
			Confidence:   0,
			UsedModel:    false,
		}, nil
	}

	// Try model-based understanding if enabled
	if s.useModel && s.modelBased != nil && s.modelBased.IsEnabled() {
		result, err := s.modelBased.Parse(ctx, query)
		if err == nil && result != nil {
			result.UsedModel = true
			s.log.Debug("Used model-based query understanding", "query", query)
			return result, nil
		}

		// Log model failure but don't return error - fall back
		if err != nil && err != ErrModelNotEnabled {
			s.log.Warn("Model-based understanding failed, falling back to keyword extraction",
				"error", err)
		}
	}

	// Fall back to keyword extraction
	result, err := s.keywordExtractor.Parse(ctx, query)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// IsModelEnabled returns true if model-based understanding is enabled.
func (s *Service) IsModelEnabled() bool {
	return s.useModel && s.modelBased != nil && s.modelBased.IsEnabled()
}

// SetModelEnabled enables or disables model-based understanding.
func (s *Service) SetModelEnabled(enabled bool) {
	s.useModel = enabled
	if s.modelBased != nil {
		s.modelBased.SetEnabled(enabled)
	}
	s.log.Info("Query understanding model mode changed", "enabled", enabled)
}
