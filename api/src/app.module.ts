import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { TerminusModule } from '@nestjs/terminus';

import configuration from './config/configuration';
import { HealthModule } from './health/health.module';
import { StoresModule } from './stores/stores.module';
import { SearchModule } from './search/search.module';
import { IndexModule } from './index/index.module';
import { McpModule } from './mcp/mcp.module';
import { ServicesModule } from './services/services.module';

@Module({
  imports: [
    // Configuration with caching for faster access
    ConfigModule.forRoot({
      isGlobal: true,
      load: [configuration],
      cache: true,
    }),

    // Health checks
    TerminusModule,
    HealthModule,

    // Core services
    ServicesModule,

    // Feature modules
    StoresModule,
    SearchModule,
    IndexModule,
    McpModule,
  ],
})
export class AppModule {}
