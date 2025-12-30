package security

import (
	"strings"
	"testing"
)

func TestValidateQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"valid simple", "hello world", false},
		{"valid unicode", "搜索 query", false},
		{"valid long", strings.Repeat("a", 1000), false},
		{"valid at max", strings.Repeat("a", MaxQueryLength), false},
		{"empty", "", true},
		{"too long", strings.Repeat("a", MaxQueryLength+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQuery(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateStoreName(t *testing.T) {
	tests := []struct {
		name    string
		store   string
		wantErr bool
	}{
		{"valid simple", "mystore", false},
		{"valid with hyphen", "my-store", false},
		{"valid with underscore", "my_store", false},
		{"valid with number", "store123", false},
		{"valid mixed", "My-Store_123", false},
		{"empty", "", true},
		{"starts with hyphen", "-store", true},
		{"starts with underscore", "_store", true},
		{"too long", strings.Repeat("a", MaxStoreNameLength+1), true},
		{"invalid chars", "my@store", true},
		{"spaces", "my store", true},
		{"dots", "my.store", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStoreName(tt.store)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStoreName(%q) error = %v, wantErr %v", tt.store, err, tt.wantErr)
			}
		})
	}
}

func TestValidateTopK(t *testing.T) {
	tests := []struct {
		name    string
		topK    int
		wantErr bool
	}{
		{"valid min", 1, false},
		{"valid default", 20, false},
		{"valid max", 1000, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too large", 1001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTopK(tt.topK)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTopK(%d) error = %v, wantErr %v", tt.topK, err, tt.wantErr)
			}
		})
	}
}

func TestValidateWeight(t *testing.T) {
	tests := []struct {
		name    string
		weight  float64
		wantErr bool
	}{
		{"valid zero", 0.0, false},
		{"valid half", 0.5, false},
		{"valid one", 1.0, false},
		{"negative", -0.1, true},
		{"too large", 1.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSparseWeight(tt.weight)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSparseWeight(%f) error = %v, wantErr %v", tt.weight, err, tt.wantErr)
			}

			err = ValidateDenseWeight(tt.weight)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDenseWeight(%f) error = %v, wantErr %v", tt.weight, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid simple", "file.txt", false},
		{"valid nested", "src/main.go", false},
		{"empty", "", true},
		{"traversal", "../etc/passwd", true},
		{"absolute", "/etc/passwd", true},
		{"null byte", "file\x00.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"valid small", "hello world", false},
		{"valid code", "func main() {}", false},
		{"too large", strings.Repeat("a", MaxFileSize+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileContent(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileContent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSearchRequestValidator(t *testing.T) {
	topK := 20
	sparseWeight := 0.5
	denseWeight := 0.5

	t.Run("valid request", func(t *testing.T) {
		v := &SearchRequestValidator{
			Query:        "test query",
			Store:        "mystore",
			TopK:         &topK,
			SparseWeight: &sparseWeight,
			DenseWeight:  &denseWeight,
		}
		if err := v.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("missing query", func(t *testing.T) {
		v := &SearchRequestValidator{
			Query: "",
			Store: "mystore",
		}
		if err := v.Validate(); err == nil {
			t.Error("Validate() should fail for empty query")
		}
	})

	t.Run("invalid store", func(t *testing.T) {
		v := &SearchRequestValidator{
			Query: "test",
			Store: "-invalid",
		}
		if err := v.Validate(); err == nil {
			t.Error("Validate() should fail for invalid store")
		}
	})

	t.Run("invalid weight", func(t *testing.T) {
		badWeight := 1.5
		v := &SearchRequestValidator{
			Query:        "test",
			Store:        "mystore",
			SparseWeight: &badWeight,
		}
		if err := v.Validate(); err == nil {
			t.Error("Validate() should fail for invalid weight")
		}
	})
}

func TestIndexRequestValidator(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		v := &IndexRequestValidator{
			Store:   "mystore",
			Path:    "src/main.go",
			Content: "package main",
		}
		if err := v.Validate(); err != nil {
			t.Errorf("Validate() error = %v", err)
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		v := &IndexRequestValidator{
			Store:   "mystore",
			Path:    "../etc/passwd",
			Content: "test",
		}
		if err := v.Validate(); err == nil {
			t.Error("Validate() should fail for path traversal")
		}
	})
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:      "query",
		Value:      "test",
		Constraint: "too short",
	}
	if !strings.Contains(err.Error(), "query") {
		t.Error("Error() should contain field name")
	}
	if !strings.Contains(err.Error(), "test") {
		t.Error("Error() should contain value")
	}

	errNoValue := &ValidationError{
		Field:      "query",
		Constraint: "required",
	}
	if !strings.Contains(errNoValue.Error(), "query") {
		t.Error("Error() should contain field name")
	}
}
