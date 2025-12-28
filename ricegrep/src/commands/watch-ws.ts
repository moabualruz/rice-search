import { Command, InvalidArgumentError } from "commander";
import { type CliConfigOptions } from "../lib/config.js";
import { createIndexingSpinner } from "../lib/sync-helpers.js";
import {
  RiceWsClient,
  type WsIndexedMessage,
} from "../lib/ws-client.js";
import {
  getBaseUrl,
  loadWatchConfig,
  sendFileToServer,
  setupFileWatcher,
  startPingInterval,
  walkDirectory,
} from "../lib/ws-watch.js";
import {
  isAtOrAboveHomeDirectory,
  MaxFileCountExceededError,
} from "../lib/utils.js";

export interface WatchWsOptions {
  store: string;
  dryRun: boolean;
  incremental: boolean;
  maxFileSize?: number;
  maxFileCount?: number;
}

/**
 * WebSocket-based watch mode
 * 
 * - Connects via WebSocket for non-blocking communication
 * - Sends files individually (server batches them)
 * - Receives 'indexed' notifications when batches complete
 */
export async function startWatchWs(options: WatchWsOptions): Promise<void> {
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
  const { config, maxFileSize, fileSystem } = loadWatchConfig(watchRoot, cliOptions);

  // Stats tracking
  let totalFiles = 0;
  let sentFiles = 0;
  let indexedChunks = 0;
  let errors = 0;

  const { spinner, onProgress } = createIndexingSpinner(watchRoot);

  // Create WebSocket client
  const client = new RiceWsClient({
    baseUrl: getBaseUrl(),
    store: options.store,
    onIndexed: (msg: WsIndexedMessage) => {
      indexedChunks += msg.chunks_queued;
      console.log(
        `\n✓ Batch ${msg.batch_id}: ${msg.files_count} files → ${msg.chunks_queued} chunks indexed`,
      );
    },
    onError: (msg) => {
      errors++;
      console.error(`\n✗ Error: ${msg.code} - ${msg.message}`);
    },
    onDisconnect: () => {
      console.log("\n⚠ WebSocket disconnected, attempting reconnect...");
    },
    onConnect: (connId) => {
      console.log(`\n✓ Connected (${connId})`);
    },
    reconnect: true,
    reconnectDelay: 3000,
  });

  try {
    // Connect to server
    spinner.text = "Connecting to Rice Search...";
    const connId = await client.connect();
    spinner.text = `Connected (${connId}). Scanning files...`;

    if (options.dryRun) {
      spinner.info("Dry run mode - no files will be sent");
    }

    // Skip initial sync in incremental mode
    if (options.incremental) {
      spinner.succeed("Incremental mode - skipping initial sync");
      console.log("Watching for file changes in", watchRoot);
    } else {
      // Walk directory and collect files
      const { files, totalFiles: total } = await walkDirectory({
        watchRoot,
        fileSystem,
        maxFileCount: config.maxFileCount,
      });
      totalFiles = total;

      spinner.text = `Found ${totalFiles} files. Sending...`;

      // Send all files (fire-and-forget via WebSocket)
      for (const file of files) {
        try {
          if (!options.dryRun) {
            await sendFileToServer(client, file.path, file.relativePath, maxFileSize);
          }

          sentFiles++;
          onProgress({
            processed: sentFiles,
            uploaded: sentFiles,
            deleted: 0,
            errors,
            total: totalFiles,
            filePath: file.relativePath,
          });
        } catch (err) {
          errors++;
          console.error(`\nFailed to send ${file.relativePath}:`, err);
        }
      }

      const errorsInfo = errors > 0 ? ` • errors ${errors}` : "";
      if (options.dryRun) {
        spinner.info(
          `Dry run complete: ${sentFiles}/${totalFiles} files would be sent${errorsInfo}`,
        );
        client.close();
        return;
      }

      spinner.succeed(
        `Initial sync: sent ${sentFiles}/${totalFiles} files${errorsInfo}`,
      );
      console.log("Watching for file changes in", watchRoot);
    }

    // Setup file watcher with verbose callbacks
    setupFileWatcher(watchRoot, fileSystem, maxFileSize, client, {
      onFileChange: (eventType, filePath) => {
        console.log(`${eventType}: ${filePath}`);
      },
      onFileDelete: (filePath) => {
        console.log(`delete: ${filePath}`);
      },
      onError: (filePath, err) => {
        console.error("Failed to process file:", filePath, err);
      },
    });

    // Keep alive with periodic pings
    const pingInterval = startPingInterval(client);

    // Handle process exit
    const cleanup = () => {
      clearInterval(pingInterval);
      client.close();
      process.exit(0);
    };

    process.on("SIGINT", cleanup);
    process.on("SIGTERM", cleanup);
  } catch (error) {
    client.close();
    if (error instanceof MaxFileCountExceededError) {
      spinner.fail("File count exceeded");
      console.error(`\n❌ ${error.message}`);
      console.error(
        "   Increase the limit with --max-file-count or RICEGREP_MAX_FILE_COUNT environment variable.\n",
      );
      process.exit(1);
    }
    const message = error instanceof Error ? error.message : "Unknown error";
    spinner.fail(`Failed to start watcher: ${message}`);
    process.exitCode = 1;
  }
}

export const watchWs = new Command("watch")
  .option(
    "-d, --dry-run",
    "Dry run the watch process (no actual file syncing)",
    false,
  )
  .option(
    "-i, --incremental",
    "Skip initial sync, only watch for file changes (assumes already indexed)",
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
  .description("Watch for file changes and sync to Rice Search")
  .action(async (_args, cmd) => {
    const options: WatchWsOptions = cmd.optsWithGlobals();
    await startWatchWs(options);
  });
