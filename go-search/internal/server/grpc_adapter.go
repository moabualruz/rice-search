// Package server provides the HTTP server that wires all services together.
package server

import (
	"context"
	"runtime"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/ricesearch/rice-search/api/proto/ricesearchpb"
	"github.com/ricesearch/rice-search/internal/search"
	"github.com/ricesearch/rice-search/internal/store"
)

// LocalGRPCAdapter implements the web.GRPCClient interface by directly
// calling services without actual gRPC networking. This allows the web UI
// to use the same interface whether running locally or against a remote server.
type LocalGRPCAdapter struct {
	server *Server
}

// NewLocalGRPCAdapter creates a new adapter that wraps the server's services.
func NewLocalGRPCAdapter(s *Server) *LocalGRPCAdapter {
	return &LocalGRPCAdapter{server: s}
}

// Search performs hybrid search with optional reranking.
func (a *LocalGRPCAdapter) Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	if req.Store == "" {
		req.Store = "default"
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
	resp, err := a.server.search.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}

	// Convert response
	results := make([]*pb.SearchResult, len(resp.Results))
	for i, r := range resp.Results {
		results[i] = &pb.SearchResult{
			ID:          r.ID,
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

// ListStores returns all stores.
func (a *LocalGRPCAdapter) ListStores(ctx context.Context, req *pb.ListStoresRequest) (*pb.ListStoresResponse, error) {
	stores, err := a.server.store.ListStores(ctx)
	if err != nil {
		return nil, err
	}

	pbStores := make([]*pb.Store, len(stores))
	for i, st := range stores {
		pbStores[i] = storeToProto(st)
	}

	return &pb.ListStoresResponse{Stores: pbStores}, nil
}

// CreateStore creates a new store.
func (a *LocalGRPCAdapter) CreateStore(ctx context.Context, req *pb.CreateStoreRequest) (*pb.Store, error) {
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

	if err := a.server.store.CreateStore(ctx, newStore); err != nil {
		return nil, err
	}

	return storeToProto(newStore), nil
}

// GetStore retrieves a store by name.
func (a *LocalGRPCAdapter) GetStore(ctx context.Context, req *pb.GetStoreRequest) (*pb.Store, error) {
	st, err := a.server.store.GetStore(ctx, req.Name)
	if err != nil {
		return nil, err
	}

	return storeToProto(st), nil
}

// DeleteStore deletes a store.
func (a *LocalGRPCAdapter) DeleteStore(ctx context.Context, req *pb.DeleteStoreRequest) (*pb.DeleteStoreResponse, error) {
	if err := a.server.store.DeleteStore(ctx, req.Name); err != nil {
		return nil, err
	}

	return &pb.DeleteStoreResponse{Success: true}, nil
}

// GetStoreStats returns statistics for a store.
func (a *LocalGRPCAdapter) GetStoreStats(ctx context.Context, req *pb.GetStoreStatsRequest) (*pb.StoreStats, error) {
	stats, err := a.server.store.GetStoreStats(ctx, req.Name)
	if err != nil {
		return nil, err
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

// Health returns service health status.
func (a *LocalGRPCAdapter) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	components := make(map[string]*pb.ComponentHealth)

	// Check Qdrant
	qdrantStatus := pb.HealthStatus_HEALTH_STATUS_HEALTHY
	qdrantMsg := "connected"
	qdrantStart := time.Now()
	if _, err := a.server.qdrant.GetCollectionInfo(ctx, "default"); err != nil {
		// Collection might not exist, that's OK
		if exists, _ := a.server.qdrant.CollectionExists(ctx, "default"); !exists {
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
	mlHealth := a.server.ml.Health()
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
		if c.Status == pb.HealthStatus_HEALTH_STATUS_DEGRADED && overallStatus != pb.HealthStatus_HEALTH_STATUS_UNHEALTHY {
			overallStatus = pb.HealthStatus_HEALTH_STATUS_DEGRADED
		}
	}

	return &pb.HealthResponse{
		Status:     overallStatus,
		Components: components,
		Version:    a.server.cfg.Version,
	}, nil
}

// Version returns version information.
func (a *LocalGRPCAdapter) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{
		Version:   a.server.cfg.Version,
		Commit:    "dev",
		BuildDate: "unknown",
		GoVersion: runtime.Version(),
	}, nil
}

// =============================================================================
// Helpers
// =============================================================================

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
