package models

import (
	"context"
	"fmt"
	"sync"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// DownloadProgress represents the progress of a model download.
type DownloadProgress struct {
	ModelID    string  `json:"model_id"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Complete   bool    `json:"complete"`
	Error      string  `json:"error,omitempty"`
}

// Registry manages ML models and their configurations.
type Registry struct {
	models      map[string]*ModelInfo
	typeConfigs map[ModelType]*ModelTypeConfig
	storage     Storage
	log         *logger.Logger
	mu          sync.RWMutex
}

// RegistryConfig holds configuration for the model registry.
type RegistryConfig struct {
	// StoragePath is the path to store model metadata.
	StoragePath string

	// ModelsDir is the directory where models are stored.
	ModelsDir string

	// LoadDefaults loads default models on startup.
	LoadDefaults bool
}

// NewRegistry creates a new model registry.
func NewRegistry(cfg RegistryConfig, log *logger.Logger) (*Registry, error) {
	var storage Storage
	if cfg.StoragePath != "" {
		storage = NewFileStorage(cfg.StoragePath)
	} else {
		storage = NewMemoryStorage()
	}

	r := &Registry{
		models:      make(map[string]*ModelInfo),
		typeConfigs: make(map[ModelType]*ModelTypeConfig),
		storage:     storage,
		log:         log,
	}

	// Load existing models from storage
	if err := r.loadModels(); err != nil {
		return nil, fmt.Errorf("failed to load models: %w", err)
	}

	// Load default models if none exist
	if cfg.LoadDefaults && len(r.models) == 0 {
		if err := r.loadDefaults(); err != nil {
			return nil, fmt.Errorf("failed to load default models: %w", err)
		}
	}

	// Load type configs
	if err := r.loadTypeConfigs(); err != nil {
		return nil, fmt.Errorf("failed to load type configs: %w", err)
	}

	return r, nil
}

// loadModels loads all models from storage.
func (r *Registry) loadModels() error {
	models, err := r.storage.LoadAllModels()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, model := range models {
		r.models[model.ID] = model
	}

	return nil
}

// loadDefaults loads default models into the registry.
func (r *Registry) loadDefaults() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range DefaultModels {
		model := DefaultModels[i]
		r.models[model.ID] = &model

		// Save to storage
		if err := r.storage.SaveModel(&model); err != nil {
			r.log.Warn("Failed to save default model", "model", model.ID, "error", err)
		}
	}

	r.log.Info("Loaded default models", "count", len(DefaultModels))
	return nil
}

// loadTypeConfigs loads type configurations.
func (r *Registry) loadTypeConfigs() error {
	configs, err := r.storage.LoadAllTypeConfigs()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Load from storage first
	for _, cfg := range configs {
		r.typeConfigs[cfg.Type] = cfg
	}

	// Fill in defaults for missing types
	for i := range DefaultTypeConfigs {
		cfg := DefaultTypeConfigs[i]
		if _, exists := r.typeConfigs[cfg.Type]; !exists {
			r.typeConfigs[cfg.Type] = &cfg

			// Save to storage
			if err := r.storage.SaveTypeConfig(&cfg); err != nil {
				r.log.Warn("Failed to save default type config", "type", cfg.Type, "error", err)
			}
		}
	}

	return nil
}

// ListModels returns all registered models, optionally filtered by type.
func (r *Registry) ListModels(ctx context.Context, filter ModelType) ([]ModelInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ModelInfo
	for _, model := range r.models {
		if filter == "" || model.Type == filter {
			result = append(result, *model)
		}
	}

	return result, nil
}

// GetModel retrieves a model by ID.
func (r *Registry) GetModel(ctx context.Context, id string) (*ModelInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	model, exists := r.models[id]
	if !exists {
		return nil, errors.NotFoundError(fmt.Sprintf("model: %s", id))
	}

	// Return copy
	modelCopy := *model
	return &modelCopy, nil
}

// SetDefaultModel sets the default model for a given type.
func (r *Registry) SetDefaultModel(ctx context.Context, modelType ModelType, modelID string) error {
	if !modelType.Valid() {
		return errors.ValidationError(fmt.Sprintf("invalid model type: %s", modelType))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if model exists
	model, exists := r.models[modelID]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	// Check if model type matches
	if model.Type != modelType {
		return errors.ValidationError(fmt.Sprintf("model %s is type %s, not %s", modelID, model.Type, modelType))
	}

	// Clear previous default
	for _, m := range r.models {
		if m.Type == modelType && m.IsDefault {
			m.IsDefault = false
			if err := r.storage.SaveModel(m); err != nil {
				r.log.Warn("Failed to clear default flag", "model", m.ID, "error", err)
			}
		}
	}

	// Set new default
	model.IsDefault = true
	if err := r.storage.SaveModel(model); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save model", err)
	}

	// Update type config
	cfg, exists := r.typeConfigs[modelType]
	if !exists {
		cfg = &ModelTypeConfig{
			Type:         modelType,
			DefaultModel: modelID,
			GPUEnabled:   model.GPUEnabled,
		}
		r.typeConfigs[modelType] = cfg
	} else {
		cfg.DefaultModel = modelID
	}

	if err := r.storage.SaveTypeConfig(cfg); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save type config", err)
	}

	r.log.Info("Set default model", "type", modelType, "model", modelID)
	return nil
}

// ToggleGPU enables or disables GPU for a model.
func (r *Registry) ToggleGPU(ctx context.Context, modelID string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	model.GPUEnabled = enabled
	if err := r.storage.SaveModel(model); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save model", err)
	}

	// Update type config if this is the default model
	if model.IsDefault {
		cfg, exists := r.typeConfigs[model.Type]
		if exists {
			cfg.GPUEnabled = enabled
			if err := r.storage.SaveTypeConfig(cfg); err != nil {
				r.log.Warn("Failed to update type config", "type", model.Type, "error", err)
			}
		}
	}

	r.log.Info("Toggled GPU", "model", modelID, "enabled", enabled)
	return nil
}

// GetDefaultForType returns the default model for a given type.
func (r *Registry) GetDefaultForType(ctx context.Context, modelType ModelType) (*ModelInfo, error) {
	if !modelType.Valid() {
		return nil, errors.ValidationError(fmt.Sprintf("invalid model type: %s", modelType))
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// First try type config
	cfg, exists := r.typeConfigs[modelType]
	if exists && cfg.DefaultModel != "" {
		if model, ok := r.models[cfg.DefaultModel]; ok {
			modelCopy := *model
			return &modelCopy, nil
		}
	}

	// Fallback: find first model with IsDefault flag
	for _, model := range r.models {
		if model.Type == modelType && model.IsDefault {
			modelCopy := *model
			return &modelCopy, nil
		}
	}

	return nil, errors.NotFoundError(fmt.Sprintf("default model for type: %s", modelType))
}

// DownloadModel downloads a model from its download URL.
// Returns a channel that reports progress.
func (r *Registry) DownloadModel(ctx context.Context, modelID string) (<-chan DownloadProgress, error) {
	r.mu.RLock()
	model, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return nil, errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	progressChan := make(chan DownloadProgress, 10)

	// TODO: Implement actual download logic
	// For now, simulate download
	go func() {
		defer close(progressChan)

		// Simulate progress
		for i := 0; i <= 100; i += 10 {
			select {
			case <-ctx.Done():
				progressChan <- DownloadProgress{
					ModelID: modelID,
					Error:   "download cancelled",
				}
				return
			case progressChan <- DownloadProgress{
				ModelID:    modelID,
				Downloaded: int64(i * int(model.Size) / 100),
				Total:      model.Size,
				Percent:    float64(i),
				Complete:   i == 100,
			}:
			}
		}

		// Mark as downloaded
		r.mu.Lock()
		model.Downloaded = true
		if err := r.storage.SaveModel(model); err != nil {
			r.log.Warn("Failed to update model download status", "model", modelID, "error", err)
		}
		r.mu.Unlock()

		r.log.Info("Downloaded model", "model", modelID)
	}()

	return progressChan, nil
}

// DeleteModel removes a model from the registry.
func (r *Registry) DeleteModel(ctx context.Context, modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	// Prevent deletion of default models
	if model.IsDefault {
		return errors.ValidationError("cannot delete default model, set another model as default first")
	}

	// Delete from storage
	if err := r.storage.DeleteModel(modelID); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to delete model", err)
	}

	delete(r.models, modelID)
	r.log.Info("Deleted model", "model", modelID)
	return nil
}

// RegisterModel adds a new model to the registry.
func (r *Registry) RegisterModel(ctx context.Context, model *ModelInfo) error {
	if err := model.Validate(); err != nil {
		return errors.ValidationError(err.Error())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already exists
	if _, exists := r.models[model.ID]; exists {
		return errors.AlreadyExistsError(fmt.Sprintf("model: %s", model.ID))
	}

	// Save to storage
	if err := r.storage.SaveModel(model); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save model", err)
	}

	r.models[model.ID] = model
	r.log.Info("Registered model", "model", model.ID)
	return nil
}

// =============================================================================
// Convenience Methods (for web handlers compatibility)
// =============================================================================

// ListAllModels returns all registered models without filtering.
// This is a convenience wrapper for ListModels with an empty filter.
func (r *Registry) ListAllModels() []*ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModelInfo, 0, len(r.models))
	for _, model := range r.models {
		result = append(result, model)
	}
	return result
}

// GetTypeConfig returns the configuration for a model type.
func (r *Registry) GetTypeConfig(modelType ModelType) *ModelTypeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cfg, exists := r.typeConfigs[modelType]
	if !exists {
		return nil
	}
	return cfg
}
