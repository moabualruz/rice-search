import { Injectable, Logger } from "@nestjs/common";

export interface NormalizedQuery {
  original: string;
  normalized: string;
  cacheKey: string;
}

@Injectable()
export class QueryNormalizerService {
  private readonly logger = new Logger(QueryNormalizerService.name);

  /**
   * Normalize a query for consistent caching and processing
   * 
   * Normalization steps:
   * 1. Trim leading/trailing whitespace
   * 2. Collapse multiple spaces to single space
   * 3. Lowercase for cache key (but keep original case for search)
   * 4. Remove trailing punctuation for cache key
   * 
   * @param query Raw search query
   * @returns Normalized query with cache key
   */
  normalize(query: string): NormalizedQuery {
    // Step 1: Trim leading/trailing whitespace
    const trimmed = query.trim();

    // Step 2: Collapse multiple whitespace characters (spaces, tabs, newlines) to single space
    const normalized = trimmed.replace(/\s+/g, " ");

    // Step 3 & 4: Generate cache key (lowercase + remove trailing punctuation)
    const cacheKey = this.generateCacheKey(normalized);

    this.logger.debug(`Query normalized: "${query}" -> "${normalized}" (cache: "${cacheKey}")`);

    return {
      original: query,
      normalized,
      cacheKey,
    };
  }

  /**
   * Generate a stable cache key from normalized query
   * Used for embedding and rerank caching
   */
  private generateCacheKey(normalized: string): string {
    // Lowercase for case-insensitive caching
    let key = normalized.toLowerCase();

    // Remove trailing punctuation (. , ! ? ; :)
    key = key.replace(/[.,!?;:]+$/, "");

    // Final trim to ensure no trailing spaces
    return key.trim();
  }
}
