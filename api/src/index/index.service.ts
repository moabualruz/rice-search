import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
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

  constructor(
    private configService: ConfigService,
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
    
    this.logger.log("Index service initialized (fire-and-forget queues: Tantivy + Embeddings)");
  }

  /**
   * Index files into both sparse (Tantivy) and dense (Milvus) indexes.
   * 
   * ALWAYS non-blocking: queues work and returns immediately.
   * - Tantivy indexing: queued via TantivyQueueService
   * - Embeddings + Milvus: queued via EmbeddingQueueService
   * 
   * Client can poll /stats or /files for status updates.
   */
  async indexFiles(
    store: string,
    files: FileToIndex[],
    force = false,
    _asyncMode = true, // Always async, param kept for API compatibility
  ): Promise<AsyncIndexResponseDto> {
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
      return {
        job_id: 'none',
        status: 'completed',
        files_accepted: files.length,
        chunks_queued: 0,
        queue_position: 0,
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

    // Index to Tantivy for BM25 sparse search
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

    // Queue embeddings for background processing (fire-and-forget)
    // Tantivy is already queued above, embeddings queue handles Milvus
    const { jobId, position } = this.embeddingQueue.enqueue(store, embeddingChunks);
    
    this.logger.log(
      `Accepted ${files.length} files → ${allChunks.length} chunks queued (Tantivy + Embeddings)`
    );

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

  /**
   * Delete files from all indexes and remove from tracking
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
          this.fileTracker.untrackFile(store, path);
        } catch (error) {
          this.logger.warn(`Failed to delete from Tantivy: ${path}: ${error}`);
        }
      }
      for (const path of normalizedPaths) {
        denseDeleted += await this.milvus.deleteByPathPrefix(store, path);
      }
    }

    // Delete by path prefix
    if (normalizedPrefix) {
      try {
        sparseDeleted += await this.tantivyQueue.delete(store, { path: normalizedPrefix });
        this.fileTracker.untrackByPrefix(store, normalizedPrefix);
      } catch (error) {
        this.logger.warn(
          `Failed to delete from Tantivy by prefix: ${normalizedPrefix}: ${error}`,
        );
      }
      denseDeleted += await this.milvus.deleteByPathPrefix(store, normalizedPrefix);
    }

    this.storeManager.touchStore(store);

    return {
      sparse_deleted: sparseDeleted,
      dense_deleted: denseDeleted,
      time_ms: Date.now() - startTime,
    };
  }

  /**
   * Re-index entire store (clear and rebuild)
   * Non-blocking: clears existing data, then queues new files for indexing.
   */
  async reindex(
    store: string,
    files: FileToIndex[],
  ): Promise<AsyncIndexResponseDto> {
    // Clear file tracking for this store
    this.fileTracker.clearStore(store);

    // Delete all existing data
    await this.deleteFiles(store, undefined, '');

    // Re-index all files with force=true
    return this.indexFiles(store, files, true);
  }

  /**
   * Sync index with current files - remove deleted files from index
   */
  async syncDeletedFiles(
    store: string,
    currentPaths: string[],
  ): Promise<{ deleted: number }> {
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

  /**
   * List indexed files with pagination, filtering, and sorting
   */
  listFiles(
    store: string,
    options: {
      page?: number;
      pageSize?: number;
      pathFilter?: string;
      language?: string;
      sortBy?: 'path' | 'size' | 'indexed_at';
      sortOrder?: 'asc' | 'desc';
    } = {},
  ): {
    files: Array<{
      path: string;
      size: number;
      hash: string;
      indexed_at: string;
      chunk_count: number;
      language?: string;
    }>;
    total: number;
    page: number;
    page_size: number;
    total_pages: number;
  } {
    const {
      page = 1,
      pageSize = 50,
      pathFilter,
      language,
      sortBy = 'path',
      sortOrder = 'asc',
    } = options;

    // Get all tracked files
    let files = this.fileTracker.getTrackedFiles(store);

    // Apply path filter
    if (pathFilter) {
      const filterLower = pathFilter.toLowerCase();
      files = files.filter((f) => f.path.toLowerCase().includes(filterLower));
    }

    // Apply language filter (extract from path extension)
    if (language) {
      const langLower = language.toLowerCase();
      files = files.filter((f) => {
        const ext = f.path.split('.').pop()?.toLowerCase();
        // Map extensions to languages
        const extToLang: Record<string, string> = {
          ts: 'typescript',
          tsx: 'typescript',
          js: 'javascript',
          jsx: 'javascript',
          py: 'python',
          rs: 'rust',
          go: 'go',
          java: 'java',
          kt: 'kotlin',
          kts: 'kotlin',
          c: 'c',
          cpp: 'cpp',
          cc: 'cpp',
          h: 'c',
          hpp: 'cpp',
          cs: 'csharp',
          rb: 'ruby',
          php: 'php',
          swift: 'swift',
          scala: 'scala',
          md: 'markdown',
          json: 'json',
          yaml: 'yaml',
          yml: 'yaml',
          toml: 'toml',
          sql: 'sql',
          sh: 'bash',
          bash: 'bash',
          zsh: 'bash',
        };
        const fileLang = ext ? extToLang[ext] : undefined;
        return fileLang?.toLowerCase() === langLower;
      });
    }

    // Sort
    files.sort((a, b) => {
      let cmp = 0;
      switch (sortBy) {
        case 'size':
          cmp = a.size - b.size;
          break;
        case 'indexed_at':
          cmp = new Date(a.indexed_at).getTime() - new Date(b.indexed_at).getTime();
          break;
        case 'path':
        default:
          cmp = a.path.localeCompare(b.path);
          break;
      }
      return sortOrder === 'desc' ? -cmp : cmp;
    });

    const total = files.length;
    const totalPages = Math.ceil(total / pageSize);
    const offset = (page - 1) * pageSize;
    const paginatedFiles = files.slice(offset, offset + pageSize);

    // Map to response format with language detection
    const extToLang: Record<string, string> = {
      ts: 'typescript',
      tsx: 'typescript',
      js: 'javascript',
      jsx: 'javascript',
      py: 'python',
      rs: 'rust',
      go: 'go',
      java: 'java',
      kt: 'kotlin',
      kts: 'kotlin',
      c: 'c',
      cpp: 'cpp',
      cc: 'cpp',
      h: 'c',
      hpp: 'cpp',
      cs: 'csharp',
      rb: 'ruby',
      php: 'php',
      swift: 'swift',
      scala: 'scala',
      md: 'markdown',
      json: 'json',
      yaml: 'yaml',
      yml: 'yaml',
      toml: 'toml',
      sql: 'sql',
      sh: 'bash',
      bash: 'bash',
      zsh: 'bash',
    };

    return {
      files: paginatedFiles.map((f) => {
        const ext = f.path.split('.').pop()?.toLowerCase();
        return {
          path: f.path,
          size: f.size,
          hash: f.hash,
          indexed_at: f.indexed_at,
          chunk_count: f.chunk_ids.length,
          language: ext ? extToLang[ext] : undefined,
        };
      }),
      total,
      page,
      page_size: pageSize,
      total_pages: totalPages,
    };
  }
}
