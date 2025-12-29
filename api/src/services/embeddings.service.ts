import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';

interface CacheEntry {
  embedding: number[];
  timestamp: number;
}

/**
 * Simple LRU Cache for embeddings
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
 * EmbeddingsService - Low-level HTTP client for embeddings.
 * 
 * No retry logic here - callers handle retries:
 * - Indexing: EmbeddingQueueService (BullMQ) re-queues failed jobs
 * - Search: Fails fast, returns partial results
 */
@Injectable()
export class EmbeddingsService implements OnModuleInit {
  private readonly logger = new Logger(EmbeddingsService.name);
  private readonly baseUrl: string;
  private readonly model: string;
  private readonly dim: number;
  private readonly timeout: number;

  // LRU cache for query embeddings (max 1000 entries)
  private readonly queryCache: LRUCache<string, CacheEntry>;
  private readonly CACHE_TTL_MS = 60 * 60 * 1000; // 1 hour
  private readonly MAX_CACHE_SIZE = 1000;

  constructor(private configService: ConfigService) {
    this.baseUrl = this.configService.get<string>('embeddings.url')!;
    this.model = this.configService.get<string>('infinity.embedModel') || 'jinaai/jina-code-embeddings-1.5b';
    this.dim = this.configService.get<number>('embeddings.dim')!;
    // Use configurable timeout, default 5 minutes for large batches
    this.timeout = this.configService.get<number>('infinity.timeout') || 300000;
    this.queryCache = new LRUCache<string, CacheEntry>(this.MAX_CACHE_SIZE);
  }

  async onModuleInit() {
    try {
      await this.healthCheck();
      this.logger.log(`Connected to embeddings service at ${this.baseUrl}`);
    } catch (error) {
      this.logger.warn(
        `Embeddings service not available at ${this.baseUrl}. Will retry on first request.`,
      );
    }
  }

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

  async getInfo(): Promise<Record<string, unknown>> {
    const response = await fetch(`${this.baseUrl}/info`, {
      method: 'GET',
      signal: AbortSignal.timeout(this.timeout),
    });
    if (!response.ok) {
      throw new Error(`Info request failed: ${response.status}`);
    }
    return response.json() as Promise<Record<string, unknown>>;
  }

  /**
   * Get embedding from cache or compute
   */
  private getCachedEmbedding(text: string): number[] | null {
    const entry = this.queryCache.get(text);
    if (entry && Date.now() - entry.timestamp < this.CACHE_TTL_MS) {
      return entry.embedding;
    }
    return null;
  }

  /**
   * Cache an embedding
   */
  private cacheEmbedding(text: string, embedding: number[]): void {
    this.queryCache.set(text, {
      embedding,
      timestamp: Date.now(),
    });
  }

  /**
   * Generate embeddings for texts using native fetch
   * @param texts Array of texts to embed
   * @param normalize L2 normalize embeddings (default: true)
   * @param truncate Auto-truncate long inputs (default: true)
   * @returns 2D array of embeddings
   */
  async embed(
    texts: string[],
    normalize = true,
    truncate = true,
  ): Promise<number[][]> {
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

    // Use OpenAI-compatible /embeddings endpoint (Infinity format)
    // No retry here - caller handles retry (queue re-queues on failure)
    const response = await fetch(`${this.baseUrl}/embeddings`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        model: this.model,
        input: uncachedTexts,
      }),
      signal: AbortSignal.timeout(this.timeout),
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Embedding request failed (${response.status}): ${errorText}`);
    }

    const data = (await response.json()) as { data: Array<{ embedding: number[] }> };
    const embeddings = data.data.map((item) => item.embedding);

    // Cache new embeddings and merge results
    for (let i = 0; i < uncachedIndices.length; i++) {
      const idx = uncachedIndices[i];
      const embedding = embeddings[i];
      results[idx] = embedding;
      this.cacheEmbedding(texts[idx], embedding);
    }

    return results as number[][];
  }

  /**
   * Embed large batches with automatic chunking and controlled concurrency
   * Limits parallel requests to avoid overwhelming the TEI service
   */
  async embedBatch(
    texts: string[],
    batchSize = 32,
    normalize = true,
    maxConcurrent = 4, // Limit concurrent requests to TEI
  ): Promise<number[][]> {
    if (texts.length === 0) {
      return [];
    }

    // Create batches
    const batches: string[][] = [];
    for (let i = 0; i < texts.length; i += batchSize) {
      batches.push(texts.slice(i, i + batchSize));
    }

    // Process batches with limited concurrency
    const results: number[][][] = [];
    for (let i = 0; i < batches.length; i += maxConcurrent) {
      const chunk = batches.slice(i, i + maxConcurrent);
      const chunkResults = await Promise.all(
        chunk.map((batch) => this.embed(batch, normalize))
      );
      results.push(...chunkResults);
    }

    // Flatten results
    return results.flat();
  }



  getDimension(): number {
    return this.dim;
  }

  getCacheStats(): { size: number; maxSize: number } {
    return {
      size: this.queryCache.size,
      maxSize: this.MAX_CACHE_SIZE,
    };
  }

  clearCache(): void {
    this.queryCache.clear();
    this.logger.log('Embedding cache cleared');
  }
}
