// Package grpcclient provides a gRPC client for connecting to the Rice Search server.
package grpcclient

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ricesearch/rice-search/api/proto/ricesearchpb"
)

// Config holds the client configuration.
type Config struct {
	// ServerAddress is the server address.
	// Supports:
	//   - "localhost:50051" (TCP)
	//   - "unix:///tmp/rice-search.sock" (Unix socket)
	//   - "auto" (try Unix socket first, fall back to TCP)
	ServerAddress string

	// UnixSocketPath is the default Unix socket path for auto-detection.
	UnixSocketPath string

	// TCPAddress is the default TCP address for auto-detection.
	TCPAddress string

	// Timeout is the connection timeout.
	Timeout time.Duration

	// MaxRetries is the maximum number of connection retries.
	MaxRetries int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		ServerAddress:  "auto",
		UnixSocketPath: "/tmp/rice-search.sock",
		TCPAddress:     "localhost:50051",
		Timeout:        10 * time.Second,
		MaxRetries:     3,
	}
}

// Client is a gRPC client for Rice Search.
type Client struct {
	cfg    Config
	conn   *grpc.ClientConn
	client pb.RiceSearchClient
}

// New creates a new gRPC client.
func New(cfg Config) (*Client, error) {
	if cfg.ServerAddress == "" {
		cfg = DefaultConfig()
	}

	addr := cfg.resolveAddress()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(100*1024*1024), // 100MB
			grpc.MaxCallSendMsgSize(100*1024*1024), // 100MB
		),
	}

	// Add Unix socket dialer if needed
	if strings.HasPrefix(addr, "unix://") {
		socketPath := strings.TrimPrefix(addr, "unix://")
		opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", socketPath, cfg.Timeout)
		}))
		addr = socketPath // grpc.Dial expects the path without scheme for custom dialer
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Use grpc.NewClient (DialContext is deprecated)
	opts = append(opts, grpc.WithBlock())
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// Wait for connection
	conn.Connect()
	select {
	case <-ctx.Done():
		conn.Close()
		return nil, fmt.Errorf("connection timeout to %s", addr)
	default:
	}

	return &Client{
		cfg:    cfg,
		conn:   conn,
		client: pb.NewRiceSearchClient(conn),
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// resolveAddress resolves the server address based on configuration.
func (cfg *Config) resolveAddress() string {
	if cfg.ServerAddress != "auto" {
		return cfg.ServerAddress
	}

	// Auto-detect: try Unix socket first (non-Windows only)
	if runtime.GOOS != "windows" && cfg.UnixSocketPath != "" {
		if _, err := os.Stat(cfg.UnixSocketPath); err == nil {
			return "unix://" + cfg.UnixSocketPath
		}
	}

	return cfg.TCPAddress
}

// =============================================================================
// Search Methods
// =============================================================================

// SearchOptions holds options for search.
type SearchOptions struct {
	TopK            int
	EnableReranking *bool
	RerankTopK      int
	IncludeContent  bool
	PathPrefix      string
	Languages       []string
	SparseWeight    *float32
	DenseWeight     *float32
}

// SearchResult represents a search result.
type SearchResult struct {
	ID          string
	Path        string
	Language    string
	StartLine   int
	EndLine     int
	Content     string
	Symbols     []string
	Score       float32
	RerankScore *float32
}

// SearchResponse represents a search response.
type SearchResponse struct {
	Query              string
	Store              string
	Results            []SearchResult
	Total              int
	SearchTimeMs       int64
	EmbedTimeMs        int64
	RetrievalTimeMs    int64
	RerankTimeMs       int64
	CandidatesReranked int
	RerankingApplied   bool
}

// Search performs a search.
func (c *Client) Search(ctx context.Context, store, query string, opts SearchOptions) (*SearchResponse, error) {
	req := &pb.SearchRequest{
		Query:           query,
		Store:           store,
		TopK:            int32(opts.TopK),
		IncludeContent:  opts.IncludeContent,
		EnableReranking: opts.EnableReranking,
		RerankTopK:      int32(opts.RerankTopK),
		SparseWeight:    opts.SparseWeight,
		DenseWeight:     opts.DenseWeight,
	}

	if opts.PathPrefix != "" || len(opts.Languages) > 0 {
		req.Filter = &pb.SearchFilter{
			PathPrefix: opts.PathPrefix,
			Languages:  opts.Languages,
		}
	}

	resp, err := c.client.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = SearchResult{
			ID:          r.Id,
			Path:        r.Path,
			Language:    r.Language,
			StartLine:   int(r.StartLine),
			EndLine:     int(r.EndLine),
			Content:     r.Content,
			Symbols:     r.Symbols,
			Score:       r.Score,
			RerankScore: r.RerankScore,
		}
	}

	return &SearchResponse{
		Query:              resp.Query,
		Store:              resp.Store,
		Results:            results,
		Total:              int(resp.Total),
		SearchTimeMs:       resp.Metadata.SearchTimeMs,
		EmbedTimeMs:        resp.Metadata.EmbedTimeMs,
		RetrievalTimeMs:    resp.Metadata.RetrievalTimeMs,
		RerankTimeMs:       resp.Metadata.RerankTimeMs,
		CandidatesReranked: int(resp.Metadata.CandidatesReranked),
		RerankingApplied:   resp.Metadata.RerankingApplied,
	}, nil
}

// =============================================================================
// Index Methods
// =============================================================================

// IndexDocument represents a document to index.
type IndexDocument struct {
	Path     string
	Content  string
	Language string
}

// IndexResult represents the result of an indexing operation.
type IndexResult struct {
	Store       string
	Indexed     int
	Skipped     int
	Failed      int
	ChunksTotal int
	Duration    time.Duration
	Errors      []IndexError
}

// IndexError represents an indexing error.
type IndexError struct {
	Path    string
	Message string
}

// Index indexes documents.
func (c *Client) Index(ctx context.Context, store string, docs []IndexDocument, force bool) (*IndexResult, error) {
	pbDocs := make([]*pb.Document, len(docs))
	for i, d := range docs {
		pbDocs[i] = &pb.Document{
			Path:     d.Path,
			Content:  d.Content,
			Language: d.Language,
		}
	}

	resp, err := c.client.IndexBatch(ctx, &pb.IndexBatchRequest{
		Store:     store,
		Documents: pbDocs,
		Force:     force,
	}, grpc.MaxCallRecvMsgSize(100*1024*1024), grpc.MaxCallSendMsgSize(100*1024*1024))
	if err != nil {
		return nil, err
	}

	errors := make([]IndexError, len(resp.Errors))
	for i, e := range resp.Errors {
		errors[i] = IndexError{
			Path:    e.Path,
			Message: e.Message,
		}
	}

	return &IndexResult{
		Store:       resp.Store,
		Indexed:     int(resp.Indexed),
		Skipped:     int(resp.Skipped),
		Failed:      int(resp.Failed),
		ChunksTotal: int(resp.ChunksTotal),
		Duration:    resp.GetDuration().AsDuration(),
		Errors:      errors,
	}, nil
}

// Delete removes documents from the index.
func (c *Client) Delete(ctx context.Context, store string, paths []string) (int, error) {
	resp, err := c.client.Delete(ctx, &pb.DeleteRequest{
		Store: store,
		Paths: paths,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Deleted), nil
}

// DeleteByPrefix removes documents by path prefix.
func (c *Client) DeleteByPrefix(ctx context.Context, store, prefix string) error {
	_, err := c.client.Delete(ctx, &pb.DeleteRequest{
		Store:      store,
		PathPrefix: prefix,
	})
	return err
}

// Sync removes documents that no longer exist.
func (c *Client) Sync(ctx context.Context, store string, currentPaths []string) (int, error) {
	resp, err := c.client.Sync(ctx, &pb.SyncRequest{
		Store:        store,
		CurrentPaths: currentPaths,
	})
	if err != nil {
		return 0, err
	}
	return int(resp.Removed), nil
}

// IndexStats represents index statistics.
type IndexStats struct {
	Store       string
	TotalChunks int
	TotalFiles  int
}

// GetIndexStats returns index statistics.
func (c *Client) GetIndexStats(ctx context.Context, store string) (*IndexStats, error) {
	resp, err := c.client.GetIndexStats(ctx, &pb.GetIndexStatsRequest{
		Store: store,
	})
	if err != nil {
		return nil, err
	}

	return &IndexStats{
		Store:       resp.Store,
		TotalChunks: int(resp.TotalChunks),
		TotalFiles:  int(resp.TotalFiles),
	}, nil
}

// =============================================================================
// Store Methods
// =============================================================================

// Store represents a search store.
type Store struct {
	Name        string
	DisplayName string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// StoreStats represents store statistics.
type StoreStats struct {
	DocumentCount int64
	ChunkCount    int64
	TotalSize     int64
	LastIndexed   time.Time
}

// ListStores returns all stores.
func (c *Client) ListStores(ctx context.Context) ([]Store, error) {
	resp, err := c.client.ListStores(ctx, &pb.ListStoresRequest{})
	if err != nil {
		return nil, err
	}

	stores := make([]Store, len(resp.Stores))
	for i, s := range resp.Stores {
		stores[i] = Store{
			Name:        s.Name,
			DisplayName: s.DisplayName,
			Description: s.Description,
			CreatedAt:   s.GetCreatedAt().AsTime(),
			UpdatedAt:   s.GetUpdatedAt().AsTime(),
		}
	}

	return stores, nil
}

// CreateStore creates a new store.
func (c *Client) CreateStore(ctx context.Context, name, description string) (*Store, error) {
	resp, err := c.client.CreateStore(ctx, &pb.CreateStoreRequest{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}

	return &Store{
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		CreatedAt:   resp.GetCreatedAt().AsTime(),
		UpdatedAt:   resp.GetUpdatedAt().AsTime(),
	}, nil
}

// GetStore retrieves a store by name.
func (c *Client) GetStore(ctx context.Context, name string) (*Store, error) {
	resp, err := c.client.GetStore(ctx, &pb.GetStoreRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}

	return &Store{
		Name:        resp.Name,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		CreatedAt:   resp.GetCreatedAt().AsTime(),
		UpdatedAt:   resp.GetUpdatedAt().AsTime(),
	}, nil
}

// DeleteStore deletes a store.
func (c *Client) DeleteStore(ctx context.Context, name string) error {
	_, err := c.client.DeleteStore(ctx, &pb.DeleteStoreRequest{
		Name: name,
	})
	return err
}

// GetStoreStats returns store statistics.
func (c *Client) GetStoreStats(ctx context.Context, name string) (*StoreStats, error) {
	resp, err := c.client.GetStoreStats(ctx, &pb.GetStoreStatsRequest{
		Name: name,
	})
	if err != nil {
		return nil, err
	}

	return &StoreStats{
		DocumentCount: resp.DocumentCount,
		ChunkCount:    resp.ChunkCount,
		TotalSize:     resp.TotalSize,
		LastIndexed:   resp.GetLastIndexed().AsTime(),
	}, nil
}

// =============================================================================
// Health & Version Methods
// =============================================================================

// HealthStatus represents the health status.
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusDegraded
	HealthStatusUnhealthy
)

func (s HealthStatus) String() string {
	switch s {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// ComponentHealth represents the health of a component.
type ComponentHealth struct {
	Status  HealthStatus
	Message string
	Latency time.Duration
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status     HealthStatus
	Components map[string]ComponentHealth
	Version    string
}

// Health checks the server health.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	resp, err := c.client.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		return nil, err
	}

	components := make(map[string]ComponentHealth)
	for name, comp := range resp.Components {
		components[name] = ComponentHealth{
			Status:  pbStatusToStatus(comp.Status),
			Message: comp.Message,
			Latency: comp.GetLatency().AsDuration(),
		}
	}

	return &HealthResponse{
		Status:     pbStatusToStatus(resp.Status),
		Components: components,
		Version:    resp.Version,
	}, nil
}

// VersionInfo represents version information.
type VersionInfo struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
}

// Version returns server version information.
func (c *Client) Version(ctx context.Context) (*VersionInfo, error) {
	resp, err := c.client.Version(ctx, &pb.VersionRequest{})
	if err != nil {
		return nil, err
	}

	return &VersionInfo{
		Version:   resp.Version,
		Commit:    resp.Commit,
		BuildDate: resp.BuildDate,
		GoVersion: resp.GoVersion,
	}, nil
}

func pbStatusToStatus(s pb.HealthStatus) HealthStatus {
	switch s {
	case pb.HealthStatus_HEALTH_STATUS_HEALTHY:
		return HealthStatusHealthy
	case pb.HealthStatus_HEALTH_STATUS_DEGRADED:
		return HealthStatusDegraded
	case pb.HealthStatus_HEALTH_STATUS_UNHEALTHY:
		return HealthStatusUnhealthy
	default:
		return HealthStatusUnknown
	}
}

// =============================================================================
// Model Management Methods
// =============================================================================

// ModelInfo represents information about a model.
type ModelInfo struct {
	ID          string
	Type        string
	DisplayName string
	Description string
	OutputDim   int
	MaxTokens   int
	Downloaded  bool
	IsDefault   bool
	GPUEnabled  bool
	Size        int64
}

// ListModels returns all available models.
func (c *Client) ListModels(ctx context.Context, typeFilter string) ([]ModelInfo, error) {
	resp, err := c.client.ListModels(ctx, &pb.ListModelsRequest{
		TypeFilter: typeFilter,
	})
	if err != nil {
		return nil, err
	}

	models := make([]ModelInfo, len(resp.Models))
	for i, m := range resp.Models {
		models[i] = ModelInfo{
			ID:          m.Id,
			Type:        m.Type,
			DisplayName: m.DisplayName,
			Description: m.Description,
			OutputDim:   int(m.OutputDim),
			MaxTokens:   int(m.MaxTokens),
			Downloaded:  m.Downloaded,
			IsDefault:   m.IsDefault,
			GPUEnabled:  m.GpuEnabled,
			Size:        m.Size,
		}
	}

	return models, nil
}

// DownloadModel downloads a specific model.
func (c *Client) DownloadModel(ctx context.Context, modelID string) (*DownloadModelResult, error) {
	resp, err := c.client.DownloadModel(ctx, &pb.DownloadModelRequest{
		ModelId: modelID,
	})
	if err != nil {
		return nil, err
	}

	return &DownloadModelResult{
		Success:  resp.Success,
		Message:  resp.Message,
		Progress: int(resp.Progress),
	}, nil
}

// DownloadModelResult represents the result of a model download.
type DownloadModelResult struct {
	Success  bool
	Message  string
	Progress int
}

// SetDefaultModel sets the default model for a model type.
func (c *Client) SetDefaultModel(ctx context.Context, modelType, modelID string) (*ModelInfo, error) {
	resp, err := c.client.SetDefaultModel(ctx, &pb.SetDefaultModelRequest{
		ModelType: modelType,
		ModelId:   modelID,
	})
	if err != nil {
		return nil, err
	}

	return &ModelInfo{
		ID:          resp.Id,
		Type:        resp.Type,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		OutputDim:   int(resp.OutputDim),
		MaxTokens:   int(resp.MaxTokens),
		Downloaded:  resp.Downloaded,
		IsDefault:   resp.IsDefault,
		GPUEnabled:  resp.GpuEnabled,
		Size:        resp.Size,
	}, nil
}

// ToggleGPU toggles GPU acceleration for a model.
func (c *Client) ToggleGPU(ctx context.Context, modelID string, enabled bool) (*ModelInfo, error) {
	resp, err := c.client.ToggleGPU(ctx, &pb.ToggleGPURequest{
		ModelId: modelID,
		Enabled: enabled,
	})
	if err != nil {
		return nil, err
	}

	return &ModelInfo{
		ID:          resp.Id,
		Type:        resp.Type,
		DisplayName: resp.DisplayName,
		Description: resp.Description,
		OutputDim:   int(resp.OutputDim),
		MaxTokens:   int(resp.MaxTokens),
		Downloaded:  resp.Downloaded,
		IsDefault:   resp.IsDefault,
		GPUEnabled:  resp.GpuEnabled,
		Size:        resp.Size,
	}, nil
}
