package metrics

import (
	"context"
	"fmt"

	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/store"
)

// Collector collects metrics from various services.
type Collector struct {
	metrics *Metrics
	stores  *store.Service
	qdrant  *qdrant.Client
}

// NewCollector creates a new metrics collector.
func NewCollector(metrics *Metrics, stores *store.Service, qdrant *qdrant.Client) *Collector {
	return &Collector{
		metrics: metrics,
		stores:  stores,
		qdrant:  qdrant,
	}
}

// Collect gathers current statistics from all services.
func (c *Collector) Collect(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Collect store stats
	if c.stores != nil {
		storeList, err := c.stores.ListStores(ctx)
		if err == nil {
			c.metrics.UpdateStoreCount(len(storeList))
			stats["stores_total"] = len(storeList)

			// Collect per-store stats
			storeStats := make([]map[string]interface{}, 0, len(storeList))
			for _, st := range storeList {
				storeInfo, err := c.CollectForStore(ctx, st.Name)
				if err == nil {
					storeStats = append(storeStats, storeInfo)
				}
			}
			stats["stores"] = storeStats
		}
	}

	// System metrics
	stats["goroutines"] = c.metrics.GoroutineCount.Value()
	stats["memory_bytes"] = c.metrics.MemoryUsage.Value()
	stats["uptime_seconds"] = c.metrics.Uptime.Value()

	// Search metrics
	stats["search_requests_total"] = c.metrics.SearchRequests.Value()
	stats["search_latency_count"] = c.metrics.SearchLatency.Count()
	stats["search_latency_sum_ms"] = c.metrics.SearchLatency.Sum()

	// Index metrics
	stats["indexed_documents_total"] = c.metrics.IndexedDocuments.Value()
	stats["indexed_chunks_total"] = c.metrics.IndexedChunks.Value()
	stats["index_latency_count"] = c.metrics.IndexLatency.Count()
	stats["index_latency_sum_ms"] = c.metrics.IndexLatency.Sum()

	// Model metrics
	stats["embed_requests_total"] = c.metrics.EmbedRequests.Value()
	stats["embed_latency_count"] = c.metrics.EmbedLatency.Count()
	stats["embed_latency_sum_ms"] = c.metrics.EmbedLatency.Sum()
	stats["rerank_requests_total"] = c.metrics.RerankRequests.Value()
	stats["rerank_latency_count"] = c.metrics.RerankLatency.Count()
	stats["rerank_latency_sum_ms"] = c.metrics.RerankLatency.Sum()

	// Connection metrics
	stats["active_connections"] = c.metrics.ActiveConnections.Value()
	stats["connections_total"] = c.metrics.ConnectionsTotal.Value()

	return stats, nil
}

// CollectForStore collects statistics for a specific store.
func (c *Collector) CollectForStore(ctx context.Context, storeName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	stats["name"] = storeName

	// Get store metadata
	if c.stores != nil {
		st, err := c.stores.GetStore(ctx, storeName)
		if err == nil {
			stats["created_at"] = st.CreatedAt.Unix()
			stats["updated_at"] = st.UpdatedAt.Unix()
		}

		// Get live stats from store service
		storeStats, err := c.stores.GetStoreStats(ctx, storeName)
		if err == nil {
			stats["documents"] = storeStats.DocumentCount
			stats["chunks"] = storeStats.ChunkCount
			stats["total_size_bytes"] = storeStats.TotalSize
			stats["last_indexed_at"] = storeStats.LastIndexed.Unix()

			// Update metrics
			c.metrics.UpdateStoreStats(storeName, storeStats.DocumentCount, storeStats.ChunkCount)
		}
	}

	// Get Qdrant collection info
	if c.qdrant != nil {
		info, err := c.qdrant.GetCollectionInfo(ctx, storeName)
		if err == nil {
			stats["qdrant_points"] = info.PointsCount
			stats["qdrant_vectors"] = info.VectorsCount
			stats["qdrant_segments"] = info.SegmentsCount
			stats["qdrant_status"] = info.Status
		}
	}

	return stats, nil
}

// Summary returns a human-readable summary of current metrics.
func (c *Collector) Summary(ctx context.Context) string {
	stats, err := c.Collect(ctx)
	if err != nil {
		return "Error collecting metrics"
	}

	// Format summary
	summary := "Rice Search Metrics Summary\n"
	summary += "===========================\n\n"

	if storesTotal, ok := stats["stores_total"].(int); ok {
		summary += "Stores: " + toString(storesTotal) + "\n"
	}

	if searchReqs, ok := stats["search_requests_total"].(int64); ok {
		summary += "Search Requests: " + toString(searchReqs) + "\n"
	}

	if indexedDocs, ok := stats["indexed_documents_total"].(int64); ok {
		summary += "Indexed Documents: " + toString(indexedDocs) + "\n"
	}

	if indexedChunks, ok := stats["indexed_chunks_total"].(int64); ok {
		summary += "Indexed Chunks: " + toString(indexedChunks) + "\n"
	}

	if embedReqs, ok := stats["embed_requests_total"].(int64); ok {
		summary += "Embed Requests: " + toString(embedReqs) + "\n"
	}

	if rerankReqs, ok := stats["rerank_requests_total"].(int64); ok {
		summary += "Rerank Requests: " + toString(rerankReqs) + "\n"
	}

	if goroutines, ok := stats["goroutines"].(float64); ok {
		summary += "Goroutines: " + toString(int(goroutines)) + "\n"
	}

	if memBytes, ok := stats["memory_bytes"].(float64); ok {
		summary += "Memory Usage: " + formatBytes(int64(memBytes)) + "\n"
	}

	if uptime, ok := stats["uptime_seconds"].(int64); ok {
		summary += "Uptime: " + formatDuration(uptime) + "\n"
	}

	return summary
}

// Helper functions

func toString(v interface{}) string {
	switch val := v.(type) {
	case int:
		return formatInt(int64(val))
	case int64:
		return formatInt(val)
	case float64:
		return formatInt(int64(val))
	default:
		return "0"
	}
}

func formatInt(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
