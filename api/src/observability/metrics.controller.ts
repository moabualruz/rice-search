import { Controller, Get, Query } from "@nestjs/common";
import { ApiOperation, ApiQuery, ApiResponse, ApiTags } from "@nestjs/swagger";
import { TelemetryService } from "../services/telemetry.service";
import { QueryLogService } from "./query-log.service";
import { EvaluationService } from "./evaluation.service";

/**
 * MetricsController exposes Prometheus-compatible metrics and observability endpoints.
 *
 * Endpoints:
 * - GET /metrics - Prometheus scrape endpoint
 * - GET /v1/observability/stats - Human-readable stats
 * - GET /v1/observability/query-stats - Query log statistics
 */
@ApiTags("observability")
@Controller()
export class MetricsController {
  constructor(
    private telemetry: TelemetryService,
    private queryLog: QueryLogService,
    private evaluation: EvaluationService,
  ) {}

  /**
   * Prometheus metrics endpoint
   * Standard /metrics path for Prometheus scraping
   */
  @Get("metrics")
  @ApiOperation({ summary: "Prometheus metrics endpoint" })
  @ApiResponse({
    status: 200,
    description: "Prometheus-formatted metrics",
    content: {
      "text/plain": {
        example: `# HELP rice_search_queries_total Total number of search queries
# TYPE rice_search_queries_total counter
rice_search_queries_total 12345`,
      },
    },
  })
  getPrometheusMetrics(): string {
    const baseMetrics = this.telemetry.exportPrometheus();

    // Add additional observability metrics
    const lines: string[] = [baseMetrics];

    // Strategy distribution
    const strategyDist = this.telemetry.getStrategyDistribution();
    lines.push(
      "# HELP rice_search_strategy_count Search queries by strategy",
    );
    lines.push("# TYPE rice_search_strategy_count gauge");
    for (const [strategy, count] of strategyDist) {
      lines.push(`rice_search_strategy_count{strategy="${strategy}"} ${count}`);
    }

    // Intent distribution
    const intentDist = this.telemetry.getIntentDistribution();
    lines.push("# HELP rice_search_intent_count Search queries by intent");
    lines.push("# TYPE rice_search_intent_count gauge");
    for (const [intent, count] of intentDist) {
      lines.push(`rice_search_intent_count{intent="${intent}"} ${count}`);
    }

    // Evaluation metrics (if loaded)
    const judgmentStats = this.evaluation.getJudgmentStats();
    lines.push(
      "# HELP rice_search_judgments_total Total relevance judgments loaded",
    );
    lines.push("# TYPE rice_search_judgments_total gauge");
    lines.push(`rice_search_judgments_total ${judgmentStats.totalJudgments}`);

    lines.push(
      "# HELP rice_search_judgment_queries_total Queries with relevance judgments",
    );
    lines.push("# TYPE rice_search_judgment_queries_total gauge");
    lines.push(
      `rice_search_judgment_queries_total ${judgmentStats.totalQueries}`,
    );

    return lines.join("\n") + "\n";
  }

  /**
   * Human-readable observability stats
   */
  @Get("v1/observability/stats")
  @ApiOperation({ summary: "Observability statistics" })
  @ApiResponse({
    status: 200,
    description: "Aggregated observability statistics",
  })
  getObservabilityStats(): {
    telemetry: {
      totalQueries: number;
      avgLatencyMs: number;
      cacheHitRate: number;
      rerankSkipRate: number;
      p50LatencyMs: number;
      p95LatencyMs: number;
      p99LatencyMs: number;
    };
    strategies: Record<string, number>;
    intents: Record<string, number>;
    judgments: {
      totalQueries: number;
      totalJudgments: number;
      avgJudgmentsPerQuery: number;
    };
  } {
    const aggregates = this.telemetry.getAggregates();

    // Convert Maps to plain objects
    const strategyDist = this.telemetry.getStrategyDistribution();
    const strategies: Record<string, number> = {};
    for (const [k, v] of strategyDist) {
      strategies[k] = v;
    }

    const intentDist = this.telemetry.getIntentDistribution();
    const intents: Record<string, number> = {};
    for (const [k, v] of intentDist) {
      intents[k] = v;
    }

    return {
      telemetry: aggregates,
      strategies,
      intents,
      judgments: this.evaluation.getJudgmentStats(),
    };
  }

  /**
   * Query log statistics for a store
   */
  @Get("v1/observability/query-stats")
  @ApiOperation({ summary: "Query log statistics for a store" })
  @ApiQuery({ name: "store", required: true, description: "Store name" })
  @ApiQuery({
    name: "days",
    required: false,
    description: "Number of days to analyze (default: 7)",
  })
  @ApiResponse({
    status: 200,
    description: "Query statistics",
  })
  async getQueryStats(
    @Query("store") store: string,
    @Query("days") days?: string,
  ): Promise<{
    store: string;
    period: { since: string; until: string };
    totalQueries: number;
    uniqueQueries: number;
    avgLatencyMs: number;
    avgResultCount: number;
    intentDistribution: Record<string, number>;
    strategyDistribution: Record<string, number>;
  }> {
    const daysNum = days ? parseInt(days, 10) : 7;
    const since = new Date();
    since.setDate(since.getDate() - daysNum);

    const stats = await this.queryLog.getQueryStats(store, since);

    return {
      store,
      period: {
        since: since.toISOString(),
        until: new Date().toISOString(),
      },
      ...stats,
    };
  }

  /**
   * Recent queries for a store (for debugging)
   */
  @Get("v1/observability/recent-queries")
  @ApiOperation({ summary: "Recent queries for a store" })
  @ApiQuery({ name: "store", required: true, description: "Store name" })
  @ApiQuery({
    name: "limit",
    required: false,
    description: "Max queries to return (default: 50)",
  })
  @ApiResponse({
    status: 200,
    description: "Recent query log entries",
  })
  async getRecentQueries(
    @Query("store") store: string,
    @Query("limit") limit?: string,
  ) {
    const limitNum = limit ? parseInt(limit, 10) : 50;
    const queries = await this.queryLog.getRecentQueries(store, limitNum);

    return {
      store,
      count: queries.length,
      queries: queries.map((q) => ({
        timestamp: q.timestamp,
        query: q.query,
        intent: q.intent,
        strategy: q.strategy,
        resultCount: q.resultCount,
        latencyMs: q.totalLatencyMs,
      })),
    };
  }

  /**
   * Telemetry records (recent search details)
   */
  @Get("v1/observability/telemetry")
  @ApiOperation({ summary: "Recent telemetry records" })
  @ApiQuery({
    name: "limit",
    required: false,
    description: "Max records to return (default: 100)",
  })
  @ApiQuery({
    name: "store",
    required: false,
    description: "Filter by store name",
  })
  @ApiResponse({
    status: 200,
    description: "Recent telemetry records",
  })
  getTelemetryRecords(
    @Query("limit") limit?: string,
    @Query("store") store?: string,
  ) {
    const limitNum = limit ? parseInt(limit, 10) : 100;

    let records;
    if (store) {
      records = this.telemetry.getRecordsByStore(store, limitNum);
    } else {
      records = this.telemetry.getRecords(limitNum);
    }

    return {
      count: records.length,
      records: records.map((r) => ({
        requestId: r.requestId,
        timestamp: r.timestamp,
        store: r.store,
        query: r.query.substring(0, 100),
        intent: r.intent,
        strategy: r.strategy,
        resultCount: r.resultCount,
        totalLatencyMs: r.totalLatencyMs,
        sparse: {
          count: r.sparse.resultCount,
          latencyMs: r.sparse.latencyMs,
        },
        dense: {
          count: r.dense.resultCount,
          latencyMs: r.dense.latencyMs,
        },
        rerank: {
          enabled: r.rerank.enabled,
          skipped: r.rerank.skipped,
          latencyMs: r.rerank.latencyMs,
        },
      })),
    };
  }
}
