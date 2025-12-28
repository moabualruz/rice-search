import { Injectable, Logger } from "@nestjs/common";
import { HybridSearchResult } from "../services/hybrid-ranker.service";

export interface AggregationInfo {
  isRepresentative: boolean;  // Is this the top chunk for its file?
  relatedChunks: number;      // How many other chunks from same file?
  fileScore: number;          // Aggregate score for the file
  chunkRankInFile: number;    // This chunk's rank within its file (1-indexed)
}

export interface AggregatedResult extends HybridSearchResult {
  aggregation?: AggregationInfo;
}

export interface AggregationConfig {
  groupByFile: boolean;
  maxChunksPerFile: number;  // Max chunks to return per file
  aggregateScores: boolean;  // Combine scores for file-level ranking
}

export interface AggregationStats {
  inputCount: number;
  outputCount: number;
  uniqueFiles: number;
  chunksDropped: number;
}

const DEFAULT_AGGREGATION_CONFIG: AggregationConfig = {
  groupByFile: false,
  maxChunksPerFile: 3,
  aggregateScores: true,
};

@Injectable()
export class AggregationService {
  private readonly logger = new Logger(AggregationService.name);

  /**
   * Aggregate results by file, keeping top chunks per file
   * 
   * @param results - Search results to aggregate
   * @param config - Aggregation configuration
   * @returns Aggregated results with file-level metadata
   */
  aggregate(
    results: HybridSearchResult[],
    config: Partial<AggregationConfig> = {},
  ): { results: AggregatedResult[]; stats: AggregationStats } {
    const mergedConfig: AggregationConfig = {
      ...DEFAULT_AGGREGATION_CONFIG,
      ...config,
    };

    // If grouping is disabled, just add empty aggregation info
    if (!mergedConfig.groupByFile) {
      return {
        results: results.map((r) => ({ ...r })),
        stats: {
          inputCount: results.length,
          outputCount: results.length,
          uniqueFiles: new Set(results.map((r) => r.path)).size,
          chunksDropped: 0,
        },
      };
    }

    // Group by file path
    const fileGroups = new Map<string, HybridSearchResult[]>();
    for (const result of results) {
      const group = fileGroups.get(result.path) || [];
      group.push(result);
      fileGroups.set(result.path, group);
    }

    // Process each file group
    const aggregated: AggregatedResult[] = [];
    let chunksDropped = 0;

    for (const [path, chunks] of fileGroups) {
      // Sort chunks within file by score (descending)
      chunks.sort((a, b) => b.final_score - a.final_score);

      // Calculate file-level aggregate score
      const fileScore = mergedConfig.aggregateScores
        ? this.calculateFileScore(chunks)
        : chunks[0].final_score;

      // Keep top N chunks per file
      const keptChunks = chunks.slice(0, mergedConfig.maxChunksPerFile);
      chunksDropped += chunks.length - keptChunks.length;

      for (let i = 0; i < keptChunks.length; i++) {
        aggregated.push({
          ...keptChunks[i],
          aggregation: {
            isRepresentative: i === 0,
            relatedChunks: chunks.length,
            fileScore,
            chunkRankInFile: i + 1,
          },
        });
      }
    }

    // Sort aggregated results:
    // 1. Representative chunks first (sorted by fileScore)
    // 2. Non-representative chunks (sorted by individual score)
    aggregated.sort((a, b) => {
      const aIsRep = a.aggregation?.isRepresentative ?? false;
      const bIsRep = b.aggregation?.isRepresentative ?? false;

      if (aIsRep && !bIsRep) return -1;
      if (!aIsRep && bIsRep) return 1;

      if (aIsRep && bIsRep) {
        // Both are representatives - sort by file score
        return (b.aggregation?.fileScore ?? 0) - (a.aggregation?.fileScore ?? 0);
      }

      // Both are non-representatives - sort by individual score
      return b.final_score - a.final_score;
    });

    const stats: AggregationStats = {
      inputCount: results.length,
      outputCount: aggregated.length,
      uniqueFiles: fileGroups.size,
      chunksDropped,
    };

    if (chunksDropped > 0) {
      this.logger.debug(
        `Aggregation: ${stats.inputCount} → ${stats.outputCount} ` +
        `(${stats.uniqueFiles} files, dropped ${chunksDropped} chunks)`
      );
    }

    return { results: aggregated, stats };
  }

  /**
   * Calculate aggregate score for a file based on its chunks
   * Uses weighted average favoring top chunks
   */
  private calculateFileScore(chunks: HybridSearchResult[]): number {
    if (chunks.length === 0) return 0;
    if (chunks.length === 1) return chunks[0].final_score;

    // Weighted average: top chunk gets most weight
    // Weights: 1st=1.0, 2nd=0.5, 3rd=0.25, etc.
    let weightedSum = 0;
    let totalWeight = 0;

    for (let i = 0; i < chunks.length; i++) {
      const weight = 1 / Math.pow(2, i);
      weightedSum += chunks[i].final_score * weight;
      totalWeight += weight;
    }

    return weightedSum / totalWeight;
  }
}
