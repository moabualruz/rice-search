// Package client provides an HTTP client for the Rice Search API.
package client

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// Client is an HTTP client for the Rice Search API.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	connectionID string
}

// Config configures the client.
type Config struct {
	// BaseURL is the base URL of the API server.
	BaseURL string

	// Timeout is the request timeout.
	Timeout time.Duration

	// ConnectionID is an optional explicit connection ID.
	// If empty, one will be auto-generated from hostname/MAC.
	ConnectionID string

	// MaxIdleConns controls the maximum number of idle (keep-alive) connections
	// across all hosts. Zero means no limit.
	MaxIdleConns int

	// MaxConnsPerHost limits the total number of connections per host.
	// Zero means no limit.
	MaxConnsPerHost int

	// IdleConnTimeout is the maximum amount of time an idle (keep-alive)
	// connection will remain idle before closing itself.
	IdleConnTimeout time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:         "http://localhost:8080",
		Timeout:         30 * time.Second,
		MaxIdleConns:    100,
		MaxConnsPerHost: 100,
		IdleConnTimeout: 90 * time.Second,
	}
}

// GenerateConnectionID creates a stable, unique connection ID for this machine.
// It uses hostname + MAC address + OS/Arch to create a deterministic identifier.
func GenerateConnectionID() string {
	var parts []string

	// Hostname
	if hostname, err := os.Hostname(); err == nil {
		parts = append(parts, hostname)
	}

	// Primary MAC address
	if mac := getPrimaryMAC(); mac != "" {
		parts = append(parts, mac)
	}

	// OS and architecture for disambiguation
	parts = append(parts, runtime.GOOS, runtime.GOARCH)

	// Create a stable hash
	data := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(data))

	// Return first 16 characters of hex-encoded hash
	return hex.EncodeToString(hash[:8])
}

// getPrimaryMAC returns the MAC address of the first non-loopback interface.
func getPrimaryMAC() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		// Skip loopback and interfaces without MAC
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		// Skip virtual interfaces (common patterns)
		name := strings.ToLower(iface.Name)
		if strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "virbr") {
			continue
		}
		return iface.HardwareAddr.String()
	}

	return ""
}

// GetConnectionInfo returns detailed info about this connection for debugging.
func GetConnectionInfo() map[string]string {
	info := make(map[string]string)

	if hostname, err := os.Hostname(); err == nil {
		info["hostname"] = hostname
	}
	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["connection_id"] = GenerateConnectionID()

	if mac := getPrimaryMAC(); mac != "" {
		info["mac"] = mac
	}

	return info
}

// New creates a new API client.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 100
	}
	if cfg.MaxConnsPerHost == 0 {
		cfg.MaxConnsPerHost = 100
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}

	// Auto-generate connection ID if not provided
	connectionID := cfg.ConnectionID
	if connectionID == "" {
		connectionID = GenerateConnectionID()
	}

	// Configure explicit connection pooling for production tuning
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxConnsPerHost / 5, // 20% per host
		MaxConnsPerHost:     cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
	}

	return &Client{
		baseURL:      cfg.BaseURL,
		connectionID: connectionID,
		httpClient: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
	}
}

// ConnectionID returns the client's connection ID.
func (c *Client) ConnectionID() string {
	return c.connectionID
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
	// Add connection ID header to all requests
	if c.connectionID != "" {
		req.Header.Set("X-Connection-ID", c.connectionID)
	}

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
