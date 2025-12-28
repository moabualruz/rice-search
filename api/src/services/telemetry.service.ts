import { Injectable, Logger } from "@nestjs/common";
import { QueryIntent } from "../intelligence/intent-classifier.service";
import { RetrievalStrategy } from "../intelligence/strategy-selector.service";

/**
 * Telemetry data for sparse (BM25) search
 */
export interface SparseTelemetry {
  resultCount: number;
  latencyMs: number;
  topScore: number;
  scoreStdDev: number;
  scoreP50?: number;
  scoreP90?: number;
}

/**
 * Telemetry data for dense (vector) search
 */
export interface DenseTelemetry {
  resultCount: number;
  latencyMs: number;
  topScore: number;
  scoreStdDev: number;
  scoreP50?: number;
  scoreP90?: number;
}

/**
 * Telemetry data for hybrid fusion (RRF)
 */
export interface FusionTelemetry {
  resultCount: number;
  latencyMs: number;
  topScore: number;
  secondScore: number;
  scoreGap: number;
  scoreRatio: number;
}

/**
 * Telemetry data for reranking
 */
export interface RerankTelemetry {
  enabled: boolean;
  candidates: number;
  latencyMs: number;
  timedOut: boolean;
  skipped: boolean;
  skipReason?: string;
  pass1Applied?: boolean;
  pass1LatencyMs?: number;
  pass2Applied?: boolean;
  pass2LatencyMs?: number;
  earlyExit?: boolean;
}

/**
 * Telemetry data for post-rank processing
 */
export interface PostrankTelemetry {
  dedupEnabled: boolean;
  dedupRemoved: number;
  dedupLatencyMs: number;
  diversityEnabled: boolean;
  diversityAvg: number;
  diversityLatencyMs: number;
  aggregationEnabled: boolean;
  uniqueFiles: number;
  totalLatencyMs: number;
}

/**
 * Telemetry data for cache hits
 */
export interface CacheTelemetry {
  embeddingHit: boolean;
  rerankHit: boolean;
}

/**
 * Complete search telemetry record
 */
export interface SearchTelemetryRecord {
  requestId: string;
  timestamp: Date;
  store: string;
  storeVersion?: string;
  query: string;
  normalizedQuery?: string;
  queryType: string; // code, natural, hybrid

  // Intelligence pipeline (Phase 1)
  intent?: QueryIntent;
  difficulty?: string;
  strategy?: RetrievalStrategy;
  strategyConfig?: {
    sparseTopK: number;
    denseTopK: number;
    rerankCandidates: number;
    useSecondPass: boolean;
  };

  sparse: SparseTelemetry;
  dense: DenseTelemetry;
  fusion: FusionTelemetry;
  rerank: RerankTelemetry;
  postrank?: PostrankTelemetry;
  cache: CacheTelemetry;

  totalLatencyMs: number;
  resultCount: number;
  topResultScore?: number;
}

/**
 * Aggregated telemetry statistics
 */
interface AggregatedStats {
  totalQueries: number;
  avgLatencyMs: number;
  cacheHitRate: number;
  rerankSkipRate: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
}

/**
 * TelemetryService collects and aggregates search telemetry for observability.
 *
 * Tracks comprehensive metrics across the search pipeline:
 * - Sparse (BM25) and Dense (vector) search performance
 * - Hybrid fusion quality signals
 * - Reranking behavior (timeouts, skips)
 * - Cache hit rates
 * - Latency percentiles
 *
 * Provides Prometheus-compatible metrics export for monitoring.
 */
@Injectable()
export class TelemetryService {
  private readonly logger = new Logger(TelemetryService.name);
  private readonly records: SearchTelemetryRecord[] = [];
  private readonly maxRecords = 10000; // Ring buffer

  // Aggregated statistics
  private aggregates: AggregatedStats = {
    totalQueries: 0,
    avgLatencyMs: 0,
    cacheHitRate: 0,
    rerankSkipRate: 0,
    p50LatencyMs: 0,
    p95LatencyMs: 0,
    p99LatencyMs: 0,
  };

  /**
   * Record a search telemetry entry
   *
   * @param telemetry - Search telemetry data
   */
  record(telemetry: SearchTelemetryRecord): void {
    // Add to ring buffer
    this.records.push(telemetry);

    // Maintain ring buffer size
    if (this.records.length > this.maxRecords) {
      this.records.shift();
    }

    // Update aggregates
    this.updateAggregates(telemetry);

    this.logger.debug(
      `Recorded telemetry: query="${telemetry.query.substring(0, 50)}", ` +
      `total=${telemetry.totalLatencyMs}ms, results=${telemetry.resultCount}`,
    );
  }

  /**
   * Get recent telemetry records
   *
   * @param limit - Maximum number of records to return (default: 100)
   * @returns Recent telemetry records
   */
  getRecords(limit = 100): SearchTelemetryRecord[] {
    const start = Math.max(0, this.records.length - limit);
    return this.records.slice(start);
  }

  /**
   * Get telemetry records for a time range
   *
   * @param since - Start date
   * @param until - End date (optional, defaults to now)
   * @returns Filtered telemetry records
   */
  getRecordsByTimeRange(since: Date, until?: Date): SearchTelemetryRecord[] {
    const endDate = until ?? new Date();
    return this.records.filter(
      (r) => r.timestamp >= since && r.timestamp <= endDate
    );
  }

  /**
   * Get telemetry records for a specific store
   *
   * @param store - Store name
   * @param limit - Maximum records (default: 100)
   * @returns Filtered telemetry records
   */
  getRecordsByStore(store: string, limit = 100): SearchTelemetryRecord[] {
    return this.records
      .filter((r) => r.store === store)
      .slice(-limit);
  }

  /**
   * Get strategy distribution from recent records
   */
  getStrategyDistribution(): Map<string, number> {
    const distribution = new Map<string, number>();
    for (const record of this.records) {
      const strategy = record.strategy ?? "unknown";
      distribution.set(strategy, (distribution.get(strategy) ?? 0) + 1);
    }
    return distribution;
  }

  /**
   * Get intent distribution from recent records
   */
  getIntentDistribution(): Map<string, number> {
    const distribution = new Map<string, number>();
    for (const record of this.records) {
      const intent = record.intent ?? "unknown";
      distribution.set(intent, (distribution.get(intent) ?? 0) + 1);
    }
    return distribution;
  }

  /**
   * Get aggregated statistics
   *
   * @returns Aggregated telemetry stats
   */
  getAggregates(): AggregatedStats {
    return { ...this.aggregates };
  }

  /**
   * Compute statistical metrics for score array
   *
   * @param scores - Array of scores
   * @returns Mean and standard deviation
   */
  computeScoreStats(scores: number[]): { mean: number; stdDev: number } {
    if (scores.length === 0) {
      return { mean: 0, stdDev: 0 };
    }

    // Calculate mean
    const mean = scores.reduce((sum, score) => sum + score, 0) / scores.length;

    // Calculate standard deviation
    const variance =
      scores.reduce((sum, score) => sum + Math.pow(score - mean, 2), 0) /
      scores.length;
    const stdDev = Math.sqrt(variance);

    return { mean, stdDev };
  }

  /**
   * Export metrics in Prometheus format
   *
   * @returns Prometheus-formatted metrics
   */
  exportPrometheus(): string {
    const lines: string[] = [];

    // Total queries counter
    lines.push("# HELP rice_search_queries_total Total number of search queries");
    lines.push("# TYPE rice_search_queries_total counter");
    lines.push(`rice_search_queries_total ${this.aggregates.totalQueries}`);

    // Average latency
    lines.push("# HELP rice_search_latency_avg_ms Average query latency in milliseconds");
    lines.push("# TYPE rice_search_latency_avg_ms gauge");
    lines.push(`rice_search_latency_avg_ms ${this.aggregates.avgLatencyMs.toFixed(2)}`);

    // Latency percentiles
    lines.push("# HELP rice_search_latency_ms Query latency percentiles in milliseconds");
    lines.push("# TYPE rice_search_latency_ms gauge");
    lines.push(`rice_search_latency_ms{quantile="0.5"} ${this.aggregates.p50LatencyMs.toFixed(2)}`);
    lines.push(`rice_search_latency_ms{quantile="0.95"} ${this.aggregates.p95LatencyMs.toFixed(2)}`);
    lines.push(`rice_search_latency_ms{quantile="0.99"} ${this.aggregates.p99LatencyMs.toFixed(2)}`);

    // Cache hit rate
    lines.push("# HELP rice_search_cache_hit_rate Embedding cache hit rate");
    lines.push("# TYPE rice_search_cache_hit_rate gauge");
    lines.push(`rice_search_cache_hit_rate ${this.aggregates.cacheHitRate.toFixed(4)}`);

    // Rerank skip rate
    lines.push("# HELP rice_search_rerank_skip_rate Reranking skip rate");
    lines.push("# TYPE rice_search_rerank_skip_rate gauge");
    lines.push(`rice_search_rerank_skip_rate ${this.aggregates.rerankSkipRate.toFixed(4)}`);

    return lines.join("\n") + "\n";
  }

  /**
   * Update aggregated statistics with new telemetry record
   *
   * @param telemetry - New telemetry record
   */
  private updateAggregates(telemetry: SearchTelemetryRecord): void {
    const prevTotal = this.aggregates.totalQueries;
    const prevAvgLatency = this.aggregates.avgLatencyMs;

    // Increment total queries
    this.aggregates.totalQueries++;

    // Update rolling average latency
    this.aggregates.avgLatencyMs =
      (prevAvgLatency * prevTotal + telemetry.totalLatencyMs) /
      this.aggregates.totalQueries;

    // Update cache hit rate (rolling average)
    const cacheHit = telemetry.cache.embeddingHit ? 1 : 0;
    const prevCacheHitRate = this.aggregates.cacheHitRate;
    this.aggregates.cacheHitRate =
      (prevCacheHitRate * prevTotal + cacheHit) /
      this.aggregates.totalQueries;

    // Update rerank skip rate (rolling average)
    const rerankSkipped = telemetry.rerank.skipped ? 1 : 0;
    const prevRerankSkipRate = this.aggregates.rerankSkipRate;
    this.aggregates.rerankSkipRate =
      (prevRerankSkipRate * prevTotal + rerankSkipped) /
      this.aggregates.totalQueries;

    // Update percentiles (computed from recent records)
    this.updatePercentiles();
  }

  /**
   * Update latency percentiles from recent records
   */
  private updatePercentiles(): void {
    if (this.records.length === 0) {
      return;
    }

    // Get latencies sorted
    const latencies = this.records
      .map((r) => r.totalLatencyMs)
      .sort((a, b) => a - b);

    // Calculate percentiles
    this.aggregates.p50LatencyMs = this.getPercentile(latencies, 0.5);
    this.aggregates.p95LatencyMs = this.getPercentile(latencies, 0.95);
    this.aggregates.p99LatencyMs = this.getPercentile(latencies, 0.99);
  }

  /**
   * Calculate percentile value from sorted array
   *
   * @param sortedValues - Sorted array of values
   * @param percentile - Percentile to calculate (0.0 - 1.0)
   * @returns Percentile value
   */
  private getPercentile(sortedValues: number[], percentile: number): number {
    if (sortedValues.length === 0) {
      return 0;
    }

    const index = Math.ceil(sortedValues.length * percentile) - 1;
    return sortedValues[Math.max(0, Math.min(index, sortedValues.length - 1))];
  }

  /**
   * Clear all telemetry records and reset aggregates
   * Useful for testing or resetting metrics
   */
  reset(): void {
    this.records.length = 0;
    this.aggregates = {
      totalQueries: 0,
      avgLatencyMs: 0,
      cacheHitRate: 0,
      rerankSkipRate: 0,
      p50LatencyMs: 0,
      p95LatencyMs: 0,
      p99LatencyMs: 0,
    };
    this.logger.log("Telemetry reset");
  }
}
