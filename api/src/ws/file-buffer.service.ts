import { Injectable, Logger } from '@nestjs/common';
import { IndexService } from '../index/index.service';
import type { FileToIndex } from '../index/dto/index-request.dto';

interface BufferedFile {
  path: string;
  content: string;
  addedAt: number;
}

interface ConnectionBuffer {
  store: string;
  files: BufferedFile[];
  timer: ReturnType<typeof setTimeout> | null;
  onFlush: (result: { chunks_queued: number; files_count: number; batch_id: string }) => void;
}

/**
 * Batches files per WebSocket connection.
 * Flushes when:
 * - 100 files accumulated, OR
 * - 3 seconds since first file in current batch
 */
@Injectable()
export class FileBufferService {
  private readonly logger = new Logger(FileBufferService.name);
  private readonly buffers = new Map<string, ConnectionBuffer>();

  private readonly BATCH_SIZE = 100;
  private readonly BATCH_TIMEOUT_MS = 3000;

  constructor(private readonly indexService: IndexService) {}

  /**
   * Register a connection with its flush callback
   */
  register(
    connId: string,
    store: string,
    onFlush: (result: { chunks_queued: number; files_count: number; batch_id: string }) => void,
  ): void {
    this.buffers.set(connId, {
      store,
      files: [],
      timer: null,
      onFlush,
    });
    this.logger.debug(`Registered buffer for connection ${connId}, store: ${store}`);
  }

  /**
   * Unregister connection and flush remaining files
   */
  async unregister(connId: string): Promise<void> {
    const buffer = this.buffers.get(connId);
    if (buffer) {
      if (buffer.timer) {
        clearTimeout(buffer.timer);
      }
      // Flush remaining files
      if (buffer.files.length > 0) {
        await this.flush(connId);
      }
      this.buffers.delete(connId);
      this.logger.debug(`Unregistered buffer for connection ${connId}`);
    }
  }

  /**
   * Add file to buffer (non-blocking, returns immediately)
   */
  addFile(connId: string, path: string, content: string): void {
    const buffer = this.buffers.get(connId);
    if (!buffer) {
      this.logger.warn(`No buffer for connection ${connId}`);
      return;
    }

    buffer.files.push({
      path,
      content,
      addedAt: Date.now(),
    });

    // Start timeout timer on first file
    if (buffer.files.length === 1) {
      buffer.timer = setTimeout(() => {
        this.flush(connId).catch((err) => {
          this.logger.error(`Flush timeout error for ${connId}: ${err}`);
        });
      }, this.BATCH_TIMEOUT_MS);
    }

    // Flush immediately if batch size reached
    if (buffer.files.length >= this.BATCH_SIZE) {
      if (buffer.timer) {
        clearTimeout(buffer.timer);
        buffer.timer = null;
      }
      this.flush(connId).catch((err) => {
        this.logger.error(`Flush batch error for ${connId}: ${err}`);
      });
    }
  }

  /**
   * Flush buffer to index service (fire-and-forget from caller's perspective)
   */
  private async flush(connId: string): Promise<void> {
    const buffer = this.buffers.get(connId);
    if (!buffer || buffer.files.length === 0) {
      return;
    }

    // Take files and reset buffer
    const files = buffer.files;
    buffer.files = [];
    buffer.timer = null;

    const batchId = `batch_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
    const filesCount = files.length;

    this.logger.log(`Flushing ${filesCount} files for connection ${connId}, batch: ${batchId}`);

    // Convert to FileToIndex format
    const filesToIndex: FileToIndex[] = files.map((f) => ({
      path: f.path,
      content: f.content,
    }));

    // Fire-and-forget indexing, then notify via callback
    this.indexService
      .indexFiles(buffer.store, filesToIndex, false)
      .then((result) => {
        buffer.onFlush({
          chunks_queued: result.chunks_queued,
          files_count: filesCount,
          batch_id: batchId,
        });
      })
      .catch((err) => {
        this.logger.error(`Index error for batch ${batchId}: ${err}`);
        // Still notify with error info
        buffer.onFlush({
          chunks_queued: 0,
          files_count: filesCount,
          batch_id: batchId,
        });
      });
  }

  /**
   * Get buffer stats for a connection
   */
  getBufferStats(connId: string): { pending_files: number } | null {
    const buffer = this.buffers.get(connId);
    if (!buffer) {
      return null;
    }
    return {
      pending_files: buffer.files.length,
    };
  }
}
