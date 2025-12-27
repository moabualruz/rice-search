import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { MilvusClient, DataType, MetricType } from '@zilliz/milvus2-sdk-node';

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

@Injectable()
export class MilvusService implements OnModuleInit {
  private readonly logger = new Logger(MilvusService.name);
  private client: MilvusClient;
  private readonly COLLECTION_PREFIX = 'lcs_';
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
}
