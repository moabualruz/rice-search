import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { randomUUID } from "crypto";
import { EmbeddingsService } from "../services/embeddings.service";
import { MilvusService, MilvusSearchResult } from "../services/milvus.service";
import { TantivyService, TantivySearchResult } from "../services/tantivy.service";
import { HybridRankerService, HybridSearchResult } from "../services/hybrid-ranker.service";
import { StoreManagerService } from "../services/store-manager.service";
import { QueryNormalizerService } from "../services/query-normalizer.service";
import { TelemetryService, SearchTelemetryRecord } from "../services/telemetry.service";
import { IntentClassifierService } from "../intelligence/intent-classifier.service";
import { StrategySelectorService, RetrievalConfig } from "../intelligence/strategy-selector.service";
import { MultiPassRerankerService, RerankStats } from "../ranking/multi-pass-reranker.service";
import { PostrankPipelineService, PostrankStats } from "../postrank/postrank-pipeline.service";
import { AggregatedResult } from "../postrank/aggregation.service";
import { SearchRequestDto } from "./dto/search-request.dto";

/**
 * Normalize path to use forward slashes consistently.
 * Handles Windows paths (backslashes) for consistent filtering.
 */
function normalizePath(filePath: string): string {
  return filePath.replace(/\\/g, "/");
}

@Injectable()
export class SearchService {
  private readonly logger = new Logger(SearchService.name);

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private milvus: MilvusService,
    private tantivy: TantivyService,
    private hybridRanker: HybridRankerService,
    private storeManager: StoreManagerService,
    private queryNormalizer: QueryNormalizerService,
    private telemetry: TelemetryService,
    private intentClassifier: IntentClassifierService,
    private strategySelector: StrategySelectorService,
    private multiPassReranker: MultiPassRerankerService,
    private postrankPipeline: PostrankPipelineService,
  ) {
    this.logger.log("Search service initialized (Intelligence Pipeline v2 + PostRank)");
  }

  async search(store: string, request: SearchRequestDto) {
    const requestId = randomUUID();
    const startTime = Date.now();
    const { query, top_k = 20, filters, group_by_file } = request;

    // 1. Normalize query for consistent caching
    const normalized = this.queryNormalizer.normalize(query);

    // Normalize path filter for consistent matching (handles Windows paths)
    const normalizedPathPrefix = filters?.path_prefix
      ? normalizePath(filters.path_prefix)
      : undefined;

    // Ensure store exists
    this.storeManager.ensureStore(store);

    // 2. Classify intent (Phase 1 - Task 1.1)
    const intent = this.intentClassifier.classify(normalized.normalized);

    // 3. Select strategy (Phase 1 - Task 1.2)
    let config = this.strategySelector.selectStrategy(intent);

    // 4. Adjust candidates based on difficulty (Phase 1 - Task 1.3)
    config = this.strategySelector.adjustCandidates(config, intent);

    // 5. Apply user overrides if provided
    config = this.strategySelector.applyOverrides(config, {
      sparseWeight: request.sparse_weight,
      denseWeight: request.dense_weight,
      rerankCandidates: request.rerank_candidates,
      enableReranking: request.enable_reranking,
    });

    this.logger.debug(
      `Query "${query.substring(0, 30)}..." intent=${intent.intent} ` +
      `(difficulty=${intent.difficulty}), strategy=${config.strategy}`
    );

    // Timing trackers
    let sparseLatencyMs = 0;
    let denseLatencyMs = 0;
    let fusionLatencyMs = 0;
    let rerankStats: RerankStats | undefined;

    // 6. Execute hybrid search with strategy config
    const hybridStartTime = Date.now();
    const { fusedResults, sparseResults, denseResults, sparseTime, denseTime } =
      await this.executeHybridSearchWithConfig(
        store,
        normalized.normalized,
        config,
        normalizedPathPrefix,
        filters?.languages,
        group_by_file,
      );
    sparseLatencyMs = sparseTime;
    denseLatencyMs = denseTime;
    fusionLatencyMs = Date.now() - hybridStartTime - Math.max(sparseTime, denseTime);

    // Compute fusion stats for telemetry
    const fusionStats = this.hybridRanker.computeFusionStats(fusedResults);

    // 7. Multi-pass rerank (Phase 1 - Task 1.4, 1.5)
    let rankedResults: HybridSearchResult[] = fusedResults;
    if (request.enable_reranking !== false && fusedResults.length > 0) {
      const rerankResult = await this.multiPassReranker.rerank(
        normalized.normalized,
        fusedResults,
        config,
      );
      rankedResults = rerankResult.results;
      rerankStats = rerankResult.stats;
    }

    // 8. Post-rank processing (Phase 2 - dedup, diversity, aggregation)
    let postrankStats: PostrankStats | undefined;
    let processedResults: AggregatedResult[] = rankedResults;

    // Only run postrank if we have results and at least one feature is enabled
    const shouldRunPostrank = rankedResults.length > 0 && (
      request.enable_dedup !== false ||
      request.enable_diversity !== false ||
      request.group_by_file === true
    );

    if (shouldRunPostrank) {
      const postrankResult = await this.postrankPipeline.process(rankedResults, {
        dedup: request.enable_dedup !== false ? {
          similarityThreshold: request.dedup_threshold,
          preserveTop: 3,
          preferLonger: true,
        } : { similarityThreshold: 1.0 }, // threshold=1.0 effectively disables
        diversity: {
          enabled: request.enable_diversity !== false,
          lambda: request.diversity_lambda,
        },
        aggregation: {
          groupByFile: request.group_by_file ?? false,
          maxChunksPerFile: request.max_chunks_per_file ?? 3,
          aggregateScores: true,
        },
      });
      processedResults = postrankResult.results;
      postrankStats = postrankResult.stats;
    }

    // Limit to top_k
    const finalResults = processedResults.slice(0, top_k);
    const totalLatencyMs = Date.now() - startTime;

    // Compute score stats for telemetry
    const sparseScores = sparseResults.map((r: TantivySearchResult) => r.bm25_score);
    const denseScores = denseResults.map((r: MilvusSearchResult) => r.dense_score);
    const sparseStats = this.telemetry.computeScoreStats(sparseScores);
    const denseStats = this.telemetry.computeScoreStats(denseScores);

    // Record telemetry with intent and strategy info
    const telemetryRecord: SearchTelemetryRecord = {
      requestId,
      timestamp: new Date(),
      store,
      query: normalized.normalized,
      queryType: intent.baseClassification.type,
      sparse: {
        resultCount: sparseResults.length,
        latencyMs: sparseLatencyMs,
        topScore: sparseScores[0] ?? 0,
        scoreStdDev: sparseStats.stdDev,
      },
      dense: {
        resultCount: denseResults.length,
        latencyMs: denseLatencyMs,
        topScore: denseScores[0] ?? 0,
        scoreStdDev: denseStats.stdDev,
      },
      fusion: {
        resultCount: fusedResults.length,
        latencyMs: fusionLatencyMs,
        topScore: fusionStats.topScore,
        secondScore: fusionStats.secondScore,
        scoreGap: fusionStats.scoreGap,
        scoreRatio: fusionStats.scoreRatio === Infinity ? 999 : fusionStats.scoreRatio,
      },
      rerank: {
        enabled: request.enable_reranking !== false,
        candidates: config.rerankCandidates,
        latencyMs: (rerankStats?.pass1Latency ?? 0) + (rerankStats?.pass2Latency ?? 0),
        timedOut: false,
        skipped: !rerankStats?.pass1Applied,
        skipReason: rerankStats?.earlyExitReason,
      },
      cache: {
        embeddingHit: false, // TODO: Get from embeddings service
        rerankHit: false, // TODO: Get from reranker service
      },
      totalLatencyMs,
      resultCount: finalResults.length,
    };
    this.telemetry.record(telemetryRecord);

    return {
      query,
      results: finalResults.map((r: AggregatedResult) => ({
        doc_id: r.doc_id,
        path: r.path,
        language: r.language,
        start_line: r.start_line,
        end_line: r.end_line,
        content: request.include_content ? r.content : undefined,
        symbols: r.symbols,
        final_score: r.final_score,
        sparse_score: r.sparse_score,
        dense_score: r.dense_score,
        sparse_rank: r.sparse_rank,
        dense_rank: r.dense_rank,
        // Aggregation info (Phase 2)
        aggregation: r.aggregation ? {
          is_representative: r.aggregation.isRepresentative,
          related_chunks: r.aggregation.relatedChunks,
          file_score: r.aggregation.fileScore,
          chunk_rank_in_file: r.aggregation.chunkRankInFile,
        } : undefined,
      })),
      total: finalResults.length,
      store,
      search_time_ms: totalLatencyMs,
      // Enhanced response with intelligence info
      intelligence: {
        intent: intent.intent,
        difficulty: intent.difficulty,
        strategy: config.strategy,
        confidence: intent.confidence,
      },
      reranking: {
        enabled: request.enable_reranking !== false,
        candidates: config.rerankCandidates,
        pass1_applied: rerankStats?.pass1Applied ?? false,
        pass1_latency_ms: rerankStats?.pass1Latency ?? 0,
        pass2_applied: rerankStats?.pass2Applied ?? false,
        pass2_latency_ms: rerankStats?.pass2Latency ?? 0,
        early_exit: rerankStats?.earlyExitTriggered ?? false,
        early_exit_reason: rerankStats?.earlyExitReason,
      },
      // Post-rank processing stats (Phase 2)
      postrank: postrankStats ? {
        dedup: {
          input_count: postrankStats.dedupStats.inputCount,
          output_count: postrankStats.dedupStats.outputCount,
          removed: postrankStats.dedupStats.removedCount,
          latency_ms: postrankStats.dedupStats.latencyMs,
        },
        diversity: {
          enabled: request.enable_diversity !== false,
          avg_diversity: postrankStats.diversityStats.avgDiversity,
          latency_ms: postrankStats.diversityStats.latencyMs,
        },
        aggregation: {
          unique_files: postrankStats.aggregationStats.uniqueFiles,
          chunks_dropped: postrankStats.aggregationStats.chunksDropped,
        },
        total_latency_ms: postrankStats.totalLatencyMs,
      } : undefined,
    };
  }

  /**
   * Execute hybrid search using strategy config
   * Respects config's topK values and weights
   */
  private async executeHybridSearchWithConfig(
    store: string,
    query: string,
    config: RetrievalConfig,
    pathPrefix?: string,
    languages?: string[],
    groupByFile?: boolean,
  ): Promise<{
    fusedResults: HybridSearchResult[];
    sparseResults: TantivySearchResult[];
    denseResults: MilvusSearchResult[];
    sparseTime: number;
    denseTime: number;
  }> {
    // Get embedding from Infinity (skip if sparse-only strategy)
    let queryEmbedding: number[] | undefined;
    if (config.denseTopK > 0) {
      const queryEmbeddings = await this.embeddings.embed([query]);
      queryEmbedding = queryEmbeddings[0];
    }

    // Run sparse (Tantivy) and dense (Milvus) search in parallel with timing
    const sparseStart = Date.now();
    let sparseTime = 0;
    const denseStart = Date.now();
    let denseTime = 0;

    const [sparseResultsRaw, denseResultsRaw] = await Promise.all([
      // Sparse search (always runs unless sparseTopK is 0)
      (async () => {
        if (config.sparseTopK === 0) {
          sparseTime = 0;
          return [];
        }
        const result = await this.tantivy.search(
          store,
          query,
          config.sparseTopK,
          pathPrefix,
          languages?.[0],
        );
        sparseTime = Date.now() - sparseStart;
        return result;
      })(),
      // Dense search (skip if sparse-only or no embedding)
      (async () => {
        if (config.denseTopK === 0 || !queryEmbedding) {
          denseTime = 0;
          return [];
        }
        const result = await this.milvus.search(
          store,
          queryEmbedding,
          config.denseTopK,
          pathPrefix,
          languages,
        );
        denseTime = Date.now() - denseStart;
        return result;
      })(),
    ]);

    // Build content map for hybrid ranking
    const contentMap = new Map<string, { content: string; symbols: string[] }>();
    for (const result of sparseResultsRaw) {
      contentMap.set(result.doc_id, {
        content: result.content,
        symbols: result.symbols,
      });
    }

    // Fuse results using RRF with config weights
    const fusedResults = this.hybridRanker.fuseResults(
      sparseResultsRaw,
      denseResultsRaw,
      contentMap,
      query,
      {
        sparseWeight: config.sparseWeight,
        denseWeight: config.denseWeight,
        groupByFile,
      },
    );

    return {
      fusedResults,
      sparseResults: sparseResultsRaw,
      denseResults: denseResultsRaw,
      sparseTime,
      denseTime,
    };
  }
}
