import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import http from 'node:http';
import https from 'node:https';

interface CacheEntry {
  embedding: number[];
  timestamp: number;
}

// Maximize concurrent connections to embeddings server
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

  // HTTP agent for connection pooling
  private readonly agent: http.Agent | https.Agent;

  constructor(private configService: ConfigService) {
    this.baseUrl = this.configService.get<string>('embeddings.url')!;
    this.model = this.configService.get<string>('infinity.embedModel') || 'jinaai/jina-code-embeddings-1.5b';
    this.dim = this.configService.get<number>('embeddings.dim')!;
    this.timeout = 60000;
    this.queryCache = new LRUCache<string, CacheEntry>(this.MAX_CACHE_SIZE);
    
    // Use appropriate agent based on URL protocol
    this.agent = this.baseUrl.startsWith('https') ? httpsAgent : httpAgent;
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

    try {
      // Use OpenAI-compatible /embeddings endpoint (Infinity format)
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
        // @ts-expect-error - Node.js fetch supports dispatcher for HTTP agent
        dispatcher: this.agent,
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Embed request failed (${response.status}): ${errorText}`);
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
    } catch (error) {
      this.logger.error(`Embedding request failed: ${error}`);
      throw error;
    }
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

  /**
   * Embed with retry logic for transient failures
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
