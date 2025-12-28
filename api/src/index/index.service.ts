import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import { EmbeddingsService } from '../services/embeddings.service';
import { BgeM3Service } from '../services/bge-m3.service';
import { MilvusService, convertSparseToMilvusFormat } from '../services/milvus.service';
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
  IndexMode,
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
  private readonly defaultMode: IndexMode;

  constructor(
    private configService: ConfigService,
    private embeddings: EmbeddingsService,
    private bgeM3: BgeM3Service,
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
    this.defaultMode = (this.configService.get<string>('search.mode') as IndexMode) || 'mixedbread';
    
    this.logger.log(`Index mode: ${this.defaultMode} (Tantivy: ${this.defaultMode === 'mixedbread' ? 'enabled' : 'skipped'})`);
  }

  /**
   * Get embeddings using the appropriate service based on mode
   */
  private async getEmbeddings(texts: string[], mode: IndexMode): Promise<number[][]> {
    if (mode === 'bge-m3') {
      // Use BGE-M3 service - process in batches
      const results: number[][] = [];
      for (let i = 0; i < texts.length; i += this.embeddingBatchSize) {
        const batch = texts.slice(i, i + this.embeddingBatchSize);
        const embeddings = await this.bgeM3.embedDense(batch);
        results.push(...embeddings);
      }
      return results;
    } else {
      // Default: Use Infinity via EmbeddingsService
      return this.embeddings.embedBatch(texts, this.embeddingBatchSize);
    }
  }

  /**
   * Index files into both sparse (Tantivy) and dense (Milvus) indexes
   * Supports incremental indexing - only re-indexes changed files
   * 
   * @param asyncMode - If true, queue embeddings in background and return immediately
   * @param mode - Embedding mode: 'mixedbread' (Infinity) or 'bge-m3'
   */
  async indexFiles(
    store: string,
    files: FileToIndex[],
    force = false,
    asyncMode = false,
    mode?: IndexMode,
  ): Promise<IndexResponseDto | AsyncIndexResponseDto> {
    const embeddingMode = mode || this.defaultMode;
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

    // For mixedbread mode: Index to Tantivy for BM25 sparse search
    // For bge-m3 mode: Skip Tantivy - BGE-M3 has built-in sparse via Milvus hybrid
    if (allChunks.length > 0 && embeddingMode === 'mixedbread') {
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
    }

    // Track files for incremental indexing
    if (allChunks.length > 0) {
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

      if (embeddingMode === 'bge-m3') {
        // BGE-M3 mode: Get both dense and sparse embeddings, store in hybrid collection
        this.logger.log(`Indexing ${texts.length} chunks with BGE-M3 hybrid mode...`);
        
        // Process in batches
        for (let i = 0; i < texts.length; i += this.embeddingBatchSize) {
          const batchTexts = texts.slice(i, i + this.embeddingBatchSize);
          const batchChunks = allChunks.slice(i, i + this.embeddingBatchSize);
          
          // Get both dense and sparse embeddings in one call
          const { dense, sparse } = await this.bgeM3.embedBoth(batchTexts);
          
          // Convert sparse weights to Milvus format (token -> hash)
          const milvusSparse = sparse.map(convertSparseToMilvusFormat);
          
          // Upsert to hybrid collection (includes content for reranking)
          await this.milvus.upsertHybrid(store, {
            doc_ids: batchChunks.map((c) => c.doc_id),
            dense_embeddings: dense,
            sparse_embeddings: milvusSparse,
            paths: batchChunks.map((c) => c.path),
            languages: batchChunks.map((c) => c.language),
            chunk_ids: batchChunks.map((c) => c.chunk_index),
            start_lines: batchChunks.map((c) => c.start_line),
            end_lines: batchChunks.map((c) => c.end_line),
            contents: batchChunks.map((c) => c.content),
            symbols: batchChunks.map((c) => c.symbols),
          });
          
          totalChunks += batchChunks.length;
        }
      } else {
        // Mixedbread mode: Dense embeddings only, stored in regular collection
        const embeddings = await this.getEmbeddings(texts, embeddingMode);

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
      }
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
   * Delete files from all indexes and remove from tracking
   * Deletes from: Tantivy (if exists), regular Milvus, and hybrid Milvus
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
      // Delete from both regular and hybrid Milvus collections
      for (const path of normalizedPaths) {
        denseDeleted += await this.milvus.deleteByPathPrefix(store, path);
        denseDeleted += await this.milvus.deleteHybridByPathPrefix(store, path);
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
      // Delete from both regular and hybrid Milvus collections
      denseDeleted += await this.milvus.deleteByPathPrefix(store, normalizedPrefix);
      denseDeleted += await this.milvus.deleteHybridByPathPrefix(store, normalizedPrefix);
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
    mode?: IndexMode,
  ): Promise<IndexResponseDto> {
    // Clear file tracking for this store
    this.fileTracker.clearStore(store);

    // Delete all existing data
    await this.deleteFiles(store, undefined, '');

    // Re-index all files with force=true, asyncMode=false
    return this.indexFiles(store, files, true, false, mode) as Promise<IndexResponseDto>;
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
