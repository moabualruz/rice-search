import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { EmbeddingsService } from '../services/embeddings.service';
import { MilvusService } from '../services/milvus.service';
import { TantivyQueueService } from '../services/tantivy-queue.service';
import { StoreManagerService } from '../services/store-manager.service';
import { FileTrackerService } from '../services/file-tracker.service';
import { EmbeddingQueueService } from '../services/embedding-queue.service';
import {
  TreeSitterChunkerService,
  TreeSitterChunk,
} from '../services/treesitter-chunker.service';
import {
  FileToIndex,
  IndexResponseDto,
  AsyncIndexResponseDto,
  DeleteResponseDto,
} from './dto/index-request.dto';

/**
 * Normalize path to use forward slashes consistently.
 * Handles Windows paths (backslashes) and ensures consistent storage/querying.
 */
function normalizePath(filePath: string): string {
  return filePath.replace(/\\/g, '/');
}

@Injectable()
export class IndexService {
  private readonly logger = new Logger(IndexService.name);
  private readonly maxFileSizeMb: number;
  private readonly embeddingBatchSize = 32;

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private milvus: MilvusService,
    private tantivyQueue: TantivyQueueService,
    private storeManager: StoreManagerService,
    private fileTracker: FileTrackerService,
    private treeSitterChunker: TreeSitterChunkerService,
    private embeddingQueue: EmbeddingQueueService,
  ) {
    this.maxFileSizeMb = this.configService.get<number>(
      'indexing.maxFileSizeMb',
    )!;
  }

  /**
   * Index files into both sparse (Tantivy) and dense (Milvus) indexes
   * Supports incremental indexing - only re-indexes changed files
   * 
   * @param asyncMode - If true, queue embeddings in background and return immediately
   */
  async indexFiles(
    store: string,
    files: FileToIndex[],
    force = false,
    asyncMode = false,
  ): Promise<IndexResponseDto | AsyncIndexResponseDto> {
    const startTime = Date.now();
    const errors: string[] = [];
    let skippedUnchanged = 0;

    // Ensure store exists
    this.storeManager.ensureStore(store);

    // Normalize all paths to use forward slashes (handles Windows paths)
    const normalizedFiles = files.map((f) => ({
      ...f,
      path: normalizePath(f.path),
    }));

    // Incremental indexing: Check which files have changed
    let filesToProcess: FileToIndex[];
    if (force) {
      filesToProcess = normalizedFiles;
      this.logger.log(`Force re-index: processing all ${normalizedFiles.length} files`);
    } else {
      const changeResult = this.fileTracker.checkFilesForChanges(
        store,
        normalizedFiles.map((f) => ({ path: f.path, content: f.content })),
      );
      
      // Combine changed and new files for processing
      filesToProcess = [
        ...changeResult.changed,
        ...changeResult.newFiles,
      ] as FileToIndex[];
      
      skippedUnchanged = changeResult.unchanged.length;
      
      if (skippedUnchanged > 0) {
        this.logger.log(
          `Incremental indexing: ${skippedUnchanged} unchanged, ` +
          `${changeResult.changed.length} changed, ${changeResult.newFiles.length} new`,
        );
      }
    }

    if (filesToProcess.length === 0) {
      if (asyncMode) {
        return {
          job_id: 'none',
          status: 'completed',
          files_accepted: files.length,
          chunks_queued: 0,
          queue_position: 0,
          skipped_unchanged: skippedUnchanged,
        };
      }
      return {
        files_processed: files.length,
        chunks_indexed: 0,
        time_ms: Date.now() - startTime,
        skipped_unchanged: skippedUnchanged,
      };
    }

    // Process files using Tree-sitter chunker
    const allChunks: TreeSitterChunk[] = [];
    const fileChunkMap: Map<string, { file: FileToIndex; chunks: TreeSitterChunk[] }> = new Map();

    for (const file of filesToProcess) {
      try {
        // Skip binary files
        if (this.treeSitterChunker.isBinary(file.content)) {
          this.logger.debug(`Skipping binary file: ${file.path}`);
          continue;
        }

        // Skip large files
        const sizeMb = Buffer.byteLength(file.content, 'utf8') / (1024 * 1024);
        if (sizeMb > this.maxFileSizeMb) {
          this.logger.debug(
            `Skipping large file (${sizeMb.toFixed(1)}MB): ${file.path}`,
          );
          continue;
        }

        // Use Tree-sitter for AST-aware chunking
        const chunks = await this.treeSitterChunker.chunkWithTreeSitter(
          file.path,
          file.content,
        );

        if (chunks.length === 0) {
          continue;
        }

        fileChunkMap.set(file.path, { file, chunks });
        allChunks.push(...chunks);
      } catch (error) {
        const errorMsg = `Failed to process ${file.path}: ${error}`;
        this.logger.warn(errorMsg);
        errors.push(errorMsg);
      }
    }

    // Batch index all chunks to Tantivy (fire-and-forget)
    // Tantivy processes in background via queue - don't block on completion
    if (allChunks.length > 0) {
      const tantivyDocs = allChunks.map((chunk) => ({
        doc_id: chunk.doc_id,
        path: chunk.path,
        language: chunk.language,
        symbols: chunk.symbols,
        content: chunk.content,
        start_line: chunk.start_line,
        end_line: chunk.end_line,
      }));

      // Fire-and-forget: queue job but don't wait for completion
      // Tantivy indexing happens in background, eventually catches up
      const { jobId } = await this.tantivyQueue.index(store, tantivyDocs);
      this.logger.debug(`Queued Tantivy job ${jobId} for ${tantivyDocs.length} chunks`);

      // Track files immediately (before Tantivy completes)
      const filesToTrack = Array.from(fileChunkMap.values()).map(({ file, chunks }) => ({
        path: file.path,
        content: file.content,
        chunkIds: chunks.map((c) => c.doc_id),
      }));
      this.fileTracker.trackFiles(store, filesToTrack);
    }

    if (allChunks.length === 0) {
      if (asyncMode) {
        return {
          job_id: 'none',
          status: 'completed',
          files_accepted: files.length,
          chunks_queued: 0,
          queue_position: 0,
          skipped_unchanged: skippedUnchanged,
          errors: errors.length > 0 ? errors : undefined,
        };
      }
      return {
        files_processed: files.length,
        chunks_indexed: 0,
        time_ms: Date.now() - startTime,
        skipped_unchanged: skippedUnchanged,
        errors: errors.length > 0 ? errors : undefined,
      };
    }
    this.storeManager.touchStore(store);

    // Prepare embedding data
    const embeddingChunks = allChunks.map((chunk) => {
      const symbolsStr = chunk.symbols.length > 0 ? chunk.symbols.join(' ') : '';
      return {
        doc_id: chunk.doc_id,
        path: chunk.path,
        language: chunk.language,
        chunk_index: chunk.chunk_index,
        start_line: chunk.start_line,
        end_line: chunk.end_line,
        text: `${chunk.path}\n${symbolsStr}\n${chunk.content}`.slice(0, 8000),
      };
    });

    // ASYNC MODE: Queue embeddings and return immediately
    if (asyncMode) {
      const { jobId, position } = this.embeddingQueue.enqueue(store, embeddingChunks);
      
      return {
        job_id: jobId,
        status: 'accepted',
        files_accepted: files.length,
        chunks_queued: allChunks.length,
        queue_position: position,
        skipped_unchanged: skippedUnchanged,
        errors: errors.length > 0 ? errors : undefined,
      };
    }

    // SYNC MODE: Wait for embeddings (original behavior)
    let totalChunks = 0;
    try {
      const texts = embeddingChunks.map((c) => c.text);
      const embeddings = await this.embeddings.embedBatch(texts, this.embeddingBatchSize);

      await this.milvus.upsert(store, {
        doc_ids: allChunks.map((c) => c.doc_id),
        embeddings,
        paths: allChunks.map((c) => c.path),
        languages: allChunks.map((c) => c.language),
        chunk_ids: allChunks.map((c) => c.chunk_index),
        start_lines: allChunks.map((c) => c.start_line),
        end_lines: allChunks.map((c) => c.end_line),
      });

      totalChunks = allChunks.length;
    } catch (error) {
      const errorMsg = `Milvus indexing failed: ${error}`;
      this.logger.error(errorMsg);
      errors.push(errorMsg);
    }

    return {
      files_processed: files.length,
      chunks_indexed: totalChunks,
      time_ms: Date.now() - startTime,
      skipped_unchanged: skippedUnchanged,
      errors: errors.length > 0 ? errors : undefined,
    };
  }

  /**
   * Delete files from both indexes and remove from tracking
   */
  async deleteFiles(
    store: string,
    paths?: string[],
    pathPrefix?: string,
  ): Promise<DeleteResponseDto> {
    const startTime = Date.now();
    let sparseDeleted = 0;
    let denseDeleted = 0;

    // Normalize paths for consistent matching
    const normalizedPaths = paths?.map(normalizePath);
    const normalizedPrefix = pathPrefix ? normalizePath(pathPrefix) : undefined;

    // Delete by specific paths
    if (normalizedPaths && normalizedPaths.length > 0) {
      for (const path of normalizedPaths) {
        try {
          sparseDeleted += await this.tantivyQueue.delete(store, { path });
          // Untrack the file
          this.fileTracker.untrackFile(store, path);
        } catch (error) {
          this.logger.warn(`Failed to delete from Tantivy: ${path}: ${error}`);
        }
      }
      // Milvus delete by path is less efficient, use prefix for each path
      for (const path of normalizedPaths) {
        denseDeleted += await this.milvus.deleteByPathPrefix(store, path);
      }
    }

    // Delete by path prefix
    if (normalizedPrefix) {
      try {
        sparseDeleted += await this.tantivyQueue.delete(store, { path: normalizedPrefix });
        // Untrack files by prefix
        this.fileTracker.untrackByPrefix(store, normalizedPrefix);
      } catch (error) {
        this.logger.warn(
          `Failed to delete from Tantivy by prefix: ${normalizedPrefix}: ${error}`,
        );
      }
      denseDeleted += await this.milvus.deleteByPathPrefix(store, normalizedPrefix);
    }

    // Update store timestamp
    this.storeManager.touchStore(store);

    return {
      sparse_deleted: sparseDeleted,
      dense_deleted: denseDeleted,
      time_ms: Date.now() - startTime,
    };
  }

  /**
   * Re-index entire store (clear and rebuild)
   * Always synchronous - waits for completion
   */
  async reindex(
    store: string,
    files: FileToIndex[],
  ): Promise<IndexResponseDto> {
    // Clear file tracking for this store
    this.fileTracker.clearStore(store);

    // Delete all existing data
    await this.deleteFiles(store, undefined, '');

    // Re-index all files with force=true, asyncMode=false
    return this.indexFiles(store, files, true, false) as Promise<IndexResponseDto>;
  }

  /**
   * Sync index with current files - remove deleted files from index
   */
  async syncDeletedFiles(
    store: string,
    currentPaths: string[],
  ): Promise<{ deleted: number }> {
    // Normalize paths for consistent matching
    const normalizedCurrentPaths = currentPaths.map(normalizePath);
    const deletedPaths = this.fileTracker.findDeletedFiles(store, normalizedCurrentPaths);
    
    if (deletedPaths.length === 0) {
      return { deleted: 0 };
    }

    this.logger.log(`Syncing: removing ${deletedPaths.length} deleted files`);
    await this.deleteFiles(store, deletedPaths);
    
    return { deleted: deletedPaths.length };
  }

  /**
   * Get indexing statistics for a store
   */
  getStoreStats(store: string): {
    tracked_files: number;
    total_size: number;
    last_updated: string;
  } {
    return this.fileTracker.getStoreStats(store);
  }
}
