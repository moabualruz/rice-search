import { Injectable, Logger } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import { HybridSearchResult } from "../services/hybrid-ranker.service";
import { InfinityService } from "../services/infinity.service";
import { TelemetryService } from "../services/telemetry.service";
import { RetrievalConfig } from "../intelligence/strategy-selector.service";

export interface RerankPass {
  name: string;
  inputCandidates: number;
  outputCandidates: number;
  timeout: number;
}

export type DistributionShape = "peaked" | "flat" | "bimodal";

export interface EarlyExitSignals {
  scoreGap: number;           // top - second
  scoreRatio: number;         // top / second
  topClusterSize: number;     // how many results within 10% of top
  distributionShape: DistributionShape;
}

export interface RerankStats {
  pass1Applied: boolean;
  pass1Latency: number;
  pass1InputCount: number;
  pass1OutputCount: number;
  pass2Applied: boolean;
  pass2Latency: number;
  pass2InputCount: number;
  pass2OutputCount: number;
  earlyExitTriggered: boolean;
  earlyExitReason?: string;
}

@Injectable()
export class MultiPassRerankerService {
  private readonly logger = new Logger(MultiPassRerankerService.name);

  // Configurable pass settings
  private readonly pass1Timeout: number;
  private readonly pass2Timeout: number;
  
  // Early exit thresholds
  private readonly earlyExitScoreRatio: number;
  private readonly earlyExitScoreGap: number;

  // Metrics
  private pass1Count = 0;
  private pass2Count = 0;
  private earlyExitCount = 0;
  private totalPass1Latency = 0;
  private totalPass2Latency = 0;

  constructor(
    private readonly configService: ConfigService,
    private readonly infinityService: InfinityService,
    private readonly telemetry: TelemetryService,
  ) {
    this.pass1Timeout = this.configService.get<number>("rerank.pass1Timeout") ?? 80;
    this.pass2Timeout = this.configService.get<number>("rerank.pass2Timeout") ?? 150;
    this.earlyExitScoreRatio = this.configService.get<number>("rerank.earlyExitScoreRatio") ?? 1.5;
    this.earlyExitScoreGap = this.configService.get<number>("rerank.earlyExitScoreGap") ?? 0.3;

    this.logger.log(
      `MultiPassRerankerService initialized: pass1Timeout=${this.pass1Timeout}ms, ` +
      `pass2Timeout=${this.pass2Timeout}ms, earlyExitRatio=${this.earlyExitScoreRatio}`
    );
  }

  /**
   * Multi-pass reranking with early exit capability
   * 
   * Pass 1 (Gate): Fast rerank to filter candidates (100 → 30)
   * Pass 2 (Precision): Deeper rerank for final ordering (30 → K) [conditional]
   * 
   * @param query - Search query
   * @param results - Fused results from HybridRankerService
   * @param config - Retrieval configuration from StrategySelectorService
   * @returns Reranked results with stats
   */
  async rerank(
    query: string,
    results: HybridSearchResult[],
    config: RetrievalConfig,
  ): Promise<{ results: HybridSearchResult[]; stats: RerankStats }> {
    const stats: RerankStats = {
      pass1Applied: false,
      pass1Latency: 0,
      pass1InputCount: 0,
      pass1OutputCount: 0,
      pass2Applied: false,
      pass2Latency: 0,
      pass2InputCount: 0,
      pass2OutputCount: 0,
      earlyExitTriggered: false,
    };

    // Skip reranking if disabled or no candidates
    if (config.rerankCandidates <= 0 || results.length === 0) {
      return { results, stats };
    }

    // Pass 1: Gate pass - fast filtering
    const pass1: RerankPass = {
      name: "gate",
      inputCandidates: Math.min(config.rerankCandidates, results.length),
      outputCandidates: Math.min(30, results.length),
      timeout: this.pass1Timeout,
    };

    const pass1Input = results.slice(0, pass1.inputCandidates);
    stats.pass1InputCount = pass1Input.length;

    const pass1StartTime = Date.now();
    let candidates: HybridSearchResult[];

    try {
      candidates = await this.executePass(query, pass1Input, pass1);
      stats.pass1Latency = Date.now() - pass1StartTime;
      stats.pass1Applied = true;
      stats.pass1OutputCount = candidates.length;
      this.pass1Count++;
      this.totalPass1Latency += stats.pass1Latency;

      this.logger.debug(
        `Pass 1 (gate): ${pass1Input.length} → ${candidates.length} candidates, ${stats.pass1Latency}ms`
      );
    } catch (error) {
      stats.pass1Latency = Date.now() - pass1StartTime;
      this.logger.warn(`Pass 1 failed, using original order: ${error}`);
      // Fall back to original order
      candidates = pass1Input;
    }

    // Early exit check
    if (!config.useSecondPass || this.shouldExitEarly(candidates, stats)) {
      return { results: candidates, stats };
    }

    // Pass 2: Precision pass - deeper rerank for ambiguous cases
    const pass2: RerankPass = {
      name: "precision",
      inputCandidates: Math.min(config.secondPassCandidates, candidates.length),
      outputCandidates: Math.min(config.secondPassCandidates, candidates.length),
      timeout: this.pass2Timeout,
    };

    const pass2Input = candidates.slice(0, pass2.inputCandidates);
    stats.pass2InputCount = pass2Input.length;

    const pass2StartTime = Date.now();

    try {
      const pass2Results = await this.executePass(query, pass2Input, pass2);
      stats.pass2Latency = Date.now() - pass2StartTime;
      stats.pass2Applied = true;
      stats.pass2OutputCount = pass2Results.length;
      this.pass2Count++;
      this.totalPass2Latency += stats.pass2Latency;

      this.logger.debug(
        `Pass 2 (precision): ${pass2Input.length} → ${pass2Results.length} candidates, ${stats.pass2Latency}ms`
      );

      // Merge pass 2 results with remaining candidates from pass 1
      const remainingCandidates = candidates.slice(pass2.inputCandidates);
      candidates = [...pass2Results, ...remainingCandidates];
    } catch (error) {
      stats.pass2Latency = Date.now() - pass2StartTime;
      this.logger.warn(`Pass 2 failed, using pass 1 results: ${error}`);
      // Keep pass 1 results on failure
    }

    return { results: candidates, stats };
  }

  /**
   * Execute a single rerank pass
   */
  private async executePass(
    query: string,
    candidates: HybridSearchResult[],
    pass: RerankPass,
  ): Promise<HybridSearchResult[]> {
    if (candidates.length === 0) {
      return [];
    }

    // Prepare documents for reranking
    const documents = candidates.map((c) => ({
      doc_id: c.doc_id,
      content: c.content,
    }));

    // Create abort controller with timeout
    const controller = new AbortController();
    const timeoutId = setTimeout(() => {
      controller.abort();
    }, pass.timeout);

    try {
      // Call InfinityService.rerank
      const rerankResults = await Promise.race([
        this.infinityService.rerank(query, documents, pass.outputCandidates),
        new Promise<never>((_, reject) => {
          controller.signal.addEventListener("abort", () => {
            const error = new Error(`${pass.name} pass timeout`);
            error.name = "TimeoutError";
            reject(error);
          });
        }),
      ]);

      // Create score map
      const scoreMap = new Map<string, number>();
      rerankResults.forEach((r) => {
        scoreMap.set(r.doc_id, r.score);
      });

      // Apply rerank scores and re-sort
      const rerankedCandidates = candidates.map((c) => {
        const rerankScore = scoreMap.get(c.doc_id);
        if (rerankScore !== undefined) {
          return {
            ...c,
            final_score: rerankScore,
          };
        }
        return c;
      });

      // Sort by new scores
      rerankedCandidates.sort((a, b) => b.final_score - a.final_score);

      return rerankedCandidates.slice(0, pass.outputCandidates);
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Analyze score distribution to determine if early exit is appropriate
   * Enhanced version with distribution shape analysis (Task 1.5)
   */
  private analyzeDistribution(results: HybridSearchResult[]): EarlyExitSignals {
    if (results.length === 0) {
      return {
        scoreGap: 0,
        scoreRatio: Infinity,
        topClusterSize: 0,
        distributionShape: "flat",
      };
    }

    const scores = results.map((r) => r.final_score);
    const top = scores[0] ?? 0;
    const second = scores[1] ?? 0;

    // Count results within 10% of top score
    const threshold = top * 0.9;
    const topClusterSize = scores.filter((s) => s >= threshold).length;

    // Calculate distribution statistics
    const mean = scores.reduce((a, b) => a + b, 0) / scores.length;
    const variance = scores.reduce((a, s) => a + Math.pow(s - mean, 2), 0) / scores.length;
    const normalizedVariance = mean > 0 ? variance / (mean * mean) : 0;

    // Determine distribution shape
    let distributionShape: DistributionShape;
    if (topClusterSize === 1 && normalizedVariance > 0.1) {
      distributionShape = "peaked"; // One clear winner
    } else if (normalizedVariance < 0.05) {
      distributionShape = "flat"; // All scores similar (uncertain)
    } else {
      distributionShape = "bimodal"; // Mixed distribution
    }

    return {
      scoreGap: top - second,
      scoreRatio: second > 0 ? top / second : Infinity,
      topClusterSize,
      distributionShape,
    };
  }

  /**
   * Determine if we should skip pass 2 based on score distribution
   */
  private shouldExitEarly(results: HybridSearchResult[], stats: RerankStats): boolean {
    if (results.length < 2) {
      stats.earlyExitTriggered = true;
      stats.earlyExitReason = "insufficient_results";
      this.earlyExitCount++;
      return true;
    }

    const signals = this.analyzeDistribution(results);

    // Exit if distribution is peaked (one clear winner)
    if (signals.distributionShape === "peaked" && signals.scoreRatio > this.earlyExitScoreRatio) {
      stats.earlyExitTriggered = true;
      stats.earlyExitReason = "peaked_distribution";
      this.earlyExitCount++;
      this.logger.debug(
        `Early exit: peaked distribution (ratio=${signals.scoreRatio.toFixed(2)})`
      );
      return true;
    }

    // Exit if very high gap between top and second
    if (signals.scoreGap > this.earlyExitScoreGap) {
      stats.earlyExitTriggered = true;
      stats.earlyExitReason = "high_score_gap";
      this.earlyExitCount++;
      this.logger.debug(
        `Early exit: high score gap (gap=${signals.scoreGap.toFixed(3)})`
      );
      return true;
    }

    // Don't exit if results are flat (uncertainty, needs second pass)
    if (signals.distributionShape === "flat") {
      this.logger.debug("No early exit: flat distribution (uncertainty)");
      return false;
    }

    return false;
  }

  /**
   * Get reranking metrics for monitoring
   */
  getMetrics(): {
    pass1Count: number;
    pass2Count: number;
    earlyExitCount: number;
    avgPass1Latency: number;
    avgPass2Latency: number;
    earlyExitRate: number;
    pass2Rate: number;
  } {
    const totalRerankAttempts = this.pass1Count;
    
    return {
      pass1Count: this.pass1Count,
      pass2Count: this.pass2Count,
      earlyExitCount: this.earlyExitCount,
      avgPass1Latency: this.pass1Count > 0
        ? Math.round(this.totalPass1Latency / this.pass1Count)
        : 0,
      avgPass2Latency: this.pass2Count > 0
        ? Math.round(this.totalPass2Latency / this.pass2Count)
        : 0,
      earlyExitRate: totalRerankAttempts > 0
        ? Math.round((this.earlyExitCount / totalRerankAttempts) * 100) / 100
        : 0,
      pass2Rate: totalRerankAttempts > 0
        ? Math.round((this.pass2Count / totalRerankAttempts) * 100) / 100
        : 0,
    };
  }

  /**
   * Reset metrics (useful for testing)
   */
  resetMetrics(): void {
    this.pass1Count = 0;
    this.pass2Count = 0;
    this.earlyExitCount = 0;
    this.totalPass1Latency = 0;
    this.totalPass2Latency = 0;
    this.logger.log("MultiPassReranker metrics reset");
  }
}
