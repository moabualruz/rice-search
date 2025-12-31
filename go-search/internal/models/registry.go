package models

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/onnx"
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
	// Export-specific fields
	IsExport      bool   `json:"is_export,omitempty"`
	ExportStatus  string `json:"export_status,omitempty"`  // downloading, exporting, validating, complete, error
	ExportMessage string `json:"export_message,omitempty"` // Current step description
	EstimatedTime int    `json:"estimated_time,omitempty"` // Estimated seconds remaining
}

// ExportJob tracks an ongoing export operation.
type ExportJob struct {
	ID            string    `json:"id"`
	ModelID       string    `json:"model_id"`
	ModelType     ModelType `json:"model_type"`
	Status        string    `json:"status"` // pending, downloading, exporting, validating, complete, error
	Message       string    `json:"message"`
	Percent       float64   `json:"percent"`
	StartedAt     time.Time `json:"started_at"`
	EstimatedTime int       `json:"estimated_time"` // Seconds remaining
	Error         string    `json:"error,omitempty"`
	Complete      bool      `json:"complete"`
}

// Registry manages ML models and their configurations.
type Registry struct {
	models      map[string]*ModelInfo
	typeConfigs map[ModelType]*ModelTypeConfig
	storage     Storage
	log         *logger.Logger
	mu          sync.RWMutex

	// HuggingFace integration
	hfClient *HuggingFaceClient
	exporter *ONNXExporter
	bus      bus.Bus

	// Export job tracking
	exportJobs   map[string]*ExportJob
	exportJobsMu sync.RWMutex
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
func NewRegistry(cfg RegistryConfig, log *logger.Logger, b bus.Bus) (*Registry, error) {
	var storage Storage
	if cfg.StoragePath != "" {
		storage = NewFileStorage(cfg.StoragePath)
	} else {
		storage = NewMemoryStorage()
	}

	// Determine models directory
	modelsDir := cfg.ModelsDir
	if modelsDir == "" {
		modelsDir = "./models"
	}

	r := &Registry{
		models:      make(map[string]*ModelInfo),
		typeConfigs: make(map[ModelType]*ModelTypeConfig),
		storage:     storage,
		log:         log,
		hfClient:    NewHuggingFaceClient(),
		exporter:    NewONNXExporter(modelsDir),
		exportJobs:  make(map[string]*ExportJob),
		bus:         b,
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

	result := make([]ModelInfo, 0)
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

	if model.DownloadURL == "" {
		return nil, errors.ValidationError("model has no download URL")
	}

	progressChan := make(chan DownloadProgress, 10)

	go func() {
		defer close(progressChan)

		// Create model directory
		modelDir := filepath.Join("./models", modelID)
		if err := os.MkdirAll(modelDir, 0755); err != nil {
			progressChan <- DownloadProgress{
				ModelID: modelID,
				Error:   fmt.Sprintf("failed to create model directory: %v", err),
			}
			return
		}

		// Files to download for different model types
		// Note: We try multiple paths for model.onnx since HuggingFace models vary
		var files []string
		switch model.Type {
		case ModelTypeEmbed:
			files = []string{"model.onnx", "tokenizer.json", "config.json"}
		case ModelTypeRerank:
			files = []string{"model.onnx", "tokenizer.json", "config.json"}
		case ModelTypeQueryUnderstand:
			files = []string{"model.onnx", "tokenizer.json", "config.json"}
		default:
			files = []string{"model.onnx", "config.json"}
		}

		totalFiles := len(files)
		var totalDownloaded int64

		for i, file := range files {
			destPath := filepath.Join(modelDir, file)

			// Skip if file already exists
			if _, err := os.Stat(destPath); err == nil {
				r.log.Debug("File already exists, skipping", "file", destPath)
				totalDownloaded += model.Size / int64(totalFiles)
				continue
			}

			// Try multiple paths for ONNX files (HuggingFace models vary in structure)
			var downloadURLs []string
			if file == "model.onnx" {
				downloadURLs = []string{
					fmt.Sprintf("%s/resolve/main/%s", model.DownloadURL, file),       // root
					fmt.Sprintf("%s/resolve/main/onnx/%s", model.DownloadURL, file),  // onnx/ folder
					fmt.Sprintf("%s/resolve/main/model/%s", model.DownloadURL, file), // model/ folder
				}
			} else {
				downloadURLs = []string{
					fmt.Sprintf("%s/resolve/main/%s", model.DownloadURL, file),
				}
			}

			var downloaded int64
			var downloadErr error
			for _, downloadURL := range downloadURLs {
				downloaded, downloadErr = r.downloadFile(ctx, downloadURL, destPath, func(n int64) {
					totalDownloaded += n
					percent := float64(totalDownloaded) / float64(model.Size) * 100
					if percent > 100 {
						percent = 100
					}

					// Publish progress event
					if r.bus != nil {
						_ = r.bus.Publish(ctx, bus.TopicModelProgress, bus.Event{
							ID:        fmt.Sprintf("dl-%s-%d", modelID, time.Now().UnixNano()),
							Type:      bus.TopicModelProgress,
							Source:    "registry",
							Timestamp: time.Now().Unix(),
							Payload: DownloadProgress{
								ModelID:    modelID,
								Downloaded: totalDownloaded,
								Total:      model.Size,
								Percent:    percent,
								Complete:   false,
							},
						})
					}

					select {
					case progressChan <- DownloadProgress{
						ModelID:    modelID,
						Downloaded: totalDownloaded,
						Total:      model.Size,
						Percent:    percent,
						Complete:   false,
					}:
					default:
						// Channel full, skip progress update
					}
				})
				if downloadErr == nil {
					r.log.Debug("Downloaded file", "file", file, "url", downloadURL, "bytes", downloaded)
					break
				}
				r.log.Debug("Download path not found, trying next", "url", downloadURL)
			}

			if downloadErr != nil {
				// Non-critical files can be skipped
				if file != "model.onnx" {
					r.log.Warn("Failed to download optional file", "file", file, "error", downloadErr)
					continue
				}
				progressChan <- DownloadProgress{
					ModelID: modelID,
					Error:   fmt.Sprintf("failed to download %s (tried multiple paths): %v", file, downloadErr),
				}
				return
			}

			// Publish progress event
			if r.bus != nil {
				_ = r.bus.Publish(ctx, bus.TopicModelProgress, bus.Event{
					ID:        fmt.Sprintf("dl-%s-%d", modelID, time.Now().UnixNano()),
					Type:      bus.TopicModelProgress,
					Source:    "registry",
					Timestamp: time.Now().Unix(),
					Payload: DownloadProgress{
						ModelID:    modelID,
						Downloaded: totalDownloaded, // Approximated for multi-file
						Total:      model.Size,
						Percent:    float64(totalDownloaded) / float64(model.Size) * 100, // This logic is imperfect due to loop, handled better inside callback
						Complete:   false,
					},
				})
			}

			_ = i // Progress tracking uses totalDownloaded
		}

		// Mark as downloaded
		r.mu.Lock()
		model.Downloaded = true
		if err := r.storage.SaveModel(model); err != nil {
			r.log.Warn("Failed to update model download status", "model", modelID, "error", err)
		}
		r.mu.Unlock()

		// Send completion
		progressChan <- DownloadProgress{
			ModelID:    modelID,
			Downloaded: model.Size,
			Total:      model.Size,
			Percent:    100,
			Complete:   true,
		}

		r.log.Info("Downloaded model", "model", modelID)

		// Validate downloaded files
		if err := r.ValidateModel(ctx, modelID); err != nil {
			r.log.Warn("Model validation failed", "model", modelID, "error", err)
		}
	}()

	return progressChan, nil
}

// ValidateInference validates a model by running test inference.
func (r *Registry) ValidateInference(ctx context.Context, modelID string) error {
	r.mu.RLock()
	model, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	if !model.Downloaded {
		return fmt.Errorf("model not downloaded: %s", modelID)
	}

	modelDir := filepath.Join("./models", modelID)
	modelPath := filepath.Join(modelDir, "model.onnx")

	// Create temporary runtime for validation (CPU only for safety)
	runtimeCfg := onnx.DefaultRuntimeConfig()
	runtimeCfg.Device = onnx.DeviceCPU

	runtime, err := onnx.NewRuntime(runtimeCfg)
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}
	defer runtime.Close()

	// Load session
	session, err := runtime.LoadSession(modelID, modelPath)
	if err != nil {
		return fmt.Errorf("failed to load model: %w", err)
	}
	defer session.Close()

	// Load tokenizer if available (required for text models)
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	if _, statErr := os.Stat(tokenizerPath); statErr == nil {
		tokCfg := onnx.DefaultTokenizerConfig()
		tokCfg.MaxLength = 512 // Use reasonable default

		tokenizer, tokErr := onnx.NewTokenizer(modelDir, tokCfg)
		if tokErr != nil {
			return fmt.Errorf("failed to load tokenizer: %w", tokErr)
		}
		defer tokenizer.Close()

		// Run dummy inference with tokenizer
		dummyTexts := []string{"test validation input"}
		encoding, encErr := tokenizer.EncodePadded(dummyTexts, true)
		if encErr != nil {
			return fmt.Errorf("tokenization failed: %w", encErr)
		}

		outputs, runErr := session.RunFloat32(encoding.InputIDs, encoding.AttentionMask, encoding.Shape())
		if runErr != nil {
			return fmt.Errorf("inference failed: %w", runErr)
		}

		// Validate output
		if len(outputs) == 0 {
			return fmt.Errorf("model produced empty output")
		}

		// Check for NaN/Inf
		for i, v := range outputs {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return fmt.Errorf("model output contains NaN/Inf at index %d", i)
			}
		}

		r.log.Info("Model inference validation passed", "model", modelID, "output_size", len(outputs))
	} else {
		// For models without tokenizer, just verify session loaded successfully
		r.log.Info("Model validation passed (no tokenizer test)", "model", modelID)
	}

	return nil
}

// ValidateModel validates that a model's files are present and valid.
func (r *Registry) ValidateModel(ctx context.Context, modelID string) error {
	r.mu.RLock()
	model, exists := r.models[modelID]
	r.mu.RUnlock()

	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	modelDir := filepath.Join("./models", modelID)

	// Check model.onnx exists and is non-empty
	onnxPath := filepath.Join(modelDir, "model.onnx")
	info, err := os.Stat(onnxPath)
	if err != nil {
		return fmt.Errorf("model.onnx not found: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("model.onnx is empty")
	}

	// Check tokenizer.json exists (required for text models)
	tokenizerPath := filepath.Join(modelDir, "tokenizer.json")
	if _, err := os.Stat(tokenizerPath); err != nil {
		r.log.Warn("tokenizer.json not found", "model", modelID)
		// Not a fatal error - some models may not need tokenizer
	}

	// Check config.json exists
	configPath := filepath.Join(modelDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		r.log.Warn("config.json not found", "model", modelID)
		// Not a fatal error
	}

	// Run inference validation
	if err := r.ValidateInference(ctx, modelID); err != nil {
		return fmt.Errorf("inference validation failed: %w", err)
	}

	// Update model validation status
	r.mu.Lock()
	model.Validated = true
	model.ValidatedAt = time.Now()
	if err := r.storage.SaveModel(model); err != nil {
		r.log.Warn("Failed to save validation status", "model", modelID, "error", err)
	}
	r.mu.Unlock()

	r.log.Info("Model validated", "model", modelID)
	return nil
}

// downloadFile downloads a file from URL to destPath with progress reporting.
func (r *Registry) downloadFile(ctx context.Context, url, destPath string, onProgress func(int64)) (int64, error) {
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers for HuggingFace
	req.Header.Set("User-Agent", "rice-search/1.0")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Minute, // Long timeout for large files
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Create progress reader
	reader := &progressReader{
		reader:     resp.Body,
		onProgress: onProgress,
	}

	// Copy with progress
	written, err := io.Copy(out, reader)
	if err != nil {
		return written, fmt.Errorf("failed to write file: %w", err)
	}

	return written, nil
}

// progressReader wraps an io.Reader to report progress.
type progressReader struct {
	reader     io.Reader
	onProgress func(int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.onProgress != nil {
		r.onProgress(int64(n))
	}
	return n, err
}

// OffloadModel removes a model's files from disk but keeps the configuration.
func (r *Registry) OffloadModel(ctx context.Context, modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	model, exists := r.models[modelID]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model: %s", modelID))
	}

	if !model.Downloaded {
		return errors.ValidationError("model is not downloaded")
	}

	// Remove files
	if err := r.removeModelFiles(modelID); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to remove model files", err)
	}

	// Update state
	model.Downloaded = false
	model.Validated = false
	model.ValidatedAt = time.Time{}

	if err := r.storage.SaveModel(model); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save model state", err)
	}

	r.log.Info("Offloaded model", "model", modelID)
	return nil
}

// DeleteModel removes a model from the registry and deletes its files.
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

	// Remove files first
	if err := r.removeModelFiles(modelID); err != nil {
		r.log.Warn("Failed to remove model files during deletion", "model", modelID, "error", err)
		// Continue to delete metadata
	}

	// Delete from storage
	if err := r.storage.DeleteModel(modelID); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to delete model", err)
	}

	delete(r.models, modelID)
	r.log.Info("Deleted model", "model", modelID)
	return nil
}

// removeModelFiles deletes the model directory from disk.
func (r *Registry) removeModelFiles(modelID string) error {
	modelDir := filepath.Join("./models", modelID)
	// Check if exists
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(modelDir)
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

// SetTypeGPU sets the GPU enabled state for all models of a given type.
func (r *Registry) SetTypeGPU(ctx context.Context, modelType ModelType, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, exists := r.typeConfigs[modelType]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model type: %s", modelType))
	}

	cfg.GPUEnabled = enabled

	// Save to storage
	if err := r.storage.SaveTypeConfig(cfg); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save type config", err)
	}

	r.log.Info("Type GPU setting updated", "type", modelType, "gpu_enabled", enabled)
	return nil
}

// ToggleTypeEnabled toggles the enabled state for a model type.
// This is primarily used for query understanding to switch between model and heuristic fallback.
func (r *Registry) ToggleTypeEnabled(ctx context.Context, modelType ModelType) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, exists := r.typeConfigs[modelType]
	if !exists {
		return errors.NotFoundError(fmt.Sprintf("model type: %s", modelType))
	}

	// Toggle the GPU enabled state (used as "enabled" for query understanding)
	cfg.GPUEnabled = !cfg.GPUEnabled

	// Save to storage
	if err := r.storage.SaveTypeConfig(cfg); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to save type config", err)
	}

	r.log.Info("Type enabled state toggled", "type", modelType, "enabled", cfg.GPUEnabled)
	return nil
}

// =============================================================================
// HuggingFace Integration
// =============================================================================

// SearchHuggingFaceModels searches for models on HuggingFace Hub.
// If onnxOnly is true, only models with ONNX support are returned.
func (r *Registry) SearchHuggingFaceModels(ctx context.Context, modelType ModelType, query string, onnxOnly bool, limit int) ([]HFModelInfo, error) {
	if limit <= 0 {
		limit = 20
	}

	var results []HFModelInfo
	var err error

	if onnxOnly {
		// Search specifically for ONNX models
		results, err = r.hfClient.SearchONNXModels(ctx, modelType, limit)
	} else {
		// Search all models for this type
		var filter string
		var pipelineTag string

		switch modelType {
		case ModelTypeEmbed:
			pipelineTag = "sentence-similarity"
		case ModelTypeRerank:
			pipelineTag = "text-classification"
			filter = "rerank"
		case ModelTypeQueryUnderstand:
			pipelineTag = "text-classification"
		}

		req := SearchModelsRequest{
			Search:      query,
			Filter:      filter,
			PipelineTag: pipelineTag,
			Limit:       limit,
			Sort:        "downloads",
			Direction:   -1,
		}
		results, err = r.hfClient.SearchModels(ctx, req)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to search HuggingFace: %w", err)
	}

	// If query is provided, filter results
	if query != "" && !onnxOnly {
		filtered := make([]HFModelInfo, 0)
		queryLower := strings.ToLower(query)
		for _, m := range results {
			if strings.Contains(strings.ToLower(m.ID), queryLower) ||
				strings.Contains(strings.ToLower(m.ModelID), queryLower) {
				filtered = append(filtered, m)
			}
		}
		results = filtered
	}

	return results, nil
}

// CheckModelHasONNX checks if a HuggingFace model has ONNX files available.
func (r *Registry) CheckModelHasONNX(ctx context.Context, modelID string) (hasONNX bool, onnxPath string, size int64, err error) {
	// First check model info for ONNX tag
	info, err := r.hfClient.GetModelInfo(ctx, modelID)
	if err != nil {
		return false, "", 0, fmt.Errorf("failed to get model info: %w", err)
	}

	if info.HasONNX() {
		// Try to find the actual ONNX file
		path, sz, err := r.hfClient.FindONNXFile(ctx, modelID)
		if err == nil {
			return true, path, sz, nil
		}
	}

	// Try to find ONNX file even if not tagged
	path, sz, err := r.hfClient.FindONNXFile(ctx, modelID)
	if err == nil {
		return true, path, sz, nil
	}

	return false, "", 0, nil
}

// IsExporterAvailable returns whether the ONNX exporter (optimum-cli) is available.
func (r *Registry) IsExporterAvailable() bool {
	return r.exporter.IsOptimumAvailable()
}

// GetExporterInstructions returns installation instructions for the exporter.
func (r *Registry) GetExporterInstructions() string {
	return r.exporter.GetInstallInstructions()
}

// DownloadOrExportModel downloads a model, automatically exporting to ONNX if needed.
// Returns a channel that reports progress.
func (r *Registry) DownloadOrExportModel(ctx context.Context, modelID string, modelType ModelType) (<-chan DownloadProgress, error) {
	progressChan := make(chan DownloadProgress, 20)

	go func() {
		defer close(progressChan)

		// Create job for tracking
		jobID := fmt.Sprintf("%s-%d", modelID, time.Now().UnixNano())
		job := &ExportJob{
			ID:        jobID,
			ModelID:   modelID,
			ModelType: modelType,
			Status:    "checking",
			Message:   "Checking model availability...",
			StartedAt: time.Now(),
		}
		r.exportJobsMu.Lock()
		r.exportJobs[jobID] = job
		r.exportJobsMu.Unlock()

		defer func() {
			// Clean up old jobs after 1 hour
			go func() {
				time.Sleep(1 * time.Hour)
				r.exportJobsMu.Lock()
				delete(r.exportJobs, jobID)
				r.exportJobsMu.Unlock()
			}()
		}()

		// Send initial status
		progressChan <- DownloadProgress{
			ModelID:       modelID,
			IsExport:      false,
			ExportStatus:  "checking",
			ExportMessage: "Checking if model has ONNX files...",
			Percent:       0,
		}

		// Check if model has ONNX
		hasONNX, onnxPath, onnxSize, err := r.CheckModelHasONNX(ctx, modelID)
		if err != nil {
			r.log.Warn("Error checking ONNX availability", "model", modelID, "error", err)
		}

		if hasONNX {
			r.log.Info("Model has ONNX, downloading directly", "model", modelID, "path", onnxPath)
			r.downloadONNXModel(ctx, modelID, modelType, onnxPath, onnxSize, progressChan, job)
		} else {
			r.log.Info("Model has no ONNX, will export", "model", modelID)

			// Check if exporter is available
			if !r.exporter.IsOptimumAvailable() {
				progressChan <- DownloadProgress{
					ModelID:       modelID,
					IsExport:      true,
					ExportStatus:  "error",
					ExportMessage: "ONNX exporter not available",
					Error:         fmt.Sprintf("Model %s requires export but optimum-cli is not installed. %s", modelID, r.exporter.GetInstallInstructions()),
				}
				job.Status = "error"
				job.Error = "Exporter not available"
				return
			}

			r.exportModelToONNX(ctx, modelID, modelType, progressChan, job)
		}
	}()

	return progressChan, nil
}

// downloadONNXModel downloads a model that has ONNX files on HuggingFace.
func (r *Registry) downloadONNXModel(ctx context.Context, modelID string, modelType ModelType, onnxPath string, onnxSize int64, progressChan chan<- DownloadProgress, job *ExportJob) {
	job.Status = "downloading"
	job.Message = "Downloading ONNX model..."

	// Create model directory
	modelDir := filepath.Join("./models", modelID)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		progressChan <- DownloadProgress{
			ModelID: modelID,
			Error:   fmt.Sprintf("Failed to create directory: %v", err),
		}
		return
	}

	// Files to download
	files := []struct {
		remote string
		local  string
		size   int64
	}{
		{onnxPath, "model.onnx", onnxSize},
		{"tokenizer.json", "tokenizer.json", 0},
		{"config.json", "config.json", 0},
	}

	totalSize := onnxSize
	if totalSize == 0 {
		totalSize = 500 * 1024 * 1024 // Estimate 500MB
	}
	var downloaded int64

	for _, f := range files {
		destPath := filepath.Join(modelDir, f.local)

		// Skip if exists
		if _, err := os.Stat(destPath); err == nil {
			continue
		}

		progressChan <- DownloadProgress{
			ModelID:       modelID,
			IsExport:      false,
			ExportStatus:  "downloading",
			ExportMessage: fmt.Sprintf("Downloading %s...", f.local),
			Downloaded:    downloaded,
			Total:         totalSize,
			Percent:       float64(downloaded) / float64(totalSize) * 100,
		}

		// Download file
		file, err := os.Create(destPath)
		if err != nil {
			if f.local == "model.onnx" {
				progressChan <- DownloadProgress{ModelID: modelID, Error: fmt.Sprintf("Failed to create %s: %v", f.local, err)}
				return
			}
			continue
		}

		err = r.hfClient.DownloadFile(ctx, modelID, f.remote, file, func(dl, total int64) {
			downloaded += dl
			pct := float64(downloaded) / float64(totalSize) * 100
			if pct > 100 {
				pct = 100
			}
			progressChan <- DownloadProgress{
				ModelID:       modelID,
				Downloaded:    downloaded,
				Total:         totalSize,
				Percent:       pct,
				ExportStatus:  "downloading",
				ExportMessage: fmt.Sprintf("Downloading %s...", f.local),
			}
		})
		file.Close()

		if err != nil {
			if f.local == "model.onnx" {
				progressChan <- DownloadProgress{ModelID: modelID, Error: fmt.Sprintf("Failed to download %s: %v", f.local, err)}
				return
			}
			r.log.Warn("Failed to download optional file", "file", f.local, "error", err)
			os.Remove(destPath)
		}
	}

	// Register the model
	r.registerDownloadedModel(ctx, modelID, modelType, progressChan, job)
}

// exportModelToONNX exports a PyTorch model to ONNX format.
func (r *Registry) exportModelToONNX(ctx context.Context, modelID string, modelType ModelType, progressChan chan<- DownloadProgress, job *ExportJob) {
	job.Status = "exporting"
	job.Message = "Exporting model to ONNX format..."

	// Estimate time based on model type (rough estimates)
	var estimatedMinutes int
	switch modelType {
	case ModelTypeEmbed:
		estimatedMinutes = 5
	case ModelTypeRerank:
		estimatedMinutes = 3
	default:
		estimatedMinutes = 4
	}
	job.EstimatedTime = estimatedMinutes * 60

	progressChan <- DownloadProgress{
		ModelID:       modelID,
		IsExport:      true,
		ExportStatus:  "exporting",
		ExportMessage: fmt.Sprintf("Starting ONNX export (estimated %d minutes)...", estimatedMinutes),
		Percent:       5,
		EstimatedTime: job.EstimatedTime,
	}

	// Start export
	task := GetTaskForModelType(modelType)
	// Special case for CodeT5+ query understanding models which need text2text-generation
	if strings.HasPrefix(modelID, "Salesforce/codet5p-") && modelType == ModelTypeQueryUnderstand {
		task = "text2text-generation"
	}
	exportChan, err := r.exporter.ExportModel(ctx, modelID, task)
	if err != nil {
		progressChan <- DownloadProgress{
			ModelID:       modelID,
			IsExport:      true,
			ExportStatus:  "error",
			ExportMessage: "Export failed to start",
			Error:         err.Error(),
		}
		job.Status = "error"
		job.Error = err.Error()
		return
	}

	// Forward export progress
	for progress := range exportChan {
		job.Status = progress.Status
		job.Message = progress.Message
		job.Percent = progress.Percent

		// Update estimated time based on progress
		elapsed := time.Since(job.StartedAt).Seconds()
		if progress.Percent > 0 {
			remaining := int((100 - progress.Percent) / progress.Percent * elapsed)
			job.EstimatedTime = remaining
		}

		progressChan <- DownloadProgress{
			ModelID:       modelID,
			IsExport:      true,
			ExportStatus:  progress.Status,
			ExportMessage: progress.Message,
			Percent:       progress.Percent,
			EstimatedTime: job.EstimatedTime,
			Error:         progress.Error,
			Complete:      progress.Complete,
		}

		if progress.Error != "" {
			job.Status = "error"
			job.Error = progress.Error
			return
		}
	}

	// Register the exported model
	r.registerDownloadedModel(ctx, modelID, modelType, progressChan, job)
}

// registerDownloadedModel registers a downloaded/exported model in the registry.
func (r *Registry) registerDownloadedModel(ctx context.Context, modelID string, modelType ModelType, progressChan chan<- DownloadProgress, job *ExportJob) {
	job.Status = "validating"
	job.Message = "Validating model..."

	progressChan <- DownloadProgress{
		ModelID:       modelID,
		IsExport:      job.Status == "exporting",
		ExportStatus:  "validating",
		ExportMessage: "Validating downloaded model...",
		Percent:       95,
	}

	// Check if model exists in registry
	r.mu.Lock()
	model, exists := r.models[modelID]
	if !exists {
		// Create new model entry
		model = &ModelInfo{
			ID:          modelID,
			Type:        modelType,
			DisplayName: modelID,
			Description: "Downloaded from HuggingFace Hub",
			Downloaded:  true,
			GPUEnabled:  true,
			DownloadURL: fmt.Sprintf("https://huggingface.co/%s", modelID),
		}

		// Set dimensions for embedding models
		if modelType == ModelTypeEmbed {
			model.OutputDim = 768 // Default, may be overridden
		}

		r.models[modelID] = model
		if err := r.storage.SaveModel(model); err != nil {
			r.log.Warn("Failed to save model", "model", modelID, "error", err)
		}
	} else {
		model.Downloaded = true
		if err := r.storage.SaveModel(model); err != nil {
			r.log.Warn("Failed to update model", "model", modelID, "error", err)
		}
	}
	r.mu.Unlock()

	// Validate the model
	if err := r.ValidateModel(ctx, modelID); err != nil {
		r.log.Warn("Model validation failed", "model", modelID, "error", err)
		progressChan <- DownloadProgress{
			ModelID:       modelID,
			ExportStatus:  "warning",
			ExportMessage: fmt.Sprintf("Downloaded but validation failed: %v", err),
			Percent:       100,
			Complete:      true,
		}
	} else {
		progressChan <- DownloadProgress{
			ModelID:       modelID,
			ExportStatus:  "complete",
			ExportMessage: "Model ready to use",
			Percent:       100,
			Complete:      true,
		}
	}

	job.Status = "complete"
	job.Complete = true
	job.Percent = 100
	r.log.Info("Model download/export complete", "model", modelID)
}

// GetExportJob returns the status of an export job.
func (r *Registry) GetExportJob(jobID string) *ExportJob {
	r.exportJobsMu.RLock()
	defer r.exportJobsMu.RUnlock()
	if job, exists := r.exportJobs[jobID]; exists {
		copy := *job
		return &copy
	}
	return nil
}

// GetExportJobByModel returns the most recent export job for a model.
func (r *Registry) GetExportJobByModel(modelID string) *ExportJob {
	r.exportJobsMu.RLock()
	defer r.exportJobsMu.RUnlock()

	var latest *ExportJob
	for _, job := range r.exportJobs {
		if job.ModelID == modelID {
			if latest == nil || job.StartedAt.After(latest.StartedAt) {
				latest = job
			}
		}
	}
	if latest != nil {
		copy := *latest
		return &copy
	}
	return nil
}

// ListActiveExportJobs returns all active (non-complete) export jobs.
func (r *Registry) ListActiveExportJobs() []*ExportJob {
	r.exportJobsMu.RLock()
	defer r.exportJobsMu.RUnlock()

	var jobs []*ExportJob
	for _, job := range r.exportJobs {
		if !job.Complete && job.Status != "error" {
			copy := *job
			jobs = append(jobs, &copy)
		}
	}
	return jobs
}
