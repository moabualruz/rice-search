import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { TantivySearchResult } from './tantivy.service';
import { MilvusSearchResult } from './milvus.service';

export interface HybridSearchResult {
  doc_id: string;
  path: string;
  language: string;
  start_line: number;
  end_line: number;
  content: string;
  symbols: string[];
  final_score: number;
  sparse_score?: number;
  dense_score?: number;
  sparse_rank?: number;
  dense_rank?: number;
}

export interface RankingOptions {
  rrfK?: number;
  sparseWeight?: number;
  denseWeight?: number;
  symbolBoost?: number;
  pathBoost?: number;
  groupByFile?: boolean;
}

export interface FusionStats {
  topScore: number;
  secondScore: number;
  scoreGap: number;      // top - second
  scoreRatio: number;    // top / second (infinity if second is 0)
  tailMean: number;      // mean of scores from position 3-10
  resultCount: number;
}

/**
 * Confidence metrics for a single retrieval modality
 */
export interface ModalityConfidence {
  modality: "sparse" | "dense";
  confidence: number;      // 0-1 confidence score
  scoreGap: number;        // Gap between top and second result
  scoreVariance: number;   // Variance of top scores
  resultCount: number;     // Number of results
}

/**
 * Configuration for confidence-weighted fusion
 */
export interface ConfidenceWeightedFusionConfig {
  sparseBaseWeight: number;
  denseBaseWeight: number;
  useScoreConfidence: boolean;  // Adjust by score distribution
  useResultOverlap: boolean;    // Adjust by result agreement
  minConfidenceWeight: number;  // Floor for confidence weight (0.1-0.5)
  maxConfidenceBoost: number;   // Ceiling for confidence boost (1.5-2.0)
}

@Injectable()
export class HybridRankerService {
  private readonly logger = new Logger(HybridRankerService.name);
  private readonly defaultRrfK: number;

  constructor(private configService: ConfigService) {
    this.defaultRrfK = this.configService.get<number>('search.rrfK')!;
  }

  /**
   * Fuse sparse and dense search results using Reciprocal Rank Fusion (RRF)
   * with code-specific heuristics
   *
   * @param sparseResults Results from Tantivy (BM25)
   * @param denseResults Results from Milvus (vector)
   * @param contentMap Map of doc_id to content
   * @param query Original search query
   * @param options Ranking options
   * @returns Fused and ranked results
   */
  fuseResults(
    sparseResults: TantivySearchResult[],
    denseResults: MilvusSearchResult[],
    contentMap: Map<string, { content: string; symbols: string[] }>,
    query: string,
    options: RankingOptions = {},
  ): HybridSearchResult[] {
    const {
      rrfK = this.defaultRrfK,
      sparseWeight = 0.5,
      denseWeight = 0.5,
      symbolBoost = 1.5,
      pathBoost = 1.2,
      groupByFile = false,
    } = options;

    // Build rank maps
    const sparseRankMap = new Map<string, { rank: number; score: number }>();
    sparseResults.forEach((r, idx) => {
      sparseRankMap.set(r.doc_id, { rank: idx + 1, score: r.bm25_score });
    });

    const denseRankMap = new Map<string, { rank: number; score: number }>();
    denseResults.forEach((r, idx) => {
      denseRankMap.set(r.doc_id, { rank: idx + 1, score: r.dense_score });
    });

    // Collect all unique doc_ids
    const allDocIds = new Set([
      ...sparseResults.map((r) => r.doc_id),
      ...denseResults.map((r) => r.doc_id),
    ]);

    // Calculate RRF scores
    const rrfScores: Map<string, number> = new Map();
    const queryTerms = this.tokenizeQuery(query);

    for (const docId of allDocIds) {
      const sparseInfo = sparseRankMap.get(docId);
      const denseInfo = denseRankMap.get(docId);

      // RRF formula: score = sum(1 / (k + rank))
      let rrfScore = 0;

      if (sparseInfo) {
        rrfScore += (sparseWeight * 1) / (rrfK + sparseInfo.rank);
      }

      if (denseInfo) {
        rrfScore += (denseWeight * 1) / (rrfK + denseInfo.rank);
      }

      // Apply code-specific boosts
      const docInfo = contentMap.get(docId);
      if (docInfo) {
        // Symbol boost: if query terms match symbols
        const symbolMatches = this.countSymbolMatches(
          queryTerms,
          docInfo.symbols,
        );
        if (symbolMatches > 0) {
          rrfScore *= Math.pow(symbolBoost, Math.min(symbolMatches, 3));
        }
      }

      // Get result to check path
      const sparseResult = sparseResults.find((r) => r.doc_id === docId);
      const denseResult = denseResults.find((r) => r.doc_id === docId);
      const resultPath = sparseResult?.path || denseResult?.path || '';

      // Path boost: if query contains path-like patterns
      if (this.queryMatchesPath(query, resultPath)) {
        rrfScore *= pathBoost;
      }

      rrfScores.set(docId, rrfScore);
    }

    // Build final results
    const results: HybridSearchResult[] = [];

    for (const docId of allDocIds) {
      const sparseResult = sparseResults.find((r) => r.doc_id === docId);
      const denseResult = denseResults.find((r) => r.doc_id === docId);
      const docInfo = contentMap.get(docId);

      results.push({
        doc_id: docId,
        path: sparseResult?.path || denseResult?.path || '',
        language: sparseResult?.language || denseResult?.language || '',
        start_line: sparseResult?.start_line || denseResult?.start_line || 1,
        end_line: sparseResult?.end_line || denseResult?.end_line || 1,
        content: docInfo?.content || sparseResult?.content || '',
        symbols: docInfo?.symbols || sparseResult?.symbols || [],
        final_score: rrfScores.get(docId) || 0,
        sparse_score: sparseRankMap.get(docId)?.score,
        dense_score: denseRankMap.get(docId)?.score,
        sparse_rank: sparseRankMap.get(docId)?.rank,
        dense_rank: denseRankMap.get(docId)?.rank,
      });
    }

    // Sort by final score
    results.sort((a, b) => b.final_score - a.final_score);

    // Group by file if requested
    if (groupByFile) {
      return this.groupResultsByFile(results);
    }

    return results;
  }

  /**
   * Fuse results with confidence-weighted modality adjustment
   *
   * High-confidence modalities get boosted weight, low-confidence get reduced.
   * This helps when one retrieval method is clearly more reliable for a query.
   *
   * @param sparseResults Results from Tantivy (BM25)
   * @param denseResults Results from Milvus (vector)
   * @param contentMap Map of doc_id to content
   * @param query Original search query
   * @param config Confidence-weighted fusion config
   * @returns Fused results with confidence info
   */
  fuseResultsWithConfidence(
    sparseResults: TantivySearchResult[],
    denseResults: MilvusSearchResult[],
    contentMap: Map<string, { content: string; symbols: string[] }>,
    query: string,
    config: ConfidenceWeightedFusionConfig,
  ): {
    results: HybridSearchResult[];
    sparseConfidence: ModalityConfidence;
    denseConfidence: ModalityConfidence;
    adjustedWeights: { sparse: number; dense: number };
  } {
    // Compute confidence for each modality
    const sparseConfidence = this.computeModalityConfidence(
      "sparse",
      sparseResults.map((r) => r.bm25_score),
    );
    const denseConfidence = this.computeModalityConfidence(
      "dense",
      denseResults.map((r) => r.dense_score),
    );

    // Compute overlap confidence if enabled
    let overlapBonus = 0;
    if (config.useResultOverlap) {
      overlapBonus = this.computeOverlapConfidence(
        sparseResults.slice(0, 20).map((r) => r.doc_id),
        denseResults.slice(0, 20).map((r) => r.doc_id),
      );
    }

    // Adjust weights based on confidence
    let adjustedSparseWeight = config.sparseBaseWeight;
    let adjustedDenseWeight = config.denseBaseWeight;

    if (config.useScoreConfidence) {
      const totalConfidence = sparseConfidence.confidence + denseConfidence.confidence;
      if (totalConfidence > 0) {
        // Scale weights by relative confidence
        const sparseRatio = sparseConfidence.confidence / totalConfidence;
        const denseRatio = denseConfidence.confidence / totalConfidence;

        // Apply with bounds
        adjustedSparseWeight = this.boundWeight(
          config.sparseBaseWeight * (1 + (sparseRatio - 0.5) * config.maxConfidenceBoost),
          config.minConfidenceWeight,
          config.sparseBaseWeight * config.maxConfidenceBoost,
        );
        adjustedDenseWeight = this.boundWeight(
          config.denseBaseWeight * (1 + (denseRatio - 0.5) * config.maxConfidenceBoost),
          config.minConfidenceWeight,
          config.denseBaseWeight * config.maxConfidenceBoost,
        );
      }
    }

    // Apply overlap bonus (boost both if they agree)
    if (overlapBonus > 0.3) {
      adjustedSparseWeight *= 1 + overlapBonus * 0.2;
      adjustedDenseWeight *= 1 + overlapBonus * 0.2;
    }

    // Normalize weights to sum to 1
    const totalWeight = adjustedSparseWeight + adjustedDenseWeight;
    adjustedSparseWeight /= totalWeight;
    adjustedDenseWeight /= totalWeight;

    this.logger.debug(
      `Confidence fusion: sparse=${sparseConfidence.confidence.toFixed(2)} ` +
      `dense=${denseConfidence.confidence.toFixed(2)} ` +
      `overlap=${overlapBonus.toFixed(2)} → ` +
      `weights sparse=${adjustedSparseWeight.toFixed(2)} dense=${adjustedDenseWeight.toFixed(2)}`,
    );

    // Fuse with adjusted weights
    const results = this.fuseResults(sparseResults, denseResults, contentMap, query, {
      sparseWeight: adjustedSparseWeight,
      denseWeight: adjustedDenseWeight,
    });

    return {
      results,
      sparseConfidence,
      denseConfidence,
      adjustedWeights: {
        sparse: adjustedSparseWeight,
        dense: adjustedDenseWeight,
      },
    };
  }

  /**
   * Compute confidence for a single modality based on score distribution
   *
   * High confidence signals:
   * - Large gap between top and second result
   * - Low variance in top scores (clear winners)
   * - Reasonable number of results
   */
  computeModalityConfidence(
    modality: "sparse" | "dense",
    scores: number[],
  ): ModalityConfidence {
    if (scores.length === 0) {
      return {
        modality,
        confidence: 0,
        scoreGap: 0,
        scoreVariance: 0,
        resultCount: 0,
      };
    }

    // Sort descending
    const sorted = [...scores].sort((a, b) => b - a);

    // Score gap between #1 and #2
    const topScore = sorted[0];
    const secondScore = sorted[1] ?? 0;
    const scoreGap = topScore - secondScore;

    // Normalized gap (relative to top score)
    const normalizedGap = topScore > 0 ? scoreGap / topScore : 0;

    // Variance of top 10 scores
    const topScores = sorted.slice(0, Math.min(10, sorted.length));
    const mean = topScores.reduce((a, b) => a + b, 0) / topScores.length;
    const variance =
      topScores.reduce((sum, s) => sum + Math.pow(s - mean, 2), 0) / topScores.length;
    const normalizedVariance = mean > 0 ? Math.sqrt(variance) / mean : 0;

    // Result count factor (more results = potentially more confidence, up to a point)
    const countFactor = Math.min(1, scores.length / 20);

    // Combine factors into confidence score (0-1)
    // Higher gap = more confident, lower variance = more confident
    const confidence = Math.min(
      1,
      normalizedGap * 0.5 +          // Gap contributes 50%
      (1 - Math.min(1, normalizedVariance)) * 0.3 +  // Low variance contributes 30%
      countFactor * 0.2,             // Count contributes 20%
    );

    return {
      modality,
      confidence,
      scoreGap,
      scoreVariance: variance,
      resultCount: scores.length,
    };
  }

  /**
   * Compute overlap between top results from both modalities
   *
   * Higher overlap = both modalities agree = higher confidence in fusion
   */
  private computeOverlapConfidence(
    sparseDocIds: string[],
    denseDocIds: string[],
  ): number {
    const sparseSet = new Set(sparseDocIds);
    const overlapCount = denseDocIds.filter((id) => sparseSet.has(id)).length;

    // Normalized overlap (0-1)
    const maxOverlap = Math.min(sparseDocIds.length, denseDocIds.length);
    return maxOverlap > 0 ? overlapCount / maxOverlap : 0;
  }

  /**
   * Bound a weight between min and max
   */
  private boundWeight(weight: number, min: number, max: number): number {
    return Math.max(min, Math.min(max, weight));
  }

  /**
   * Tokenize query into terms
   */
  private tokenizeQuery(query: string): string[] {
    return query
      .toLowerCase()
      .split(/[\s\-_.,:;!?()[\]{}'"]+/)
      .filter((t) => t.length > 1);
  }

  /**
   * Count how many query terms match symbols
   */
  private countSymbolMatches(queryTerms: string[], symbols: string[]): number {
    const symbolSet = new Set(symbols.map((s) => s.toLowerCase()));
    return queryTerms.filter((t) => symbolSet.has(t)).length;
  }

  /**
   * Check if query contains path-like patterns that match result path
   */
  private queryMatchesPath(query: string, resultPath: string): boolean {
    // Check for path separator patterns
    const pathPatterns = query.match(/[\w\-_.]+(?:\/[\w\-_.]+)+/g) || [];
    const filenamePatterns = query.match(/[\w\-_.]+\.\w+/g) || [];

    for (const pattern of [...pathPatterns, ...filenamePatterns]) {
      if (resultPath.toLowerCase().includes(pattern.toLowerCase())) {
        return true;
      }
    }

    return false;
  }

  /**
   * Group results by file, keeping top chunk per file
   */
  private groupResultsByFile(
    results: HybridSearchResult[],
  ): HybridSearchResult[] {
    const fileMap = new Map<string, HybridSearchResult>();

    for (const result of results) {
      const existing = fileMap.get(result.path);
      if (!existing || result.final_score > existing.final_score) {
        fileMap.set(result.path, result);
      }
    }

    return Array.from(fileMap.values()).sort(
      (a, b) => b.final_score - a.final_score,
    );
  }

  /**
   * Simple sparse-only ranking (for when dense is not available)
   */
  rankSparseOnly(results: TantivySearchResult[]): HybridSearchResult[] {
    return results.map((r) => ({
      doc_id: r.doc_id,
      path: r.path,
      language: r.language,
      start_line: r.start_line,
      end_line: r.end_line,
      content: r.content,
      symbols: r.symbols,
      final_score: r.bm25_score,
      sparse_score: r.bm25_score,
      sparse_rank: r.rank,
    }));
  }

  /**
   * Simple dense-only ranking (for when sparse is not available)
   */
  rankDenseOnly(
    results: MilvusSearchResult[],
    contentMap: Map<string, { content: string; symbols: string[] }>,
  ): HybridSearchResult[] {
    return results.map((r) => {
      const docInfo = contentMap.get(r.doc_id);
      return {
        doc_id: r.doc_id,
        path: r.path,
        language: r.language,
        start_line: r.start_line,
        end_line: r.end_line,
        content: docInfo?.content || '',
        symbols: docInfo?.symbols || [],
        final_score: r.dense_score,
        dense_score: r.dense_score,
        dense_rank: r.dense_rank,
      };
    });
  }

  /**
   * Compute statistics about the fused result distribution
   * Used for confidence estimation and early exit decisions
   *
   * @param results Hybrid search results sorted by final_score (descending)
   * @returns Statistics about score distribution
   */
  computeFusionStats(results: HybridSearchResult[]): FusionStats {
    const resultCount = results.length;

    // Handle edge cases
    if (resultCount === 0) {
      return {
        topScore: 0,
        secondScore: 0,
        scoreGap: 0,
        scoreRatio: 0,
        tailMean: 0,
        resultCount: 0,
      };
    }

    const topScore = results[0].final_score;

    if (resultCount === 1) {
      return {
        topScore,
        secondScore: 0,
        scoreGap: topScore,
        scoreRatio: Infinity,
        tailMean: 0,
        resultCount: 1,
      };
    }

    const secondScore = results[1].final_score;
    const scoreGap = topScore - secondScore;
    const scoreRatio = secondScore === 0 ? Infinity : topScore / secondScore;

    // Compute tail mean (positions 3-10, or as many as available)
    let tailMean = 0;
    if (resultCount > 2) {
      const tailStart = 2;
      const tailEnd = Math.min(10, resultCount);
      const tailScores = results
        .slice(tailStart, tailEnd)
        .map((r) => r.final_score);

      if (tailScores.length > 0) {
        tailMean =
          tailScores.reduce((sum, score) => sum + score, 0) / tailScores.length;
      }
    }

    return {
      topScore,
      secondScore,
      scoreGap,
      scoreRatio,
      tailMean,
      resultCount,
    };
  }
}
