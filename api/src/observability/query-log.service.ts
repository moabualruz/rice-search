import { Injectable, Logger, OnModuleInit, OnModuleDestroy } from "@nestjs/common";
import { ConfigService } from "@nestjs/config";
import * as fs from "node:fs";
import * as path from "node:path";

/**
 * Query log entry for replay and analysis
 */
export interface QueryLogEntry {
  timestamp: string;
  requestId: string;
  store: string;
  storeVersion?: string;
  query: string;
  normalizedQuery: string;
  intent: string;
  difficulty: string;
  strategy: string;
  topK: number;
  resultCount: number;
  totalLatencyMs: number;
  topResultPath?: string;
  topResultScore?: number;
  filters?: {
    pathPrefix?: string;
    languages?: string[];
  };
  options?: {
    enableReranking?: boolean;
    enableDedup?: boolean;
    enableDiversity?: boolean;
    groupByFile?: boolean;
  };
}

/**
 * QueryLogService persists search queries to JSONL files for replay and evaluation.
 *
 * Features:
 * - JSONL format (one JSON object per line) for easy streaming and analysis
 * - Daily rotation (new file per day)
 * - Async writes with buffering for performance
 * - File size limits to prevent disk exhaustion
 *
 * Log files are stored in:
 *   {DATA_DIR}/query-logs/{store}/{YYYY-MM-DD}.jsonl
 */
@Injectable()
export class QueryLogService implements OnModuleInit, OnModuleDestroy {
  private readonly logger = new Logger(QueryLogService.name);
  private readonly logDir: string;
  private readonly maxFileSizeMb: number;
  private readonly flushIntervalMs: number;

  // Write buffer for performance
  private writeBuffer: Map<string, string[]> = new Map();
  private flushTimer: NodeJS.Timeout | null = null;

  constructor(private configService: ConfigService) {
    const dataDir = this.configService.get<string>("DATA_DIR") ?? "/data";
    this.logDir = path.join(dataDir, "query-logs");
    this.maxFileSizeMb = this.configService.get<number>("QUERY_LOG_MAX_SIZE_MB") ?? 100;
    this.flushIntervalMs = this.configService.get<number>("QUERY_LOG_FLUSH_INTERVAL_MS") ?? 5000;
  }

  onModuleInit(): void {
    // Ensure log directory exists
    this.ensureLogDir();

    // Start periodic flush
    this.flushTimer = setInterval(() => {
      this.flushAll();
    }, this.flushIntervalMs);

    this.logger.log(`Query logging enabled: ${this.logDir}`);
  }

  onModuleDestroy(): void {
    // Flush remaining entries on shutdown
    if (this.flushTimer) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }
    this.flushAllSync();
    this.logger.log("Query log service shutdown, all entries flushed");
  }

  /**
   * Log a search query
   *
   * @param entry - Query log entry
   */
  log(entry: QueryLogEntry): void {
    const key = this.getLogKey(entry.store);
    const line = JSON.stringify(entry) + "\n";

    // Add to buffer
    const buffer = this.writeBuffer.get(key) ?? [];
    buffer.push(line);
    this.writeBuffer.set(key, buffer);

    // Flush immediately if buffer is large
    if (buffer.length >= 100) {
      this.flush(key);
    }
  }

  /**
   * Read query log entries for a store and date range
   *
   * @param store - Store name
   * @param since - Start date
   * @param until - End date (optional, defaults to today)
   * @returns Array of query log entries
   */
  async readLogs(
    store: string,
    since: Date,
    until?: Date,
  ): Promise<QueryLogEntry[]> {
    const endDate = until ?? new Date();
    const entries: QueryLogEntry[] = [];

    // Iterate over date range
    const currentDate = new Date(since);
    while (currentDate <= endDate) {
      const filePath = this.getLogFilePath(store, currentDate);

      if (fs.existsSync(filePath)) {
        const content = await fs.promises.readFile(filePath, "utf-8");
        const lines = content.split("\n").filter((line) => line.trim());

        for (const line of lines) {
          try {
            const entry = JSON.parse(line) as QueryLogEntry;
            const entryDate = new Date(entry.timestamp);

            // Filter by timestamp
            if (entryDate >= since && entryDate <= endDate) {
              entries.push(entry);
            }
          } catch {
            // Skip malformed lines
          }
        }
      }

      // Move to next day
      currentDate.setDate(currentDate.getDate() + 1);
    }

    return entries;
  }

  /**
   * Get recent queries for a store (last N entries)
   *
   * @param store - Store name
   * @param limit - Maximum entries to return
   * @returns Recent query log entries
   */
  async getRecentQueries(store: string, limit = 100): Promise<QueryLogEntry[]> {
    // Read today's log
    const today = new Date();
    const entries = await this.readLogs(store, today, today);

    // Return last N entries
    return entries.slice(-limit);
  }

  /**
   * Get unique queries for evaluation (deduplicated by normalized query)
   *
   * @param store - Store name
   * @param since - Start date
   * @param limit - Maximum unique queries
   * @returns Unique query entries
   */
  async getUniqueQueries(
    store: string,
    since: Date,
    limit = 500,
  ): Promise<QueryLogEntry[]> {
    const allEntries = await this.readLogs(store, since);

    // Deduplicate by normalized query
    const seen = new Set<string>();
    const unique: QueryLogEntry[] = [];

    for (const entry of allEntries) {
      if (!seen.has(entry.normalizedQuery)) {
        seen.add(entry.normalizedQuery);
        unique.push(entry);
        if (unique.length >= limit) {
          break;
        }
      }
    }

    return unique;
  }

  /**
   * Get query statistics for a store
   *
   * @param store - Store name
   * @param since - Start date
   * @returns Query statistics
   */
  async getQueryStats(
    store: string,
    since: Date,
  ): Promise<{
    totalQueries: number;
    uniqueQueries: number;
    avgLatencyMs: number;
    avgResultCount: number;
    intentDistribution: Record<string, number>;
    strategyDistribution: Record<string, number>;
  }> {
    const entries = await this.readLogs(store, since);

    if (entries.length === 0) {
      return {
        totalQueries: 0,
        uniqueQueries: 0,
        avgLatencyMs: 0,
        avgResultCount: 0,
        intentDistribution: {},
        strategyDistribution: {},
      };
    }

    // Calculate stats
    const uniqueQueries = new Set(entries.map((e) => e.normalizedQuery)).size;
    const avgLatencyMs =
      entries.reduce((sum, e) => sum + e.totalLatencyMs, 0) / entries.length;
    const avgResultCount =
      entries.reduce((sum, e) => sum + e.resultCount, 0) / entries.length;

    // Intent distribution
    const intentDistribution: Record<string, number> = {};
    for (const entry of entries) {
      intentDistribution[entry.intent] =
        (intentDistribution[entry.intent] ?? 0) + 1;
    }

    // Strategy distribution
    const strategyDistribution: Record<string, number> = {};
    for (const entry of entries) {
      strategyDistribution[entry.strategy] =
        (strategyDistribution[entry.strategy] ?? 0) + 1;
    }

    return {
      totalQueries: entries.length,
      uniqueQueries,
      avgLatencyMs,
      avgResultCount,
      intentDistribution,
      strategyDistribution,
    };
  }

  /**
   * Export queries for offline evaluation
   *
   * @param store - Store name
   * @param since - Start date
   * @param format - Export format ('jsonl' or 'csv')
   * @returns Exported data as string
   */
  async exportQueries(
    store: string,
    since: Date,
    format: "jsonl" | "csv" = "jsonl",
  ): Promise<string> {
    const entries = await this.readLogs(store, since);

    if (format === "jsonl") {
      return entries.map((e) => JSON.stringify(e)).join("\n");
    }

    // CSV format
    const headers = [
      "timestamp",
      "requestId",
      "query",
      "intent",
      "strategy",
      "resultCount",
      "latencyMs",
    ];
    const rows = entries.map((e) =>
      [
        e.timestamp,
        e.requestId,
        `"${e.query.replace(/"/g, '""')}"`,
        e.intent,
        e.strategy,
        e.resultCount,
        e.totalLatencyMs,
      ].join(","),
    );

    return [headers.join(","), ...rows].join("\n");
  }

  /**
   * Create log key from store name
   */
  private getLogKey(store: string): string {
    const date = new Date().toISOString().split("T")[0];
    return `${store}/${date}`;
  }

  /**
   * Get log file path for store and date
   */
  private getLogFilePath(store: string, date: Date): string {
    const dateStr = date.toISOString().split("T")[0];
    const storeDir = path.join(this.logDir, store);
    return path.join(storeDir, `${dateStr}.jsonl`);
  }

  /**
   * Ensure log directory exists
   */
  private ensureLogDir(): void {
    try {
      fs.mkdirSync(this.logDir, { recursive: true });
    } catch (error) {
      this.logger.error(`Failed to create log directory: ${error}`);
    }
  }

  /**
   * Ensure store directory exists
   */
  private ensureStoreDir(store: string): void {
    const storeDir = path.join(this.logDir, store);
    try {
      fs.mkdirSync(storeDir, { recursive: true });
    } catch {
      // Directory may already exist
    }
  }

  /**
   * Flush buffer for a specific key
   */
  private flush(key: string): void {
    const buffer = this.writeBuffer.get(key);
    if (!buffer || buffer.length === 0) {
      return;
    }

    const [store, date] = key.split("/");
    this.ensureStoreDir(store);

    const filePath = path.join(this.logDir, store, `${date}.jsonl`);
    const content = buffer.join("");

    // Check file size before writing
    try {
      const stats = fs.statSync(filePath);
      if (stats.size > this.maxFileSizeMb * 1024 * 1024) {
        this.logger.warn(`Log file ${filePath} exceeded size limit, rotating`);
        // Rotate by adding timestamp suffix
        const rotatedPath = filePath.replace(".jsonl", `-${Date.now()}.jsonl`);
        fs.renameSync(filePath, rotatedPath);
      }
    } catch {
      // File doesn't exist yet, that's fine
    }

    // Append to file asynchronously
    fs.appendFile(filePath, content, (err) => {
      if (err) {
        this.logger.error(`Failed to write query log: ${err}`);
      }
    });

    // Clear buffer
    this.writeBuffer.delete(key);
  }

  /**
   * Flush all buffers
   */
  private flushAll(): void {
    for (const key of this.writeBuffer.keys()) {
      this.flush(key);
    }
  }

  /**
   * Flush all buffers synchronously (for shutdown)
   */
  private flushAllSync(): void {
    for (const [key, buffer] of this.writeBuffer.entries()) {
      if (buffer.length === 0) {
        continue;
      }

      const [store, date] = key.split("/");
      this.ensureStoreDir(store);

      const filePath = path.join(this.logDir, store, `${date}.jsonl`);
      const content = buffer.join("");

      try {
        fs.appendFileSync(filePath, content);
      } catch (err) {
        this.logger.error(`Failed to flush query log: ${err}`);
      }
    }
    this.writeBuffer.clear();
  }
}
