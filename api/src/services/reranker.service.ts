import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { HybridSearchResult } from "./hybrid-ranker.service";
import { InfinityService } from "./infinity.service";

export interface RerankOptions {
  enabled?: boolean;
  candidates?: number;
  timeout?: number;
}

export interface RerankResult {
  doc_id: string;
  score: number;
}

/**
 * RerankerService handles neural reranking of search results with timeout and fallback.
 * 
 * Reranking improves result quality by 20-35% on natural language queries by using
 * a cross-encoder model to re-score the top candidates from hybrid search.
 * 
 * Features:
 * - Configurable timeout (default 100ms)
 * - Fail-open: returns original results on timeout/error
 * - Heuristic to skip reranking when not needed
 * - Metrics logging for performance monitoring
 */
@Injectable()
export class RerankerService {
  private readonly logger = new Logger(RerankerService.name);
  
  // Configuration
  private readonly enabled: boolean;
  private readonly defaultTimeout: number;
  private readonly defaultCandidates: number;
  private readonly modelName: string;
  
  // Metrics
  private rerankCount = 0;
  private timeoutCount = 0;
  private skipCount = 0;
  private totalLatencyMs = 0;

  constructor(
    private readonly configService: ConfigService,
    private readonly infinityService: InfinityService,
  ) {
    this.enabled = this.configService.get<boolean>("rerank.enabled") ?? true;
    this.defaultTimeout = this.configService.get<number>("rerank.timeoutMs") ?? 100;
    this.defaultCandidates = this.configService.get<number>("rerank.candidates") ?? 30;
    this.modelName = this.configService.get<string>("rerank.model") ?? "mxbai-rerank-base-v2";
    
    this.logger.log(
      `RerankerService initialized: enabled=${this.enabled}, timeout=${this.defaultTimeout}ms, ` +
      `candidates=${this.defaultCandidates}, model=${this.modelName}`,
    );
  }

  /**
   * Rerank search results using neural reranker
   * 
   * @param query - Original search query
   * @param results - Fused results from HybridRankerService
   * @param options - Optional reranking configuration
   * @returns Reranked results or original results on error/timeout
   */
  async rerank(
    query: string,
    results: HybridSearchResult[],
    options: RerankOptions = {},
  ): Promise<HybridSearchResult[]> {
    const enabled = options.enabled ?? this.enabled;
    const candidates = options.candidates ?? this.defaultCandidates;
    const timeout = options.timeout ?? this.defaultTimeout;

    // Early return if disabled
    if (!enabled) {
      return results;
    }

    // Check if reranking is worthwhile
    if (!this.shouldRerank(query, results)) {
      this.skipCount++;
      this.logger.debug(`Skipping rerank for query: "${query}" (skip count: ${this.skipCount})`);
      return results;
    }

    // Take top N candidates for reranking
    const topCandidates = results.slice(0, candidates);
    
    if (topCandidates.length === 0) {
      return results;
    }

    const startTime = Date.now();

    try {
      // Call InfinityService.rerank with timeout
      const rerankScores = await this.callRerankWithTimeout(
        query,
        topCandidates,
        timeout,
      );

      const latency = Date.now() - startTime;
      this.rerankCount++;
      this.totalLatencyMs += latency;

      this.logger.debug(
        `Rerank successful: ${topCandidates.length} candidates, ${latency}ms ` +
        `(avg: ${Math.round(this.totalLatencyMs / this.rerankCount)}ms)`,
      );

      // Merge rerank scores back into results
      return this.mergeRerankScores(results, rerankScores, candidates);

    } catch (error) {
      const latency = Date.now() - startTime;

      if (error instanceof Error && error.name === "TimeoutError") {
        this.timeoutCount++;
        this.logger.warn(
          `Rerank timeout after ${timeout}ms (timeout count: ${this.timeoutCount}/${this.rerankCount + this.timeoutCount})`,
        );
      } else {
        this.logger.warn(`Rerank failed: ${error} (latency: ${latency}ms)`);
      }

      // Fail-open: return original results
      return results;
    }
  }

  /**
   * Determine if reranking is worth the latency cost
   * 
   * Skip reranking if:
   * - Query is very short (< 3 chars) - likely a code symbol
   * - No results to rerank
   * - Top result has much higher score than others (confident)
   * 
   * @param query - Search query
   * @param results - Search results
   * @returns true if should rerank, false to skip
   */
  shouldRerank(query: string, results: HybridSearchResult[]): boolean {
    // Skip if no results
    if (results.length === 0) {
      return false;
    }

    // Skip if query is very short (likely a code symbol)
    if (query.trim().length < 3) {
      return false;
    }

    // Skip if only 1-2 results (no need to reorder)
    if (results.length <= 2) {
      return false;
    }

    // Skip if top result is much better than second (confident)
    // This happens with exact symbol matches in code queries
    if (results.length >= 2) {
      const topScore = results[0].final_score;
      const secondScore = results[1].final_score;
      
      // If top is 3x better than second, we're confident
      if (topScore > secondScore * 3) {
        return false;
      }
    }

    return true;
  }

  /**
   * Call rerank endpoint with hard timeout using AbortSignal
   * 
   * @param query - Search query
   * @param candidates - Top candidates to rerank
   * @param timeoutMs - Timeout in milliseconds
   * @returns Rerank scores for each candidate
   */
  private async callRerankWithTimeout(
    query: string,
    candidates: HybridSearchResult[],
    timeoutMs: number,
  ): Promise<RerankResult[]> {
    // Create abort controller with timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      controller.abort();
    }, timeoutMs);

    try {
      // Prepare documents for reranking
      const documents = candidates.map((c) => ({
        doc_id: c.doc_id,
        content: c.content,
      }));

      // Call InfinityService.rerank with the candidates
      const rerankResults = await Promise.race([
        this.infinityService.rerank(query, documents, candidates.length),
        new Promise<never>((_, reject) => {
          controller.signal.addEventListener("abort", () => {
            const error = new Error("Rerank timeout");
            error.name = "TimeoutError";
            reject(error);
          });
        }),
      ]);

      // Map results to RerankResult format
      return rerankResults.map((r) => ({
        doc_id: r.doc_id,
        score: r.score,
      }));

    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Merge rerank scores back into results and re-sort
   * 
   * @param results - Original results
   * @param rerankScores - Rerank scores for top candidates
   * @param candidatesCount - Number of candidates that were reranked
   * @returns Results with rerank scores merged and re-sorted
   */
  private mergeRerankScores(
    results: HybridSearchResult[],
    rerankScores: RerankResult[],
    candidatesCount: number,
  ): HybridSearchResult[] {
    // Create map of doc_id to rerank score
    const rerankMap = new Map<string, number>();
    rerankScores.forEach((r) => {
      rerankMap.set(r.doc_id, r.score);
    });

    // Add rerank scores to results
    const rerankedResults = results.map((result, idx) => {
      const rerankScore = rerankMap.get(result.doc_id);
      
      if (rerankScore !== undefined) {
        // This result was reranked - use rerank score as final score
        return {
          ...result,
          final_score: rerankScore,
          rerank_score: rerankScore,
          rerank_rank: idx + 1,
        } as HybridSearchResult & { rerank_score: number; rerank_rank: number };
      } else {
        // This result was not reranked - keep original score
        return result;
      }
    });

    // Re-sort by final score (now includes rerank scores)
    rerankedResults.sort((a, b) => b.final_score - a.final_score);

    return rerankedResults;
  }

  /**
   * Get reranking metrics for monitoring
   */
  getMetrics(): {
    enabled: boolean;
    model: string;
    rerankCount: number;
    timeoutCount: number;
    skipCount: number;
    avgLatencyMs: number;
    timeoutRate: number;
  } {
    const totalAttempts = this.rerankCount + this.timeoutCount;
    
    return {
      enabled: this.enabled,
      model: this.modelName,
      rerankCount: this.rerankCount,
      timeoutCount: this.timeoutCount,
      skipCount: this.skipCount,
      avgLatencyMs: this.rerankCount > 0 
        ? Math.round(this.totalLatencyMs / this.rerankCount)
        : 0,
      timeoutRate: totalAttempts > 0
        ? Math.round((this.timeoutCount / totalAttempts) * 100) / 100
        : 0,
    };
  }

  /**
   * Reset metrics (useful for testing)
   */
  resetMetrics(): void {
    this.rerankCount = 0;
    this.timeoutCount = 0;
    this.skipCount = 0;
    this.totalLatencyMs = 0;
    this.logger.log("Reranker metrics reset");
  }
}
