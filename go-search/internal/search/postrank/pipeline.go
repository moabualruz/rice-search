package postrank

import (
	"context"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// Pipeline orchestrates all post-ranking operations.
type Pipeline struct {
	dedup       *DedupService
	diversity   *DiversityService
	aggregation *AggregationService
	config      Config
	log         *logger.Logger
}

// Config holds post-ranking configuration.
type Config struct {
	// EnableDedup enables semantic deduplication.
	EnableDedup bool

	// DedupThreshold is the cosine similarity threshold for deduplication (0-1).
	DedupThreshold float32

	// EnableDiversity enables MMR diversity reordering.
	EnableDiversity bool

	// DiversityLambda controls the relevance/diversity tradeoff (0=diverse, 1=relevant).
	DiversityLambda float32

	// GroupByFile enables file-based aggregation.
	GroupByFile bool

	// MaxChunksPerFile limits chunks per file when grouping.
	MaxChunksPerFile int
}

// DefaultConfig returns default post-ranking configuration.
func DefaultConfig() Config {
	return Config{
		EnableDedup:      true,
		DedupThreshold:   0.85,
		EnableDiversity:  true,
		DiversityLambda:  0.7,
		GroupByFile:      false,
		MaxChunksPerFile: 3,
	}
}

// ResultWithEmbedding wraps a search result with its embedding.
type ResultWithEmbedding struct {
	ID           string
	Path         string
	Language     string
	StartLine    int
	EndLine      int
	Content      string
	Symbols      []string
	Score        float32
	RerankScore  *float32
	ConnectionID string
	Embedding    []float32
}

// PostRankResult contains the results and statistics from post-ranking.
type PostRankResult struct {
	Results      []ResultWithEmbedding
	Dedup        DedupResult
	Diversity    DiversityResult
	Aggregation  AggregationResult
	TotalLatency int64
}

// NewPipeline creates a new post-ranking pipeline.
func NewPipeline(config Config, log *logger.Logger) *Pipeline {
	return &Pipeline{
		dedup:       NewDedupService(config.DedupThreshold, log),
		diversity:   NewDiversityService(config.DiversityLambda, log),
		aggregation: NewAggregationService(config.MaxChunksPerFile, log),
		config:      config,
		log:         log,
	}
}

// Process applies all configured post-ranking operations in order:
// 1. Deduplication (removes near-duplicates)
// 2. Diversity (MMR reordering)
// 3. Aggregation (group by file)
func (p *Pipeline) Process(ctx context.Context, results []ResultWithEmbedding, topK int) (*PostRankResult, error) {
	start := time.Now()

	if len(results) == 0 {
		return &PostRankResult{
			Results:      results,
			TotalLatency: 0,
		}, nil
	}

	pr := &PostRankResult{
		Results: results,
	}

	// Step 1: Deduplication
	if p.config.EnableDedup {
		deduped, stats := p.dedup.Deduplicate(ctx, pr.Results)
		pr.Results = deduped
		pr.Dedup = stats

		p.log.Debug("Post-rank deduplication complete",
			"input", stats.InputCount,
			"output", stats.OutputCount,
			"removed", stats.Removed,
			"latency_ms", stats.LatencyMs,
		)
	} else {
		pr.Dedup = DedupResult{
			InputCount:  len(results),
			OutputCount: len(results),
			Removed:     0,
			LatencyMs:   0,
		}
	}

	// Step 2: Diversity (MMR)
	if p.config.EnableDiversity && len(pr.Results) > 1 {
		diverse, stats := p.diversity.ApplyMMR(ctx, pr.Results, topK)
		pr.Results = diverse
		pr.Diversity = stats

		p.log.Debug("Post-rank diversity complete",
			"avg_diversity", stats.AvgDiversity,
			"latency_ms", stats.LatencyMs,
		)
	} else {
		pr.Diversity = DiversityResult{
			Enabled:      false,
			AvgDiversity: 0,
			LatencyMs:    0,
		}
	}

	// Step 3: Aggregation (group by file)
	if p.config.GroupByFile {
		aggregated, stats := p.aggregation.GroupByFile(ctx, pr.Results)
		pr.Results = aggregated
		pr.Aggregation = stats

		p.log.Debug("Post-rank aggregation complete",
			"unique_files", stats.UniqueFiles,
			"chunks_dropped", stats.ChunksDropped,
			"latency_ms", stats.LatencyMs,
		)
	} else {
		pr.Aggregation = AggregationResult{
			UniqueFiles:   0,
			ChunksDropped: 0,
			LatencyMs:     0,
		}
	}

	pr.TotalLatency = time.Since(start).Milliseconds()

	return pr, nil
}

// UpdateConfig updates the pipeline configuration at runtime.
func (p *Pipeline) UpdateConfig(config Config) {
	p.config = config
	p.dedup = NewDedupService(config.DedupThreshold, p.log)
	p.diversity = NewDiversityService(config.DiversityLambda, p.log)
	p.aggregation = NewAggregationService(config.MaxChunksPerFile, p.log)
}

// GetConfig returns the current pipeline configuration.
func (p *Pipeline) GetConfig() Config {
	return p.config
}
