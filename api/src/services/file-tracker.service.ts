import { Injectable, Logger, OnModuleInit } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import * as crypto from 'crypto';
import * as fs from 'fs';
import * as path from 'path';

/**
 * Tracked file info
 */
export interface TrackedFile {
  path: string;
  hash: string;
  mtime: number;
  size: number;
  indexed_at: string;
  chunk_ids: string[];
}

/**
 * Store tracking data
 */
interface StoreTrackingData {
  store: string;
  files: Record<string, TrackedFile>;
  last_updated: string;
}

/**
 * FileTrackerService - Tracks file hashes and mtimes for incremental indexing
 *
 * Provides:
 * - Content hash computation (xxhash-style fast hash)
 * - File change detection
 * - Tracking data persistence to disk
 */
@Injectable()
export class FileTrackerService implements OnModuleInit {
  private readonly logger = new Logger(FileTrackerService.name);
  private readonly dataDir: string;
  private readonly trackingDir: string;
  private stores: Map<string, StoreTrackingData> = new Map();

  constructor(private configService: ConfigService) {
    this.dataDir = this.configService.get<string>('data.dir') || '/data';
    this.trackingDir = path.join(this.dataDir, 'tracking');
  }

  async onModuleInit() {
    // Ensure tracking directory exists
    try {
      fs.mkdirSync(this.trackingDir, { recursive: true });
      this.logger.log(`File tracking directory: ${this.trackingDir}`);
    } catch (error) {
      this.logger.warn(`Failed to create tracking directory: ${error}`);
    }
  }

  /**
   * Compute content hash using SHA-256 (first 16 chars for speed)
   */
  computeHash(content: string): string {
    return crypto.createHash('sha256').update(content).digest('hex').slice(0, 16);
  }

  /**
   * Compute file hash from file path
   */
  computeFileHash(filePath: string): string | null {
    try {
      const content = fs.readFileSync(filePath, 'utf-8');
      return this.computeHash(content);
    } catch {
      return null;
    }
  }

  /**
   * Get tracking file path for a store
   */
  private getTrackingFilePath(store: string): string {
    return path.join(this.trackingDir, `${store}.json`);
  }

  /**
   * Load tracking data for a store
   */
  loadStoreTracking(store: string): StoreTrackingData {
    if (this.stores.has(store)) {
      return this.stores.get(store)!;
    }

    const trackingFile = this.getTrackingFilePath(store);
    let data: StoreTrackingData;

    try {
      if (fs.existsSync(trackingFile)) {
        const content = fs.readFileSync(trackingFile, 'utf-8');
        data = JSON.parse(content);
        this.logger.log(`Loaded tracking data for store '${store}': ${Object.keys(data.files).length} files`);
      } else {
        data = {
          store,
          files: {},
          last_updated: new Date().toISOString(),
        };
      }
    } catch (error) {
      this.logger.warn(`Failed to load tracking data for store '${store}': ${error}`);
      data = {
        store,
        files: {},
        last_updated: new Date().toISOString(),
      };
    }

    this.stores.set(store, data);
    return data;
  }

  /**
   * Save tracking data for a store
   */
  saveStoreTracking(store: string): void {
    const data = this.stores.get(store);
    if (!data) return;

    data.last_updated = new Date().toISOString();
    const trackingFile = this.getTrackingFilePath(store);

    try {
      fs.mkdirSync(this.trackingDir, { recursive: true });
      fs.writeFileSync(trackingFile, JSON.stringify(data, null, 2));
    } catch (error) {
      this.logger.warn(`Failed to save tracking data for store '${store}': ${error}`);
    }
  }

  /**
   * Check if a file has changed since last indexing
   */
  hasFileChanged(store: string, filePath: string, content: string): boolean {
    const data = this.loadStoreTracking(store);
    const tracked = data.files[filePath];

    if (!tracked) {
      // New file - needs indexing
      return true;
    }

    // Compute current hash
    const currentHash = this.computeHash(content);

    // Compare hashes
    return tracked.hash !== currentHash;
  }

  /**
   * Check multiple files for changes
   * Returns: { changed: string[], unchanged: string[], deleted: string[] }
   */
  checkFilesForChanges(
    store: string,
    files: { path: string; content: string }[],
  ): {
    changed: { path: string; content: string }[];
    unchanged: string[];
    newFiles: { path: string; content: string }[];
  } {
    const data = this.loadStoreTracking(store);
    const changed: { path: string; content: string }[] = [];
    const unchanged: string[] = [];
    const newFiles: { path: string; content: string }[] = [];

    for (const file of files) {
      const tracked = data.files[file.path];
      const currentHash = this.computeHash(file.content);

      if (!tracked) {
        // New file
        newFiles.push(file);
      } else if (tracked.hash !== currentHash) {
        // Changed file
        changed.push(file);
      } else {
        // Unchanged
        unchanged.push(file.path);
      }
    }

    return { changed, unchanged, newFiles };
  }

  /**
   * Track a file after successful indexing
   */
  trackFile(
    store: string,
    filePath: string,
    content: string,
    chunkIds: string[],
  ): void {
    const data = this.loadStoreTracking(store);

    data.files[filePath] = {
      path: filePath,
      hash: this.computeHash(content),
      mtime: Date.now(),
      size: content.length,
      indexed_at: new Date().toISOString(),
      chunk_ids: chunkIds,
    };
  }

  /**
   * Track multiple files after successful indexing
   */
  trackFiles(
    store: string,
    files: { path: string; content: string; chunkIds: string[] }[],
  ): void {
    for (const file of files) {
      this.trackFile(store, file.path, file.content, file.chunkIds);
    }
    this.saveStoreTracking(store);
  }

  /**
   * Remove tracking for a file
   */
  untrackFile(store: string, filePath: string): string[] {
    const data = this.loadStoreTracking(store);
    const tracked = data.files[filePath];
    const chunkIds = tracked?.chunk_ids || [];

    delete data.files[filePath];
    this.saveStoreTracking(store);

    return chunkIds;
  }

  /**
   * Remove tracking for files by path prefix
   */
  untrackByPrefix(store: string, pathPrefix: string): string[] {
    const data = this.loadStoreTracking(store);
    const allChunkIds: string[] = [];

    for (const [filePath, tracked] of Object.entries(data.files)) {
      if (filePath.startsWith(pathPrefix)) {
        allChunkIds.push(...tracked.chunk_ids);
        delete data.files[filePath];
      }
    }

    this.saveStoreTracking(store);
    return allChunkIds;
  }

  /**
   * Get all tracked files for a store
   */
  getTrackedFiles(store: string): TrackedFile[] {
    const data = this.loadStoreTracking(store);
    return Object.values(data.files);
  }

  /**
   * Get tracking info for a specific file
   */
  getFileInfo(store: string, filePath: string): TrackedFile | null {
    const data = this.loadStoreTracking(store);
    return data.files[filePath] || null;
  }

  /**
   * Find deleted files (tracked but no longer in provided list)
   */
  findDeletedFiles(store: string, currentPaths: string[]): string[] {
    const data = this.loadStoreTracking(store);
    const currentSet = new Set(currentPaths);
    const deleted: string[] = [];

    for (const trackedPath of Object.keys(data.files)) {
      if (!currentSet.has(trackedPath)) {
        deleted.push(trackedPath);
      }
    }

    return deleted;
  }

  /**
   * Clear all tracking for a store
   */
  clearStore(store: string): void {
    this.stores.delete(store);
    const trackingFile = this.getTrackingFilePath(store);

    try {
      if (fs.existsSync(trackingFile)) {
        fs.unlinkSync(trackingFile);
      }
    } catch (error) {
      this.logger.warn(`Failed to delete tracking file: ${error}`);
    }
  }

  /**
   * Get store statistics
   */
  getStoreStats(store: string): {
    tracked_files: number;
    total_size: number;
    last_updated: string;
  } {
    const data = this.loadStoreTracking(store);
    const files = Object.values(data.files);

    return {
      tracked_files: files.length,
      total_size: files.reduce((sum, f) => sum + f.size, 0),
      last_updated: data.last_updated,
    };
  }
}
