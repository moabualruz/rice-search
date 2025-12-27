import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse } from '@nestjs/swagger';
import {
  HealthCheck,
  HealthCheckService,
  HttpHealthIndicator,
} from '@nestjs/terminus';
import { ConfigService } from '@nestjs/config';

@ApiTags('health')
@Controller()
export class HealthController {
  constructor(
    private health: HealthCheckService,
    private http: HttpHealthIndicator,
    private configService: ConfigService,
  ) {}

  @Get('healthz')
  @ApiOperation({ summary: 'Health check endpoint' })
  @ApiResponse({ status: 200, description: 'Service is healthy' })
  @HealthCheck()
  check() {
    return { status: 'ok' };
  }

  @Get('v1/version')
  @ApiOperation({ summary: 'Get API version information' })
  @ApiResponse({ status: 200, description: 'Version information' })
  version() {
    return {
      version: '0.1.0',
      build_time: new Date().toISOString(),
      git_commit: process.env.GIT_COMMIT || '',
      node_version: process.version,
    };
  }

  @Get('v1/health')
  @ApiOperation({ summary: 'Detailed health check with dependencies' })
  @ApiResponse({ status: 200, description: 'Detailed health status' })
  @HealthCheck()
  async detailedHealth() {
    const embeddingsUrl = this.configService.get<string>('embeddings.url');

    return this.health.check([
      () =>
        this.http.pingCheck('embeddings', `${embeddingsUrl}/health`, {
          timeout: 5000,
        }),
    ]);
  }
}
