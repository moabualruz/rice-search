/**
 * Shared WebSocket watch utilities
 * Used by both watch-ws.ts (verbose) and mcp.ts (silent)
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { type CliConfigOptions, loadConfig } from "./config.js";
import { createFileSystem } from "./context.js";
import { DEFAULT_IGNORE_PATTERNS, type FileSystem } from "./file.js";
import { MaxFileCountExceededError } from "./utils.js";
import type { RiceWsClient } from "./ws-client.js";

/**
 * Get base URL from environment or default
 */
export function getBaseUrl(): string {
  return process.env.RICEGREP_BASE_URL || "http://localhost:8080";
}

/**
 * Read file content safely
 */
export async function readFileContent(filePath: string): Promise<string | null> {
  try {
    return await fs.promises.readFile(filePath, "utf-8");
  } catch {
    return null;
  }
}

/**
 * Check if file should be skipped based on size
 */
export function shouldSkipFile(content: string, maxFileSize: number): boolean {
  const sizeBytes = Buffer.byteLength(content, "utf-8");
  return sizeBytes > maxFileSize;
}

/**
 * File info for walking
 */
export interface FileInfo {
  path: string;
  relativePath: string;
}

/**
 * Walk options
 */
export interface WalkOptions {
  watchRoot: string;
  fileSystem: FileSystem;
  maxFileCount?: number;
  onFile?: (file: FileInfo) => void;
}

/**
 * Walk result
 */
export interface WalkResult {
  files: FileInfo[];
  totalFiles: number;
}

/**
 * Walk directory and collect files
 */
export async function walkDirectory(options: WalkOptions): Promise<WalkResult> {
  const { watchRoot, fileSystem, maxFileCount, onFile } = options;
  const files: FileInfo[] = [];
  let totalFiles = 0;

  const walk = async (dir: string): Promise<void> => {
    const entries = await fs.promises.readdir(dir, { withFileTypes: true });

    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      const relativePath = path.relative(watchRoot, fullPath);

      if (fileSystem.isIgnored(fullPath, watchRoot)) {
        continue;
      }

      if (entry.isDirectory()) {
        await walk(fullPath);
      } else if (entry.isFile()) {
        totalFiles++;
        const fileInfo = { path: fullPath, relativePath };
        files.push(fileInfo);
        onFile?.(fileInfo);

        if (maxFileCount && totalFiles > maxFileCount) {
          throw new MaxFileCountExceededError(totalFiles, maxFileCount);
        }
      }
    }
  };

  await walk(watchRoot);
  return { files, totalFiles };
}

/**
 * Watch options
 */
export interface WatchConfig {
  store: string;
  maxFileSize?: number;
  maxFileCount?: number;
}

/**
 * Load watch configuration
 */
export function loadWatchConfig(watchRoot: string, cliOptions: CliConfigOptions = {}): {
  config: ReturnType<typeof loadConfig>;
  maxFileSize: number;
  fileSystem: FileSystem;
} {
  const config = loadConfig(watchRoot, cliOptions);
  const maxFileSize = config.maxFileSize || 1024 * 1024 * 100; // 100MB default

  const fileSystem = createFileSystem({
    ignorePatterns: [...DEFAULT_IGNORE_PATTERNS],
  });
  fileSystem.loadRicegrepignore(watchRoot);

  return { config, maxFileSize, fileSystem };
}

/**
 * Send file via WebSocket (fire-and-forget)
 */
export async function sendFileToServer(
  client: RiceWsClient,
  filePath: string,
  relativePath: string,
  maxFileSize: number,
): Promise<boolean> {
  const content = await readFileContent(filePath);
  if (!content) return false;
  if (shouldSkipFile(content, maxFileSize)) return false;

  client.sendFile(relativePath, content);
  return true;
}

/**
 * Setup file watcher with callback
 */
export function setupFileWatcher(
  watchRoot: string,
  fileSystem: FileSystem,
  maxFileSize: number,
  client: RiceWsClient,
  options: {
    onFileChange?: (eventType: string, filePath: string) => void;
    onFileDelete?: (filePath: string) => void;
    onError?: (filePath: string, error: unknown) => void;
    silent?: boolean;
  } = {},
): void {
  fs.watch(watchRoot, { recursive: true }, async (eventType, rawFilename) => {
    const filename = rawFilename?.toString();
    if (!filename) return;

    const filePath = path.join(watchRoot, filename);
    const relativePath = filename;

    if (fileSystem.isIgnored(filePath, watchRoot)) {
      return;
    }

    try {
      const stat = fs.statSync(filePath);
      if (!stat.isFile()) return;

      const sent = await sendFileToServer(client, filePath, relativePath, maxFileSize);
      if (sent && options.onFileChange) {
        options.onFileChange(eventType, filePath);
      }
    } catch {
      // File might have been deleted
      if (!fs.existsSync(filePath)) {
        client
          .deleteFiles({ paths: [relativePath] })
          .then(() => {
            if (options.onFileDelete) {
              options.onFileDelete(filePath);
            }
          })
          .catch((err) => {
            if (options.onError && !options.silent) {
              options.onError(filePath, err);
            }
          });
      }
    }
  });
}

/**
 * Start periodic ping to keep connection alive
 */
export function startPingInterval(client: RiceWsClient, intervalMs = 30000): NodeJS.Timeout {
  return setInterval(() => {
    if (client.isConnected()) {
      client.ping();
    }
  }, intervalMs);
}
