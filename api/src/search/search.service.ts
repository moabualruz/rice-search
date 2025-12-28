import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { randomUUID } from "crypto";
import { EmbeddingsService } from "../services/embeddings.service";
import { MilvusService, MilvusSearchResult } from "../services/milvus.service";
import { TantivyService, TantivySearchResult } from "../services/tantivy.service";
import { HybridRankerService, HybridSearchResult } from "../services/hybrid-ranker.service";
import { StoreManagerService } from "../services/store-manager.service";
import { RerankerService } from "../services/reranker.service";
import { QueryClassifierService, QueryClassification } from "../services/query-classifier.service";
import { QueryNormalizerService } from "../services/query-normalizer.service";
import { TelemetryService, SearchTelemetryRecord } from "../services/telemetry.service";
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
  private readonly sparseTopK: number;
  private readonly denseTopK: number;
  private readonly defaultRerankCandidates: number;

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private milvus: MilvusService,
    private tantivy: TantivyService,
    private hybridRanker: HybridRankerService,
    private storeManager: StoreManagerService,
    private reranker: RerankerService,
    private queryClassifier: QueryClassifierService,
    private queryNormalizer: QueryNormalizerService,
    private telemetry: TelemetryService,
  ) {
    this.sparseTopK = this.configService.get<number>("search.sparseTopK")!;
    this.denseTopK = this.configService.get<number>("search.denseTopK")!;
    this.defaultRerankCandidates = this.configService.get<number>("rerank.candidates") ?? 30;
    
    this.logger.log("Search service initialized (Tantivy BM25 + Milvus vectors)");
  }

  async search(store: string, request: SearchRequestDto) {
    const requestId = randomUUID();
    const startTime = Date.now();
    const { query, top_k = 20, filters, group_by_file } = request;
    
    // Timing trackers
    let sparseLatencyMs = 0;
    let denseLatencyMs = 0;
    let fusionLatencyMs = 0;
    let rerankLatencyMs = 0;
    let rerankTimedOut = false;
    let rerankSkipped = false;
    let rerankSkipReason: string | undefined;

    // Normalize query for consistent caching
    const normalized = this.queryNormalizer.normalize(query);

    // Normalize path filter for consistent matching (handles Windows paths)
    const normalizedPathPrefix = filters?.path_prefix
      ? normalizePath(filters.path_prefix)
      : undefined;

    // Ensure store exists
    this.storeManager.ensureStore(store);

    // Classify query for adaptive behavior
    const classification = this.queryClassifier.classify(normalized.normalized);

    // Auto-optimize weights based on query type if enabled
    let sparseWeight = request.sparse_weight;
    let denseWeight = request.dense_weight;

    if (request.auto_optimize !== false) {
      const optimizedWeights = this.getOptimizedWeights(classification.type);
      
      // Only override if not explicitly set
      if (sparseWeight === undefined) {
        sparseWeight = optimizedWeights.sparse;
      }
      if (denseWeight === undefined) {
        denseWeight = optimizedWeights.dense;
      }

      this.logger.debug(
        `Query "${query.substring(0, 30)}..." classified as ${classification.type} ` +
        `(confidence: ${classification.confidence.toFixed(2)}), ` +
        `weights: sparse=${sparseWeight}, dense=${denseWeight}`
      );
    }

    // Determine adaptive rerank candidates based on query classification (Task 0.3)
    const rerankCandidates = this.getAdaptiveRerankCandidates(
      classification,
      request.rerank_candidates,
    );

    // Execute hybrid search: Tantivy (BM25) + Milvus (vectors)
    const hybridStartTime = Date.now();
    const { fusedResults, sparseResults, denseResults, sparseTime, denseTime } = 
      await this.executeHybridSearchWithTiming(
        store,
        normalized.normalized,
        normalizedPathPrefix,
        filters?.languages,
        sparseWeight,
        denseWeight,
        group_by_file,
      );
    sparseLatencyMs = sparseTime;
    denseLatencyMs = denseTime;
    fusionLatencyMs = Date.now() - hybridStartTime - Math.max(sparseTime, denseTime);

    // Compute fusion stats for telemetry
    const fusionStats = this.hybridRanker.computeFusionStats(fusedResults);

    // Apply reranking if enabled
    let rankedResults = fusedResults;
    if (request.enable_reranking !== false && fusedResults.length > 0) {
      const rerankStart = Date.now();
      try {
        rankedResults = await this.reranker.rerank(query, fusedResults, {
          candidates: rerankCandidates,
          timeout: this.configService.get<number>("reranker.timeout"),
        });
        rerankLatencyMs = Date.now() - rerankStart;
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        this.logger.warn(`Reranking failed, using RRF results: ${errorMessage}`);
        rerankLatencyMs = Date.now() - rerankStart;
        rerankTimedOut = true;
      }
    } else if (fusedResults.length === 0) {
      rerankSkipped = true;
      rerankSkipReason = "no_results";
    } else if (request.enable_reranking === false) {
      rerankSkipped = true;
      rerankSkipReason = "disabled";
    }

    // Limit to top_k
    const finalResults = rankedResults.slice(0, top_k);
    const totalLatencyMs = Date.now() - startTime;

    // Compute score stats for telemetry
    const sparseScores = sparseResults.map((r: TantivySearchResult) => r.bm25_score);
    const denseScores = denseResults.map((r: MilvusSearchResult) => r.dense_score);
    const sparseStats = this.telemetry.computeScoreStats(sparseScores);
    const denseStats = this.telemetry.computeScoreStats(denseScores);

    // Record telemetry
    const telemetryRecord: SearchTelemetryRecord = {
      requestId,
      timestamp: new Date(),
      store,
      query: normalized.normalized,
      queryType: classification.type,
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
        candidates: rerankCandidates,
        latencyMs: rerankLatencyMs,
        timedOut: rerankTimedOut,
        skipped: rerankSkipped,
        skipReason: rerankSkipReason,
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
      results: finalResults.map((r: HybridSearchResult) => ({
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
      })),
      total: finalResults.length,
      store,
      search_time_ms: totalLatencyMs,
      reranking: {
        enabled: request.enable_reranking !== false,
        candidates: rerankCandidates,
        latency_ms: rerankLatencyMs,
        timed_out: rerankTimedOut,
      },
    };
  }

  /**
   * Execute hybrid search using Tantivy (BM25) + Milvus (vectors)
   * Results are fused using RRF (Reciprocal Rank Fusion)
   */
  private async executeHybridSearch(
    store: string,
    query: string,
    pathPrefix?: string,
    languages?: string[],
    sparseWeight?: number,
    denseWeight?: number,
    groupByFile?: boolean,
  ) {
    // Get embedding from Infinity
    const queryEmbeddings = await this.embeddings.embed([query]);
    const queryEmbedding = queryEmbeddings[0];

    // Run sparse (Tantivy) and dense (Milvus) search in parallel
    const [sparseResults, denseResults] = await Promise.all([
      this.tantivy.search(
        store,
        query,
        this.sparseTopK,
        pathPrefix,
        languages?.[0],
      ),
      queryEmbedding
        ? this.milvus.search(
            store,
            queryEmbedding,
            this.denseTopK,
            pathPrefix,
            languages,
          )
        : Promise.resolve([]),
    ]);

    // Build content map for hybrid ranking
    const contentMap = new Map<string, { content: string; symbols: string[] }>();
    for (const result of sparseResults) {
      contentMap.set(result.doc_id, {
        content: result.content,
        symbols: result.symbols,
      });
    }

    // Fuse results using RRF
    return this.hybridRanker.fuseResults(
      sparseResults,
      denseResults,
      contentMap,
      query,
      {
        sparseWeight,
        denseWeight,
        groupByFile,
      },
    );
  }

  /**
   * Get optimized weights based on query type
   * Code queries → favor sparse (BM25 better for exact matches)
   * Natural language → favor dense (semantic better for concepts)
   * Hybrid → balanced
   */
  private getOptimizedWeights(queryType: "code" | "natural" | "hybrid"): {
    sparse: number;
    dense: number;
  } {
    switch (queryType) {
      case "code":
        // Code queries: BM25 excels at exact symbol/keyword matching
        return { sparse: 0.7, dense: 0.3 };
      case "natural":
        // Natural language: semantic search better for concepts
        return { sparse: 0.3, dense: 0.7 };
      case "hybrid":
      default:
        // Balanced for mixed queries
        return { sparse: 0.5, dense: 0.5 };
    }
  }

  /**
   * Determine adaptive rerank candidates based on query classification (Task 0.3)
   * Navigational queries → fewer candidates (exact match expected)
   * Exploratory queries → more candidates (cast wider net)
   * High specificity → fewer candidates
   */
  private getAdaptiveRerankCandidates(
    classification: QueryClassification,
    requestCandidates?: number,
  ): number {
    // If explicitly specified in request, honor it
    if (requestCandidates !== undefined) {
      return requestCandidates;
    }

    const { signals, type } = classification;

    // Navigational queries (exact file/symbol lookup) → minimal candidates
    if (signals.isNavigational) {
      return 15;
    }

    // Exploratory queries (broad concept search) → cast wider net
    if (signals.isExploratory) {
      return 50;
    }

    // Code queries with high specificity → fewer candidates needed
    if (type === "code" && signals.specificity >= 0.6) {
      return 20;
    }

    // Natural language with low specificity → more candidates
    if (type === "natural" && signals.specificity < 0.3) {
      return 40;
    }

    // Default to configured value
    return this.defaultRerankCandidates;
  }

  /**
   * Execute hybrid search with timing information
   * Wraps executeHybridSearch and returns individual timing for sparse/dense
   */
  private async executeHybridSearchWithTiming(
    store: string,
    query: string,
    pathPrefix?: string,
    languages?: string[],
    sparseWeight?: number,
    denseWeight?: number,
    groupByFile?: boolean,
  ): Promise<{
    fusedResults: HybridSearchResult[];
    sparseResults: TantivySearchResult[];
    denseResults: MilvusSearchResult[];
    sparseTime: number;
    denseTime: number;
  }> {
    // Get embedding from Infinity
    const queryEmbeddings = await this.embeddings.embed([query]);
    const queryEmbedding = queryEmbeddings[0];

    // Run sparse (Tantivy) and dense (Milvus) search in parallel with timing
    const sparseStart = Date.now();
    let sparseTime = 0;
    const denseStart = Date.now();
    let denseTime = 0;

    const [sparseResultsRaw, denseResultsRaw] = await Promise.all([
      (async () => {
        const result = await this.tantivy.search(
          store,
          query,
          this.sparseTopK,
          pathPrefix,
          languages?.[0],
        );
        sparseTime = Date.now() - sparseStart;
        return result;
      })(),
      (async () => {
        if (!queryEmbedding) {
          denseTime = Date.now() - denseStart;
          return [];
        }
        const result = await this.milvus.search(
          store,
          queryEmbedding,
          this.denseTopK,
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

    // Fuse results using RRF
    const fusedResults = this.hybridRanker.fuseResults(
      sparseResultsRaw,
      denseResultsRaw,
      contentMap,
      query,
      {
        sparseWeight,
        denseWeight,
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
