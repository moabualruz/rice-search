// Package query provides query understanding and parsing for Rice Search.
package query

// QueryIntent represents the user's intent when searching.
type QueryIntent string

const (
	// IntentFind - looking for specific code locations.
	IntentFind QueryIntent = "find"

	// IntentExplain - seeking understanding of how code works.
	IntentExplain QueryIntent = "explain"

	// IntentList - requesting enumeration of items.
	IntentList QueryIntent = "list"

	// IntentFix - debugging or fixing issues.
	IntentFix QueryIntent = "fix"

	// IntentCompare - comparing two or more things.
	IntentCompare QueryIntent = "compare"

	// IntentUnknown - intent cannot be determined.
	IntentUnknown QueryIntent = "unknown"
)

// ParsedQuery represents the result of query understanding.
type ParsedQuery struct {
	// Original is the raw user query.
	Original string `json:"original"`

	// Normalized is the cleaned/standardized query.
	Normalized string `json:"normalized"`

	// Keywords are extracted important terms.
	Keywords []string `json:"keywords"`

	// CodeTerms are code-specific terms (function, class, etc).
	CodeTerms []string `json:"code_terms"`

	// ActionIntent is the detected action (find, explain, etc).
	ActionIntent QueryIntent `json:"action_intent"`

	// TargetType is what the user is looking for (function, class, file, error).
	TargetType string `json:"target_type"`

	// Expanded contains synonym expansions of terms.
	Expanded []string `json:"expanded"`

	// SearchQuery is the final optimized query for search.
	SearchQuery string `json:"search_query"`

	// Confidence is how confident we are in the parsing (0-1).
	Confidence float32 `json:"confidence"`

	// UsedModel indicates if ML model was used for understanding.
	UsedModel bool `json:"used_model"`
}

// TargetType constants for common code targets.
const (
	TargetFunction = "function"
	TargetClass    = "class"
	TargetVariable = "variable"
	TargetFile     = "file"
	TargetError    = "error"
	TargetTest     = "test"
	TargetConfig   = "config"
	TargetAPI      = "api"
	TargetDatabase = "database"
	TargetAuth     = "auth"
	TargetUnknown  = "unknown"
)
