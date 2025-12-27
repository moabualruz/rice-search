import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import http from 'node:http';
import https from 'node:https';

interface CacheEntry<T> {
  data: T;
  timestamp: number;
}

interface RerankDocument {
  doc_id: string;
  content: string;
}

interface RerankResult {
  doc_id: string;
  score: number;
  index: number;
}

// Maximize concurrent connections to Infinity server
const httpAgent = new http.Agent({
  keepAlive: true,
  keepAliveMsecs: 30000,
  maxSockets: Infinity, // No limit
  maxFreeSockets: 256,
  scheduling: 'fifo',
});

const httpsAgent = new https.Agent({
  keepAlive: true,
  keepAliveMsecs: 30000,
  maxSockets: Infinity, // No limit
  maxFreeSockets: 256,
  scheduling: 'fifo',
});

/**
 * Simple LRU Cache for embeddings and rerank results
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
      // Move to end (most recently used)
      this.cache.delete(key);
      this.cache.set(key, value);
    }
    return value;
  }

  set(key: K, value: V): void {
    if (this.cache.has(key)) {
      this.cache.delete(key);
    } else if (this.cache.size >= this.maxSize) {
      // Delete oldest (first) entry
      const firstKey = this.cache.keys().next().value;
      if (firstKey !== undefined) {
        this.cache.delete(firstKey);
      }
    }
    this.cache.set(key, value);
  }

  has(key: K): boolean {
    return this.cache.has(key);
  }

  get size(): number {
    return this.cache.size;
  }

  clear(): void {
    this.cache.clear();
  }
}

/**
 * InfinityService
 * 
 * Wraps calls to the Infinity server which hosts both:
 * - mixedbread-ai/mxbai-embed-large-v1 (1024-dim embeddings)
 * - mixedbread-ai/mxbai-rerank-base-v2 (neural reranking)
 * 
 * Infinity uses OpenAI-compatible API format:
 * - POST /embeddings with { model, input }
 * - POST /rerank with { model, query, documents, top_n }
 */
@Injectable()
export class InfinityService implements OnModuleInit {
  private readonly logger = new Logger(InfinityService.name);
  private readonly baseUrl: string;
  private readonly embedModel: string;
  private readonly rerankModel: string;
  private readonly timeout: number;

  // LRU caches
  private readonly embeddingCache: LRUCache<string, CacheEntry<number[]>>;
  private readonly rerankCache: LRUCache<string, CacheEntry<RerankResult[]>>;
  private readonly CACHE_TTL_MS = 60 * 60 * 1000; // 1 hour
  private readonly MAX_EMBEDDING_CACHE_SIZE = 1000;
  private readonly MAX_RERANK_CACHE_SIZE = 500;

  // HTTP agent for connection pooling
  private readonly agent: http.Agent | https.Agent;

  constructor(private configService: ConfigService) {
    // Configuration with defaults from RERANKING_PLAN.md
    this.baseUrl = this.configService.get<string>('infinity.url') || 'http://infinity:80';
    this.embedModel = this.configService.get<string>('infinity.embedModel') || 'mixedbread-ai/mxbai-embed-large-v1';
    this.rerankModel = this.configService.get<string>('infinity.rerankModel') || 'mixedbread-ai/mxbai-rerank-base-v2';
    this.timeout = this.configService.get<number>('infinity.timeout') || 30000;

    this.embeddingCache = new LRUCache<string, CacheEntry<number[]>>(this.MAX_EMBEDDING_CACHE_SIZE);
    this.rerankCache = new LRUCache<string, CacheEntry<RerankResult[]>>(this.MAX_RERANK_CACHE_SIZE);
    
    // Use appropriate agent based on URL protocol
    this.agent = this.baseUrl.startsWith('https') ? httpsAgent : httpAgent;
  }

  async onModuleInit() {
    try {
      await this.healthCheck();
      this.logger.log(`Connected to Infinity service at ${this.baseUrl}`);
      const models = await this.getModels();
      this.logger.log(`Loaded models: ${models.join(', ')}`);
    } catch (error) {
      this.logger.warn(
        `Infinity service not available at ${this.baseUrl}. Will retry on first request.`,
      );
    }
  }

  /**
   * Health check for Infinity service
   */
  async healthCheck(): Promise<boolean> {
    try {
      const response = await fetch(`${this.baseUrl}/health`, {
        method: 'GET',
        signal: AbortSignal.timeout(5000),
      });
      return response.ok;
    } catch {
      return false;
    }
  }

  /**
   * Get list of loaded models from Infinity
   */
  async getModels(): Promise<string[]> {
    try {
      const response = await fetch(`${this.baseUrl}/models`, {
        method: 'GET',
        signal: AbortSignal.timeout(this.timeout),
      });
      
      if (!response.ok) {
        this.logger.warn(`Failed to fetch models: ${response.status}`);
        return [this.embedModel, this.rerankModel];
      }
      
      const data = await response.json() as { data: Array<{ id: string }> };
      return data.data.map((m) => m.id);
    } catch (error) {
      this.logger.warn(`Error fetching models: ${error}`);
      return [this.embedModel, this.rerankModel];
    }
  }

  /**
   * Get embedding from cache or compute
   */
  private getCachedEmbedding(text: string): number[] | null {
    const entry = this.embeddingCache.get(text);
    if (entry && Date.now() - entry.timestamp < this.CACHE_TTL_MS) {
      return entry.data;
    }
    return null;
  }

  /**
   * Cache an embedding
   */
  private cacheEmbedding(text: string, embedding: number[]): void {
    this.embeddingCache.set(text, {
      data: embedding,
      timestamp: Date.now(),
    });
  }

  /**
   * Generate cache key for rerank request
   */
  private getRerankCacheKey(query: string, docIds: string[]): string {
    return `${query}::${docIds.join(',')}`;
  }

  /**
   * Get rerank results from cache
   */
  private getCachedRerank(query: string, docIds: string[]): RerankResult[] | null {
    const key = this.getRerankCacheKey(query, docIds);
    const entry = this.rerankCache.get(key);
    if (entry && Date.now() - entry.timestamp < this.CACHE_TTL_MS) {
      return entry.data;
    }
    return null;
  }

  /**
   * Cache rerank results
   */
  private cacheRerank(query: string, docIds: string[], results: RerankResult[]): void {
    const key = this.getRerankCacheKey(query, docIds);
    this.rerankCache.set(key, {
      data: results,
      timestamp: Date.now(),
    });
  }

  /**
   * Generate embeddings for texts using Infinity (OpenAI-compatible format)
   * @param texts Array of texts to embed
   * @returns 2D array of embeddings (1024-dim)
   */
  async embed(texts: string[]): Promise<number[][]> {
    if (texts.length === 0) {
      return [];
    }

    // Check cache for single query (common case for search)
    if (texts.length === 1) {
      const cached = this.getCachedEmbedding(texts[0]);
      if (cached) {
        return [cached];
      }
    }

    // Separate cached and uncached texts
    const results: (number[] | null)[] = texts.map((text) =>
      this.getCachedEmbedding(text),
    );
    const uncachedIndices: number[] = [];
    const uncachedTexts: string[] = [];

    for (let i = 0; i < results.length; i++) {
      if (results[i] === null) {
        uncachedIndices.push(i);
        uncachedTexts.push(texts[i]);
      }
    }

    // If all cached, return immediately
    if (uncachedTexts.length === 0) {
      return results as number[][];
    }

    try {
      const response = await fetch(`${this.baseUrl}/embeddings`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          model: this.embedModel,
          input: uncachedTexts,
        }),
        signal: AbortSignal.timeout(this.timeout),
        // @ts-expect-error - Node.js fetch supports dispatcher for HTTP agent
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Embed request failed (${response.status}): ${errorText}`);
      }

      const data = await response.json() as { 
        data: Array<{ embedding: number[] }>;
      };

      const embeddings = data.data.map((item) => item.embedding);

      // Cache new embeddings and merge results
      for (let i = 0; i < uncachedIndices.length; i++) {
        const idx = uncachedIndices[i];
        const embedding = embeddings[i];
        results[idx] = embedding;
        this.cacheEmbedding(texts[idx], embedding);
      }

      return results as number[][];
    } catch (error) {
      this.logger.error(`Embedding request failed: ${error}`);
      throw error;
    }
  }

  /**
   * Embed large batches with automatic chunking and controlled concurrency
   * @param texts Array of texts to embed
   * @param batchSize Number of texts per batch (default: 32)
   * @returns 2D array of embeddings
   */
  async embedBatch(
    texts: string[],
    batchSize = 32,
  ): Promise<number[][]> {
    if (texts.length === 0) {
      return [];
    }

    // Create batches
    const batches: string[][] = [];
    for (let i = 0; i < texts.length; i += batchSize) {
      batches.push(texts.slice(i, i + batchSize));
    }

    // Process batches with limited concurrency (4 concurrent requests)
    const maxConcurrent = 4;
    const results: number[][][] = [];
    
    for (let i = 0; i < batches.length; i += maxConcurrent) {
      const chunk = batches.slice(i, i + maxConcurrent);
      const chunkResults = await Promise.all(
        chunk.map((batch) => this.embed(batch))
      );
      results.push(...chunkResults);
    }

    // Flatten results
    return results.flat();
  }

  /**
   * Embed with retry logic for transient failures
   * @param texts Array of texts to embed
   * @param maxRetries Maximum number of retry attempts (default: 3)
   * @param retryDelay Initial retry delay in ms (default: 1000)
   * @returns 2D array of embeddings
   */
  async embedWithRetry(
    texts: string[],
    maxRetries = 3,
    retryDelay = 1000,
  ): Promise<number[][]> {
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        return await this.embed(texts);
      } catch (error) {
        lastError = error as Error;
        this.logger.warn(`Embed attempt ${attempt + 1} failed: ${error}`);
        if (attempt < maxRetries - 1) {
          await new Promise((resolve) =>
            setTimeout(resolve, retryDelay * (attempt + 1)),
          );
        }
      }
    }

    throw lastError;
  }

  /**
   * Rerank documents using Infinity's reranking model
   * @param query Search query
   * @param documents Array of documents with doc_id and content
   * @param topN Number of top results to return
   * @returns Array of reranked results with scores
   */
  async rerank(
    query: string,
    documents: RerankDocument[],
    topN: number,
  ): Promise<RerankResult[]> {
    if (documents.length === 0) {
      return [];
    }

    // Check cache
    const docIds = documents.map((d) => d.doc_id);
    const cached = this.getCachedRerank(query, docIds);
    if (cached) {
      return cached.slice(0, topN);
    }

    try {
      const response = await fetch(`${this.baseUrl}/rerank`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          model: this.rerankModel,
          query,
          documents: documents.map((d) => d.content),
          top_n: topN,
          return_documents: false,
        }),
        signal: AbortSignal.timeout(this.timeout),
        // @ts-expect-error - Node.js fetch supports dispatcher for HTTP agent
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Rerank request failed (${response.status}): ${errorText}`);
      }

      const data = await response.json() as {
        results: Array<{ index: number; relevance_score: number }>;
      };

      // Map results back to doc_ids
      const results: RerankResult[] = data.results.map((item) => ({
        doc_id: documents[item.index].doc_id,
        score: item.relevance_score,
        index: item.index,
      }));

      // Cache results
      this.cacheRerank(query, docIds, results);

      return results;
    } catch (error) {
      this.logger.error(`Rerank request failed: ${error}`);
      throw error;
    }
  }

  /**
   * Rerank with retry logic and fallback
   * @param query Search query
   * @param documents Array of documents
   * @param topN Number of top results
   * @param maxRetries Maximum retry attempts
   * @returns Reranked results, or original order on failure
   */
  async rerankWithRetry(
    query: string,
    documents: RerankDocument[],
    topN: number,
    maxRetries = 2,
  ): Promise<RerankResult[]> {
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        return await this.rerank(query, documents, topN);
      } catch (error) {
        lastError = error as Error;
        this.logger.warn(`Rerank attempt ${attempt + 1} failed: ${error}`);
        if (attempt < maxRetries - 1) {
          await new Promise((resolve) => setTimeout(resolve, 500 * (attempt + 1)));
        }
      }
    }

    // Fallback: return documents in original order
    this.logger.warn(`Rerank failed after ${maxRetries} attempts, returning original order`);
    return documents.slice(0, topN).map((doc, index) => ({
      doc_id: doc.doc_id,
      score: 1.0 - (index * 0.01), // Decreasing scores
      index,
    }));
  }

  /**
   * Get embedding dimension (1024 for mxbai-embed-large-v1)
   */
  getDimension(): number {
    return 1024;
  }

  /**
   * Get cache statistics
   */
  getCacheStats(): { 
    embeddings: { size: number; maxSize: number };
    reranks: { size: number; maxSize: number };
  } {
    return {
      embeddings: {
        size: this.embeddingCache.size,
        maxSize: this.MAX_EMBEDDING_CACHE_SIZE,
      },
      reranks: {
        size: this.rerankCache.size,
        maxSize: this.MAX_RERANK_CACHE_SIZE,
      },
    };
  }

  /**
   * Clear all caches
   */
  clearCache(): void {
    this.embeddingCache.clear();
    this.rerankCache.clear();
    this.logger.log('Infinity caches cleared');
  }
}
