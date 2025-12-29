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
// Currently returns ErrModelNotEnabled as models are not yet integrated.
// TODO: Integrate with query_understand model type from internal/ml/
func (m *ModelBasedUnderstanding) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
	if !m.enabled {
		return nil, ErrModelNotEnabled
	}

	// TODO: Future implementation:
	// 1. Load query understanding model from internal/ml/registry
	// 2. Tokenize query
	// 3. Run inference to get:
	//    - Intent classification (find/explain/list/fix/compare)
	//    - Target type extraction (function/class/file/error)
	//    - Entity extraction (key terms and code elements)
	//    - Confidence score
	// 4. Post-process model outputs into ParsedQuery structure
	// 5. Apply synonym expansion using CodeTerms
	// 6. Build optimized search query

	m.log.Debug("Model-based understanding requested but not implemented")
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
