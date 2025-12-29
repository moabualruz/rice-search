// Package models provides ML model management and mapping for Rice Search.
package models

import (
	"fmt"
	"time"
)

// ModelType represents the type of ML model.
type ModelType string

const (
	// ModelTypeEmbed is for dense embedding models.
	ModelTypeEmbed ModelType = "embed"

	// ModelTypeRerank is for reranking models.
	ModelTypeRerank ModelType = "rerank"

	// ModelTypeQueryUnderstand is for query understanding models.
	ModelTypeQueryUnderstand ModelType = "query_understand"
)

// Valid validates the model type.
func (mt ModelType) Valid() bool {
	switch mt {
	case ModelTypeEmbed, ModelTypeRerank, ModelTypeQueryUnderstand:
		return true
	default:
		return false
	}
}

// String implements fmt.Stringer.
func (mt ModelType) String() string {
	return string(mt)
}

// ModelInfo describes a registered ML model.
type ModelInfo struct {
	// ID is the unique model identifier (e.g., "jinaai/jina-code-embeddings-1.5b").
	ID string `json:"id" yaml:"id"`

	// Type is the model type (embed, rerank, query_understand).
	Type ModelType `json:"type" yaml:"type"`

	// DisplayName is the human-readable model name.
	DisplayName string `json:"display_name" yaml:"display_name"`

	// Description is the model description.
	Description string `json:"description" yaml:"description"`

	// OutputDim is the output dimension for embedding models.
	OutputDim int `json:"output_dim,omitempty" yaml:"output_dim,omitempty"`

	// MaxTokens is the maximum sequence length the model can handle.
	MaxTokens int `json:"max_tokens" yaml:"max_tokens"`

	// Downloaded indicates if the model is installed locally.
	Downloaded bool `json:"downloaded" yaml:"downloaded"`

	// IsDefault indicates if this is the default model for its type.
	IsDefault bool `json:"is_default" yaml:"is_default"`

	// GPUEnabled indicates if GPU acceleration is enabled for this model.
	GPUEnabled bool `json:"gpu_enabled" yaml:"gpu_enabled"`

	// Size is the model size in bytes.
	Size int64 `json:"size" yaml:"size"`

	// DownloadURL is the URL to download the model.
	DownloadURL string `json:"download_url" yaml:"download_url"`
}

// Validate validates the model info.
func (mi *ModelInfo) Validate() error {
	if mi.ID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}

	if !mi.Type.Valid() {
		return fmt.Errorf("invalid model type: %s", mi.Type)
	}

	if mi.DisplayName == "" {
		return fmt.Errorf("display name cannot be empty")
	}

	if mi.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}

	if mi.Type == ModelTypeEmbed && mi.OutputDim <= 0 {
		return fmt.Errorf("output_dim must be positive for embedding models")
	}

	return nil
}

// ModelMapper defines how to map inputs/outputs to/from a model.
type ModelMapper struct {
	// ID is the unique mapper identifier.
	ID string `json:"id" yaml:"id"`

	// Name is the mapper name.
	Name string `json:"name" yaml:"name"`

	// ModelID is the ID of the model this mapper is for.
	ModelID string `json:"model_id" yaml:"model_id"`

	// Type is the model type this mapper handles.
	Type ModelType `json:"type" yaml:"type"`

	// PromptTemplate is an optional template for formatting inputs.
	PromptTemplate string `json:"prompt_template,omitempty" yaml:"prompt_template,omitempty"`

	// InputMapping maps logical input fields to model input fields.
	// Example: {"query": "text", "context": "context"}
	InputMapping map[string]string `json:"input_mapping" yaml:"input_mapping"`

	// OutputMapping maps model output fields to logical output fields.
	// Example: {"embedding": "embeddings", "score": "scores"}
	OutputMapping map[string]string `json:"output_mapping" yaml:"output_mapping"`

	// CreatedAt is when the mapper was created.
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`

	// UpdatedAt is when the mapper was last updated.
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// Validate validates the model mapper.
func (mm *ModelMapper) Validate() error {
	if mm.ID == "" {
		return fmt.Errorf("mapper ID cannot be empty")
	}

	if mm.Name == "" {
		return fmt.Errorf("mapper name cannot be empty")
	}

	if mm.ModelID == "" {
		return fmt.Errorf("model ID cannot be empty")
	}

	if !mm.Type.Valid() {
		return fmt.Errorf("invalid model type: %s", mm.Type)
	}

	if mm.InputMapping == nil {
		return fmt.Errorf("input mapping cannot be nil")
	}

	if mm.OutputMapping == nil {
		return fmt.Errorf("output mapping cannot be nil")
	}

	return nil
}

// Touch updates the UpdatedAt timestamp.
func (mm *ModelMapper) Touch() {
	mm.UpdatedAt = time.Now()
}

// ModelTypeConfig defines configuration for a specific model type.
type ModelTypeConfig struct {
	// Type is the model type.
	Type ModelType `json:"type" yaml:"type"`

	// DefaultModel is the default model ID for this type.
	DefaultModel string `json:"default_model" yaml:"default_model"`

	// GPUEnabled indicates if GPU is enabled for this model type.
	GPUEnabled bool `json:"gpu_enabled" yaml:"gpu_enabled"`

	// Fallback is an optional fallback model ID (used for query_understand).
	Fallback string `json:"fallback,omitempty" yaml:"fallback,omitempty"`
}

// Validate validates the model type config.
func (mtc *ModelTypeConfig) Validate() error {
	if !mtc.Type.Valid() {
		return fmt.Errorf("invalid model type: %s", mtc.Type)
	}

	if mtc.DefaultModel == "" {
		return fmt.Errorf("default model cannot be empty")
	}

	return nil
}
