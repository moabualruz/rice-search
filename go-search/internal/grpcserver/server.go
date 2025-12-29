// Package grpcserver provides the gRPC server implementation for Rice Search.
package grpcserver

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/ricesearch/rice-search/api/proto/ricesearchpb"
	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/store"
)

// Config holds the gRPC server configuration.
type Config struct {
	// TCPAddr is the TCP address to listen on (e.g., ":50051").
	TCPAddr string

	// UnixSocketPath is the Unix socket path for local connections.
	// Empty string disables Unix socket listening.
	UnixSocketPath string

	// Version is the server version.
	Version string

	// Commit is the git commit hash.
	Commit string

	// BuildDate is the build date.
	BuildDate string

	// MaxRecvMsgSize is the maximum message size in bytes (default: 16MB).
	MaxRecvMsgSize int

	// MaxSendMsgSize is the maximum message size in bytes (default: 16MB).
	MaxSendMsgSize int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		TCPAddr:        ":50051",
		UnixSocketPath: "", // Disabled by default on Windows
		Version:        "dev",
		Commit:         "none",
		BuildDate:      "unknown",
		MaxRecvMsgSize: 16 * 1024 * 1024, // 16MB
		MaxSendMsgSize: 16 * 1024 * 1024, // 16MB
	}
}

// Server is the gRPC server that implements the RiceSearch service.
type Server struct {
	pb.UnimplementedRiceSearchServer

	cfg        Config
	log        *logger.Logger
	grpcServer *grpc.Server

	// Services
	ml        ml.Service
	qdrant    *qdrant.Client
	storeSvc  *store.Service
	indexSvc  *index.Pipeline
	searchSvc *search.Service

	// Listeners
	tcpListener  net.Listener
	unixListener net.Listener
}

// New creates a new gRPC server.
func New(cfg Config, log *logger.Logger, mlSvc ml.Service, qc *qdrant.Client, storeSvc *store.Service, indexSvc *index.Pipeline, searchSvc *search.Service) *Server {
	if cfg.TCPAddr == "" {
		cfg = DefaultConfig()
	}

	return &Server{
		cfg:       cfg,
		log:       log,
		ml:        mlSvc,
		qdrant:    qc,
		storeSvc:  storeSvc,
		indexSvc:  indexSvc,
		searchSvc: searchSvc,
	}
}

// Start starts the gRPC server on both TCP and Unix socket (if configured).
func (s *Server) Start() error {
	// Create gRPC server with options
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(s.cfg.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(s.cfg.MaxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  10 * time.Second,
			Timeout:               3 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	s.grpcServer = grpc.NewServer(opts...)
	pb.RegisterRiceSearchServer(s.grpcServer, s)

	// Start TCP listener
	tcpLis, err := net.Listen("tcp", s.cfg.TCPAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on TCP %s: %w", s.cfg.TCPAddr, err)
	}
	s.tcpListener = tcpLis
	s.log.Info("gRPC server listening on TCP", "addr", s.cfg.TCPAddr)

	go func() {
		if err := s.grpcServer.Serve(tcpLis); err != nil {
			s.log.Error("TCP server error", "error", err)
		}
	}()

	// Start Unix socket listener (if configured and not on Windows)
	if s.cfg.UnixSocketPath != "" && runtime.GOOS != "windows" {
		// Remove existing socket file
		_ = os.Remove(s.cfg.UnixSocketPath)

		unixLis, err := net.Listen("unix", s.cfg.UnixSocketPath)
		if err != nil {
			s.log.Warn("Failed to listen on Unix socket", "path", s.cfg.UnixSocketPath, "error", err)
		} else {
			s.unixListener = unixLis
			// Set permissions to allow local access
			_ = os.Chmod(s.cfg.UnixSocketPath, 0666)
			s.log.Info("gRPC server listening on Unix socket", "path", s.cfg.UnixSocketPath)

			go func() {
				if err := s.grpcServer.Serve(unixLis); err != nil {
					s.log.Error("Unix socket server error", "error", err)
				}
			}()
		}
	}

	return nil
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.log.Info("Stopping gRPC server...")
		s.grpcServer.GracefulStop()
	}

	// Clean up Unix socket
	if s.cfg.UnixSocketPath != "" {
		_ = os.Remove(s.cfg.UnixSocketPath)
	}
}

// =============================================================================
// Search Methods
// =============================================================================

// Search performs hybrid search with optional reranking.
func (s *Server) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}
	if req.Store == "" {
		return nil, status.Error(codes.InvalidArgument, "store is required")
	}

	// Build search request
	searchReq := search.Request{
		Query:          req.Query,
		Store:          req.Store,
		TopK:           int(req.TopK),
		IncludeContent: req.IncludeContent,
	}

	if req.EnableReranking != nil {
		searchReq.EnableReranking = req.EnableReranking
	}
	if req.RerankTopK > 0 {
		searchReq.RerankTopK = int(req.RerankTopK)
	}
	if req.SparseWeight != nil {
		searchReq.SparseWeight = req.SparseWeight
	}
	if req.DenseWeight != nil {
		searchReq.DenseWeight = req.DenseWeight
	}

	if req.Filter != nil {
		searchReq.Filter = &search.Filter{
			PathPrefix: req.Filter.PathPrefix,
			Languages:  req.Filter.Languages,
		}
	}

	// Execute search
	resp, err := s.searchSvc.Search(ctx, searchReq)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Convert response
	results := make([]*pb.SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = &pb.SearchResult{
			Id:          r.ID,
			Path:        r.Path,
			Language:    r.Language,
			StartLine:   int32(r.StartLine),
			EndLine:     int32(r.EndLine),
			Content:     r.Content,
			Symbols:     r.Symbols,
			Score:       r.Score,
			RerankScore: r.RerankScore,
		}
	}

	return &pb.SearchResponse{
		Query:   resp.Query,
		Store:   resp.Store,
		Results: results,
		Total:   int32(resp.Total),
		Metadata: &pb.SearchMetadata{
			SearchTimeMs:       resp.Metadata.SearchTimeMs,
			EmbedTimeMs:        resp.Metadata.EmbedTimeMs,
			RetrievalTimeMs:    resp.Metadata.RetrievalTimeMs,
			RerankTimeMs:       resp.Metadata.RerankTimeMs,
			CandidatesReranked: int32(resp.Metadata.CandidatesReranked),
			RerankingApplied:   resp.Metadata.RerankingApplied,
		},
	}, nil
}

// SearchStream performs search and streams results.
func (s *Server) SearchStream(req *pb.SearchRequest, stream pb.RiceSearch_SearchStreamServer) error {
	// For now, use the regular search and stream results
	resp, err := s.Search(stream.Context(), req)
	if err != nil {
		return err
	}

	// Check for nil response (defensive - Search should always return non-nil on success)
	if resp == nil || resp.Results == nil {
		return nil
	}

	for _, result := range resp.Results {
		if err := stream.Send(result); err != nil {
			return err
		}
	}

	return nil
}

// =============================================================================
// Index Methods
// =============================================================================

// Index indexes documents via streaming.
func (s *Server) Index(stream pb.RiceSearch_IndexServer) error {
	var storeName string
	var docs []*index.Document
	var force bool

	// Collect all documents from stream
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "receive error: %v", err)
		}

		if storeName == "" && req.Store != "" {
			storeName = req.Store
		}
		if req.Force {
			force = true
		}

		if req.Document != nil {
			doc := index.NewDocument(req.Document.Path, req.Document.Content)
			if req.Document.Language != "" {
				doc.Language = req.Document.Language
			}
			docs = append(docs, doc)
		}
	}

	if storeName == "" {
		storeName = "default"
	}

	// Execute indexing
	result, err := s.indexSvc.Index(stream.Context(), index.IndexRequest{
		Store:     storeName,
		Documents: docs,
		Force:     force,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "indexing failed: %v", err)
	}

	// Convert response
	return stream.SendAndClose(indexResultToProto(result))
}

// IndexBatch indexes a batch of documents.
func (s *Server) IndexBatch(ctx context.Context, req *pb.IndexBatchRequest) (*pb.IndexResponse, error) {
	if req.Store == "" {
		req.Store = "default"
	}

	docs := make([]*index.Document, len(req.Documents))
	for i, d := range req.Documents {
		doc := index.NewDocument(d.Path, d.Content)
		if d.Language != "" {
			doc.Language = d.Language
		}
		docs[i] = doc
	}

	result, err := s.indexSvc.Index(ctx, index.IndexRequest{
		Store:     req.Store,
		Documents: docs,
		Force:     req.Force,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "indexing failed: %v", err)
	}

	return indexResultToProto(result), nil
}

// Delete removes documents from the index.
func (s *Server) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if req.Store == "" {
		return nil, status.Error(codes.InvalidArgument, "store is required")
	}

	var deleted int32

	if len(req.Paths) > 0 {
		if err := s.indexSvc.Delete(ctx, req.Store, req.Paths); err != nil {
			return nil, status.Errorf(codes.Internal, "delete failed: %v", err)
		}
		deleted = int32(len(req.Paths))
	}

	if req.PathPrefix != "" {
		if err := s.indexSvc.DeleteByPrefix(ctx, req.Store, req.PathPrefix); err != nil {
			return nil, status.Errorf(codes.Internal, "delete by prefix failed: %v", err)
		}
	}

	return &pb.DeleteResponse{Deleted: deleted}, nil
}

// Sync removes documents that no longer exist.
func (s *Server) Sync(ctx context.Context, req *pb.SyncRequest) (*pb.SyncResponse, error) {
	if req.Store == "" {
		return nil, status.Error(codes.InvalidArgument, "store is required")
	}

	removed, err := s.indexSvc.Sync(ctx, req.Store, req.CurrentPaths)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sync failed: %v", err)
	}

	return &pb.SyncResponse{Removed: int32(removed)}, nil
}

// Reindex clears and rebuilds the entire index.
func (s *Server) Reindex(ctx context.Context, req *pb.ReindexRequest) (*pb.IndexResponse, error) {
	if req.Store == "" {
		req.Store = "default"
	}

	docs := make([]*index.Document, len(req.Documents))
	for i, d := range req.Documents {
		doc := index.NewDocument(d.Path, d.Content)
		if d.Language != "" {
			doc.Language = d.Language
		}
		docs[i] = doc
	}

	result, err := s.indexSvc.Reindex(ctx, index.IndexRequest{
		Store:     req.Store,
		Documents: docs,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "reindex failed: %v", err)
	}

	return indexResultToProto(result), nil
}

// GetIndexStats returns indexing statistics.
func (s *Server) GetIndexStats(ctx context.Context, req *pb.GetIndexStatsRequest) (*pb.IndexStats, error) {
	if req.Store == "" {
		return nil, status.Error(codes.InvalidArgument, "store is required")
	}

	stats, err := s.indexSvc.GetStats(ctx, req.Store)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get stats failed: %v", err)
	}

	return &pb.IndexStats{
		Store:       stats.Store,
		TotalChunks: int32(stats.TotalChunks),
		TotalFiles:  int32(stats.TotalFiles),
	}, nil
}

// ListFiles returns a paginated list of indexed files.
func (s *Server) ListFiles(_ context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	if req.Store == "" {
		req.Store = "default"
	}

	page := int(req.Page)
	if page < 1 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize < 1 {
		pageSize = 50
	}

	// Get files from pipeline tracker
	files, total := s.indexSvc.ListFiles(req.Store, page, pageSize)

	// Convert to protobuf
	pbFiles := make([]*pb.IndexedFile, len(files))
	for i, f := range files {
		pbFiles[i] = &pb.IndexedFile{
			Path:      f.Path,
			Hash:      f.Hash,
			IndexedAt: timestamppb.New(f.IndexedAt),
			Status:    "indexed",
		}
	}

	totalPages := int32(total / pageSize)
	if total%pageSize > 0 {
		totalPages++
	}

	return &pb.ListFilesResponse{
		Files:      pbFiles,
		Total:      int32(total),
		Page:       int32(page),
		PageSize:   int32(pageSize),
		TotalPages: totalPages,
	}, nil
}

// GetChunks retrieves all chunks for a specific file.
func (s *Server) GetChunks(ctx context.Context, req *pb.GetChunksRequest) (*pb.GetChunksResponse, error) {
	if req.Store == "" {
		return nil, status.Error(codes.InvalidArgument, "store is required")
	}
	if req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	// Get chunks from Qdrant
	chunks, err := s.qdrant.GetChunksByPath(ctx, req.Store, req.Path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get chunks: %v", err)
	}

	// Convert to protobuf
	pbChunks := make([]*pb.Chunk, len(chunks))
	for i, c := range chunks {
		pbChunks[i] = &pb.Chunk{
			Id:        c.ID,
			StartLine: int32(c.Payload.StartLine),
			EndLine:   int32(c.Payload.EndLine),
			Content:   c.Payload.Content,
		}
	}

	return &pb.GetChunksResponse{
		Path:   req.Path,
		Chunks: pbChunks,
		Total:  int32(len(pbChunks)),
	}, nil
}

// =============================================================================
// Store Methods
// =============================================================================

// ListStores returns all stores.
func (s *Server) ListStores(ctx context.Context, _ *pb.ListStoresRequest) (*pb.ListStoresResponse, error) {
	stores, err := s.storeSvc.ListStores(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list stores failed: %v", err)
	}

	pbStores := make([]*pb.Store, len(stores))
	for i, st := range stores {
		pbStores[i] = storeToProto(st)
	}

	return &pb.ListStoresResponse{Stores: pbStores}, nil
}

// CreateStore creates a new store.
func (s *Server) CreateStore(ctx context.Context, req *pb.CreateStoreRequest) (*pb.Store, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	newStore := store.NewStore(req.Name)
	if req.DisplayName != "" {
		newStore.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		newStore.Description = req.Description
	}
	if req.Config != nil {
		if req.Config.ChunkSize > 0 {
			newStore.Config.ChunkSize = int(req.Config.ChunkSize)
		}
		if req.Config.ChunkOverlap > 0 {
			newStore.Config.ChunkOverlap = int(req.Config.ChunkOverlap)
		}
	}

	if err := s.storeSvc.CreateStore(ctx, newStore); err != nil {
		return nil, status.Errorf(codes.Internal, "create store failed: %v", err)
	}

	return storeToProto(newStore), nil
}

// GetStore retrieves a store by name.
func (s *Server) GetStore(ctx context.Context, req *pb.GetStoreRequest) (*pb.Store, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	st, err := s.storeSvc.GetStore(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "store not found: %v", err)
	}

	return storeToProto(st), nil
}

// DeleteStore deletes a store.
func (s *Server) DeleteStore(ctx context.Context, req *pb.DeleteStoreRequest) (*pb.DeleteStoreResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if err := s.storeSvc.DeleteStore(ctx, req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete store failed: %v", err)
	}

	return &pb.DeleteStoreResponse{Success: true}, nil
}

// GetStoreStats returns statistics for a store.
func (s *Server) GetStoreStats(ctx context.Context, req *pb.GetStoreStatsRequest) (*pb.StoreStats, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	stats, err := s.storeSvc.GetStoreStats(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get store stats failed: %v", err)
	}

	pbStats := &pb.StoreStats{
		DocumentCount: stats.DocumentCount,
		ChunkCount:    stats.ChunkCount,
		TotalSize:     stats.TotalSize,
	}
	if !stats.LastIndexed.IsZero() {
		pbStats.LastIndexed = timestamppb.New(stats.LastIndexed)
	}

	return pbStats, nil
}

// =============================================================================
// Health & Version Methods
// =============================================================================

// Health returns service health status.
func (s *Server) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	components := make(map[string]*pb.ComponentHealth)

	// Check Qdrant
	qdrantStatus := pb.HealthStatus_HEALTH_STATUS_HEALTHY
	qdrantMsg := "connected"
	qdrantStart := time.Now()
	if _, err := s.qdrant.GetCollectionInfo(ctx, "default"); err != nil {
		// Collection might not exist, that's OK
		if exists, _ := s.qdrant.CollectionExists(ctx, "default"); !exists {
			qdrantMsg = "connected (no default collection)"
		}
	}
	components["qdrant"] = &pb.ComponentHealth{
		Status:  qdrantStatus,
		Message: qdrantMsg,
		Latency: durationpb.New(time.Since(qdrantStart)),
	}

	// Check ML service
	mlStatus := pb.HealthStatus_HEALTH_STATUS_HEALTHY
	mlMsg := "ready"
	mlHealth := s.ml.Health()
	if !mlHealth.Healthy {
		mlStatus = pb.HealthStatus_HEALTH_STATUS_DEGRADED
		mlMsg = "models not loaded"
		if mlHealth.Error != "" {
			mlMsg = mlHealth.Error
		}
	}
	components["ml"] = &pb.ComponentHealth{
		Status:  mlStatus,
		Message: mlMsg,
	}

	// Overall status
	overallStatus := pb.HealthStatus_HEALTH_STATUS_HEALTHY
	for _, c := range components {
		if c.Status == pb.HealthStatus_HEALTH_STATUS_UNHEALTHY {
			overallStatus = pb.HealthStatus_HEALTH_STATUS_UNHEALTHY
			break
		}
		if c.Status == pb.HealthStatus_HEALTH_STATUS_DEGRADED {
			overallStatus = pb.HealthStatus_HEALTH_STATUS_DEGRADED
			// Continue checking for UNHEALTHY
		}
	}

	return &pb.HealthResponse{
		Status:     overallStatus,
		Components: components,
		Version:    s.cfg.Version,
	}, nil
}

// Version returns version information.
func (s *Server) Version(_ context.Context, _ *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{
		Version:   s.cfg.Version,
		Commit:    s.cfg.Commit,
		BuildDate: s.cfg.BuildDate,
		GoVersion: runtime.Version(),
	}, nil
}

// =============================================================================
// Helpers
// =============================================================================

func indexResultToProto(result *index.IndexResult) *pb.IndexResponse {
	errors := make([]*pb.IndexError, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = &pb.IndexError{
			Path:    e.Path,
			Message: e.Message,
		}
	}

	docInfos := make([]*pb.DocumentInfo, len(result.DocumentInfo))
	for i, d := range result.DocumentInfo {
		docInfos[i] = &pb.DocumentInfo{
			Path:       d.Path,
			Hash:       d.Hash,
			ChunkCount: int32(d.ChunkCount),
			Status:     d.Status,
		}
	}

	return &pb.IndexResponse{
		Store:        result.Store,
		Indexed:      int32(result.Indexed),
		Skipped:      int32(result.Skipped),
		Failed:       int32(result.Failed),
		ChunksTotal:  int32(result.ChunksTotal),
		Duration:     durationpb.New(result.Duration),
		Errors:       errors,
		DocumentInfo: docInfos,
	}
}

func storeToProto(st *store.Store) *pb.Store {
	pbStore := &pb.Store{
		Name:        st.Name,
		DisplayName: st.DisplayName,
		Description: st.Description,
		Config: &pb.StoreConfig{
			EmbedModel:   st.Config.EmbedModel,
			SparseModel:  st.Config.SparseModel,
			ChunkSize:    int32(st.Config.ChunkSize),
			ChunkOverlap: int32(st.Config.ChunkOverlap),
		},
		Stats: &pb.StoreStats{
			DocumentCount: st.Stats.DocumentCount,
			ChunkCount:    st.Stats.ChunkCount,
			TotalSize:     st.Stats.TotalSize,
		},
		CreatedAt: timestamppb.New(st.CreatedAt),
		UpdatedAt: timestamppb.New(st.UpdatedAt),
	}

	if !st.Stats.LastIndexed.IsZero() {
		pbStore.Stats.LastIndexed = timestamppb.New(st.Stats.LastIndexed)
	}

	return pbStore
}
