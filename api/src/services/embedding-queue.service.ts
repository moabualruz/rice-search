import { Injectable, Logger, OnModuleDestroy } from '@nestjs/common';
import { EmbeddingsService } from './embeddings.service';
import { MilvusService } from './milvus.service';

interface EmbeddingJob {
  id: string;
  store: string;
  chunks: Array<{
    doc_id: string;
    path: string;
    language: string;
    chunk_index: number;
    start_line: number;
    end_line: number;
    text: string; // Text to embed
  }>;
  resolve: (result: EmbeddingJobResult) => void;
  reject: (error: Error) => void;
  createdAt: number;
}

interface EmbeddingJobResult {
  jobId: string;
  chunksProcessed: number;
  success: boolean;
  error?: string;
}

interface QueueStats {
  pending: number;
  processing: number;
  completed: number;
  failed: number;
}

/**
 * Background embedding queue for non-blocking index operations.
 * 
 * - Index API returns immediately with job ID
 * - Embeddings are generated in background
 * - Results are stored in Milvus when ready
 */
@Injectable()
export class EmbeddingQueueService implements OnModuleDestroy {
  private readonly logger = new Logger(EmbeddingQueueService.name);
  
  private queue: EmbeddingJob[] = [];
  private processing = false;
  private stats: QueueStats = {
    pending: 0,
    processing: 0,
    completed: 0,
    failed: 0,
  };
  
  // Process queue continuously
  private processInterval: NodeJS.Timeout | null = null;
  private readonly BATCH_SIZE = 32;
  private readonly MILVUS_BATCH_SIZE = 3000; // Max chunks per Milvus upsert

  constructor(
    private embeddings: EmbeddingsService,
    private milvus: MilvusService,
  ) {
    // Start queue processor
    this.startProcessor();
  }

  onModuleDestroy() {
    if (this.processInterval) {
      clearInterval(this.processInterval);
    }
  }

  /**
   * Enqueue chunks for background embedding.
   * Returns immediately with job ID.
   */
  enqueue(
    store: string,
    chunks: EmbeddingJob['chunks'],
  ): { jobId: string; position: number } {
    const jobId = `emb_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
    
    // Create job but don't wait for completion
    const job: EmbeddingJob = {
      id: jobId,
      store,
      chunks,
      resolve: () => {}, // Will be set if someone waits
      reject: () => {},
      createdAt: Date.now(),
    };
    
    this.queue.push(job);
    this.stats.pending++;
    
    this.logger.log(`Queued job ${jobId} with ${chunks.length} chunks. Queue size: ${this.queue.length}`);
    
    return {
      jobId,
      position: this.queue.length,
    };
  }

  /**
   * Enqueue and wait for completion (for cases where you need the result)
   */
  async enqueueAndWait(
    store: string,
    chunks: EmbeddingJob['chunks'],
  ): Promise<EmbeddingJobResult> {
    const jobId = `emb_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
    
    return new Promise((resolve, reject) => {
      const job: EmbeddingJob = {
        id: jobId,
        store,
        chunks,
        resolve,
        reject,
        createdAt: Date.now(),
      };
      
      this.queue.push(job);
      this.stats.pending++;
      
      this.logger.log(`Queued job ${jobId} (waiting) with ${chunks.length} chunks`);
    });
  }

  /**
   * Get current queue statistics
   */
  getStats(): QueueStats & { queueLength: number } {
    return {
      ...this.stats,
      queueLength: this.queue.length,
    };
  }

  /**
   * Start the background queue processor
   */
  private startProcessor() {
    // Process queue every 100ms if not already processing
    this.processInterval = setInterval(() => {
      if (!this.processing && this.queue.length > 0) {
        this.processNext();
      }
    }, 100);
  }

  /**
   * Process the next job in the queue
   */
  private async processNext() {
    if (this.queue.length === 0) return;
    
    this.processing = true;
    const job = this.queue.shift()!;
    this.stats.pending--;
    this.stats.processing++;
    
    const startTime = Date.now();
    
    try {
      // Generate embeddings for all chunks (max concurrency)
      const texts = job.chunks.map((c) => c.text);
      const embeddings = await this.embeddings.embedBatch(texts, this.BATCH_SIZE);
      
      // Store in Milvus in batches to avoid RESOURCE_EXHAUSTED
      for (let i = 0; i < job.chunks.length; i += this.MILVUS_BATCH_SIZE) {
        const batchEnd = Math.min(i + this.MILVUS_BATCH_SIZE, job.chunks.length);
        const batchChunks = job.chunks.slice(i, batchEnd);
        const batchEmbeddings = embeddings.slice(i, batchEnd);

        await this.milvus.upsert(job.store, {
          doc_ids: batchChunks.map((c) => c.doc_id),
          embeddings: batchEmbeddings,
          paths: batchChunks.map((c) => c.path),
          languages: batchChunks.map((c) => c.language),
          chunk_ids: batchChunks.map((c) => c.chunk_index),
          start_lines: batchChunks.map((c) => c.start_line),
          end_lines: batchChunks.map((c) => c.end_line),
        });
      }
      
      const duration = Date.now() - startTime;
      this.logger.log(`Job ${job.id} completed: ${job.chunks.length} chunks in ${duration}ms`);
      
      this.stats.processing--;
      this.stats.completed++;
      
      job.resolve({
        jobId: job.id,
        chunksProcessed: job.chunks.length,
        success: true,
      });
    } catch (error) {
      const errorMsg = error instanceof Error ? error.message : String(error);
      this.logger.error(`Job ${job.id} failed: ${errorMsg}`);
      
      this.stats.processing--;
      this.stats.failed++;
      
      job.resolve({
        jobId: job.id,
        chunksProcessed: 0,
        success: false,
        error: errorMsg,
      });
    } finally {
      this.processing = false;
      
      // Immediately process next if queue not empty
      if (this.queue.length > 0) {
        setImmediate(() => this.processNext());
      }
    }
  }
}
