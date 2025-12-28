import cluster from 'node:cluster';
import { Injectable, Logger, OnModuleDestroy } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Queue, Worker, QueueEvents, Job } from 'bullmq';
import { EmbeddingsService } from './embeddings.service';
import { MilvusService } from './milvus.service';

interface EmbeddingChunk {
  doc_id: string;
  path: string;
  language: string;
  chunk_index: number;
  start_line: number;
  end_line: number;
  text: string;
}

interface EmbeddingJob {
  store: string;
  chunks: EmbeddingChunk[];
  attempt?: number;
}

interface EmbeddingJobResult {
  chunksProcessed: number;
  success: boolean;
}

/**
 * BullMQ-based embedding queue for reliable, persistent indexing.
 * 
 * Features:
 * - Persistent queue (Redis-backed)
 * - Infinite retry on failure (re-queues failed jobs)
 * - WARN logging on failures, never gives up
 * - Survives service restarts
 */
@Injectable()
export class EmbeddingQueueService implements OnModuleDestroy {
  private readonly logger = new Logger(EmbeddingQueueService.name);
  
  private queue: Queue<EmbeddingJob> | null = null;
  private worker: Worker<EmbeddingJob, EmbeddingJobResult> | null = null;
  private queueEvents: QueueEvents | null = null;
  
  private readonly redisConfig: { host: string; port: number };
  private readonly isQueueProcessor: boolean;
  private readonly jobTimeout: number;
  
  private readonly QUEUE_NAME = 'embedding_queue';
  private readonly BATCH_SIZE = 32;
  private readonly MILVUS_BATCH_SIZE = 3000;
  private readonly RETRY_DELAY_MS = 2000; // Wait before retry

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private milvus: MilvusService,
  ) {
    this.redisConfig = {
      host: this.configService.get<string>('redis.host', 'localhost'),
      port: this.configService.get<number>('redis.port', 6379),
    };
    
    // In cluster mode, only worker 1 processes jobs
    const workerId = cluster.isWorker ? cluster.worker?.id : 0;
    this.isQueueProcessor = !cluster.isWorker || workerId === 1;
    
    this.jobTimeout = this.configService.get<number>('EMBEDDING_JOB_TIMEOUT_MS', 600000); // 10 min default
    
    this.initializeQueue();
    
    const role = this.isQueueProcessor ? 'PROCESSOR' : 'CLIENT-ONLY';
    this.logger.log(
      `Embedding queue initialized [${role}] (Redis: ${this.redisConfig.host}:${this.redisConfig.port})`
    );
  }

  private initializeQueue() {
    // Create queue (all workers can add jobs)
    this.queue = new Queue<EmbeddingJob>(this.QUEUE_NAME, {
      connection: this.redisConfig,
      defaultJobOptions: {
        removeOnComplete: 100,
        removeOnFail: false, // Keep failed jobs for manual re-queue
      },
    });

    // Create QueueEvents for waiting
    this.queueEvents = new QueueEvents(this.QUEUE_NAME, {
      connection: this.redisConfig,
    });
    this.queueEvents.setMaxListeners(100);
    this.queue.setMaxListeners(100);

    // Only the designated processor creates workers
    if (this.isQueueProcessor) {
      this.worker = new Worker<EmbeddingJob, EmbeddingJobResult>(
        this.QUEUE_NAME,
        async (job: Job<EmbeddingJob>) => this.processJob(job),
        {
          connection: this.redisConfig,
          concurrency: 2, // Allow 2 concurrent embedding jobs
        },
      );

      this.worker.on('failed', async (job, err) => {
        if (!job) return;
        
        const attempt = (job.data.attempt ?? 0) + 1;
        this.logger.warn(
          `Embedding job ${job.id} failed (attempt ${attempt}): ${err.message}. Re-queuing in ${this.RETRY_DELAY_MS}ms...`
        );
        
        // Re-queue with retry loop to ensure job is not lost
        this.requeue(job.data, attempt);
      });

      this.worker.on('completed', (job) => {
        this.logger.log(
          `Embedding job ${job.id} completed: ${job.data.chunks.length} chunks indexed to store ${job.data.store}`
        );
      });
    }
  }

  /**
   * Re-queue a failed job with retry logic.
   * Keeps trying to re-queue until successful - jobs are never lost.
   */
  private requeue(jobData: EmbeddingJob, attempt: number, requeueAttempt = 1): void {
    setTimeout(async () => {
      try {
        await this.queue?.add('embedding', {
          ...jobData,
          attempt,
        }, {
          priority: 10, // Lower priority for retries
        });
        this.logger.debug(`Re-queued embedding job (attempt ${attempt})`);
      } catch (requeueError) {
        // If re-queue fails, keep trying with exponential backoff
        const nextDelay = Math.min(this.RETRY_DELAY_MS * Math.pow(2, requeueAttempt), 30000);
        this.logger.warn(
          `Failed to re-queue embedding job (requeue attempt ${requeueAttempt}): ${requeueError}. Retrying in ${nextDelay}ms...`
        );
        this.requeue(jobData, attempt, requeueAttempt + 1);
      }
    }, requeueAttempt === 1 ? this.RETRY_DELAY_MS : this.RETRY_DELAY_MS * Math.pow(2, requeueAttempt - 1));
  }

  /**
   * Process an embedding job - completes full cycle: embed → Milvus upsert
   * Only returns success after ALL steps complete.
   */
  private async processJob(job: Job<EmbeddingJob>): Promise<EmbeddingJobResult> {
    const { store, chunks } = job.data;
    const attempt = job.data.attempt ?? 1;
    
    this.logger.debug(
      `Processing embedding job ${job.id} (attempt ${attempt}): ${chunks.length} chunks for store ${store}`
    );

    // Generate embeddings
    const texts = chunks.map((c) => c.text);
    const embeddings = await this.embeddings.embedBatch(texts, this.BATCH_SIZE);

    // Store in Milvus in batches
    for (let i = 0; i < chunks.length; i += this.MILVUS_BATCH_SIZE) {
      const batchEnd = Math.min(i + this.MILVUS_BATCH_SIZE, chunks.length);
      const batchChunks = chunks.slice(i, batchEnd);
      const batchEmbeddings = embeddings.slice(i, batchEnd);

      await this.milvus.upsert(store, {
        doc_ids: batchChunks.map((c) => c.doc_id),
        embeddings: batchEmbeddings,
        paths: batchChunks.map((c) => c.path),
        languages: batchChunks.map((c) => c.language),
        chunk_ids: batchChunks.map((c) => c.chunk_index),
        start_lines: batchChunks.map((c) => c.start_line),
        end_lines: batchChunks.map((c) => c.end_line),
      });
    }

    return {
      chunksProcessed: chunks.length,
      success: true,
    };
  }

  /**
   * Enqueue chunks for background embedding.
   * Returns immediately with job ID.
   */
  enqueue(
    store: string,
    chunks: EmbeddingChunk[],
  ): { jobId: string; position: number } {
    if (!this.queue) {
      throw new Error('Embedding queue not initialized');
    }

    const jobId = `emb_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
    
    // Fire and forget - add to queue
    this.queue.add('embedding', {
      store,
      chunks,
      attempt: 1,
    }, {
      jobId,
    }).catch((err) => {
      this.logger.warn(`Failed to enqueue job ${jobId}: ${err.message}`);
    });

    this.logger.log(`Queued embedding job ${jobId} with ${chunks.length} chunks`);
    
    return {
      jobId,
      position: 0, // BullMQ doesn't expose position easily
    };
  }

  /**
   * Enqueue and wait for completion
   */
  async enqueueAndWait(
    store: string,
    chunks: EmbeddingChunk[],
  ): Promise<EmbeddingJobResult> {
    if (!this.queue || !this.queueEvents) {
      throw new Error('Embedding queue not initialized');
    }

    const job = await this.queue.add('embedding', {
      store,
      chunks,
      attempt: 1,
    });

    const result = await job.waitUntilFinished(this.queueEvents, this.jobTimeout);
    return result;
  }

  /**
   * Get queue statistics
   */
  async getStats(): Promise<{
    waiting: number;
    active: number;
    completed: number;
    failed: number;
  }> {
    if (!this.queue) {
      return { waiting: 0, active: 0, completed: 0, failed: 0 };
    }

    const [waiting, active, completed, failed] = await Promise.all([
      this.queue.getWaitingCount(),
      this.queue.getActiveCount(),
      this.queue.getCompletedCount(),
      this.queue.getFailedCount(),
    ]);

    return { waiting, active, completed, failed };
  }

  async onModuleDestroy() {
    this.logger.log('Shutting down embedding queue...');
    
    if (this.worker) {
      await this.worker.close();
    }
    if (this.queueEvents) {
      await this.queueEvents.close();
    }
    if (this.queue) {
      await this.queue.close();
    }
    
    this.logger.log('Embedding queue shut down');
  }
}
