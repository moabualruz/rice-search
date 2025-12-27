import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import axios, { AxiosInstance } from 'axios';

@Injectable()
export class EmbeddingsService implements OnModuleInit {
  private readonly logger = new Logger(EmbeddingsService.name);
  private client: AxiosInstance;
  private baseUrl: string;
  private dim: number;

  constructor(private configService: ConfigService) {
    this.baseUrl = this.configService.get<string>('embeddings.url')!;
    this.dim = this.configService.get<number>('embeddings.dim')!;

    this.client = axios.create({
      baseURL: this.baseUrl,
      timeout: 60000,
      headers: { 'Content-Type': 'application/json' },
    });
  }

  async onModuleInit() {
    // Check connection on startup
    try {
      await this.healthCheck();
      this.logger.log(`Connected to embeddings service at ${this.baseUrl}`);
    } catch (error) {
      this.logger.warn(
        `Embeddings service not available at ${this.baseUrl}. Will retry on first request.`,
      );
    }
  }

  async healthCheck(): Promise<boolean> {
    try {
      const response = await this.client.get('/health');
      return response.status === 200;
    } catch {
      return false;
    }
  }

  async getInfo(): Promise<Record<string, unknown>> {
    const response = await this.client.get('/info');
    return response.data;
  }

  /**
   * Generate embeddings for texts
   * @param texts Array of texts to embed
   * @param normalize L2 normalize embeddings (default: true)
   * @param truncate Auto-truncate long inputs (default: true)
   * @returns 2D array of embeddings
   */
  async embed(
    texts: string[],
    normalize = true,
    truncate = true,
  ): Promise<number[][]> {
    if (texts.length === 0) {
      return [];
    }

    try {
      const response = await this.client.post('/embed', {
        inputs: texts,
        normalize,
        truncate,
      });

      return response.data;
    } catch (error) {
      this.logger.error(`Embedding request failed: ${error}`);
      throw error;
    }
  }

  /**
   * Embed large batches with automatic chunking
   */
  async embedBatch(
    texts: string[],
    batchSize = 32,
    normalize = true,
  ): Promise<number[][]> {
    if (texts.length === 0) {
      return [];
    }

    const allEmbeddings: number[][] = [];

    for (let i = 0; i < texts.length; i += batchSize) {
      const batch = texts.slice(i, i + batchSize);
      const embeddings = await this.embed(batch, normalize);
      allEmbeddings.push(...embeddings);
    }

    return allEmbeddings;
  }

  /**
   * Embed with retry logic for transient failures
   */
  async embedWithRetry(
    texts: string[],
    maxRetries = 3,
    retryDelay = 1000,
  ): Promise<number[][]> {
    let lastError: Error | null = null;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        return await this.embed(texts);
      } catch (error) {
        lastError = error as Error;
        this.logger.warn(`Embed attempt ${attempt + 1} failed: ${error}`);
        if (attempt < maxRetries - 1) {
          await new Promise((resolve) =>
            setTimeout(resolve, retryDelay * (attempt + 1)),
          );
        }
      }
    }

    throw lastError;
  }

  getDimension(): number {
    return this.dim;
  }
}
