import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  // Enable validation
  app.useGlobalPipes(
    new ValidationPipe({
      whitelist: true,
      transform: true,
      forbidNonWhitelisted: true,
    }),
  );

  // Enable CORS
  app.enableCors();

  // Swagger documentation
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

  const port = process.env.PORT || 8080;
  await app.listen(port);

  console.log(`🍚 Rice Search API running on http://localhost:${port}`);
  console.log(`📚 API documentation available at http://localhost:${port}/docs`);
}

bootstrap();
