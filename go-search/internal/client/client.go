// Package client provides an HTTP client for the Rice Search API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is an HTTP client for the Rice Search API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// Config configures the client.
type Config struct {
	// BaseURL is the base URL of the API server.
	BaseURL string

	// Timeout is the request timeout.
	Timeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL: "http://localhost:8080",
		Timeout: 30 * time.Second,
	}
}

// New creates a new API client.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &Client{
		baseURL: cfg.BaseURL,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Store represents a store.
type Store struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
}

// StoreStats represents store statistics.
type StoreStats struct {
	FileCount  int64 `json:"file_count"`
	ChunkCount int64 `json:"chunk_count"`
	TotalSize  int64 `json:"total_size"`
}

// IndexRequest represents a request to index files.
type IndexRequest struct {
	Files []IndexFile `json:"files"`
	Force bool        `json:"force,omitempty"`
}

// IndexFile represents a file to index.
type IndexFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// IndexResult represents the result of indexing.
type IndexResult struct {
	Store       string `json:"store"`
	Indexed     int    `json:"indexed"`
	Skipped     int    `json:"skipped"`
	Failed      int    `json:"failed"`
	ChunksTotal int    `json:"chunks_total"`
	DurationMs  int64  `json:"duration_ms"`
}

// SearchRequest represents a search request.
type SearchRequest struct {
	Query           string   `json:"query"`
	TopK            int      `json:"top_k,omitempty"`
	Filter          *Filter  `json:"filter,omitempty"`
	EnableReranking *bool    `json:"enable_reranking,omitempty"`
	IncludeContent  bool     `json:"include_content,omitempty"`
	SparseWeight    *float32 `json:"sparse_weight,omitempty"`
	DenseWeight     *float32 `json:"dense_weight,omitempty"`
}

// Filter defines search filters.
type Filter struct {
	PathPrefix string   `json:"path_prefix,omitempty"`
	Languages  []string `json:"languages,omitempty"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Language    string   `json:"language"`
	StartLine   int      `json:"start_line"`
	EndLine     int      `json:"end_line"`
	Content     string   `json:"content,omitempty"`
	Symbols     []string `json:"symbols,omitempty"`
	Score       float32  `json:"score"`
	RerankScore *float32 `json:"rerank_score,omitempty"`
}

// SearchResponse represents a search response.
type SearchResponse struct {
	Query    string         `json:"query"`
	Store    string         `json:"store"`
	Results  []SearchResult `json:"results"`
	Total    int            `json:"total"`
	Metadata SearchMetadata `json:"metadata"`
}

// SearchMetadata contains search timing information.
type SearchMetadata struct {
	SearchTimeMs       int64 `json:"search_time_ms"`
	EmbedTimeMs        int64 `json:"embed_time_ms"`
	RetrievalTimeMs    int64 `json:"retrieval_time_ms"`
	RerankTimeMs       int64 `json:"rerank_time_ms,omitempty"`
	CandidatesReranked int   `json:"candidates_reranked,omitempty"`
	RerankingApplied   bool  `json:"reranking_applied"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// APIError represents an API error response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Health checks if the API is healthy.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	var resp HealthResponse
	if err := c.get(ctx, "/healthz", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListStores returns all stores.
func (c *Client) ListStores(ctx context.Context) ([]Store, error) {
	var resp struct {
		Stores []Store `json:"stores"`
	}
	if err := c.get(ctx, "/v1/stores", &resp); err != nil {
		return nil, err
	}
	return resp.Stores, nil
}

// GetStore returns a store by name.
func (c *Client) GetStore(ctx context.Context, name string) (*Store, error) {
	var store Store
	if err := c.get(ctx, fmt.Sprintf("/v1/stores/%s", name), &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// CreateStore creates a new store.
func (c *Client) CreateStore(ctx context.Context, name, description string) (*Store, error) {
	req := map[string]string{
		"name":        name,
		"description": description,
	}
	var store Store
	if err := c.post(ctx, "/v1/stores", req, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

// DeleteStore deletes a store.
func (c *Client) DeleteStore(ctx context.Context, name string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/stores/%s", name))
}

// GetStoreStats returns statistics for a store.
func (c *Client) GetStoreStats(ctx context.Context, name string) (*StoreStats, error) {
	var stats StoreStats
	if err := c.get(ctx, fmt.Sprintf("/v1/stores/%s/stats", name), &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// Index indexes files into a store.
func (c *Client) Index(ctx context.Context, store string, req IndexRequest) (*IndexResult, error) {
	var result IndexResult
	if err := c.post(ctx, fmt.Sprintf("/v1/stores/%s/index", store), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Search performs a search.
func (c *Client) Search(ctx context.Context, store string, req SearchRequest) (*SearchResponse, error) {
	var resp SearchResponse
	if err := c.post(ctx, fmt.Sprintf("/v1/stores/%s/search", store), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// get performs a GET request.
func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	return c.do(req, result)
}

// post performs a POST request.
func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.do(req, result)
}

// delete performs a DELETE request.
func (c *Client) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	return c.do(req, nil)
}

// do executes a request.
func (c *Client) do(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		return &apiErr
	}

	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}
