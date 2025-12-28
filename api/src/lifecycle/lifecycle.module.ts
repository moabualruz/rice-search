import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { StoreVersioningService } from "./store-versioning.service";
import { ABEvaluationService } from "./ab-evaluation.service";
import { UnionQueryService } from "./union-query.service";
import { SchemaEvolutionService } from "./schema-evolution.service";

@Module({
  imports: [ConfigModule],
  providers: [
    StoreVersioningService,
    ABEvaluationService,
    UnionQueryService,
    SchemaEvolutionService,
  ],
  exports: [
    StoreVersioningService,
    ABEvaluationService,
    UnionQueryService,
    SchemaEvolutionService,
  ],
})
export class LifecycleModule {}
