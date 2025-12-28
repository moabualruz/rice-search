import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { EmbeddingsService } from "../services/embeddings.service";
import { MilvusService } from "../services/milvus.service";
import { TantivyService } from "../services/tantivy.service";
import { HybridRankerService } from "../services/hybrid-ranker.service";
import { StoreManagerService } from "../services/store-manager.service";
import { RerankerService } from "../services/reranker.service";
import { QueryClassifierService } from "../services/query-classifier.service";
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

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private milvus: MilvusService,
    private tantivy: TantivyService,
    private hybridRanker: HybridRankerService,
    private storeManager: StoreManagerService,
    private reranker: RerankerService,
    private queryClassifier: QueryClassifierService,
  ) {
    this.sparseTopK = this.configService.get<number>("search.sparseTopK")!;
    this.denseTopK = this.configService.get<number>("search.denseTopK")!;
    
    this.logger.log("Search service initialized (Tantivy BM25 + Milvus vectors)");
  }

  async search(store: string, request: SearchRequestDto) {
    const startTime = Date.now();
    const { query, top_k = 20, filters, group_by_file } = request;
    let rerankTimeMs = 0;
    let rerankTimedOut = false;

    // Normalize path filter for consistent matching (handles Windows paths)
    const normalizedPathPrefix = filters?.path_prefix
      ? normalizePath(filters.path_prefix)
      : undefined;

    // Ensure store exists
    this.storeManager.ensureStore(store);

    // Auto-optimize weights based on query type if enabled
    let sparseWeight = request.sparse_weight;
    let denseWeight = request.dense_weight;

    if (request.auto_optimize !== false) {
      const classification = this.queryClassifier.classify(query);
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

    // Execute hybrid search: Tantivy (BM25) + Milvus (vectors)
    const fusedResults = await this.executeHybridSearch(
      store,
      query,
      normalizedPathPrefix,
      filters?.languages,
      sparseWeight,
      denseWeight,
      group_by_file,
    );

    // Apply reranking if enabled
    let rankedResults = fusedResults;
    if (request.enable_reranking !== false && fusedResults.length > 0) {
      const rerankStart = Date.now();
      try {
        rankedResults = await this.reranker.rerank(query, fusedResults, {
          candidates: request.rerank_candidates,
          timeout: this.configService.get<number>("reranker.timeout"),
        });
        rerankTimeMs = Date.now() - rerankStart;
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : String(error);
        this.logger.warn(`Reranking failed, using RRF results: ${errorMessage}`);
        rerankTimeMs = Date.now() - rerankStart;
        rerankTimedOut = true;
      }
    }

    // Limit to top_k
    const finalResults = rankedResults.slice(0, top_k);
    const searchTimeMs = Date.now() - startTime;

    return {
      query,
      results: finalResults.map((r) => ({
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
      search_time_ms: searchTimeMs,
      reranking: {
        enabled: request.enable_reranking !== false,
        candidates: request.rerank_candidates || 30,
        latency_ms: rerankTimeMs,
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
}
