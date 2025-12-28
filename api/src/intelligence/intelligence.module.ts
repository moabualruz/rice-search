import { Module } from "@nestjs/common";
import { ConfigModule } from "@nestjs/config";
import { IntentClassifierService } from "./intent-classifier.service";
import { StrategySelectorService } from "./strategy-selector.service";
import { ServicesModule } from "../services/services.module";

@Module({
  imports: [ConfigModule, ServicesModule],
  providers: [IntentClassifierService, StrategySelectorService],
  exports: [IntentClassifierService, StrategySelectorService],
})
export class IntelligenceModule {}
