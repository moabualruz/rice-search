import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { EmbeddingsService } from '../services/embeddings.service';
import { MilvusService } from '../services/milvus.service';
import { TantivyService } from '../services/tantivy.service';
import { HybridRankerService } from '../services/hybrid-ranker.service';
import { StoreManagerService } from '../services/store-manager.service';
import { SearchRequestDto } from './dto/search-request.dto';

/**
 * Normalize path to use forward slashes consistently.
 * Handles Windows paths (backslashes) for consistent filtering.
 */
function normalizePath(filePath: string): string {
  return filePath.replace(/\\/g, '/');
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
  ) {
    this.sparseTopK = this.configService.get<number>('search.sparseTopK')!;
    this.denseTopK = this.configService.get<number>('search.denseTopK')!;
  }

  async search(store: string, request: SearchRequestDto) {
    const startTime = Date.now();
    const { query, top_k = 20, filters, sparse_weight, dense_weight, group_by_file } = request;

    // Normalize path filter for consistent matching (handles Windows paths)
    const normalizedPathPrefix = filters?.path_prefix
      ? normalizePath(filters.path_prefix)
      : undefined;

    // Ensure store exists
    this.storeManager.ensureStore(store);

    // Get embedding once (cached after first call)
    const queryEmbeddings = await this.embeddings.embed([query]);
    const queryEmbedding = queryEmbeddings[0];

    // Run sparse and dense search in parallel
    const [sparseResults, denseResults] = await Promise.all([
      this.tantivy.search(
        store,
        query,
        this.sparseTopK,
        normalizedPathPrefix,
        filters?.languages?.[0],
      ),
      queryEmbedding
        ? this.milvus.search(
            store,
            queryEmbedding,
            this.denseTopK,
            normalizedPathPrefix,
            filters?.languages,
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

    // Fuse results
    const fusedResults = this.hybridRanker.fuseResults(
      sparseResults,
      denseResults,
      contentMap,
      query,
      {
        sparseWeight: sparse_weight,
        denseWeight: dense_weight,
        groupByFile: group_by_file,
      },
    );

    // Limit to top_k
    const finalResults = fusedResults.slice(0, top_k);

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
    };
  }
}
