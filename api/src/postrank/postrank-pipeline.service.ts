import { Injectable, Logger } from "@nestjs/common";
import { HybridSearchResult } from "../services/hybrid-ranker.service";
import { DeduplicationService, DeduplicationConfig, DeduplicationStats } from "./deduplication.service";
import { AggregationService, AggregationConfig, AggregatedResult, AggregationStats } from "./aggregation.service";
import { DiversityService, DiversityConfig, DiversityStats } from "./diversity.service";

export interface PostrankOptions {
  dedup?: Partial<DeduplicationConfig>;
  diversity?: Partial<DiversityConfig>;
  aggregation?: Partial<AggregationConfig>;
}

export interface PostrankStats {
  inputCount: number;
  afterDedup: number;
  afterDiversity: number;
  outputCount: number;
  dedupStats: DeduplicationStats;
  diversityStats: DiversityStats;
  aggregationStats: AggregationStats;
  totalLatencyMs: number;
}

@Injectable()
export class PostrankPipelineService {
  private readonly logger = new Logger(PostrankPipelineService.name);

  constructor(
    private readonly deduplication: DeduplicationService,
    private readonly aggregation: AggregationService,
    private readonly diversity: DiversityService,
  ) {
    this.logger.log("PostrankPipelineService initialized");
  }

  /**
   * Process results through the post-rank pipeline:
   * 1. Deduplication (remove near-duplicates)
   * 2. Diversity (ensure variety via MMR)
   * 3. Aggregation (group by file)
   * 
   * @param results - Reranked search results
   * @param options - Pipeline configuration
   * @returns Processed results with stats
   */
  async process(
    results: HybridSearchResult[],
    options: PostrankOptions = {},
  ): Promise<{ results: AggregatedResult[]; stats: PostrankStats }> {
    const startTime = Date.now();
    let processed: HybridSearchResult[] = results;

    // Step 1: Deduplication
    const dedupResult = await this.deduplication.deduplicate(processed, options.dedup);
    processed = dedupResult.results;
    const afterDedup = processed.length;

    // Step 2: Diversity (before aggregation to ensure variety across files)
    const diversityResult = await this.diversity.diversify(processed, options.diversity);
    processed = diversityResult.results;
    const afterDiversity = processed.length;

    // Step 3: Aggregation (group by file if requested)
    const aggregationResult = this.aggregation.aggregate(processed, options.aggregation);

    const totalLatencyMs = Date.now() - startTime;

    const stats: PostrankStats = {
      inputCount: results.length,
      afterDedup,
      afterDiversity,
      outputCount: aggregationResult.results.length,
      dedupStats: dedupResult.stats,
      diversityStats: diversityResult.stats,
      aggregationStats: aggregationResult.stats,
      totalLatencyMs,
    };

    this.logger.debug(
      `PostrankPipeline: ${stats.inputCount} → ${stats.afterDedup} (dedup) ` +
      `→ ${stats.afterDiversity} (diversity) → ${stats.outputCount} (final), ${totalLatencyMs}ms`
    );

    return {
      results: aggregationResult.results,
      stats,
    };
  }
}
