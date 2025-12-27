import { Controller, Get } from '@nestjs/common';
import { ApiTags, ApiOperation, ApiResponse } from '@nestjs/swagger';
import { HealthCheck, HealthCheckService } from '@nestjs/terminus';
import { ConfigService } from '@nestjs/config';

@ApiTags('health')
@Controller()
export class HealthController {
  constructor(
    private health: HealthCheckService,
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
      async () => {
        try {
          const controller = new AbortController();
          const timeoutId = setTimeout(() => controller.abort(), 5000);
          const response = await fetch(`${embeddingsUrl}/health`, {
            signal: controller.signal,
          });
          clearTimeout(timeoutId);
          return {
            embeddings: {
              status: response.ok ? 'up' : 'down',
            },
          };
        } catch {
          return {
            embeddings: {
              status: 'down',
            },
          };
        }
      },
    ]);
  }
}
