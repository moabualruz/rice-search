import { Injectable, Logger } from "@nestjs/common";

/**
 * A single result in a ranked list
 */
export interface RankedResult {
  docId: string;
  path: string;
  score: number;
  rank: number;
}

/**
 * Relevance judgment for evaluation
 */
export interface RelevanceJudgment {
  queryId: string;
  docId: string;
  relevance: number; // 0 = not relevant, 1 = marginally, 2 = relevant, 3 = highly
}

/**
 * Evaluation result for a single query
 */
export interface QueryEvaluation {
  queryId: string;
  query: string;
  ndcg: number;
  ndcgAt5: number;
  ndcgAt10: number;
  recall: number;
  recallAt5: number;
  recallAt10: number;
  mrr: number;
  precision: number;
  precisionAt5: number;
  precisionAt10: number;
  avgPrecision: number; // MAP component
  resultCount: number;
  relevantFound: number;
  totalRelevant: number;
}

/**
 * Aggregated evaluation metrics
 */
export interface EvaluationSummary {
  queryCount: number;
  meanNdcg: number;
  meanNdcgAt5: number;
  meanNdcgAt10: number;
  meanRecall: number;
  meanRecallAt5: number;
  meanRecallAt10: number;
  meanMrr: number;
  meanPrecision: number;
  map: number; // Mean Average Precision
  queriesWithNoRelevant: number;
  queriesWithPerfectRecall: number;
}

/**
 * Version comparison result
 */
export interface VersionComparison {
  versionA: string;
  versionB: string;
  metricsA: EvaluationSummary;
  metricsB: EvaluationSummary;
  ndcgDelta: number;
  recallDelta: number;
  mrrDelta: number;
  mapDelta: number;
  winner: "A" | "B" | "tie";
  confidenceLevel: number; // 0-1, higher = more queries evaluated
}

/**
 * EvaluationService computes IR metrics for search quality assessment.
 *
 * Metrics computed:
 * - NDCG (Normalized Discounted Cumulative Gain): Measures ranking quality
 * - Recall@K: Fraction of relevant documents found in top K
 * - MRR (Mean Reciprocal Rank): Inverse of first relevant result's rank
 * - Precision@K: Fraction of top K that are relevant
 * - MAP (Mean Average Precision): Average precision across all recall levels
 *
 * Usage:
 * 1. Load relevance judgments (from human labels or click data)
 * 2. Evaluate search results against judgments
 * 3. Compare versions for A/B testing decisions
 */
@Injectable()
export class EvaluationService {
  private readonly logger = new Logger(EvaluationService.name);

  // Relevance judgments: Map<queryId, Map<docId, relevance>>
  private judgments: Map<string, Map<string, number>> = new Map();

  /**
   * Load relevance judgments from JSONL data
   *
   * @param data - Array of relevance judgments
   */
  loadJudgments(data: RelevanceJudgment[]): void {
    for (const judgment of data) {
      let queryJudgments = this.judgments.get(judgment.queryId);
      if (!queryJudgments) {
        queryJudgments = new Map();
        this.judgments.set(judgment.queryId, queryJudgments);
      }
      queryJudgments.set(judgment.docId, judgment.relevance);
    }

    this.logger.log(
      `Loaded ${data.length} relevance judgments for ${this.judgments.size} queries`,
    );
  }

  /**
   * Clear all relevance judgments
   */
  clearJudgments(): void {
    this.judgments.clear();
  }

  /**
   * Get relevance for a document
   *
   * @param queryId - Query identifier
   * @param docId - Document identifier
   * @returns Relevance score (0 if not judged)
   */
  getRelevance(queryId: string, docId: string): number {
    return this.judgments.get(queryId)?.get(docId) ?? 0;
  }

  /**
   * Evaluate search results for a single query
   *
   * @param queryId - Query identifier
   * @param query - Query text (for logging)
   * @param results - Ranked results from search
   * @returns Evaluation metrics for this query
   */
  evaluateQuery(
    queryId: string,
    query: string,
    results: RankedResult[],
  ): QueryEvaluation {
    const queryJudgments = this.judgments.get(queryId);

    if (!queryJudgments) {
      // No judgments for this query - return zeros
      return {
        queryId,
        query,
        ndcg: 0,
        ndcgAt5: 0,
        ndcgAt10: 0,
        recall: 0,
        recallAt5: 0,
        recallAt10: 0,
        mrr: 0,
        precision: 0,
        precisionAt5: 0,
        precisionAt10: 0,
        avgPrecision: 0,
        resultCount: results.length,
        relevantFound: 0,
        totalRelevant: 0,
      };
    }

    // Get relevances for results
    const relevances = results.map(
      (r) => queryJudgments.get(r.docId) ?? 0,
    );

    // Count total relevant documents
    const totalRelevant = Array.from(queryJudgments.values()).filter(
      (r) => r > 0,
    ).length;

    // Compute metrics
    const ndcg = this.computeNdcg(relevances, totalRelevant);
    const ndcgAt5 = this.computeNdcg(relevances.slice(0, 5), totalRelevant);
    const ndcgAt10 = this.computeNdcg(relevances.slice(0, 10), totalRelevant);

    const relevantFound = relevances.filter((r) => r > 0).length;
    const recall = totalRelevant > 0 ? relevantFound / totalRelevant : 0;
    const recallAt5 =
      totalRelevant > 0
        ? relevances.slice(0, 5).filter((r) => r > 0).length / totalRelevant
        : 0;
    const recallAt10 =
      totalRelevant > 0
        ? relevances.slice(0, 10).filter((r) => r > 0).length / totalRelevant
        : 0;

    const mrr = this.computeMrr(relevances);

    const precision =
      results.length > 0 ? relevantFound / results.length : 0;
    const precisionAt5 =
      Math.min(results.length, 5) > 0
        ? relevances.slice(0, 5).filter((r) => r > 0).length /
          Math.min(results.length, 5)
        : 0;
    const precisionAt10 =
      Math.min(results.length, 10) > 0
        ? relevances.slice(0, 10).filter((r) => r > 0).length /
          Math.min(results.length, 10)
        : 0;

    const avgPrecision = this.computeAveragePrecision(relevances, totalRelevant);

    return {
      queryId,
      query,
      ndcg,
      ndcgAt5,
      ndcgAt10,
      recall,
      recallAt5,
      recallAt10,
      mrr,
      precision,
      precisionAt5,
      precisionAt10,
      avgPrecision,
      resultCount: results.length,
      relevantFound,
      totalRelevant,
    };
  }

  /**
   * Evaluate multiple queries and compute summary metrics
   *
   * @param evaluations - Array of query evaluations
   * @returns Aggregated metrics
   */
  summarize(evaluations: QueryEvaluation[]): EvaluationSummary {
    if (evaluations.length === 0) {
      return {
        queryCount: 0,
        meanNdcg: 0,
        meanNdcgAt5: 0,
        meanNdcgAt10: 0,
        meanRecall: 0,
        meanRecallAt5: 0,
        meanRecallAt10: 0,
        meanMrr: 0,
        meanPrecision: 0,
        map: 0,
        queriesWithNoRelevant: 0,
        queriesWithPerfectRecall: 0,
      };
    }

    const mean = (arr: number[]): number =>
      arr.length > 0 ? arr.reduce((a, b) => a + b, 0) / arr.length : 0;

    // Filter out queries with no relevance judgments for mean calculation
    const withJudgments = evaluations.filter((e) => e.totalRelevant > 0);

    return {
      queryCount: evaluations.length,
      meanNdcg: mean(withJudgments.map((e) => e.ndcg)),
      meanNdcgAt5: mean(withJudgments.map((e) => e.ndcgAt5)),
      meanNdcgAt10: mean(withJudgments.map((e) => e.ndcgAt10)),
      meanRecall: mean(withJudgments.map((e) => e.recall)),
      meanRecallAt5: mean(withJudgments.map((e) => e.recallAt5)),
      meanRecallAt10: mean(withJudgments.map((e) => e.recallAt10)),
      meanMrr: mean(withJudgments.map((e) => e.mrr)),
      meanPrecision: mean(withJudgments.map((e) => e.precision)),
      map: mean(withJudgments.map((e) => e.avgPrecision)),
      queriesWithNoRelevant: evaluations.filter((e) => e.totalRelevant === 0)
        .length,
      queriesWithPerfectRecall: withJudgments.filter((e) => e.recall === 1)
        .length,
    };
  }

  /**
   * Compare two versions using the same query set
   *
   * @param versionA - First version identifier
   * @param versionB - Second version identifier
   * @param evaluationsA - Evaluations for version A
   * @param evaluationsB - Evaluations for version B
   * @returns Comparison results
   */
  compareVersions(
    versionA: string,
    versionB: string,
    evaluationsA: QueryEvaluation[],
    evaluationsB: QueryEvaluation[],
  ): VersionComparison {
    const metricsA = this.summarize(evaluationsA);
    const metricsB = this.summarize(evaluationsB);

    const ndcgDelta = metricsB.meanNdcg - metricsA.meanNdcg;
    const recallDelta = metricsB.meanRecall - metricsA.meanRecall;
    const mrrDelta = metricsB.meanMrr - metricsA.meanMrr;
    const mapDelta = metricsB.map - metricsA.map;

    // Determine winner based on primary metric (NDCG)
    let winner: "A" | "B" | "tie" = "tie";
    const threshold = 0.01; // 1% improvement threshold
    if (ndcgDelta > threshold) {
      winner = "B";
    } else if (ndcgDelta < -threshold) {
      winner = "A";
    }

    // Confidence based on number of queries evaluated
    const minQueries = Math.min(evaluationsA.length, evaluationsB.length);
    const confidenceLevel = Math.min(1, minQueries / 100); // 100+ queries = full confidence

    return {
      versionA,
      versionB,
      metricsA,
      metricsB,
      ndcgDelta,
      recallDelta,
      mrrDelta,
      mapDelta,
      winner,
      confidenceLevel,
    };
  }

  /**
   * Compute Normalized Discounted Cumulative Gain
   *
   * @param relevances - Relevance scores in rank order
   * @param totalRelevant - Total number of relevant documents
   * @returns NDCG score (0-1)
   */
  private computeNdcg(relevances: number[], totalRelevant: number): number {
    if (relevances.length === 0 || totalRelevant === 0) {
      return 0;
    }

    // DCG = sum of (2^rel - 1) / log2(rank + 1)
    const dcg = relevances.reduce((sum, rel, idx) => {
      const gain = Math.pow(2, rel) - 1;
      const discount = Math.log2(idx + 2); // +2 because rank is 1-indexed
      return sum + gain / discount;
    }, 0);

    // Ideal DCG - sort relevances descending
    const idealRelevances = [...relevances].sort((a, b) => b - a);
    const idcg = idealRelevances.reduce((sum, rel, idx) => {
      const gain = Math.pow(2, rel) - 1;
      const discount = Math.log2(idx + 2);
      return sum + gain / discount;
    }, 0);

    return idcg > 0 ? dcg / idcg : 0;
  }

  /**
   * Compute Mean Reciprocal Rank
   *
   * @param relevances - Relevance scores in rank order
   * @returns MRR score (0-1)
   */
  private computeMrr(relevances: number[]): number {
    // Find first relevant result
    const firstRelevantIdx = relevances.findIndex((r) => r > 0);
    if (firstRelevantIdx === -1) {
      return 0;
    }
    return 1 / (firstRelevantIdx + 1);
  }

  /**
   * Compute Average Precision (for MAP)
   *
   * @param relevances - Relevance scores in rank order
   * @param totalRelevant - Total number of relevant documents
   * @returns Average precision score
   */
  private computeAveragePrecision(
    relevances: number[],
    totalRelevant: number,
  ): number {
    if (totalRelevant === 0) {
      return 0;
    }

    let relevantSoFar = 0;
    let precisionSum = 0;

    for (let i = 0; i < relevances.length; i++) {
      if (relevances[i] > 0) {
        relevantSoFar++;
        precisionSum += relevantSoFar / (i + 1);
      }
    }

    return precisionSum / totalRelevant;
  }

  /**
   * Generate synthetic relevance judgments from click data
   * Click = relevant (relevance 1), no click = unknown (not judged)
   *
   * @param clicks - Array of {queryId, docId} pairs
   * @returns Relevance judgments
   */
  clicksToJudgments(
    clicks: Array<{ queryId: string; docId: string }>,
  ): RelevanceJudgment[] {
    return clicks.map((click) => ({
      queryId: click.queryId,
      docId: click.docId,
      relevance: 1,
    }));
  }

  /**
   * Get judgment statistics
   */
  getJudgmentStats(): {
    totalQueries: number;
    totalJudgments: number;
    avgJudgmentsPerQuery: number;
  } {
    let totalJudgments = 0;
    for (const queryJudgments of this.judgments.values()) {
      totalJudgments += queryJudgments.size;
    }

    return {
      totalQueries: this.judgments.size,
      totalJudgments,
      avgJudgmentsPerQuery:
        this.judgments.size > 0 ? totalJudgments / this.judgments.size : 0,
    };
  }
}
