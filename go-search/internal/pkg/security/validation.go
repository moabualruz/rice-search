// Package security provides security utilities for input validation,
// sanitization, and sensitive data masking.
package security

import (
	"fmt"
	"regexp"
	"unicode/utf8"
)

// Validation limits as documented in 17-security.md.
const (
	// Query limits.
	MinQueryLength = 1
	MaxQueryLength = 10000

	// Store name limits.
	MinStoreNameLength = 1
	MaxStoreNameLength = 64

	// Path limits.
	MinPathLength = 1
	// MaxPathLength already defined in security.go = 1024

	// Result limits.
	MinTopK = 1
	MaxTopK = 1000

	// Default result limits.
	DefaultTopK             = 20
	DefaultRerankTopK       = 30
	DefaultMaxChunksPerFile = 3

	// Weight limits.
	MinWeight = 0.0
	MaxWeight = 1.0

	// Content limits.
	MaxFileSize    = 10 * 1024 * 1024 // 10MB
	MaxRequestSize = 10 * 1024 * 1024 // 10MB
)

// ValidationError represents a field validation error.
type ValidationError struct {
	Field      string
	Value      interface{}
	Constraint string
}

func (e *ValidationError) Error() string {
	if e.Value != nil {
		return fmt.Sprintf("validation failed for %s: %s (got: %v)", e.Field, e.Constraint, e.Value)
	}
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Constraint)
}

// storeNameRegex matches valid store names: alphanumeric, hyphen, underscore.
var storeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ValidateQuery validates a search query string.
// Requirements: Required, 1-10000 chars, valid UTF-8.
func ValidateQuery(query string) error {
	if query == "" {
		return &ValidationError{
			Field:      "query",
			Constraint: "required",
		}
	}

	length := utf8.RuneCountInString(query)
	if length < MinQueryLength {
		return &ValidationError{
			Field:      "query",
			Value:      length,
			Constraint: fmt.Sprintf("minimum length is %d characters", MinQueryLength),
		}
	}

	if length > MaxQueryLength {
		return &ValidationError{
			Field:      "query",
			Value:      length,
			Constraint: fmt.Sprintf("maximum length is %d characters", MaxQueryLength),
		}
	}

	if !utf8.ValidString(query) {
		return &ValidationError{
			Field:      "query",
			Constraint: "must be valid UTF-8",
		}
	}

	return nil
}

// ValidateStoreName validates a store name.
// Requirements: Required, 1-64 chars, alphanumeric + hyphen + underscore, must start with alphanumeric.
func ValidateStoreName(name string) error {
	if name == "" {
		return &ValidationError{
			Field:      "store",
			Constraint: "required",
		}
	}

	if len(name) < MinStoreNameLength {
		return &ValidationError{
			Field:      "store",
			Value:      len(name),
			Constraint: fmt.Sprintf("minimum length is %d characters", MinStoreNameLength),
		}
	}

	if len(name) > MaxStoreNameLength {
		return &ValidationError{
			Field:      "store",
			Value:      len(name),
			Constraint: fmt.Sprintf("maximum length is %d characters", MaxStoreNameLength),
		}
	}

	if !storeNameRegex.MatchString(name) {
		return &ValidationError{
			Field:      "store",
			Value:      name,
			Constraint: "must contain only alphanumeric characters, hyphens, and underscores, and start with alphanumeric",
		}
	}

	return nil
}

// ValidateTopK validates the top_k parameter.
// Requirements: 1-1000.
func ValidateTopK(topK int) error {
	if topK < MinTopK {
		return &ValidationError{
			Field:      "top_k",
			Value:      topK,
			Constraint: fmt.Sprintf("minimum value is %d", MinTopK),
		}
	}

	if topK > MaxTopK {
		return &ValidationError{
			Field:      "top_k",
			Value:      topK,
			Constraint: fmt.Sprintf("maximum value is %d", MaxTopK),
		}
	}

	return nil
}

// ValidateWeight validates a weight parameter (sparse_weight, dense_weight, etc.).
// Requirements: 0.0-1.0.
func ValidateWeight(field string, weight float64) error {
	if weight < MinWeight {
		return &ValidationError{
			Field:      field,
			Value:      weight,
			Constraint: fmt.Sprintf("minimum value is %.1f", MinWeight),
		}
	}

	if weight > MaxWeight {
		return &ValidationError{
			Field:      field,
			Value:      weight,
			Constraint: fmt.Sprintf("maximum value is %.1f", MaxWeight),
		}
	}

	return nil
}

// ValidateSparseWeight validates the sparse_weight parameter.
func ValidateSparseWeight(weight float64) error {
	return ValidateWeight("sparse_weight", weight)
}

// ValidateDenseWeight validates the dense_weight parameter.
func ValidateDenseWeight(weight float64) error {
	return ValidateWeight("dense_weight", weight)
}

// ValidateDiversityLambda validates the diversity_lambda parameter.
// Requirements: 0.0-1.0 (0=diverse, 1=relevant).
func ValidateDiversityLambda(lambda float64) error {
	return ValidateWeight("diversity_lambda", lambda)
}

// ValidateDedupThreshold validates the dedup_threshold parameter.
// Requirements: 0.0-1.0.
func ValidateDedupThreshold(threshold float64) error {
	return ValidateWeight("dedup_threshold", threshold)
}

// ValidateFilePath validates a file path for indexing.
// Requirements: Required, 1-1024 chars, no null bytes, no path traversal.
func ValidateFilePath(path string) error {
	if path == "" {
		return &ValidationError{
			Field:      "path",
			Constraint: "required",
		}
	}

	if len(path) < MinPathLength {
		return &ValidationError{
			Field:      "path",
			Value:      len(path),
			Constraint: fmt.Sprintf("minimum length is %d characters", MinPathLength),
		}
	}

	// Use the security path validation which checks for traversal, null bytes, etc.
	if err := ValidatePath(path); err != nil {
		return &ValidationError{
			Field:      "path",
			Value:      SanitizeForLog(path),
			Constraint: err.Error(),
		}
	}

	return nil
}

// ValidateFileContent validates file content for indexing.
// Requirements: Valid UTF-8, <= 10MB.
func ValidateFileContent(content string) error {
	if len(content) > MaxFileSize {
		return &ValidationError{
			Field:      "content",
			Value:      formatSize(len(content)),
			Constraint: fmt.Sprintf("maximum size is %s", formatSize(MaxFileSize)),
		}
	}

	if !utf8.ValidString(content) {
		return &ValidationError{
			Field:      "content",
			Constraint: "must be valid UTF-8",
		}
	}

	return nil
}

// ValidatePageSize validates pagination page size.
// Requirements: 1-1000.
func ValidatePageSize(pageSize int) error {
	if pageSize < 1 {
		return &ValidationError{
			Field:      "page_size",
			Value:      pageSize,
			Constraint: "minimum value is 1",
		}
	}

	if pageSize > MaxTopK {
		return &ValidationError{
			Field:      "page_size",
			Value:      pageSize,
			Constraint: fmt.Sprintf("maximum value is %d", MaxTopK),
		}
	}

	return nil
}

// ValidatePage validates pagination page number.
// Requirements: >= 1.
func ValidatePage(page int) error {
	if page < 1 {
		return &ValidationError{
			Field:      "page",
			Value:      page,
			Constraint: "minimum value is 1",
		}
	}

	return nil
}

// SearchRequestValidator provides validation for search requests.
type SearchRequestValidator struct {
	Query           string
	Store           string
	TopK            *int
	SparseWeight    *float64
	DenseWeight     *float64
	DedupThreshold  *float64
	DiversityLambda *float64
}

// Validate validates all fields in the search request.
func (v *SearchRequestValidator) Validate() error {
	if err := ValidateQuery(v.Query); err != nil {
		return err
	}

	if err := ValidateStoreName(v.Store); err != nil {
		return err
	}

	if v.TopK != nil {
		if err := ValidateTopK(*v.TopK); err != nil {
			return err
		}
	}

	if v.SparseWeight != nil {
		if err := ValidateSparseWeight(*v.SparseWeight); err != nil {
			return err
		}
	}

	if v.DenseWeight != nil {
		if err := ValidateDenseWeight(*v.DenseWeight); err != nil {
			return err
		}
	}

	if v.DedupThreshold != nil {
		if err := ValidateDedupThreshold(*v.DedupThreshold); err != nil {
			return err
		}
	}

	if v.DiversityLambda != nil {
		if err := ValidateDiversityLambda(*v.DiversityLambda); err != nil {
			return err
		}
	}

	return nil
}

// IndexRequestValidator provides validation for index requests.
type IndexRequestValidator struct {
	Store   string
	Path    string
	Content string
}

// Validate validates all fields in the index request.
func (v *IndexRequestValidator) Validate() error {
	if err := ValidateStoreName(v.Store); err != nil {
		return err
	}

	if err := ValidateFilePath(v.Path); err != nil {
		return err
	}

	if err := ValidateFileContent(v.Content); err != nil {
		return err
	}

	return nil
}
