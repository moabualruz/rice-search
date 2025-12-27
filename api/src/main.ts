import cluster from 'node:cluster';
import { cpus } from 'node:os';
import { NestFactory } from '@nestjs/core';
import {
  FastifyAdapter,
  NestFastifyApplication,
} from '@nestjs/platform-fastify';
import { ValidationPipe, Logger } from '@nestjs/common';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { AppModule } from './app.module';
import compression from '@fastify/compress';

const isDev = process.env.NODE_ENV !== 'production';
const enableClustering = process.env.CLUSTER_MODE === 'true' && !isDev;
const numWorkers = parseInt(process.env.CLUSTER_WORKERS || '0', 10) || cpus().length;

async function bootstrapWorker() {
  const logger = new Logger('Bootstrap');

  // Create Fastify adapter with maximum concurrency optimizations
  const fastifyAdapter = new FastifyAdapter({
    logger: false, // Use NestJS logger
    bodyLimit: 1024 * 1024 * 1024, // 1GB - local traffic only
    caseSensitive: false,
    ignoreTrailingSlash: true,
    // Maximize connection capacity
    connectionTimeout: 0, // No connection timeout
    keepAliveTimeout: 72000, // 72 seconds - reuse connections longer
    maxParamLength: 500,
    // Disable overhead
    requestIdHeader: false,
    disableRequestLogging: true,
  });

  const app = await NestFactory.create(
    AppModule,
    fastifyAdapter,
    {
      bufferLogs: true,
      abortOnError: !isDev,
    },
  ) as NestFastifyApplication;

  // Register compression (gzip/brotli)
  await app.register(compression, {
    encodings: ['gzip', 'deflate'],
    threshold: 1024, // Only compress responses > 1KB
  });

  // Optimized validation pipe
  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      transform: true,
      forbidNonWhitelisted: true,
      transformOptions: {
        enableImplicitConversion: true,
      },
      disableErrorMessages: !isDev,
      stopAtFirstError: true,
    }),
  );

  // CORS with caching
  app.enableCors({
    origin: true,
    credentials: true,
    maxAge: 86400, // Cache preflight for 24 hours
  });

  // Swagger - only in development
  if (isDev) {
    const config = new DocumentBuilder()
      .setTitle('Rice Search API')
      .setDescription(
        'Rice Search - Hybrid code search API with BM25 + semantic retrieval',
      )
      .setVersion('0.1.0')
      .addTag('health', 'Health check endpoints')
      .addTag('stores', 'Store management')
      .addTag('index', 'Indexing operations')
      .addTag('search', 'Search operations')
      .addTag('mcp', 'MCP protocol endpoints')
      .build();

    const document = SwaggerModule.createDocument(app, config);
    SwaggerModule.setup('docs', app, document);
  }

  const port = process.env.PORT || 8080;
  const host = process.env.HOST || '0.0.0.0';

  await app.listen(port, host);

  const workerId = cluster.isWorker ? `Worker ${cluster.worker?.id}` : 'Main';
  logger.log(`[${workerId}] Rice Search API on http://${host}:${port}`);
  
  if (isDev) {
    logger.log(`[${workerId}] Swagger docs at http://localhost:${port}/docs`);
  }
  logger.log(`Fastify adapter with compression enabled`);
}

async function bootstrap() {
  if (enableClustering && cluster.isPrimary) {
    console.log(`[Master] Starting ${numWorkers} workers...`);

    for (let i = 0; i < numWorkers; i++) {
      cluster.fork();
    }

    cluster.on('exit', (worker, code, signal) => {
      console.log(`[Master] Worker ${worker.process.pid} died (${signal || code}). Restarting...`);
      cluster.fork();
    });

    cluster.on('online', (worker) => {
      console.log(`[Master] Worker ${worker.process.pid} is online`);
    });
  } else {
    await bootstrapWorker();
  }
}

bootstrap();
