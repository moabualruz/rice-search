import { createHash } from "node:crypto";
import * as fs from "node:fs";
import * as os from "node:os";
import * as path from "node:path";

import { isText } from "istextorbinary";
import pLimit from "p-limit";
import xxhashWasm from "xxhash-wasm";

import { exceedsMaxFileSize, type RicegrepConfig } from "./config.js";
import type { FileSystem } from "./file.js";
import type { BatchFile, Store } from "./store.js";
import type { InitialSyncProgress, InitialSyncResult } from "./sync-helpers.js";

// Batch size for uploads - small batches sent concurrently
const UPLOAD_BATCH_SIZE = 15;
// Concurrency for file reading - low to avoid EMFILE errors on Windows
const FILE_READ_CONCURRENCY = 20;
// Max concurrent batch uploads to server
const UPLOAD_CONCURRENCY = 10;



export const isTest = process.env.RICEGREP_IS_TEST === "1";

/** Error thrown when storage limits are exceeded */
export class QuotaExceededError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "QuotaExceededError";
  }
}

/** Error thrown when the file count exceeds the configured limit */
export class MaxFileCountExceededError extends Error {
  constructor(fileCount: number, maxFileCount: number) {
    super(
      `File count (${fileCount}) exceeds the maximum allowed (${maxFileCount}). No files were uploaded.`,
    );
    this.name = "MaxFileCountExceededError";
  }
}

/** Check if an error message indicates a storage limit issue */
function isQuotaError(errorMessage: string): boolean {
  return (
    errorMessage.includes("storage") ||
    errorMessage.includes("limit") ||
    errorMessage.includes("space")
  );
}

function isSubpath(parent: string, child: string): boolean {
  const parentPath = path.resolve(parent);
  const childPath = path.resolve(child);

  const parentWithSep = parentPath.endsWith(path.sep)
    ? parentPath
    : parentPath + path.sep;

  return childPath.startsWith(parentWithSep);
}

/**
 * Checks if a path is at or above the home directory.
 * Returns true if the path is the home directory, a parent of it, or the root.
 *
 * @param targetPath - The path to check
 * @returns true if the path is at or above home directory, false if it's a subdirectory of home
 */
export function isAtOrAboveHomeDirectory(targetPath: string): boolean {
  const homeDir = os.homedir();
  const resolvedTarget = path.resolve(targetPath);
  const resolvedHome = path.resolve(homeDir);

  if (resolvedTarget === resolvedHome) {
    return true;
  }

  const targetWithSep = resolvedTarget.endsWith(path.sep)
    ? resolvedTarget
    : resolvedTarget + path.sep;

  if (resolvedHome.startsWith(targetWithSep)) {
    return true;
  }

  return false;
}

const XXHASH_PREFIX = "xxh64:";

/** Lazily initialized xxhash instance */
const xxhashPromise = xxhashWasm();

/**
 * Computes SHA-256 hash of a buffer (used for backward compatibility)
 */
function computeSha256Hash(buffer: Buffer): string {
  return createHash("sha256").update(buffer).digest("hex");
}

/**
 * Computes xxhash64 hash of a buffer.
 * Returns the hash prefixed with "xxh64:" to identify the algorithm.
 */
export async function computeBufferHash(buffer: Buffer): Promise<string> {
  const { h64Raw } = await xxhashPromise;
  const hash = h64Raw(new Uint8Array(buffer)).toString(16).padStart(16, "0");
  return XXHASH_PREFIX + hash;
}

/**
 * Computes a hash of the file using xxhash64.
 */
export async function computeFileHash(
  filePath: string,
  readFileSyncFn: (p: string) => Buffer,
): Promise<string> {
  const buffer = readFileSyncFn(filePath);
  return computeBufferHash(buffer);
}

/**
 * Checks if a stored hash matches the computed hash of a buffer.
 * Supports both old SHA-256 hashes (no prefix) and new xxhash64 hashes (xxh64: prefix).
 */
export async function hashesMatch(
  storedHash: string,
  buffer: Buffer,
): Promise<boolean> {
  if (storedHash.startsWith(XXHASH_PREFIX)) {
    const computedHash = await computeBufferHash(buffer);
    return storedHash === computedHash;
  }
  const computedSha256 = computeSha256Hash(buffer);
  return storedHash === computedSha256;
}

export function isDevelopment(): boolean {
  if (process.env.NODE_ENV === "development" || isTest) {
    return true;
  }

  return false;
}

/**
 * Lists file hashes from the store, optionally filtered by path prefix.
 *
 * @param store - The store instance
 * @param storeId - The ID of the store
 * @param pathPrefix - Optional path prefix to filter files (only files starting with this path are returned)
 * @returns A map of external IDs to their hashes
 */
export async function listStoreFileHashes(
  store: Store,
  storeId: string,
  pathPrefix?: string,
): Promise<Map<string, string | undefined>> {
  const byExternalId = new Map<string, string | undefined>();
  for await (const file of store.listFiles(storeId, { pathPrefix })) {
    const externalId = file.external_id ?? undefined;
    if (!externalId) continue;
    const metadata = file.metadata;
    const hash: string | undefined =
      metadata && typeof metadata.hash === "string" ? metadata.hash : undefined;
    byExternalId.set(externalId, hash);
  }
  return byExternalId;
}

export async function ensureAuthenticated(): Promise<void> {
  // Rice Search is always local - no authentication needed
  return;
}

export async function deleteFile(
  store: Store,
  storeId: string,
  filePath: string,
): Promise<void> {
  await store.deleteFile(storeId, filePath);
}

export async function uploadFile(
  store: Store,
  storeId: string,
  filePath: string,
  fileName: string,
  config?: RicegrepConfig,
  incremental = false,
): Promise<boolean> {
  if (config && exceedsMaxFileSize(filePath, config.maxFileSize)) {
    return false;
  }

  const buffer = await fs.promises.readFile(filePath);
  if (buffer.length === 0) {
    return false;
  }

  const hash = await computeBufferHash(buffer);
  const options = {
    external_id: filePath,
    // When incremental=true, let server decide if file changed (force=false)
    // When incremental=false, force reindex (force=true)
    overwrite: !incremental,
    metadata: {
      path: filePath,
      hash,
    },
  };

  try {
    await store.uploadFile(
      storeId,
      fs.createReadStream(filePath) as unknown as File | ReadableStream,
      options,
    );
  } catch (streamErr) {
    const streamErrMsg =
      streamErr instanceof Error ? streamErr.message : String(streamErr);

    // Check for quota errors and throw immediately to stop processing
    if (isQuotaError(streamErrMsg)) {
      throw new QuotaExceededError(streamErrMsg);
    }

    if (!isText(filePath)) {
      return false;
    }
    try {
      await store.uploadFile(
        storeId,
        new File([buffer], fileName, { type: "text/plain" }),
        options,
      );
    } catch (fileErr) {
      const fileErrMsg =
        fileErr instanceof Error ? fileErr.message : String(fileErr);

      // Check for quota errors and throw immediately to stop processing
      if (isQuotaError(fileErrMsg)) {
        throw new QuotaExceededError(fileErrMsg);
      }

      throw fileErr;
    }
  }
  return true;
}

export async function initialSync(
  store: Store,
  fileSystem: FileSystem,
  storeId: string,
  repoRoot: string,
  dryRun?: boolean,
  onProgress?: (info: InitialSyncProgress) => void,
  config?: RicegrepConfig,
  incremental = false,
): Promise<InitialSyncResult> {
  const storeHashes = await listStoreFileHashes(store, storeId, repoRoot);
  const allFiles = Array.from(fileSystem.getFiles(repoRoot));
  const repoFiles = allFiles.filter(
    (filePath) => !fileSystem.isIgnored(filePath, repoRoot),
  );

  if (config && Number.isFinite(config.maxFileCount) && repoFiles.length > config.maxFileCount) {
    throw new MaxFileCountExceededError(repoFiles.length, config.maxFileCount);
  }

  const repoFileSet = new Set(repoFiles);

  const filesToDelete = Array.from(storeHashes.keys()).filter(
    (filePath) => isSubpath(repoRoot, filePath) && !repoFileSet.has(filePath),
  );

  const total = repoFiles.length + filesToDelete.length;
  let processed = 0;
  let uploaded = 0;
  let deleted = 0;
  let errors = 0;
  let quotaExceeded = false;
  let quotaErrorMessage = "";

  // Concurrency limits
  const readLimit = pLimit(FILE_READ_CONCURRENCY);
  const uploadLimit = pLimit(UPLOAD_CONCURRENCY);

  // Track in-flight batch uploads (non-blocking)
  const batchPromises: Promise<void>[] = [];

  // Current batch accumulator
  let currentBatch: BatchFile[] = [];

  // Send batch to server (non-blocking - runs concurrently)
  const sendBatch = (batch: BatchFile[]) => {
    if (batch.length === 0) return;

    const batchPromise = uploadLimit(async () => {
      if (quotaExceeded) return;

      if (dryRun) {
        for (const file of batch) {
          console.log("Dry run: would have uploaded", file.path);
        }
        uploaded += batch.length;
      } else {
        try {
          const result = await store.uploadBatch(storeId, batch, !incremental);
          uploaded += result.indexed;
          if (result.errors) {
            errors += result.errors.length;
          }
        } catch (err) {
          const errMsg = err instanceof Error ? err.message : String(err);
          if (isQuotaError(errMsg)) {
            quotaExceeded = true;
            quotaErrorMessage = errMsg;
          } else {
            errors += batch.length;
          }
        }
      }
    });

    batchPromises.push(batchPromise);
  };

  // Process files: read concurrently, batch and send concurrently
  const filePromises = repoFiles.map((filePath) =>
    readLimit(async () => {
      if (quotaExceeded) {
        processed += 1;
        return;
      }

      try {
        // Skip large files
        if (config && exceedsMaxFileSize(filePath, config.maxFileSize)) {
          processed += 1;
          onProgress?.({ processed, uploaded, deleted, errors, total, filePath });
          return;
        }

        // Read file content
        const buffer = await fs.promises.readFile(filePath);
        if (buffer.length === 0) {
          processed += 1;
          onProgress?.({ processed, uploaded, deleted, errors, total, filePath });
          return;
        }

        // Check if file changed
        const existingHash = storeHashes.get(filePath);
        const hashMatches = existingHash
          ? await hashesMatch(existingHash, buffer)
          : false;

        processed += 1;

        if (!hashMatches) {
          // Add to batch
          currentBatch.push({
            path: filePath,
            content: buffer.toString("utf-8"),
          });

          // Send batch when full (non-blocking)
          if (currentBatch.length >= UPLOAD_BATCH_SIZE) {
            sendBatch(currentBatch);
            currentBatch = [];
          }
        }

        onProgress?.({ processed, uploaded, deleted, errors, total, filePath });
      } catch (err) {
        processed += 1;
        errors += 1;
        const errorMessage = err instanceof Error ? err.message : String(err);
        onProgress?.({
          processed,
          uploaded,
          deleted,
          errors,
          total,
          filePath,
          lastError: errorMessage,
        });
      }
    }),
  );

  // Wait for all file reads to complete
  await Promise.all(filePromises);

  // Send remaining batch
  sendBatch(currentBatch);

  // Wait for all batch uploads to complete
  await Promise.all(batchPromises);

  // Delete files that no longer exist (concurrent)
  const deleteLimit = pLimit(UPLOAD_CONCURRENCY);
  await Promise.all(
    filesToDelete.map((filePath) =>
      deleteLimit(async () => {
        if (quotaExceeded) {
          processed += 1;
          return;
        }

        try {
          if (dryRun) {
            console.log("Dry run: would have deleted", filePath);
          } else {
            await store.deleteFile(storeId, filePath);
          }
          deleted += 1;
          processed += 1;
          onProgress?.({ processed, uploaded, deleted, errors, total, filePath });
        } catch (err) {
          processed += 1;
          errors += 1;
          const errorMessage = err instanceof Error ? err.message : String(err);
          onProgress?.({
            processed,
            uploaded,
            deleted,
            errors,
            total,
            filePath,
            lastError: errorMessage,
          });
        }
      }),
    ),
  );

  if (quotaExceeded) {
    throw new QuotaExceededError(quotaErrorMessage);
  }

  return { processed, uploaded, deleted, errors, total };
}
