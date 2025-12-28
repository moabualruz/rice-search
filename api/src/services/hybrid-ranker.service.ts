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
