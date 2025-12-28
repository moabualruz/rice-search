import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { HybridSearchResult } from "../services/hybrid-ranker.service";
import { EmbeddingsService } from "../services/embeddings.service";

export interface DeduplicationConfig {
  similarityThreshold: number;  // 0-1, default 0.85
  preferLonger: boolean;        // Keep longer chunk when deduping
  preserveTop: number;          // Never dedup top N results
}

export interface DeduplicationStats {
  inputCount: number;
  outputCount: number;
  removedCount: number;
  latencyMs: number;
}

const DEFAULT_DEDUP_CONFIG: DeduplicationConfig = {
  similarityThreshold: 0.85,
  preferLonger: true,
  preserveTop: 3,
};

@Injectable()
export class DeduplicationService {
  private readonly logger = new Logger(DeduplicationService.name);
  private readonly defaultThreshold: number;

  // Metrics
  private dedupCount = 0;
  private totalRemoved = 0;
  private totalLatency = 0;

  constructor(
    private readonly configService: ConfigService,
    private readonly embeddingsService: EmbeddingsService,
  ) {
    this.defaultThreshold = this.configService.get<number>("postrank.dedupThreshold") ?? 0.85;
    this.logger.log(`DeduplicationService initialized: threshold=${this.defaultThreshold}`);
  }

  /**
   * Deduplicate results based on semantic similarity
   * 
   * @param results - Search results to deduplicate
   * @param config - Deduplication configuration
   * @returns Deduplicated results with stats
   */
  async deduplicate(
    results: HybridSearchResult[],
    config: Partial<DeduplicationConfig> = {},
  ): Promise<{ results: HybridSearchResult[]; stats: DeduplicationStats }> {
    const startTime = Date.now();
    const mergedConfig: DeduplicationConfig = {
      ...DEFAULT_DEDUP_CONFIG,
      similarityThreshold: this.defaultThreshold,
      ...config,
    };

    // Early return if not enough results to deduplicate
    if (results.length <= mergedConfig.preserveTop) {
      return {
        results,
        stats: {
          inputCount: results.length,
          outputCount: results.length,
          removedCount: 0,
          latencyMs: Date.now() - startTime,
        },
      };
    }

    // Get embeddings for all result contents
    const contents = results.map((r) => r.content);
    const embeddings = await this.embeddingsService.embed(contents);

    // Track which indices to keep
    const keptIndices: number[] = [];
    const removedIndices = new Set<number>();

    // Always keep top N results (preserveTop)
    for (let i = 0; i < Math.min(mergedConfig.preserveTop, results.length); i++) {
      keptIndices.push(i);
    }

    // Process remaining results
    for (let i = mergedConfig.preserveTop; i < results.length; i++) {
      let isDuplicate = false;
      let duplicateOfIdx = -1;

      // Check similarity against all kept results
      for (const keptIdx of keptIndices) {
        const similarity = this.cosineSimilarity(embeddings[i], embeddings[keptIdx]);

        if (similarity >= mergedConfig.similarityThreshold) {
          isDuplicate = true;
          duplicateOfIdx = keptIdx;
          break;
        }
      }

      if (isDuplicate && duplicateOfIdx >= 0) {
        // If preferLonger and this result is longer, swap
        if (
          mergedConfig.preferLonger &&
          results[i].content.length > results[duplicateOfIdx].content.length
        ) {
          // Replace the shorter one with the longer one
          const keptPosition = keptIndices.indexOf(duplicateOfIdx);
          keptIndices[keptPosition] = i;
          removedIndices.add(duplicateOfIdx);
          this.logger.debug(
            `Swapped shorter chunk (${results[duplicateOfIdx].content.length} chars) ` +
            `with longer (${results[i].content.length} chars)`
          );
        } else {
          removedIndices.add(i);
        }
      } else {
        keptIndices.push(i);
      }
    }

    const latencyMs = Date.now() - startTime;
    const stats: DeduplicationStats = {
      inputCount: results.length,
      outputCount: keptIndices.length,
      removedCount: removedIndices.size,
      latencyMs,
    };

    // Update metrics
    this.dedupCount++;
    this.totalRemoved += stats.removedCount;
    this.totalLatency += latencyMs;

    if (stats.removedCount > 0) {
      this.logger.debug(
        `Deduplication: ${stats.inputCount} → ${stats.outputCount} ` +
        `(removed ${stats.removedCount}, ${latencyMs}ms)`
      );
    }

    // Return results in original order (by kept indices)
    const dedupedResults = keptIndices
      .sort((a, b) => a - b)
      .map((i) => results[i]);

    return { results: dedupedResults, stats };
  }

  /**
   * Compute cosine similarity between two vectors
   */
  private cosineSimilarity(a: number[], b: number[]): number {
    if (a.length !== b.length || a.length === 0) {
      return 0;
    }

    let dotProduct = 0;
    let normA = 0;
    let normB = 0;

    for (let i = 0; i < a.length; i++) {
      dotProduct += a[i] * b[i];
      normA += a[i] * a[i];
      normB += b[i] * b[i];
    }

    const denominator = Math.sqrt(normA) * Math.sqrt(normB);
    return denominator > 0 ? dotProduct / denominator : 0;
  }

  /**
   * Get deduplication metrics
   */
  getMetrics(): {
    dedupCount: number;
    totalRemoved: number;
    avgLatencyMs: number;
    avgRemovalRate: number;
  } {
    return {
      dedupCount: this.dedupCount,
      totalRemoved: this.totalRemoved,
      avgLatencyMs: this.dedupCount > 0 ? Math.round(this.totalLatency / this.dedupCount) : 0,
      avgRemovalRate: this.dedupCount > 0 ? this.totalRemoved / this.dedupCount : 0,
    };
  }

  /**
   * Reset metrics
   */
  resetMetrics(): void {
    this.dedupCount = 0;
    this.totalRemoved = 0;
    this.totalLatency = 0;
  }
}
