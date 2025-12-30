// Package models provides ML model management and mapping for Rice Search.
package models

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HuggingFaceClient provides access to the HuggingFace Hub API.
type HuggingFaceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewHuggingFaceClient creates a new HuggingFace Hub client.
func NewHuggingFaceClient() *HuggingFaceClient {
	return &HuggingFaceClient{
		baseURL: "https://huggingface.co",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// HFModelInfo represents a model from the HuggingFace API.
type HFModelInfo struct {
	ID           string   `json:"id"`
	ModelID      string   `json:"modelId"`
	Author       string   `json:"author"`
	Downloads    int64    `json:"downloads"`
	Likes        int      `json:"likes"`
	Tags         []string `json:"tags"`
	PipelineTag  string   `json:"pipeline_tag"`
	LibraryName  string   `json:"library_name"`
	Private      bool     `json:"private"`
	LastModified string   `json:"lastModified"`
	Siblings     []HFFile `json:"siblings,omitempty"`
}

// HFFile represents a file in a HuggingFace model repository.
type HFFile struct {
	RFilename string `json:"rfilename"`
	Size      int64  `json:"size,omitempty"`
	LFS       *struct {
		Size int64 `json:"size"`
	} `json:"lfs,omitempty"`
}

// HFTreeEntry represents a file or directory in the model tree.
type HFTreeEntry struct {
	Type string `json:"type"` // "file" or "directory"
	Path string `json:"path"`
	Size int64  `json:"size"`
	OID  string `json:"oid"`
	LFS  *struct {
		OID  string `json:"oid"`
		Size int64  `json:"size"`
	} `json:"lfs,omitempty"`
}

// HasONNX checks if the model has ONNX files.
func (m *HFModelInfo) HasONNX() bool {
	for _, tag := range m.Tags {
		if tag == "onnx" {
			return true
		}
	}
	return false
}

// SearchModelsRequest contains parameters for searching models.
type SearchModelsRequest struct {
	// Filter by tags (comma-separated, e.g., "onnx,sentence-similarity")
	Filter string
	// Text search query
	Search string
	// Filter by pipeline tag (e.g., "sentence-similarity", "text-classification")
	PipelineTag string
	// Maximum number of results
	Limit int
	// Sort field
	Sort string
	// Sort direction (-1 for descending)
	Direction int
}

// SearchModels searches for models on HuggingFace Hub.
func (c *HuggingFaceClient) SearchModels(ctx context.Context, req SearchModelsRequest) ([]HFModelInfo, error) {
	params := url.Values{}

	if req.Filter != "" {
		params.Set("filter", req.Filter)
	}
	if req.Search != "" {
		params.Set("search", req.Search)
	}
	if req.PipelineTag != "" {
		params.Set("pipeline_tag", req.PipelineTag)
	}
	if req.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", req.Limit))
	}
	if req.Sort != "" {
		params.Set("sort", req.Sort)
	}
	if req.Direction != 0 {
		params.Set("direction", fmt.Sprintf("%d", req.Direction))
	}

	apiURL := fmt.Sprintf("%s/api/models?%s", c.baseURL, params.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to search models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status %d", resp.StatusCode)
	}

	var models []HFModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return models, nil
}

// SearchONNXModels searches for models with ONNX support for a specific task.
func (c *HuggingFaceClient) SearchONNXModels(ctx context.Context, modelType ModelType, limit int) ([]HFModelInfo, error) {
	var filter string
	var pipelineTag string

	switch modelType {
	case ModelTypeEmbed:
		filter = "onnx,sentence-similarity"
		pipelineTag = "sentence-similarity"
	case ModelTypeRerank:
		filter = "onnx"
		pipelineTag = "text-classification"
	case ModelTypeQueryUnderstand:
		filter = "onnx"
		pipelineTag = "text-classification"
	default:
		filter = "onnx"
	}

	return c.SearchModels(ctx, SearchModelsRequest{
		Filter:      filter,
		PipelineTag: pipelineTag,
		Limit:       limit,
		Sort:        "downloads",
		Direction:   -1, // Descending
	})
}

// GetModelInfo gets detailed information about a specific model.
func (c *HuggingFaceClient) GetModelInfo(ctx context.Context, modelID string) (*HFModelInfo, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s", c.baseURL, modelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get model info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed with status %d", resp.StatusCode)
	}

	var model HFModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &model, nil
}

// ListModelFiles lists files in a model repository.
func (c *HuggingFaceClient) ListModelFiles(ctx context.Context, modelID string, path string) ([]HFTreeEntry, error) {
	apiURL := fmt.Sprintf("%s/api/models/%s/tree/main", c.baseURL, modelID)
	if path != "" {
		apiURL = fmt.Sprintf("%s/%s", apiURL, path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed with status %d", resp.StatusCode)
	}

	var entries []HFTreeEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return entries, nil
}

// FindONNXFile finds the ONNX model file in a repository.
// Returns the path to the ONNX file and its size, or an error if not found.
func (c *HuggingFaceClient) FindONNXFile(ctx context.Context, modelID string) (path string, size int64, err error) {
	// First check the onnx/ subdirectory (most common)
	entries, err := c.ListModelFiles(ctx, modelID, "onnx")
	if err == nil {
		for _, entry := range entries {
			if entry.Type == "file" && strings.HasSuffix(entry.Path, "model.onnx") {
				size := entry.Size
				if entry.LFS != nil {
					size = entry.LFS.Size
				}
				return entry.Path, size, nil
			}
		}
	}

	// Check root directory
	entries, err = c.ListModelFiles(ctx, modelID, "")
	if err != nil {
		return "", 0, fmt.Errorf("failed to list files: %w", err)
	}

	for _, entry := range entries {
		if entry.Type == "file" && entry.Path == "model.onnx" {
			size := entry.Size
			if entry.LFS != nil {
				size = entry.LFS.Size
			}
			return entry.Path, size, nil
		}
	}

	return "", 0, fmt.Errorf("no ONNX file found in model %s", modelID)
}

// GetFileDownloadURL returns the download URL for a file in a model repository.
func (c *HuggingFaceClient) GetFileDownloadURL(modelID, filePath string) string {
	return fmt.Sprintf("%s/%s/resolve/main/%s", c.baseURL, modelID, filePath)
}

// DownloadFile downloads a file from HuggingFace.
func (c *HuggingFaceClient) DownloadFile(ctx context.Context, modelID, filePath string, w io.Writer, onProgress func(downloaded, total int64)) error {
	downloadURL := c.GetFileDownloadURL(modelID, filePath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			written, werr := w.Write(buf[:n])
			if werr != nil {
				return fmt.Errorf("failed to write: %w", werr)
			}
			downloaded += int64(written)
			if onProgress != nil {
				onProgress(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read: %w", err)
		}
	}

	return nil
}

// ToModelInfo converts HuggingFace model info to our ModelInfo type.
func (m *HFModelInfo) ToModelInfo(modelType ModelType) *ModelInfo {
	// Try to extract output dimensions for embedding models from tags
	outputDim := 384 // Default
	for _, tag := range m.Tags {
		// Some models have dimension in tags like "768d" or similar
		if strings.HasSuffix(tag, "d") {
			var dim int
			if _, err := fmt.Sscanf(tag, "%dd", &dim); err == nil && dim > 0 {
				outputDim = dim
			}
		}
	}

	// Use default dimensions based on known models
	switch {
	case strings.Contains(m.ID, "MiniLM-L6"):
		outputDim = 384
	case strings.Contains(m.ID, "bge-small"):
		outputDim = 384
	case strings.Contains(m.ID, "bge-base"):
		outputDim = 768
	case strings.Contains(m.ID, "bge-large"):
		outputDim = 1024
	case strings.Contains(m.ID, "bge-m3"):
		outputDim = 1024
	case strings.Contains(m.ID, "e5-small"):
		outputDim = 384
	case strings.Contains(m.ID, "e5-base"):
		outputDim = 768
	case strings.Contains(m.ID, "e5-large"):
		outputDim = 1024
	}

	// Estimate max tokens
	maxTokens := 512
	for _, tag := range m.Tags {
		if strings.Contains(tag, "8192") || strings.Contains(m.ID, "8k") {
			maxTokens = 8192
		} else if strings.Contains(tag, "4096") || strings.Contains(m.ID, "4k") {
			maxTokens = 4096
		}
	}

	return &ModelInfo{
		ID:          m.ID,
		Type:        modelType,
		DisplayName: m.ID, // Will be overridden
		Description: fmt.Sprintf("Downloaded from HuggingFace Hub (%d downloads)", m.Downloads),
		OutputDim:   outputDim,
		MaxTokens:   maxTokens,
		Downloaded:  false,
		IsDefault:   false,
		GPUEnabled:  true,
		Size:        0, // Will be set after finding ONNX file
		DownloadURL: fmt.Sprintf("https://huggingface.co/%s", m.ID),
	}
}
