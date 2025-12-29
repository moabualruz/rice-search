package ml

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// ModelInfo describes a required model.
type ModelInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // embedder, sparse, reranker
	HuggingFace string   `json:"huggingface"`
	Files       []string `json:"files"`
	Size        string   `json:"size"`
	SHA256      string   `json:"sha256,omitempty"` // checksum of model.onnx
}

// ModelManifest contains all required models.
type ModelManifest struct {
	Version string      `json:"version"`
	Models  []ModelInfo `json:"models"`
}

// DefaultManifest returns the default model manifest.
func DefaultManifest() ModelManifest {
	return ModelManifest{
		Version: "1.0.0",
		Models: []ModelInfo{
			{
				Name:        "jina-embeddings-v3",
				Description: "Jina Embeddings v3 - 1536 dimensions, code-optimized",
				Type:        "embedder",
				HuggingFace: "jinaai/jina-embeddings-v3",
				Files:       []string{"model.onnx", "tokenizer.json", "config.json"},
				Size:        "~1.2GB",
			},
			{
				Name:        "splade-v3",
				Description: "SPLADE v3 - Sparse lexical embeddings",
				Type:        "sparse",
				HuggingFace: "naver/splade-v3",
				Files:       []string{"model.onnx", "tokenizer.json", "config.json"},
				Size:        "~500MB",
			},
			{
				Name:        "jina-reranker-v2",
				Description: "Jina Reranker v2 - Cross-encoder for reranking",
				Type:        "reranker",
				HuggingFace: "jinaai/jina-reranker-v2-base-multilingual",
				Files:       []string{"model.onnx", "tokenizer.json", "config.json"},
				Size:        "~800MB",
			},
		},
	}
}

// ModelManager handles model discovery and downloading.
type ModelManager struct {
	modelsDir string
	manifest  ModelManifest
	log       *logger.Logger
	client    *http.Client
}

// NewModelManager creates a new model manager.
func NewModelManager(modelsDir string, log *logger.Logger) *ModelManager {
	return &ModelManager{
		modelsDir: modelsDir,
		manifest:  DefaultManifest(),
		log:       log,
		client:    &http.Client{},
	}
}

// ModelsDir returns the models directory.
func (m *ModelManager) ModelsDir() string {
	return m.modelsDir
}

// ListModels returns all models in the manifest.
func (m *ModelManager) ListModels() []ModelInfo {
	return m.manifest.Models
}

// GetModel returns info for a specific model.
func (m *ModelManager) GetModel(name string) (ModelInfo, bool) {
	for _, model := range m.manifest.Models {
		if model.Name == name {
			return model, true
		}
	}
	return ModelInfo{}, false
}

// ModelStatus represents the status of a model.
type ModelStatus struct {
	Name      string   `json:"name"`
	Installed bool     `json:"installed"`
	Path      string   `json:"path,omitempty"`
	Missing   []string `json:"missing,omitempty"`
}

// CheckModel checks if a model is installed.
func (m *ModelManager) CheckModel(name string) ModelStatus {
	model, ok := m.GetModel(name)
	if !ok {
		return ModelStatus{Name: name, Installed: false}
	}

	modelDir := filepath.Join(m.modelsDir, name)
	status := ModelStatus{
		Name: name,
		Path: modelDir,
	}

	// Check for required files
	for _, file := range model.Files {
		filePath := filepath.Join(modelDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			status.Missing = append(status.Missing, file)
		}
	}

	status.Installed = len(status.Missing) == 0
	return status
}

// CheckAllModels checks all models in the manifest.
func (m *ModelManager) CheckAllModels() []ModelStatus {
	statuses := make([]ModelStatus, len(m.manifest.Models))
	for i, model := range m.manifest.Models {
		statuses[i] = m.CheckModel(model.Name)
	}
	return statuses
}

// DownloadProgress reports download progress.
type DownloadProgress struct {
	Model      string  `json:"model"`
	File       string  `json:"file"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Complete   bool    `json:"complete"`
	Error      string  `json:"error,omitempty"`
}

// ProgressCallback is called with download progress updates.
type ProgressCallback func(DownloadProgress)

// DownloadModel downloads a model from HuggingFace.
func (m *ModelManager) DownloadModel(name string, progress ProgressCallback) error {
	model, ok := m.GetModel(name)
	if !ok {
		return errors.NotFoundError(fmt.Sprintf("model: %s", name))
	}

	modelDir := filepath.Join(m.modelsDir, name)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to create model directory", err)
	}

	m.log.Info("Downloading model", "name", name, "from", model.HuggingFace)

	for _, file := range model.Files {
		if err := m.downloadFile(model, file, modelDir, progress); err != nil {
			return err
		}
	}

	if progress != nil {
		progress(DownloadProgress{
			Model:    name,
			Complete: true,
			Percent:  100,
		})
	}

	return nil
}

func (m *ModelManager) downloadFile(model ModelInfo, file, destDir string, progress ProgressCallback) error {
	destPath := filepath.Join(destDir, file)

	// Skip if already exists
	if _, err := os.Stat(destPath); err == nil {
		m.log.Debug("File already exists, skipping", "file", file)
		return nil
	}

	// Construct HuggingFace URL
	// Format: https://huggingface.co/{repo}/resolve/main/{file}
	url := fmt.Sprintf("https://huggingface.co/%s/resolve/main/%s", model.HuggingFace, file)

	m.log.Info("Downloading file", "file", file, "url", url)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to create request", err)
	}

	// Execute request
	resp, err := m.client.Do(req)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "download failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(errors.CodeInternal, fmt.Sprintf("download failed: HTTP %d", resp.StatusCode))
	}

	// Create destination file
	out, err := os.Create(destPath)
	if err != nil {
		return errors.Wrap(errors.CodeInternal, "failed to create file", err)
	}
	defer out.Close()

	// Copy with progress
	total := resp.ContentLength
	var downloaded int64

	reader := &progressReader{
		reader: resp.Body,
		callback: func(n int64) {
			downloaded += n
			if progress != nil {
				percent := float64(0)
				if total > 0 {
					percent = float64(downloaded) / float64(total) * 100
				}
				progress(DownloadProgress{
					Model:      model.Name,
					File:       file,
					Downloaded: downloaded,
					Total:      total,
					Percent:    percent,
				})
			}
		},
	}

	if _, err := io.Copy(out, reader); err != nil {
		os.Remove(destPath) // Clean up partial file
		return errors.Wrap(errors.CodeInternal, "download failed", err)
	}

	m.log.Info("Downloaded file", "file", file, "size", downloaded)
	return nil
}

// DownloadAllModels downloads all models in the manifest.
func (m *ModelManager) DownloadAllModels(progress ProgressCallback) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(m.manifest.Models))

	for _, model := range m.manifest.Models {
		// Check if already installed
		status := m.CheckModel(model.Name)
		if status.Installed {
			m.log.Info("Model already installed, skipping", "name", model.Name)
			continue
		}

		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			if err := m.DownloadModel(name, progress); err != nil {
				errChan <- fmt.Errorf("%s: %w", name, err)
			}
		}(model.Name)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return errors.New(errors.CodeInternal, "some downloads failed: "+strings.Join(errs, "; "))
	}

	return nil
}

// VerifyModel verifies a model's checksum.
func (m *ModelManager) VerifyModel(name string) (bool, error) {
	model, ok := m.GetModel(name)
	if !ok {
		return false, errors.NotFoundError(fmt.Sprintf("model: %s", name))
	}

	if model.SHA256 == "" {
		// No checksum to verify
		return true, nil
	}

	modelPath := filepath.Join(m.modelsDir, name, "model.onnx")
	checksum, err := fileChecksum(modelPath)
	if err != nil {
		return false, err
	}

	return checksum == model.SHA256, nil
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// progressReader wraps an io.Reader to report progress.
type progressReader struct {
	reader   io.Reader
	callback func(int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.callback != nil {
		r.callback(int64(n))
	}
	return n, err
}

// SaveManifest saves the manifest to a file.
func (m *ModelManager) SaveManifest(path string) error {
	data, err := json.MarshalIndent(m.manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadManifest loads a manifest from a file.
func (m *ModelManager) LoadManifest(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &m.manifest)
}
