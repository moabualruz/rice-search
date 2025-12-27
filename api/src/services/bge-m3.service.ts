import { Injectable, Logger, OnModuleInit } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import http from "node:http";
import https from "node:https";

interface CacheEntry<T> {
  data: T;
  timestamp: number;
}

interface DenseEmbedding {
  embedding: number[];
  index: number;
}

interface SparseEmbedding {
  weights: Record<string, number>;
  index: number;
}

interface EncodeResponse {
  dense?: DenseEmbedding[];
  sparse?: SparseEmbedding[];
  colbert?: Array<{ vectors: number[][]; index: number }>;
  model: string;
  usage: { texts: number; time_ms: number };
}

interface RerankResult {
  document: string;
  score: number;
  index: number;
}

interface RerankResponse {
  results: RerankResult[];
  model: string;
  usage: { query: number; documents: number; time_ms: number };
}

// Maximize concurrent connections
const httpAgent = new http.Agent({
  keepAlive: true,
  keepAliveMsecs: 30000,
  maxSockets: Infinity,
  maxFreeSockets: 256,
  scheduling: "fifo",
});

const httpsAgent = new https.Agent({
  keepAlive: true,
  keepAliveMsecs: 30000,
  maxSockets: Infinity,
  maxFreeSockets: 256,
  scheduling: "fifo",
});

/**
 * Simple LRU Cache
 */
class LRUCache<K, V> {
  private cache = new Map<K, V>();
  private readonly maxSize: number;

  constructor(maxSize: number) {
    this.maxSize = maxSize;
  }

  get(key: K): V | undefined {
    const value = this.cache.get(key);
    if (value !== undefined) {
      this.cache.delete(key);
      this.cache.set(key, value);
    }
    return value;
  }

  set(key: K, value: V): void {
    if (this.cache.has(key)) {
      this.cache.delete(key);
    } else if (this.cache.size >= this.maxSize) {
      const firstKey = this.cache.keys().next().value;
      if (firstKey !== undefined) {
        this.cache.delete(firstKey);
      }
    }
    this.cache.set(key, value);
  }

  get size(): number {
    return this.cache.size;
  }

  clear(): void {
    this.cache.clear();
  }
}

/**
 * BgeM3Service
 *
 * Client for the BGE-M3 FastAPI service which provides:
 * - Dense embeddings (1024-dim vectors)
 * - Sparse embeddings (lexical weights, replaces BM25)
 * - ColBERT multi-vector reranking
 *
 * In BGE-M3 mode, this service handles both embedding AND sparse search,
 * eliminating the need for Tantivy.
 */
@Injectable()
export class BgeM3Service implements OnModuleInit {
  private readonly logger = new Logger(BgeM3Service.name);
  private readonly baseUrl: string;
  private readonly timeout: number;

  // LRU caches
  private readonly denseCache: LRUCache<string, CacheEntry<number[]>>;
  private readonly sparseCache: LRUCache<string, CacheEntry<Record<string, number>>>;
  private readonly CACHE_TTL_MS = 60 * 60 * 1000; // 1 hour
  private readonly MAX_CACHE_SIZE = 1000;

  // HTTP agent
  private readonly agent: http.Agent | https.Agent;

  constructor(private configService: ConfigService) {
    this.baseUrl = this.configService.get<string>("bgeM3.url") || "http://bge-m3:80";
    this.timeout = this.configService.get<number>("bgeM3.timeout") || 30000;

    this.denseCache = new LRUCache<string, CacheEntry<number[]>>(this.MAX_CACHE_SIZE);
    this.sparseCache = new LRUCache<string, CacheEntry<Record<string, number>>>(this.MAX_CACHE_SIZE);

    this.agent = this.baseUrl.startsWith("https") ? httpsAgent : httpAgent;
  }

  async onModuleInit() {
    try {
      const healthy = await this.healthCheck();
      if (healthy) {
        this.logger.log(`Connected to BGE-M3 service at ${this.baseUrl}`);
      } else {
        this.logger.warn(`BGE-M3 service not available at ${this.baseUrl}. Will retry on first request.`);
      }
    } catch {
      this.logger.warn(`BGE-M3 service not available at ${this.baseUrl}. Service may not be started.`);
    }
  }

  /**
   * Health check
   */
  async healthCheck(): Promise<boolean> {
    try {
      const response = await fetch(`${this.baseUrl}/health`, {
        method: "GET",
        signal: AbortSignal.timeout(5000),
      });
      return response.ok;
    } catch {
      return false;
    }
  }

  /**
   * Get cached dense embedding
   */
  private getCachedDense(text: string): number[] | null {
    const entry = this.denseCache.get(text);
    if (entry && Date.now() - entry.timestamp < this.CACHE_TTL_MS) {
      return entry.data;
    }
    return null;
  }

  /**
   * Get cached sparse embedding
   */
  private getCachedSparse(text: string): Record<string, number> | null {
    const entry = this.sparseCache.get(text);
    if (entry && Date.now() - entry.timestamp < this.CACHE_TTL_MS) {
      return entry.data;
    }
    return null;
  }

  /**
   * Generate dense embeddings (1024-dim)
   * @param texts Array of texts to embed
   * @returns 2D array of embeddings
   */
  async embedDense(texts: string[]): Promise<number[][]> {
    if (texts.length === 0) {
      return [];
    }

    // Check cache for single query
    if (texts.length === 1) {
      const cached = this.getCachedDense(texts[0]);
      if (cached) {
        return [cached];
      }
    }

    // Separate cached and uncached
    const results: (number[] | null)[] = texts.map((t) => this.getCachedDense(t));
    const uncachedIndices: number[] = [];
    const uncachedTexts: string[] = [];

    for (let i = 0; i < results.length; i++) {
      if (results[i] === null) {
        uncachedIndices.push(i);
        uncachedTexts.push(texts[i]);
      }
    }

    if (uncachedTexts.length === 0) {
      return results as number[][];
    }

    try {
      const response = await fetch(`${this.baseUrl}/encode`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          texts: uncachedTexts,
          return_dense: true,
          return_sparse: false,
          return_colbert: false,
          normalize: true,
        }),
        signal: AbortSignal.timeout(this.timeout),
        // @ts-expect-error - Node.js fetch supports dispatcher
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`BGE-M3 encode failed (${response.status}): ${errorText}`);
      }

      const data = (await response.json()) as EncodeResponse;

      if (!data.dense) {
        throw new Error("BGE-M3 did not return dense embeddings");
      }

      // Merge results and cache
      for (let i = 0; i < uncachedIndices.length; i++) {
        const idx = uncachedIndices[i];
        const embedding = data.dense[i].embedding;
        results[idx] = embedding;
        this.denseCache.set(texts[idx], { data: embedding, timestamp: Date.now() });
      }

      return results as number[][];
    } catch (error) {
      this.logger.error(`BGE-M3 dense embedding failed: ${error}`);
      throw error;
    }
  }

  /**
   * Generate sparse embeddings (lexical weights)
   * These can be used for BM25-like scoring without Tantivy
   * @param texts Array of texts to embed
   * @returns Array of token -> weight maps
   */
  async embedSparse(texts: string[]): Promise<Record<string, number>[]> {
    if (texts.length === 0) {
      return [];
    }

    // Check cache for single query
    if (texts.length === 1) {
      const cached = this.getCachedSparse(texts[0]);
      if (cached) {
        return [cached];
      }
    }

    // Separate cached and uncached
    const results: (Record<string, number> | null)[] = texts.map((t) => this.getCachedSparse(t));
    const uncachedIndices: number[] = [];
    const uncachedTexts: string[] = [];

    for (let i = 0; i < results.length; i++) {
      if (results[i] === null) {
        uncachedIndices.push(i);
        uncachedTexts.push(texts[i]);
      }
    }

    if (uncachedTexts.length === 0) {
      return results as Record<string, number>[];
    }

    try {
      const response = await fetch(`${this.baseUrl}/encode`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          texts: uncachedTexts,
          return_dense: false,
          return_sparse: true,
          return_colbert: false,
        }),
        signal: AbortSignal.timeout(this.timeout),
        // @ts-expect-error - Node.js fetch supports dispatcher
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`BGE-M3 sparse encode failed (${response.status}): ${errorText}`);
      }

      const data = (await response.json()) as EncodeResponse;

      if (!data.sparse) {
        throw new Error("BGE-M3 did not return sparse embeddings");
      }

      // Merge results and cache
      for (let i = 0; i < uncachedIndices.length; i++) {
        const idx = uncachedIndices[i];
        const weights = data.sparse[i].weights;
        results[idx] = weights;
        this.sparseCache.set(texts[idx], { data: weights, timestamp: Date.now() });
      }

      return results as Record<string, number>[];
    } catch (error) {
      this.logger.error(`BGE-M3 sparse embedding failed: ${error}`);
      throw error;
    }
  }

  /**
   * Generate both dense and sparse embeddings in one call
   * More efficient than calling embedDense and embedSparse separately
   * @param texts Array of texts
   * @returns Object with dense and sparse embeddings
   */
  async embedBoth(texts: string[]): Promise<{
    dense: number[][];
    sparse: Record<string, number>[];
  }> {
    if (texts.length === 0) {
      return { dense: [], sparse: [] };
    }

    try {
      const response = await fetch(`${this.baseUrl}/encode`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          texts,
          return_dense: true,
          return_sparse: true,
          return_colbert: false,
          normalize: true,
        }),
        signal: AbortSignal.timeout(this.timeout),
        // @ts-expect-error - Node.js fetch supports dispatcher
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`BGE-M3 encode failed (${response.status}): ${errorText}`);
      }

      const data = (await response.json()) as EncodeResponse;

      if (!data.dense || !data.sparse) {
        throw new Error("BGE-M3 did not return both dense and sparse embeddings");
      }

      const dense = data.dense.map((d) => d.embedding);
      const sparse = data.sparse.map((s) => s.weights);

      // Cache both
      for (let i = 0; i < texts.length; i++) {
        this.denseCache.set(texts[i], { data: dense[i], timestamp: Date.now() });
        this.sparseCache.set(texts[i], { data: sparse[i], timestamp: Date.now() });
      }

      return { dense, sparse };
    } catch (error) {
      this.logger.error(`BGE-M3 combined embedding failed: ${error}`);
      throw error;
    }
  }

  /**
   * Rerank documents using ColBERT scoring
   * @param query Query text
   * @param documents Array of document contents
   * @param topK Number of results to return
   * @returns Sorted rerank results with scores
   */
  async rerank(
    query: string,
    documents: string[],
    topK?: number
  ): Promise<Array<{ document: string; score: number; index: number }>> {
    if (documents.length === 0) {
      return [];
    }

    try {
      const response = await fetch(`${this.baseUrl}/rerank`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          query,
          documents,
          top_k: topK,
        }),
        signal: AbortSignal.timeout(this.timeout),
        // @ts-expect-error - Node.js fetch supports dispatcher
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`BGE-M3 rerank failed (${response.status}): ${errorText}`);
      }

      const data = (await response.json()) as RerankResponse;
      return data.results;
    } catch (error) {
      this.logger.error(`BGE-M3 rerank failed: ${error}`);
      throw error;
    }
  }

  /**
   * Compute sparse similarity score between query and document weights
   * This is used for BM25-like scoring when Tantivy is not available
   * @param queryWeights Query lexical weights
   * @param docWeights Document lexical weights
   * @returns Similarity score
   */
  computeSparseScore(
    queryWeights: Record<string, number>,
    docWeights: Record<string, number>
  ): number {
    let score = 0;
    for (const [token, queryWeight] of Object.entries(queryWeights)) {
      const docWeight = docWeights[token];
      if (docWeight !== undefined) {
        score += queryWeight * docWeight;
      }
    }
    return score;
  }

  /**
   * Get embedding dimension (1024 for BGE-M3)
   */
  getDimension(): number {
    return 1024;
  }

  /**
   * Get cache statistics
   */
  getCacheStats(): {
    dense: { size: number; maxSize: number };
    sparse: { size: number; maxSize: number };
  } {
    return {
      dense: { size: this.denseCache.size, maxSize: this.MAX_CACHE_SIZE },
      sparse: { size: this.sparseCache.size, maxSize: this.MAX_CACHE_SIZE },
    };
  }

  /**
   * Clear all caches
   */
  clearCache(): void {
    this.denseCache.clear();
    this.sparseCache.clear();
    this.logger.log("BGE-M3 caches cleared");
  }
}
