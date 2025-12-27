import { Module } from '@nestjs/common';
import { McpController } from './mcp.controller';
import { McpService } from './mcp.service';
import { SearchModule } from '../search/search.module';
import { IndexModule } from '../index/index.module';

@Module({
  imports: [SearchModule, IndexModule],
  controllers: [McpController],
  providers: [McpService],
  exports: [McpService],
})
export class McpModule {}
