import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { HybridSearchResult } from "../services/hybrid-ranker.service";
import { EmbeddingsService } from "../services/embeddings.service";

export interface DiversityConfig {
  enabled: boolean;
  lambda: number;        // MMR lambda: 0=max diversity, 1=max relevance
  minResults: number;    // Minimum results to apply diversity
}

export interface DiversityStats {
  inputCount: number;
  outputCount: number;
  avgDiversity: number;  // Average diversity score of selected results
  latencyMs: number;
}

const DEFAULT_DIVERSITY_CONFIG: DiversityConfig = {
  enabled: true,
  lambda: 0.7,
  minResults: 3,
};

@Injectable()
export class DiversityService {
  private readonly logger = new Logger(DiversityService.name);
  private readonly defaultLambda: number;

  // Metrics
  private diversifyCount = 0;
  private totalLatency = 0;

  constructor(
    private readonly configService: ConfigService,
    private readonly embeddingsService: EmbeddingsService,
  ) {
    this.defaultLambda = this.configService.get<number>("postrank.diversityLambda") ?? 0.7;
    this.logger.log(`DiversityService initialized: lambda=${this.defaultLambda}`);
  }

  /**
   * Maximal Marginal Relevance (MMR) reranking for diversity
   * 
   * MMR = λ * Relevance(d) - (1-λ) * max(Similarity(d, selected))
   * 
   * @param results - Search results to diversify
   * @param config - Diversity configuration
   * @returns Diversified results with stats
   */
  async diversify(
    results: HybridSearchResult[],
    config: Partial<DiversityConfig> = {},
  ): Promise<{ results: HybridSearchResult[]; stats: DiversityStats }> {
    const startTime = Date.now();
    const mergedConfig: DiversityConfig = {
      ...DEFAULT_DIVERSITY_CONFIG,
      lambda: this.defaultLambda,
      ...config,
    };

    // Skip if disabled or not enough results
    if (!mergedConfig.enabled || results.length < mergedConfig.minResults) {
      return {
        results,
        stats: {
          inputCount: results.length,
          outputCount: results.length,
          avgDiversity: 1.0,
          latencyMs: Date.now() - startTime,
        },
      };
    }

    // Get embeddings for all result contents
    const contents = results.map((r) => r.content);
    const embeddings = await this.embeddingsService.embed(contents);

    // Track selected indices and diversity scores
    const selected: number[] = [0]; // Always start with top result
    const remaining = new Set(results.map((_, i) => i).slice(1));
    const diversityScores: number[] = [1.0]; // Top result has max diversity

    // Normalize relevance scores
    const maxScore = results[0].final_score;
    const normalizedScores = results.map((r) =>
      maxScore > 0 ? r.final_score / maxScore : 0
    );

    // MMR selection loop
    while (remaining.size > 0 && selected.length < results.length) {
      let bestIdx = -1;
      let bestMMR = -Infinity;
      let bestDiversity = 0;

      for (const idx of remaining) {
        // Relevance component
        const relevance = normalizedScores[idx];

        // Diversity component: 1 - max similarity to any selected result
        let maxSimilarity = 0;
        for (const selIdx of selected) {
          const similarity = this.cosineSimilarity(embeddings[idx], embeddings[selIdx]);
          maxSimilarity = Math.max(maxSimilarity, similarity);
        }
        const diversity = 1 - maxSimilarity;

        // MMR score
        const mmr =
          mergedConfig.lambda * relevance +
          (1 - mergedConfig.lambda) * diversity;

        if (mmr > bestMMR) {
          bestMMR = mmr;
          bestIdx = idx;
          bestDiversity = diversity;
        }
      }

      if (bestIdx >= 0) {
        selected.push(bestIdx);
        diversityScores.push(bestDiversity);
        remaining.delete(bestIdx);
      } else {
        break;
      }
    }

    const latencyMs = Date.now() - startTime;
    const avgDiversity =
      diversityScores.length > 0
        ? diversityScores.reduce((a, b) => a + b, 0) / diversityScores.length
        : 0;

    const stats: DiversityStats = {
      inputCount: results.length,
      outputCount: selected.length,
      avgDiversity,
      latencyMs,
    };

    // Update metrics
    this.diversifyCount++;
    this.totalLatency += latencyMs;

    this.logger.debug(
      `Diversity: ${stats.inputCount} results, avgDiversity=${avgDiversity.toFixed(3)}, ${latencyMs}ms`
    );

    // Return results in MMR order
    return {
      results: selected.map((i) => results[i]),
      stats,
    };
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
   * Get diversity metrics
   */
  getMetrics(): {
    diversifyCount: number;
    avgLatencyMs: number;
  } {
    return {
      diversifyCount: this.diversifyCount,
      avgLatencyMs: this.diversifyCount > 0 ? Math.round(this.totalLatency / this.diversifyCount) : 0,
    };
  }

  /**
   * Reset metrics
   */
  resetMetrics(): void {
    this.diversifyCount = 0;
    this.totalLatency = 0;
  }
}
