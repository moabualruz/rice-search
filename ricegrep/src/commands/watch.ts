import * as fs from "node:fs";
import * as path from "node:path";
import { Command, InvalidArgumentError } from "commander";
import { type CliConfigOptions, loadConfig } from "../lib/config.js";
import { createFileSystem, createStore } from "../lib/context.js";
import { DEFAULT_IGNORE_PATTERNS } from "../lib/file.js";
import {
  createIndexingSpinner,
  formatDryRunSummary,
} from "../lib/sync-helpers.js";
import {
  deleteFile,
  initialSync,
  isAtOrAboveHomeDirectory,
  MaxFileCountExceededError,
  QuotaExceededError,
  uploadFile,
} from "../lib/utils.js";

export interface WatchOptions {
  store: string;
  dryRun: boolean;
  incremental: boolean;
  maxFileSize?: number;
  maxFileCount?: number;
}

export async function startWatch(options: WatchOptions): Promise<void> {
  let refreshInterval: NodeJS.Timeout | undefined;

  try {
    const store = await createStore();

    // Refresh JWT token every 5 minutes (before 15-minute expiration)
    if (!options.dryRun) {
      const REFRESH_INTERVAL = 5 * 60 * 1000;
      refreshInterval = setInterval(async () => {
        try {
          await store.refreshClient?.();
        } catch (err) {
          console.error(
            "Failed to refresh JWT token:",
            err instanceof Error ? err.message : "Unknown error",
          );
        }
      }, REFRESH_INTERVAL);
      // Allow process to exit even if interval is active (fs.watch keeps it alive anyway)
      refreshInterval.unref();
    }

    const fileSystem = createFileSystem({
      ignorePatterns: [...DEFAULT_IGNORE_PATTERNS],
    });
    const watchRoot = process.cwd();

    if (isAtOrAboveHomeDirectory(watchRoot)) {
      console.error(
        "Error: Cannot watch home directory or any parent directory.",
      );
      console.error(
        "Please run this command from within a specific project subdirectory.",
      );
      process.exitCode = 1;
      return;
    }

    const cliOptions: CliConfigOptions = {
      maxFileSize: options.maxFileSize,
      maxFileCount: options.maxFileCount,
    };
    const config = loadConfig(watchRoot, cliOptions);
    console.debug("Watching for file changes in", watchRoot);

    const { spinner, onProgress } = createIndexingSpinner(watchRoot);
    try {
      try {
        await store.retrieve(options.store);
      } catch {
        await store.create({
          name: options.store,
          description:
            "Rice Search store - Local hybrid search with BM25 + semantic embeddings",
        });
      }
      const result = await initialSync(
        store,
        fileSystem,
        options.store,
        watchRoot,
        options.dryRun,
        onProgress,
        config,
        options.incremental,
      );
      const deletedInfo =
        result.deleted > 0 ? ` • deleted ${result.deleted}` : "";
      const errorsInfo = result.errors > 0 ? ` • errors ${result.errors}` : "";
      if (result.errors > 0) {
        spinner.warn(
          `Initial sync complete (${result.processed}/${result.total}) • uploaded ${result.uploaded}${deletedInfo}${errorsInfo}`,
        );
        console.error(
          `\n⚠️  ${result.errors} file(s) failed to upload. Run with DEBUG=ricegrep* for more details.`,
        );
      } else {
        spinner.succeed(
          `Initial sync complete (${result.processed}/${result.total}) • uploaded ${result.uploaded}${deletedInfo}`,
        );
      }
      if (options.dryRun) {
        console.log(
          formatDryRunSummary(result, {
            actionDescription: "found",
            includeTotal: true,
          }),
        );
        return;
      }
    } catch (e) {
      if (e instanceof QuotaExceededError) {
        spinner.fail("Storage limit exceeded");
        console.error(
          "\n❌ Storage limit exceeded. Please free up space or adjust your file count limits.\n",
        );
        process.exit(1);
      }
      if (e instanceof MaxFileCountExceededError) {
        spinner.fail("File count exceeded");
        console.error(`\n❌ ${e.message}`);
        console.error(
          "   Increase the limit with --max-file-count or RICEGREP_MAX_FILE_COUNT environment variable.\n",
        );
        process.exit(1);
      }
      spinner.fail("Initial upload failed");
      throw e;
    }

    console.log("Watching for file changes in", watchRoot);
    fileSystem.loadRicegrepignore(watchRoot);
    fs.watch(watchRoot, { recursive: true }, (eventType, rawFilename) => {
      const filename = rawFilename?.toString();
      if (!filename) {
        return;
      }
      const filePath = path.join(watchRoot, filename);

      if (fileSystem.isIgnored(filePath, watchRoot)) {
        return;
      }

      try {
        const stat = fs.statSync(filePath);
        if (!stat.isFile()) {
          return;
        }

        uploadFile(store, options.store, filePath, filename, config)
          .then((didUpload) => {
            if (didUpload) {
              console.log(`${eventType}: ${filePath}`);
            }
          })
          .catch((err) => {
            console.error("Failed to upload changed file:", filePath, err);
          });
      } catch {
        if (filePath.startsWith(watchRoot) && !fs.existsSync(filePath)) {
          deleteFile(store, options.store, filePath)
            .then(() => {
              console.log(`delete: ${filePath}`);
            })
            .catch((err) => {
              console.error("Failed to delete file:", filePath, err);
            });
        }
      }
    });
  } catch (error) {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
    const message = error instanceof Error ? error.message : "Unknown error";
    console.error("Failed to start watcher:", message);
    process.exitCode = 1;
  }
}

export const watch = new Command("watch")
  .option(
    "-d, --dry-run",
    "Dry run the watch process (no actual file syncing)",
    false,
  )
  .option(
    "-i, --incremental",
    "Incremental sync (only upload changed files, let server decide)",
    false,
  )
  .option(
    "--max-file-size <bytes>",
    "Maximum file size in bytes to upload",
    (value) => {
      const parsed = Number.parseInt(value, 10);
      if (Number.isNaN(parsed) || parsed <= 0) {
        throw new InvalidArgumentError("Must be a positive integer.");
      }
      return parsed;
    },
  )
  .option(
    "--max-file-count <count>",
    "Maximum number of files to upload",
    (value) => {
      const parsed = Number.parseInt(value, 10);
      if (Number.isNaN(parsed) || parsed <= 0) {
        throw new InvalidArgumentError("Must be a positive integer.");
      }
      return parsed;
    },
  )
  .description("Watch for file changes")
  .action(async (_args, cmd) => {
    const options: WatchOptions = cmd.optsWithGlobals();
    await startWatch(options);
  });
