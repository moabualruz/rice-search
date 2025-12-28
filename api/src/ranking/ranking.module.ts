import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { MultiPassRerankerService } from "./multi-pass-reranker.service";
import { ServicesModule } from "../services/services.module";

@Module({
  imports: [ConfigModule, ServicesModule],
  providers: [MultiPassRerankerService],
  exports: [MultiPassRerankerService],
})
export class RankingModule {}
