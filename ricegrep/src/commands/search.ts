import { join, normalize } from "node:path";
import type { Command } from "commander";
import { Command as CommanderCommand, InvalidArgumentError } from "commander";
import {
  type CliConfigOptions,
  loadConfig,
  type RicegrepConfig,
} from "../lib/config.js";
import { createFileSystem, createStore } from "../lib/context.js";
import { DEFAULT_IGNORE_PATTERNS } from "../lib/file.js";
import type {
  AskResponse,
  ChunkType,
  FileMetadata,
  SearchOptions,
  SearchResponse,
  Store,
} from "../lib/store.js";
import {
  createIndexingSpinner,
  formatDryRunSummary,
} from "../lib/sync-helpers.js";
import {
  initialSync,
  isAtOrAboveHomeDirectory,
  QuotaExceededError,
} from "../lib/utils.js";

function extractSources(response: AskResponse): { [key: number]: ChunkType } {
  const sources: { [key: number]: ChunkType } = {};
  const answer = response.answer;

  // Match ALL cite tags and capture the i="..."
  const citeTags = answer.match(/<cite i="(\d+(?:-\d+)?)"/g) ?? [];

  for (const tag of citeTags) {
    // Extract the index or index range inside the tag.
    const index = tag.match(/i="(\d+(?:-\d+)?)"/)?.[1];
    if (!index) continue;

    // Case 1: Single index
    if (!index.includes("-")) {
      const idx = Number(index);
      if (!Number.isNaN(idx) && idx < response.sources.length) {
        sources[idx] = response.sources[idx];
      }
      continue;
    }

    // Case 2: Range "start-end"
    const [start, end] = index.split("-").map(Number);

    if (
      !Number.isNaN(start) &&
      !Number.isNaN(end) &&
      start >= 0 &&
      end >= start &&
      end < response.sources.length
    ) {
      for (let i = start; i <= end; i++) {
        sources[i] = response.sources[i];
      }
    }
  }

  return sources;
}

function formatAskResponse(response: AskResponse, show_content: boolean) {
  const sources = extractSources(response);
  const sourceEntries = Object.entries(sources).map(
    ([index, chunk]) => `${index}: ${formatChunk(chunk, show_content)}`,
  );
  return `${response.answer}\n\n${sourceEntries.join("\n")}`;
}

function formatSearchResponse(response: SearchResponse, show_content: boolean) {
  return response.data
    .map((chunk) => formatChunk(chunk, show_content))
    .join("\n");
}

/**
 * Format intelligence and stats for verbose output
 */
function formatIntelligenceStats(response: SearchResponse): string {
  const lines: string[] = [];

  if (response.intelligence) {
    const intel = response.intelligence;
    lines.push(
      `[Intelligence] intent=${intel.intent} difficulty=${intel.difficulty} strategy=${intel.strategy} confidence=${(intel.confidence * 100).toFixed(0)}%`,
    );
  }

  if (response.reranking) {
    const rr = response.reranking;
    const earlyExit = rr.early_exit
      ? ` (early exit: ${rr.early_exit_reason})`
      : "";
    lines.push(
      `[Reranking] enabled=${rr.enabled} pass1=${rr.pass1_latency_ms}ms pass2=${rr.pass2_latency_ms}ms${earlyExit}`,
    );
  }

  if (response.postrank) {
    const pr = response.postrank;
    const parts: string[] = [];
    if (pr.dedup) {
      parts.push(`dedup=${pr.dedup.removed} removed`);
    }
    if (pr.diversity) {
      parts.push(`diversity=${(pr.diversity.avg_diversity * 100).toFixed(0)}%`);
    }
    if (pr.aggregation && pr.aggregation.unique_files > 0) {
      parts.push(`files=${pr.aggregation.unique_files}`);
    }
    if (parts.length > 0) {
      lines.push(`[PostRank] ${parts.join(" | ")} (${pr.total_latency_ms}ms)`);
    }
  }

  return lines.join("\n");
}

function isWebResult(chunk: ChunkType): boolean {
  return (
    "filename" in chunk &&
    typeof chunk.filename === "string" &&
    chunk.filename.startsWith("http")
  );
}

function formatChunk(chunk: ChunkType, show_content: boolean) {
  const pwd = process.cwd();

  if (isWebResult(chunk) && chunk.type === "text") {
    const url = "filename" in chunk ? chunk.filename : "Unknown URL";
    const content = show_content ? chunk.text : "";
    return `${url} (${(chunk.score * 100).toFixed(2)}% match)${content ? `\n${content}` : ""}`;
  }

  const path =
    (chunk.metadata as FileMetadata)?.path?.replace(pwd, "") ?? "Unknown path";
  let line_range = "";
  let content = "";
  
  // Rice Search only handles text chunks (code files)
  const start_line = (chunk.generated_metadata?.start_line ?? 0) + 1;
  const end_line = start_line + (chunk.generated_metadata?.num_lines ?? 1);
  line_range = `:${start_line}-${end_line}`;
  content = show_content ? chunk.text : "";

  return `.${path}${line_range} (${(chunk.score * 100).toFixed(2)}% match)${content ? `\n${content}` : ""}`;
}

function parseBooleanEnv(
  envVar: string | undefined,
  defaultValue: boolean,
): boolean {
  if (envVar === undefined) return defaultValue;
  const lower = envVar.toLowerCase();
  return lower === "1" || lower === "true" || lower === "yes" || lower === "y";
}

/**
 * Syncs local files to the store with progress indication.
 * @returns true if the caller should return early (dry-run mode), false otherwise
 */
async function syncFiles(
  store: Store,
  storeName: string,
  root: string,
  dryRun: boolean,
  config?: RicegrepConfig,
): Promise<boolean> {
  const { spinner, onProgress } = createIndexingSpinner(root);

  try {
    const fileSystem = createFileSystem({
      ignorePatterns: [...DEFAULT_IGNORE_PATTERNS],
    });
    const result = await initialSync(
      store,
      fileSystem,
      storeName,
      root,
      dryRun,
      onProgress,
      config,
    );

    while (true) {
      const info = await store.getInfo(storeName);
      spinner.text = `Indexing ${info.counts.pending + info.counts.in_progress} file(s)`;
      if (info.counts.pending === 0 && info.counts.in_progress === 0) {
        break;
      }
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }

    spinner.succeed("Indexing complete");

    if (dryRun) {
      console.log(
        formatDryRunSummary(result, {
          actionDescription: "would have indexed",
        }),
      );
      return true;
    }

    return false;
  } catch (error) {
    spinner.stop();
    throw error;
  }
}

export const search: Command = new CommanderCommand("search")
  .description("File pattern searcher")
  .option("-i", "Makes the search case-insensitive", false)
  .option("-r", "Recursive search", false)
  .option(
    "-m, --max-count <max_count>",
    "The maximum number of results to return",
    process.env.RICEGREP_MAX_COUNT || "10",
  )
  .option(
    "-c, --content",
    "Show content of the results",
    parseBooleanEnv(process.env.RICEGREP_CONTENT, false),
  )
  .option(
    "-a, --answer",
    "Generate an answer to the question based on the results",
    parseBooleanEnv(process.env.RICEGREP_ANSWER, false),
  )
  .option(
    "-s, --sync",
    "Syncs the local files to the store before searching",
    parseBooleanEnv(process.env.RICEGREP_SYNC, false),
  )
  .option(
    "-d, --dry-run",
    "Dry run the search process (no actual file syncing)",
    parseBooleanEnv(process.env.RICEGREP_DRY_RUN, false),
  )
  .option(
    "--no-rerank",
    "Disable reranking of search results",
    parseBooleanEnv(process.env.RICEGREP_RERANK, true), // `true` here means that reranking is enabled by default
  )
  .option(
    "--no-dedup",
    "Disable semantic deduplication of results",
    parseBooleanEnv(process.env.RICEGREP_DEDUP, true),
  )
  .option(
    "--no-diversity",
    "Disable MMR diversity in results",
    parseBooleanEnv(process.env.RICEGREP_DIVERSITY, true),
  )
  .option(
    "--no-expansion",
    "Disable query expansion",
    parseBooleanEnv(process.env.RICEGREP_EXPANSION, true),
  )
  .option(
    "--group-by-file",
    "Group results by file",
    parseBooleanEnv(process.env.RICEGREP_GROUP_BY_FILE, false),
  )
  .option(
    "-v, --verbose",
    "Show detailed search stats (intelligence, timing, dedup)",
    parseBooleanEnv(process.env.RICEGREP_VERBOSE, false),
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
  .option(
    "-w, --web",
    "Include web search results",
    parseBooleanEnv(process.env.RICEGREP_WEB, false),
  )
  .argument("<pattern>", "The pattern to search for")
  .argument("[path]", "The path to search in")
  .allowUnknownOption(true)
  .allowExcessArguments(true)
  .action(async (pattern, exec_path, _options, cmd) => {
    const options: {
      store: string;
      maxCount: string;
      content: boolean;
      answer: boolean;
      sync: boolean;
      dryRun: boolean;
      rerank: boolean;
      dedup: boolean;
      diversity: boolean;
      expansion: boolean;
      groupByFile: boolean;
      verbose: boolean;
      maxFileSize?: number;
      maxFileCount?: number;
      web: boolean;
    } = cmd.optsWithGlobals();
    if (exec_path?.startsWith("--")) {
      exec_path = "";
    }

    const root = process.cwd();
    const cliOptions: CliConfigOptions = {
      maxFileSize: options.maxFileSize,
      maxFileCount: options.maxFileCount,
    };
    const config = loadConfig(root, cliOptions);

    const search_path = exec_path?.startsWith("/")
      ? exec_path
      : normalize(join(root, exec_path ?? ""));

    if (options.sync && isAtOrAboveHomeDirectory(search_path)) {
      console.error(
        "Error: Cannot sync home directory or any parent directory.",
      );
      console.error(
        "Please run this command from within a specific project subdirectory.",
      );
      process.exitCode = 1;
      return;
    }

    try {
      // Only show backend URL when syncing (indexing), not for pure search
      const store = await createStore({ silent: !options.sync });

      if (options.sync) {
        const shouldReturn = await syncFiles(
          store,
          options.store,
          search_path,
          options.dryRun,
          config,
        );
        if (shouldReturn) {
          return;
        }
      }

      const storeIds = [options.store];

      const filters = {
        all: [
          {
            key: "path",
            operator: "starts_with" as const,
            value: search_path,
          },
        ],
      };

      // Build SearchOptions from CLI flags
      // ricegrep is a thin client - server makes all retrieval decisions
      const searchOptions: SearchOptions = {
        rerank: options.rerank,
        enableDedup: options.dedup,
        enableDiversity: options.diversity,
        enableExpansion: options.expansion,
        groupByFile: options.groupByFile,
        includeContent: true,
      };

      let response: string;
      let results: SearchResponse;

      if (!options.answer) {
        results = await store.search(
          storeIds,
          pattern,
          parseInt(options.maxCount, 10),
          searchOptions,
          filters,
        );
        response = formatSearchResponse(results, options.content);
      } else {
        const askResults = await store.ask(
          storeIds,
          pattern,
          parseInt(options.maxCount, 10),
          searchOptions,
          filters,
        );
        response = formatAskResponse(askResults, options.content);
        results = { data: askResults.sources };
      }

      // Show verbose stats if requested
      if (options.verbose && results.intelligence) {
        console.log(formatIntelligenceStats(results));
        console.log("");
      }

      console.log(response);

      // Show summary line with timing
      if (results.search_time_ms !== undefined) {
        const dedupInfo = results.postrank?.dedup
          ? ` (${results.postrank.dedup.removed} duplicates removed)`
          : "";
        console.log(
          `\n${results.total ?? results.data.length} results in ${results.search_time_ms}ms${dedupInfo}`,
        );
      }
    } catch (error) {
      if (error instanceof QuotaExceededError) {
        console.error(
          "Storage limit exceeded. Please free up space or adjust your file count limits.\n",
        );
      }
      process.exitCode = 1;
    }
  });
