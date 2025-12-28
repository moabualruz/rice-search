import { Module } from "@nestjs/common";
import { QueryLogService } from "./query-log.service";
import { EvaluationService } from "./evaluation.service";
import { MetricsController } from "./metrics.controller";
import { ServicesModule } from "../services/services.module";

/**
 * ObservabilityModule provides search quality monitoring and evaluation.
 *
 * Components:
 * - QueryLogService: Persists queries to JSONL for replay and analysis
 * - EvaluationService: Computes IR metrics (NDCG, Recall, MRR, MAP)
 * - MetricsController: Prometheus-compatible /metrics endpoint
 *
 * The module integrates with TelemetryService (from ServicesModule) for
 * real-time metrics and adds persistent logging + offline evaluation.
 */
@Module({
  imports: [ServicesModule],
  controllers: [MetricsController],
  providers: [QueryLogService, EvaluationService],
  exports: [QueryLogService, EvaluationService],
})
export class ObservabilityModule {}
