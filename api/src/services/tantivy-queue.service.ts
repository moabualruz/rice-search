import cluster from 'node:cluster';
import { Injectable, Logger, OnModuleDestroy } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { Queue, Worker, QueueEvents, Job } from 'bullmq';
import { TantivyService, TantivyDocument } from './tantivy.service';

// Job types
export interface TantivyIndexJob {
  type: 'index';
  store: string;
  documents: TantivyDocument[];
}

export interface TantivyDeleteJob {
  type: 'delete';
  store: string;
  options: { path?: string; docId?: string };
}

export type TantivyJob = TantivyIndexJob | TantivyDeleteJob;

// Result types
export interface TantivyIndexResult {
  indexed: number;
  errors: number;
}

export interface TantivyDeleteResult {
  deleted: number;
}

export type TantivyJobResult = TantivyIndexResult | TantivyDeleteResult;

/**
 * Dynamic per-store queue service for Tantivy operations.
 * 
 * Each store gets its own queue and worker (concurrency=1), allowing:
 * - Parallel indexing across different stores
 * - Serialized indexing within each store (prevents LockBusy)
 * 
 * In cluster mode, only worker #1 processes jobs (creates BullMQ workers).
 * All workers can add jobs to the queue, but only one processes them.
 * 
 * Queues and workers are created on-demand and cached.
 */
@Injectable()
export class TantivyQueueService implements OnModuleDestroy {
  private readonly logger = new Logger(TantivyQueueService.name);
  
  // Per-store queues, workers, and events
  private queues: Map<string, Queue<TantivyJob>> = new Map();
  private workers: Map<string, Worker<TantivyJob, TantivyJobResult>> = new Map();
  private queueEvents: Map<string, QueueEvents> = new Map();
  
  private readonly redisConfig: { host: string; port: number };
  
  // Only worker #1 (or non-clustered) processes jobs
  private readonly isQueueProcessor: boolean;
  
  // Job timeout in ms (5 minutes default - indexing can be slow)
  private readonly jobTimeout: number;
  
  // Retry delay before re-queuing failed jobs
  private readonly RETRY_DELAY_MS = 2000;

  constructor(
    private configService: ConfigService,
    private tantivy: TantivyService,
  ) {
    this.redisConfig = {
      host: this.configService.get<string>('redis.host', 'localhost'),
      port: this.configService.get<number>('redis.port', 6379),
    };
    
    // In cluster mode, only worker 1 processes Tantivy jobs
    // This prevents multiple workers from competing for the same Tantivy index lock
    const workerId = cluster.isWorker ? cluster.worker?.id : 0;
    this.isQueueProcessor = !cluster.isWorker || workerId === 1;
    
    // Job timeout: 5 minutes default (indexing large batches can be slow)
    this.jobTimeout = this.configService.get<number>('TANTIVY_JOB_TIMEOUT_MS', 300000);
    
    const role = this.isQueueProcessor ? 'PROCESSOR' : 'CLIENT-ONLY';
    this.logger.log(
      `Tantivy queue service initialized [${role}] (Redis: ${this.redisConfig.host}:${this.redisConfig.port}, Worker: ${workerId ?? 'main'}, Timeout: ${this.jobTimeout}ms)`
    );
  }

  /**
   * Get or create queue infrastructure for a store
   * Only the designated queue processor creates workers.
   */
  private getOrCreateQueue(store: string): {
    queue: Queue<TantivyJob>;
    events: QueueEvents;
  } {
    // Sanitize store name for queue (no special chars allowed)
    const safeStore = store.replace(/[^a-zA-Z0-9_-]/g, '_');
    const queueName = `tantivy_${safeStore}`;

    if (!this.queues.has(store)) {
      // Create queue (all workers can add jobs)
      const queue = new Queue<TantivyJob>(queueName, {
        connection: this.redisConfig,
        defaultJobOptions: {
          removeOnComplete: 100,
          removeOnFail: false, // Keep failed jobs for re-queue
        },
      });
      this.queues.set(store, queue);

      // Only the queue processor creates workers (prevents LockBusy in cluster mode)
      if (this.isQueueProcessor) {
        this.logger.log(`Creating queue WORKER for store: ${store}`);

        const worker = new Worker<TantivyJob, TantivyJobResult>(
          queueName,
          async (job: Job<TantivyJob>) => this.processJob(job),
          {
            connection: this.redisConfig,
            concurrency: 1, // CRITICAL: Only one job at a time per store
          },
        );

        worker.on('failed', async (job, err) => {
          if (!job) return;
          
          this.logger.warn(
            `Tantivy job ${job.id} failed for store ${store}: ${err.message}. Re-queuing...`,
          );
          
          // Re-queue with retry loop to ensure job is not lost
          this.requeueJob(queue, job.name, job.data, store);
        });

        worker.on('completed', (job) => {
          const data = job.data;
          if (data.type === 'index') {
            this.logger.log(`Tantivy job ${job.id} completed: ${data.documents.length} docs indexed to store ${store}`);
          } else {
            this.logger.debug(`Tantivy job ${job.id} completed for store ${store}`);
          }
        });

        this.workers.set(store, worker);
      } else {
        this.logger.debug(`Creating queue CLIENT for store: ${store} (worker processes on another node)`);
      }

      // Create QueueEvents for waitUntilFinished (all workers need this)
      const events = new QueueEvents(queueName, {
        connection: this.redisConfig,
      });
      // Prevent MaxListenersExceededWarning when many jobs are waiting
      events.setMaxListeners(100);
      queue.setMaxListeners(100);
      this.queueEvents.set(store, events);
    }

    return {
      queue: this.queues.get(store)!,
      events: this.queueEvents.get(store)!,
    };
  }

  /**
   * Process a Tantivy job (called by worker) - completes full index/delete cycle.
   * Only returns success after the operation fully completes.
   */
  private async processJob(job: Job<TantivyJob>): Promise<TantivyJobResult> {
    const { data } = job;

    if (data.type === 'index') {
      this.logger.debug(
        `Processing index job: ${data.documents.length} docs for store ${data.store}`,
      );
      return this.tantivy.indexDirect(data.store, data.documents);
    } else if (data.type === 'delete') {
      this.logger.debug(`Processing delete job for store ${data.store}`);
      const deleted = await this.tantivy.deleteDirect(data.store, data.options);
      return { deleted };
    }

    throw new Error(`Unknown job type: ${(data as TantivyJob).type}`);
  }

  /**
   * Re-queue a failed job with retry logic.
   * Keeps trying to re-queue until successful - jobs are never lost.
   */
  private requeueJob(
    queue: Queue<TantivyJob>,
    jobName: string,
    jobData: TantivyJob,
    store: string,
    requeueAttempt = 1,
  ): void {
    const delay = requeueAttempt === 1 
      ? this.RETRY_DELAY_MS 
      : Math.min(this.RETRY_DELAY_MS * Math.pow(2, requeueAttempt - 1), 30000);

    setTimeout(async () => {
      try {
        await queue.add(jobName, jobData, {
          priority: 10, // Lower priority for retries
        });
        this.logger.debug(`Re-queued Tantivy job for store ${store}`);
      } catch (requeueError) {
        // If re-queue fails, keep trying with exponential backoff
        this.logger.warn(
          `Failed to re-queue Tantivy job for store ${store} (attempt ${requeueAttempt}): ${requeueError}. Retrying...`,
        );
        this.requeueJob(queue, jobName, jobData, store, requeueAttempt + 1);
      }
    }, delay);
  }

  /**
   * Queue documents for indexing (fire-and-forget).
   * Returns immediately after queuing - does not wait for completion.
   */
  async index(
    store: string,
    documents: TantivyDocument[],
  ): Promise<{ jobId: string }> {
    const { queue } = this.getOrCreateQueue(store);

    const job = await queue.add('index', {
      type: 'index',
      store,
      documents,
    });

    this.logger.debug(`Queued index job ${job.id} with ${documents.length} docs for store ${store}`);
    return { jobId: job.id! };
  }

  /**
   * Queue documents for indexing and wait for completion.
   * Use this only when you need to ensure indexing finished.
   */
  async indexAndWait(
    store: string,
    documents: TantivyDocument[],
  ): Promise<TantivyIndexResult> {
    const { queue, events } = this.getOrCreateQueue(store);

    const job = await queue.add('index', {
      type: 'index',
      store,
      documents,
    });

    // Wait for job to complete
    const result = await job.waitUntilFinished(events, this.jobTimeout);
    return result as TantivyIndexResult;
  }

  /**
   * Queue delete operation. Returns when job completes.
   */
  async delete(
    store: string,
    options: { path?: string; docId?: string },
  ): Promise<number> {
    const { queue, events } = this.getOrCreateQueue(store);

    const job = await queue.add('delete', {
      type: 'delete',
      store,
      options,
    });

    const result = await job.waitUntilFinished(events, this.jobTimeout);
    return (result as TantivyDeleteResult).deleted;
  }

  /**
   * Get queue stats for a specific store
   */
  async getStoreStats(store: string): Promise<{
    waiting: number;
    active: number;
    completed: number;
    failed: number;
  } | null> {
    const queue = this.queues.get(store);
    if (!queue) {
      return null;
    }

    const [waiting, active, completed, failed] = await Promise.all([
      queue.getWaitingCount(),
      queue.getActiveCount(),
      queue.getCompletedCount(),
      queue.getFailedCount(),
    ]);

    return { waiting, active, completed, failed };
  }

  /**
   * Get all active store queues
   */
  getActiveStores(): string[] {
    return Array.from(this.queues.keys());
  }

  /**
   * Cleanup on module destroy
   */
  async onModuleDestroy() {
    this.logger.log('Shutting down Tantivy queue service...');

    // Close all workers
    for (const [store, worker] of this.workers) {
      this.logger.debug(`Closing worker for store: ${store}`);
      await worker.close();
    }

    // Close all queue events
    for (const [store, events] of this.queueEvents) {
      this.logger.debug(`Closing events for store: ${store}`);
      await events.close();
    }

    // Close all queues
    for (const [store, queue] of this.queues) {
      this.logger.debug(`Closing queue for store: ${store}`);
      await queue.close();
    }

    this.workers.clear();
    this.queueEvents.clear();
    this.queues.clear();

    this.logger.log('Tantivy queue service shut down');
  }
}
