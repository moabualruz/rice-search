import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { 
  MilvusClient, 
  DataType, 
  MetricType,
  IndexType,
  RRFRanker,
} from '@zilliz/milvus2-sdk-node';

export interface MilvusSearchResult {
  doc_id: string;
  path: string;
  language: string;
  chunk_id: number;
  start_line: number;
  end_line: number;
  dense_score: number;
  dense_rank: number;
}

export interface MilvusHybridSearchResult extends MilvusSearchResult {
  // Combined score from RRF fusion
  hybrid_score: number;
}

/**
 * Sparse vector as dictionary of string keys (token IDs) to float values
 * Keys must be stringified integers for Milvus
 */
export type SparseVector = Record<string, number>;

/**
 * Convert token string to integer ID using FNV-1a hash
 * Milvus sparse vectors require integer keys
 */
function tokenToId(token: string): number {
  let hash = 2166136261; // FNV offset basis
  for (let i = 0; i < token.length; i++) {
    hash ^= token.charCodeAt(i);
    hash = Math.imul(hash, 16777619); // FNV prime
  }
  // Ensure positive 32-bit integer
  return hash >>> 0;
}

/**
 * Convert BGE-M3 sparse weights (token -> weight) to Milvus format (stringified int -> weight)
 */
export function convertSparseToMilvusFormat(weights: Record<string, number>): SparseVector {
  const result: SparseVector = {};
  for (const [token, weight] of Object.entries(weights)) {
    const id = tokenToId(token);
    result[String(id)] = weight;
  }
  return result;
}

@Injectable()
export class MilvusService implements OnModuleInit {
  private readonly logger = new Logger(MilvusService.name);
  private client: MilvusClient;
  private readonly COLLECTION_PREFIX = 'lcs_';
  private readonly HYBRID_COLLECTION_PREFIX = 'lcs_hybrid_'; // For BGE-M3 mode with sparse vectors
  private dim: number;

  // Cache collection existence to avoid repeated checks
  private readonly collectionCache = new Map<string, boolean>();
  private readonly CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes
  private readonly collectionCacheTimestamps = new Map<string, number>();

  constructor(private configService: ConfigService) {
    const host = this.configService.get<string>('milvus.host')!;
    const port = this.configService.get<number>('milvus.port')!;
    this.dim = this.configService.get<number>('embeddings.dim')!;

    this.client = new MilvusClient({
      address: `${host}:${port}`,
      // Connection pool settings for high concurrency
      maxRetries: 3,
      retryDelay: 100,
      timeout: 30000,
    });
  }

  async onModuleInit() {
    try {
      const health = await this.client.checkHealth();
      if (health.isHealthy) {
        this.logger.log('Connected to Milvus');
        // Pre-warm collection cache
        await this.warmCollectionCache();
      }
    } catch (error) {
      this.logger.warn(`Milvus not available. Will retry on first request.`);
    }
  }

  /**
   * Pre-warm the collection cache on startup
   */
  private async warmCollectionCache(): Promise<void> {
    try {
      const collections = await this.client.listCollections();
      for (const col of collections.data) {
        const colData = col as { name?: string } | string;
        if (typeof colData === 'string' && colData.startsWith(this.COLLECTION_PREFIX)) {
          const store = colData.replace(this.COLLECTION_PREFIX, '');
          this.setCollectionCached(store, true);
        } else if (typeof colData === 'object' && colData.name?.startsWith(this.COLLECTION_PREFIX)) {
          const store = colData.name.replace(this.COLLECTION_PREFIX, '');
          this.setCollectionCached(store, true);
        }
      }
      this.logger.log(`Collection cache warmed with ${this.collectionCache.size} collections`);
    } catch (error) {
      this.logger.warn(`Failed to warm collection cache: ${error}`);
    }
  }

  /**
   * Check if collection exists (cached)
   */
  private isCollectionCached(store: string): boolean | null {
    const cached = this.collectionCache.get(store);
    const timestamp = this.collectionCacheTimestamps.get(store);
    
    if (cached !== undefined && timestamp && Date.now() - timestamp < this.CACHE_TTL_MS) {
      return cached;
    }
    return null;
  }

  /**
   * Update collection cache
   */
  private setCollectionCached(store: string, exists: boolean): void {
    this.collectionCache.set(store, exists);
    this.collectionCacheTimestamps.set(store, Date.now());
  }

  /**
   * Invalidate collection cache entry
   */
  private invalidateCollectionCache(store: string): void {
    this.collectionCache.delete(store);
    this.collectionCacheTimestamps.delete(store);
  }

  private collectionName(store: string): string {
    return `${this.COLLECTION_PREFIX}${store}`;
  }

  async createCollection(store: string): Promise<boolean> {
    const name = this.collectionName(store);

    // Check if exists
    const exists = await this.client.hasCollection({ collection_name: name });
    if (exists.value) {
      this.logger.log(`Collection ${name} already exists`);
      return false;
    }

    // Create collection with schema
    await this.client.createCollection({
      collection_name: name,
      fields: [
        {
          name: 'doc_id',
          data_type: DataType.VarChar,
          is_primary_key: true,
          max_length: 512,
        },
        {
          name: 'embedding',
          data_type: DataType.FloatVector,
          dim: this.dim,
        },
        {
          name: 'path',
          data_type: DataType.VarChar,
          max_length: 1024,
        },
        {
          name: 'language',
          data_type: DataType.VarChar,
          max_length: 64,
        },
        {
          name: 'chunk_id',
          data_type: DataType.Int64,
        },
        {
          name: 'start_line',
          data_type: DataType.Int64,
        },
        {
          name: 'end_line',
          data_type: DataType.Int64,
        },
      ],
    });

    // Create HNSW index
    await this.client.createIndex({
      collection_name: name,
      field_name: 'embedding',
      index_type: 'HNSW',
      metric_type: MetricType.COSINE,
      params: { M: 16, efConstruction: 200 },
    });

    // Load collection
    await this.client.loadCollection({ collection_name: name });

    // Update cache
    this.setCollectionCached(store, true);

    this.logger.log(`Created collection ${name} with HNSW index`);
    return true;
  }

  async dropCollection(store: string): Promise<boolean> {
    const name = this.collectionName(store);
    const exists = await this.client.hasCollection({ collection_name: name });

    if (exists.value) {
      await this.client.dropCollection({ collection_name: name });
      // Invalidate cache
      this.invalidateCollectionCache(store);
      this.logger.log(`Dropped collection ${name}`);
      return true;
    }
    return false;
  }

  async collectionExists(store: string): Promise<boolean> {
    // Check cache first
    const cached = this.isCollectionCached(store);
    if (cached !== null) {
      return cached;
    }

    // Query Milvus
    const result = await this.client.hasCollection({
      collection_name: this.collectionName(store),
    });
    const exists = Boolean(result.value);
    
    // Update cache
    this.setCollectionCached(store, exists);
    return exists;
  }

  async getCollectionStats(
    store: string,
  ): Promise<{ exists: boolean; count: number }> {
    const name = this.collectionName(store);
    const exists = await this.client.hasCollection({ collection_name: name });

    if (!exists.value) {
      return { exists: false, count: 0 };
    }

    const stats = await this.client.getCollectionStatistics({
      collection_name: name,
    });

    return {
      exists: true,
      count: parseInt(stats.data.row_count || '0', 10),
    };
  }

  async upsert(
    store: string,
    data: {
      doc_ids: string[];
      embeddings: number[][];
      paths: string[];
      languages: string[];
      chunk_ids: number[];
      start_lines: number[];
      end_lines: number[];
    },
  ): Promise<number> {
    const name = this.collectionName(store);

    // Ensure collection exists
    const exists = await this.collectionExists(store);
    if (!exists) {
      await this.createCollection(store);
    }

    // Prepare insert data
    const insertData = data.doc_ids.map((doc_id, i) => ({
      doc_id,
      embedding: data.embeddings[i],
      path: data.paths[i],
      language: data.languages[i],
      chunk_id: data.chunk_ids[i],
      start_line: data.start_lines[i],
      end_line: data.end_lines[i],
    }));

    // Delete existing docs with same IDs (upsert)
    if (data.doc_ids.length > 0) {
      const expr = `doc_id in [${data.doc_ids.map((id) => `"${id}"`).join(',')}]`;
      try {
        await this.client.delete({ collection_name: name, filter: expr });
      } catch {
        // Ignore delete errors (documents might not exist)
      }
    }

    // Insert
    const result = await this.client.insert({
      collection_name: name,
      data: insertData,
    });

    return typeof result.insert_cnt === 'string'
      ? parseInt(result.insert_cnt, 10)
      : result.insert_cnt;
  }

  async deleteByDocIds(store: string, docIds: string[]): Promise<number> {
    const name = this.collectionName(store);
    const exists = await this.collectionExists(store);

    if (!exists || docIds.length === 0) {
      return 0;
    }

    const expr = `doc_id in [${docIds.map((id) => `"${id}"`).join(',')}]`;
    const result = await this.client.delete({
      collection_name: name,
      filter: expr,
    });

    return typeof result.delete_cnt === 'string'
      ? parseInt(result.delete_cnt, 10)
      : result.delete_cnt;
  }

  async deleteByPathPrefix(store: string, pathPrefix: string): Promise<number> {
    const name = this.collectionName(store);
    const exists = await this.collectionExists(store);

    if (!exists) {
      return 0;
    }

    const expr = `path like "${pathPrefix}%"`;
    try {
      const result = await this.client.delete({
        collection_name: name,
        filter: expr,
      });
      return typeof result.delete_cnt === 'string'
        ? parseInt(result.delete_cnt, 10)
        : result.delete_cnt;
    } catch (error) {
      this.logger.warn(`Path prefix delete failed: ${error}`);
      return 0;
    }
  }

  async search(
    store: string,
    queryEmbedding: number[],
    topK = 80,
    pathPrefix?: string,
    languages?: string[],
  ): Promise<MilvusSearchResult[]> {
    const name = this.collectionName(store);
    
    // Use cached collection check for hot path
    const cached = this.isCollectionCached(store);
    if (cached === false) {
      return [];
    }
    
    // Only query Milvus if not cached or cache says exists
    if (cached === null) {
      const exists = await this.collectionExists(store);
      if (!exists) {
        return [];
      }
    }

    // Build filter expression
    const filters: string[] = [];
    if (pathPrefix) {
      filters.push(`path like "%${pathPrefix}%"`);
    }
    if (languages && languages.length > 0) {
      const langList = languages.map((l) => `"${l}"`).join(',');
      filters.push(`language in [${langList}]`);
    }

    const expr = filters.length > 0 ? filters.join(' and ') : undefined;

    // Search
    const result = await this.client.search({
      collection_name: name,
      data: [queryEmbedding],
      limit: topK,
      filter: expr,
      output_fields: [
        'doc_id',
        'path',
        'language',
        'chunk_id',
        'start_line',
        'end_line',
      ],
      params: { ef: Math.max(64, topK * 2) },
    });

    // Format results
    return result.results.map((hit, index) => ({
      doc_id: hit.doc_id as string,
      path: hit.path as string,
      language: hit.language as string,
      chunk_id: hit.chunk_id as number,
      start_line: hit.start_line as number,
      end_line: hit.end_line as number,
      dense_score: hit.score,
      dense_rank: index + 1,
    }));
  }

  // ============================================
  // BGE-M3 Hybrid Collection Methods
  // These use both dense and sparse vectors
  // ============================================

  private hybridCollectionName(store: string): string {
    return `${this.HYBRID_COLLECTION_PREFIX}${store}`;
  }

  /**
   * Create a hybrid collection with both dense and sparse vector fields
   * Used for BGE-M3 mode where we store both embedding types
   * Also stores content for reranking (no Tantivy dependency)
   */
  async createHybridCollection(store: string): Promise<boolean> {
    const name = this.hybridCollectionName(store);

    // Check if exists
    const exists = await this.client.hasCollection({ collection_name: name });
    if (exists.value) {
      this.logger.log(`Hybrid collection ${name} already exists`);
      return false;
    }

    // Create collection with both dense and sparse vector fields + content
    await this.client.createCollection({
      collection_name: name,
      fields: [
        {
          name: 'doc_id',
          data_type: DataType.VarChar,
          is_primary_key: true,
          max_length: 512,
        },
        {
          name: 'dense_vector',
          data_type: DataType.FloatVector,
          dim: this.dim,
        },
        {
          name: 'sparse_vector',
          data_type: DataType.SparseFloatVector,
          // No dimension needed for sparse vectors
        },
        {
          name: 'path',
          data_type: DataType.VarChar,
          max_length: 1024,
        },
        {
          name: 'language',
          data_type: DataType.VarChar,
          max_length: 64,
        },
        {
          name: 'chunk_id',
          data_type: DataType.Int64,
        },
        {
          name: 'start_line',
          data_type: DataType.Int64,
        },
        {
          name: 'end_line',
          data_type: DataType.Int64,
        },
        {
          name: 'content',
          data_type: DataType.VarChar,
          max_length: 65535, // Max content length
        },
        {
          name: 'symbols',
          data_type: DataType.VarChar,
          max_length: 8192, // JSON array of symbols
        },
      ],
    });

    // Create indices for both vector types
    await this.client.createIndex([
      // Dense vector index (HNSW with cosine similarity)
      {
        collection_name: name,
        field_name: 'dense_vector',
        index_type: IndexType.HNSW,
        metric_type: MetricType.COSINE,
        params: { M: 16, efConstruction: 200 },
      },
      // Sparse vector index (inverted index with inner product)
      {
        collection_name: name,
        field_name: 'sparse_vector',
        index_type: IndexType.SPARSE_INVERTED_INDEX,
        metric_type: MetricType.IP, // Only IP supported for sparse
        params: { drop_ratio_build: 0.2 }, // Drop smallest 20% during build
      },
    ]);

    // Load collection
    await this.client.loadCollection({ collection_name: name });

    // Update cache
    this.setCollectionCached(`hybrid_${store}`, true);

    this.logger.log(`Created hybrid collection ${name} with dense + sparse indices`);
    return true;
  }

  async dropHybridCollection(store: string): Promise<boolean> {
    const name = this.hybridCollectionName(store);
    const exists = await this.client.hasCollection({ collection_name: name });

    if (exists.value) {
      await this.client.dropCollection({ collection_name: name });
      this.invalidateCollectionCache(`hybrid_${store}`);
      this.logger.log(`Dropped hybrid collection ${name}`);
      return true;
    }
    return false;
  }

  async hybridCollectionExists(store: string): Promise<boolean> {
    const cached = this.isCollectionCached(`hybrid_${store}`);
    if (cached !== null) {
      return cached;
    }

    const result = await this.client.hasCollection({
      collection_name: this.hybridCollectionName(store),
    });
    const exists = Boolean(result.value);
    this.setCollectionCached(`hybrid_${store}`, exists);
    return exists;
  }

  /**
   * Upsert data with both dense and sparse vectors
   * Used for BGE-M3 mode indexing
   */
  async upsertHybrid(
    store: string,
    data: {
      doc_ids: string[];
      dense_embeddings: number[][];
      sparse_embeddings: SparseVector[];
      paths: string[];
      languages: string[];
      chunk_ids: number[];
      start_lines: number[];
      end_lines: number[];
      contents?: string[];
      symbols?: string[][];
    },
  ): Promise<number> {
    const name = this.hybridCollectionName(store);

    // Ensure collection exists
    const exists = await this.hybridCollectionExists(store);
    if (!exists) {
      await this.createHybridCollection(store);
    }

    // Prepare insert data
    const insertData = data.doc_ids.map((doc_id, i) => ({
      doc_id,
      dense_vector: data.dense_embeddings[i],
      sparse_vector: data.sparse_embeddings[i],
      path: data.paths[i],
      language: data.languages[i],
      chunk_id: data.chunk_ids[i],
      start_line: data.start_lines[i],
      end_line: data.end_lines[i],
      content: (data.contents?.[i] || '').slice(0, 65000), // Truncate to max length
      symbols: JSON.stringify(data.symbols?.[i] || []),
    }));

    // Delete existing docs with same IDs (upsert)
    if (data.doc_ids.length > 0) {
      const expr = `doc_id in [${data.doc_ids.map((id) => `"${id}"`).join(',')}]`;
      try {
        await this.client.delete({ collection_name: name, filter: expr });
      } catch {
        // Ignore delete errors (documents might not exist)
      }
    }

    // Insert
    const result = await this.client.insert({
      collection_name: name,
      data: insertData,
    });

    return typeof result.insert_cnt === 'string'
      ? parseInt(result.insert_cnt, 10)
      : result.insert_cnt;
  }

  /**
   * Hybrid search using both dense and sparse vectors with RRF fusion
   * This eliminates the need for Tantivy in BGE-M3 mode
   */
  async hybridSearch(
    store: string,
    denseQuery: number[],
    sparseQuery: SparseVector,
    topK = 80,
    pathPrefix?: string,
    languages?: string[],
  ): Promise<(MilvusHybridSearchResult & { content: string; symbols: string[] })[]> {
    const name = this.hybridCollectionName(store);

    // Check collection exists
    const cached = this.isCollectionCached(`hybrid_${store}`);
    if (cached === false) {
      return [];
    }

    if (cached === null) {
      const exists = await this.hybridCollectionExists(store);
      if (!exists) {
        return [];
      }
    }

    // Build filter expression
    const filters: string[] = [];
    if (pathPrefix) {
      filters.push(`path like "%${pathPrefix}%"`);
    }
    if (languages && languages.length > 0) {
      const langList = languages.map((l) => `"${l}"`).join(',');
      filters.push(`language in [${langList}]`);
    }

    const filter = filters.length > 0 ? filters.join(' and ') : undefined;

    // Perform hybrid search with RRF ranker
    const result = await this.client.hybridSearch({
      collection_name: name,
      data: [
        // Dense vector search
        {
          data: [denseQuery],
          anns_field: 'dense_vector',
          params: { ef: Math.max(64, topK * 2) },
        },
        // Sparse vector search  
        {
          data: [sparseQuery],
          anns_field: 'sparse_vector',
          params: { drop_ratio_search: 0.2 },
        },
      ],
      rerank: RRFRanker(60), // k=60 for RRF algorithm
      limit: topK,
      filter,
      output_fields: [
        'doc_id',
        'path',
        'language',
        'chunk_id',
        'start_line',
        'end_line',
        'content',
        'symbols',
      ],
    });

    // Format results
    return result.results.map((hit, index) => {
      // Parse symbols from JSON string
      let symbols: string[] = [];
      try {
        const symbolsStr = hit.symbols as string;
        if (symbolsStr) {
          symbols = JSON.parse(symbolsStr);
        }
      } catch {
        // Ignore parse errors
      }

      return {
        doc_id: hit.doc_id as string,
        path: hit.path as string,
        language: hit.language as string,
        chunk_id: hit.chunk_id as number,
        start_line: hit.start_line as number,
        end_line: hit.end_line as number,
        content: (hit.content as string) || '',
        symbols,
        dense_score: 0, // RRF doesn't provide individual scores
        dense_rank: index + 1,
        hybrid_score: hit.score,
      };
    });
  }

  /**
   * Delete from hybrid collection by doc IDs
   */
  async deleteHybridByDocIds(store: string, docIds: string[]): Promise<number> {
    const name = this.hybridCollectionName(store);
    const exists = await this.hybridCollectionExists(store);

    if (!exists || docIds.length === 0) {
      return 0;
    }

    const expr = `doc_id in [${docIds.map((id) => `"${id}"`).join(',')}]`;
    const result = await this.client.delete({
      collection_name: name,
      filter: expr,
    });

    return typeof result.delete_cnt === 'string'
      ? parseInt(result.delete_cnt, 10)
      : result.delete_cnt;
  }

  /**
   * Delete from hybrid collection by path prefix
   */
  async deleteHybridByPathPrefix(store: string, pathPrefix: string): Promise<number> {
    const name = this.hybridCollectionName(store);
    const exists = await this.hybridCollectionExists(store);

    if (!exists) {
      return 0;
    }

    const expr = `path like "${pathPrefix}%"`;
    try {
      const result = await this.client.delete({
        collection_name: name,
        filter: expr,
      });
      return typeof result.delete_cnt === 'string'
        ? parseInt(result.delete_cnt, 10)
        : result.delete_cnt;
    } catch (error) {
      this.logger.warn(`Hybrid path prefix delete failed: ${error}`);
      return 0;
    }
  }
}
