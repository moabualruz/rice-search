package query

import (
	"context"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

var (
	// ErrModelNotEnabled is returned when model-based understanding is disabled.
	ErrModelNotEnabled = errors.New(errors.CodeMLError, "model-based query understanding not enabled")
)

// ModelBasedUnderstanding implements ML model-based query understanding.
// This is a stub for future integration with query_understand model type.
type ModelBasedUnderstanding struct {
	enabled bool
	modelID string
	log     *logger.Logger
}

// NewModelBasedUnderstanding creates a new model-based understanding service.
func NewModelBasedUnderstanding(log *logger.Logger) *ModelBasedUnderstanding {
	return &ModelBasedUnderstanding{
		enabled: false,
		modelID: "query_understand", // Will integrate with internal/ml/registry
		log:     log,
	}
}

// Parse analyzes a query using ML model.
//
// DESIGN NOTE: This is intentionally a stub that returns ErrModelNotEnabled.
// The system is designed to fall back to KeywordExtractor (heuristic-based
// understanding) which provides excellent results for code search queries.
//
// The default query_understand model (Salesforce/codet5p-220m) is a text
// generation model, not a classifier. Real ML-based query understanding
// would require one of:
//
//  1. A fine-tuned classification model trained on code search intents
//  2. Embedding-based approach with simple intent classifiers
//  3. Few-shot prompting with a larger language model
//
// The heuristic fallback (KeywordExtractor) handles:
//   - Intent detection via pattern matching (find/explain/list/fix/compare)
//   - Target type extraction (function/class/file/error)
//   - Keyword extraction with stop word removal
//   - Code term identification and synonym expansion
//
// This architecture allows seamless upgrade to ML-based understanding
// when suitable models become available, without changing the API.
func (m *ModelBasedUnderstanding) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
	if !m.enabled {
		return nil, ErrModelNotEnabled
	}

	// When enabled, this would:
	// 1. Load query understanding model from internal/ml via ONNX runtime
	// 2. Tokenize query using model's tokenizer
	// 3. Run inference for intent/target classification
	// 4. Post-process outputs into ParsedQuery structure
	// 5. Apply synonym expansion using CodeTerms
	//
	// Currently returns ErrModelNotEnabled to trigger heuristic fallback.
	m.log.Debug("Model-based understanding enabled but no suitable model loaded, falling back")
	return nil, ErrModelNotEnabled
}

// IsEnabled returns whether model-based understanding is enabled.
func (m *ModelBasedUnderstanding) IsEnabled() bool {
	return m.enabled
}

// SetEnabled enables or disables model-based understanding.
func (m *ModelBasedUnderstanding) SetEnabled(enabled bool) {
	m.enabled = enabled
	if enabled {
		m.log.Info("Model-based query understanding enabled", "model_id", m.modelID)
	} else {
		m.log.Info("Model-based query understanding disabled")
	}
}
