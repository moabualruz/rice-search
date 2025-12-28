import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { DeduplicationService } from "./deduplication.service";
import { AggregationService } from "./aggregation.service";
import { DiversityService } from "./diversity.service";
import { PostrankPipelineService } from "./postrank-pipeline.service";
import { ServicesModule } from "../services/services.module";

@Module({
  imports: [ConfigModule, ServicesModule],
  providers: [
    DeduplicationService,
    AggregationService,
    DiversityService,
    PostrankPipelineService,
  ],
  exports: [
    DeduplicationService,
    AggregationService,
    DiversityService,
    PostrankPipelineService,
  ],
})
export class PostrankModule {}
