import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { EmbeddingsService } from "../services/embeddings.service";
import { BgeM3Service } from "../services/bge-m3.service";
import { MilvusService, convertSparseToMilvusFormat } from "../services/milvus.service";
import { TantivyService } from "../services/tantivy.service";
import { HybridRankerService } from "../services/hybrid-ranker.service";
import { StoreManagerService } from "../services/store-manager.service";
import { RerankerService } from "../services/reranker.service";
import { QueryClassifierService } from "../services/query-classifier.service";
import { SearchRequestDto, SearchMode } from "./dto/search-request.dto";

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
  private readonly defaultMode: SearchMode;

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private bgeM3: BgeM3Service,
    private milvus: MilvusService,
    private tantivy: TantivyService,
    private hybridRanker: HybridRankerService,
    private storeManager: StoreManagerService,
    private reranker: RerankerService,
    private queryClassifier: QueryClassifierService,
  ) {
    this.sparseTopK = this.configService.get<number>("search.sparseTopK")!;
    this.denseTopK = this.configService.get<number>("search.denseTopK")!;
    this.defaultMode = (this.configService.get<string>("search.mode") as SearchMode) || "mixedbread";
  }

  async search(store: string, request: SearchRequestDto) {
    const startTime = Date.now();
    const { query, top_k = 20, filters, group_by_file } = request;
    let rerankTimeMs = 0;
    let rerankTimedOut = false;

    // Determine search mode
    const mode = request.mode || this.defaultMode;

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

    // Execute search based on mode
    let fusedResults;
    if (mode === "bge-m3") {
      fusedResults = await this.searchBgeM3Mode(
        store,
        query,
        normalizedPathPrefix,
        filters?.languages,
        sparseWeight,
        denseWeight,
        group_by_file,
      );
    } else {
      // Default: mixedbread mode (uses Infinity + Tantivy)
      fusedResults = await this.searchMixedbreadMode(
        store,
        query,
        normalizedPathPrefix,
        filters?.languages,
        sparseWeight,
        denseWeight,
        group_by_file,
      );
    }

    // After fusion, apply reranking if enabled
    if (request.enable_reranking !== false && fusedResults.length > 0) {
      const rerankStart = Date.now();
      try {
        fusedResults = await this.reranker.rerank(query, fusedResults, {
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
    const finalResults = fusedResults.slice(0, top_k);

    const searchTimeMs = Date.now() - startTime;

    return {
      query,
      mode,
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
   * Mixedbread mode: Uses Infinity embeddings + Tantivy BM25
   */
  private async searchMixedbreadMode(
    store: string,
    query: string,
    pathPrefix?: string,
    languages?: string[],
    sparseWeight?: number,
    denseWeight?: number,
    groupByFile?: boolean,
  ) {
    // Get embedding from Infinity (via EmbeddingsService)
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
   * BGE-M3 mode: Uses Milvus hybrid search with both dense and sparse vectors
   * 
   * IMPORTANT: This mode requires data to be indexed with BGE-M3 mode.
   * Use `SEARCH_MODE=bge-m3` when starting the API to index with BGE-M3.
   * 
   * Unlike mixedbread mode, this does NOT use Tantivy.
   * Both dense and sparse search happen in Milvus with RRF fusion.
   * Content is stored in Milvus for reranking support.
   */
  private async searchBgeM3Mode(
    store: string,
    query: string,
    pathPrefix?: string,
    languages?: string[],
    _sparseWeight?: number,
    _denseWeight?: number,
    _groupByFile?: boolean,
  ) {
    // Get both dense and sparse embeddings from BGE-M3
    const { dense, sparse } = await this.bgeM3.embedBoth([query]);
    const queryDense = dense[0];
    const querySparse = convertSparseToMilvusFormat(sparse[0]);

    // Perform hybrid search in Milvus (RRF fusion happens in Milvus)
    // Content and symbols are returned directly from Milvus
    const hybridResults = await this.milvus.hybridSearch(
      store,
      queryDense,
      querySparse,
      this.sparseTopK + this.denseTopK, // Get more results since RRF will filter
      pathPrefix,
      languages,
    );

    // Convert hybrid results to the format expected by the rest of the pipeline
    return hybridResults.map((result, index) => ({
      doc_id: result.doc_id,
      path: result.path,
      language: result.language,
      start_line: result.start_line,
      end_line: result.end_line,
      content: result.content,
      symbols: result.symbols,
      final_score: result.hybrid_score,
      sparse_score: 0, // RRF doesn't provide individual scores
      dense_score: 0,
      sparse_rank: index + 1,
      dense_rank: index + 1,
    }));
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
