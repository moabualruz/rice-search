import { Module } from '@nestjs/common';
import { McpController } from './mcp.controller';
import { McpService } from './mcp.service';
import { SearchModule } from '../search/search.module';
import { IndexModule } from '../index/index.module';
import { StoresModule } from '../stores/stores.module';
import { ServicesModule } from '../services/services.module';

@Module({
  imports: [
    SearchModule,
    IndexModule,
    StoresModule,
    ServicesModule, // For FileTrackerService
  ],
  controllers: [McpController],
  providers: [McpService],
  exports: [McpService],
})
export class McpModule {}
