import * as fs from "node:fs/promises";

export interface FileMetadata {
  path: string;
  hash: string;
}

// Local types for Rice Search
export interface ScoredTextInputChunk {
  type: "text";
  text: string;
  score: number;
  metadata?: FileMetadata;
  chunk_index?: number;
  generated_metadata?: {
    start_line?: number;
    num_lines?: number;
  };
}

export interface SearchFilter {
  all?: Array<{
    key: string;
    operator: "starts_with" | "equals";
    value: string;
  }>;
}

export type ChunkType = ScoredTextInputChunk;

export interface StoreFile {
  external_id: string | null;
  metadata: FileMetadata | null;
}

export interface UploadFileOptions {
  external_id: string;
  overwrite?: boolean;
  metadata?: FileMetadata;
}

// ============================================================================
// Search Options - All decisions made by server
// ============================================================================

/**
 * Search options passed to the server.
 * ricegrep is a thin client - the server makes all retrieval decisions.
 */
export interface SearchOptions {
  // Basic options
  rerank?: boolean;           // Enable neural reranking (default: true)
  includeContent?: boolean;   // Include code content in results (default: true)

  // Retrieval weights (server uses these as hints, may adjust based on query)
  sparseWeight?: number;      // BM25 weight 0-1 (default: 0.5)
  denseWeight?: number;       // Semantic weight 0-1 (default: 0.5)

  // Post-processing (server applies these after retrieval)
  enableDedup?: boolean;      // Semantic deduplication (default: true)
  dedupThreshold?: number;    // Similarity threshold 0-1 (default: 0.85)
  enableDiversity?: boolean;  // MMR diversity (default: true)
  diversityLambda?: number;   // 0=diverse, 1=relevant (default: 0.7)
  groupByFile?: boolean;      // Group results by file (default: false)
  maxChunksPerFile?: number;  // Max chunks per file when grouping (default: 3)

  // Query processing (server handles expansion)
  enableExpansion?: boolean;  // Query expansion (default: true)
}

// ============================================================================
// Search Response - Full metadata from server
// ============================================================================

/**
 * Intelligence info returned by server (Phase 1)
 */
export interface IntelligenceInfo {
  intent: "navigational" | "factual" | "exploratory" | "analytical";
  difficulty: "easy" | "medium" | "hard";
  strategy: "sparse-only" | "balanced" | "dense-heavy" | "deep-rerank";
  confidence: number;
}

/**
 * Reranking stats returned by server (Phase 1)
 */
export interface RerankingInfo {
  enabled: boolean;
  candidates: number;
  pass1_applied: boolean;
  pass1_latency_ms: number;
  pass2_applied: boolean;
  pass2_latency_ms: number;
  early_exit: boolean;
  early_exit_reason?: string;
}

/**
 * PostRank stats returned by server (Phase 2)
 */
export interface PostrankInfo {
  dedup: {
    input_count: number;
    output_count: number;
    removed: number;
    latency_ms: number;
  };
  diversity: {
    enabled: boolean;
    avg_diversity: number;
    latency_ms: number;
  };
  aggregation: {
    unique_files: number;
    chunks_dropped: number;
  };
  total_latency_ms: number;
}

/**
 * Aggregation info for grouped results (Phase 2)
 */
export interface AggregationInfo {
  is_representative: boolean;
  related_chunks: number;
  file_score: number;
  chunk_rank_in_file: number;
}

/**
 * Individual search result
 */
export interface SearchResultItem {
  doc_id: string;
  path: string;
  language: string;
  start_line: number;
  end_line: number;
  content?: string;
  symbols: string[];
  final_score: number;
  sparse_score?: number;
  dense_score?: number;
  sparse_rank?: number;
  dense_rank?: number;
  aggregation?: AggregationInfo;
}

/**
 * Full search response from server
 */
export interface SearchResponse {
  data: ChunkType[];
  // Server metadata (available in full response)
  query?: string;
  total?: number;
  store?: string;
  search_time_ms?: number;
  intelligence?: IntelligenceInfo;
  reranking?: RerankingInfo;
  postrank?: PostrankInfo;
}

export interface AskResponse {
  answer: string;
  sources: ChunkType[];
}

export interface CreateStoreOptions {
  name: string;
  description?: string;
}

export interface StoreInfo {
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  counts: {
    pending: number;
    in_progress: number;
  };
}

/**
 * Interface for store operations
 */
export interface ListFilesOptions {
  pathPrefix?: string;
}

export interface Store {
  /**
   * List files in a store as an async iterator
   *
   * @param storeId - The ID of the store
   * @param options - Optional filtering options
   * @param options.pathPrefix - Only return files whose path starts with this prefix
   */
  listFiles(
    storeId: string,
    options?: ListFilesOptions,
  ): AsyncGenerator<StoreFile>;

  /**
   * Upload a file to a store
   */
  uploadFile(
    storeId: string,
    file: File | ReadableStream,
    options: UploadFileOptions,
  ): Promise<void>;

  /**
   * Delete a file from a store by its external ID
   */
  deleteFile(storeId: string, externalId: string): Promise<void>;

  /**
   * Search in one or more stores.
   * All retrieval decisions are made by the server.
   */
  search(
    storeIds: string[],
    query: string,
    top_k?: number,
    search_options?: SearchOptions,
    filters?: SearchFilter,
  ): Promise<SearchResponse>;

  /**
   * Retrieve store information
   */
  retrieve(storeId: string): Promise<unknown>;

  /**
   * Create a new store
   */
  create(options: CreateStoreOptions): Promise<unknown>;

  /**
   * Ask a question to one or more stores
   */
  ask(
    storeIds: string[],
    question: string,
    top_k?: number,
    search_options?: SearchOptions,
    filters?: SearchFilter,
  ): Promise<AskResponse>;

  /**
   * Get store information
   */
  getInfo(storeId: string): Promise<StoreInfo>;

  /**
   * Refresh the client with a new JWT token (optional, for long-running sessions)
   */
  refreshClient?(): Promise<void>;
}



interface TestStoreDB {
  info: StoreInfo;
  files: Record<
    string,
    {
      metadata: FileMetadata;
      content: string;
    }
  >;
}

export class TestStore implements Store {
  path: string;
  private mutex: Promise<void> = Promise.resolve();

  constructor() {
    const path = process.env.RICEGREP_TEST_STORE_PATH;
    if (!path) {
      throw new Error("RICEGREP_TEST_STORE_PATH is not set");
    }
    this.path = path;
  }

  private async synchronized<T>(fn: () => Promise<T>): Promise<T> {
    let unlock: () => void = () => {};
    const newLock = new Promise<void>((resolve) => {
      unlock = resolve;
    });

    const previousLock = this.mutex;
    this.mutex = newLock;

    await previousLock;

    try {
      return await fn();
    } finally {
      unlock();
    }
  }

  private async load(): Promise<TestStoreDB> {
    try {
      const content = await fs.readFile(this.path, "utf-8");
      return JSON.parse(content);
    } catch {
      return {
        info: {
          name: "Test Store",
          description: "A test store",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          counts: { pending: 0, in_progress: 0 },
        },
        files: {},
      };
    }
  }

  private async save(data: TestStoreDB): Promise<void> {
    await fs.writeFile(this.path, JSON.stringify(data, null, 2));
  }

  private async readContent(file: File | ReadableStream): Promise<string> {
    if (
      "text" in file &&
      typeof (file as { text: unknown }).text === "function"
    ) {
      return await (file as File).text();
    }

    const chunks: Buffer[] = [];
    if (
      typeof (file as unknown as AsyncIterable<unknown>)[
        Symbol.asyncIterator
      ] === "function"
    ) {
      for await (const chunk of file as unknown as AsyncIterable<
        Uint8Array | string
      >) {
        chunks.push(Buffer.from(chunk));
      }
      return Buffer.concat(chunks).toString("utf-8");
    }

    if ("getReader" in file) {
      const reader = (file as ReadableStream).getReader();
      const decoder = new TextDecoder();
      let res = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        res += decoder.decode(value, { stream: true });
      }
      res += decoder.decode();
      return res;
    }

    throw new Error("Unknown file type");
  }

  async *listFiles(
    _storeId: string,
    options?: ListFilesOptions,
  ): AsyncGenerator<StoreFile> {
    const db = await this.load();
    for (const [external_id, file] of Object.entries(db.files)) {
      if (
        options?.pathPrefix &&
        file.metadata?.path &&
        !file.metadata.path.startsWith(options.pathPrefix)
      ) {
        continue;
      }
      yield {
        external_id,
        metadata: file.metadata,
      };
    }
  }

  async uploadFile(
    _storeId: string,
    file: File | ReadableStream,
    options: UploadFileOptions,
  ): Promise<void> {
    const content = await this.readContent(file);
    await this.synchronized(async () => {
      const db = await this.load();
      db.files[options.external_id] = {
        metadata: options.metadata || { path: options.external_id, hash: "" },
        content,
      };
      await this.save(db);
    });
  }

  async deleteFile(_storeId: string, externalId: string): Promise<void> {
    await this.synchronized(async () => {
      const db = await this.load();
      delete db.files[externalId];
      await this.save(db);
    });
  }

  async search(
    _storeIds: string[],
    query: string,
    top_k?: number,
    search_options?: SearchOptions,
    filters?: SearchFilter,
  ): Promise<SearchResponse> {
    const db = await this.load();
    const results: ChunkType[] = [];
    const limit = top_k || 10;

    for (const file of Object.values(db.files)) {
      if (filters?.all) {
        const pathFilter = filters.all.find(
          (f) => "key" in f && f.key === "path" && f.operator === "starts_with",
        );
        if (
          pathFilter &&
          "value" in pathFilter &&
          file.metadata &&
          !file.metadata.path.startsWith(pathFilter.value as string)
        ) {
          continue;
        }
      }

      const lines = file.content.split("\n");
      for (let i = 0; i < lines.length; i++) {
        if (lines[i].toLowerCase().includes(query.toLowerCase())) {
          results.push({
            type: "text",
            text:
              lines[i] + (search_options?.rerank ? "" : " without reranking"),
            score: 1.0,
            metadata: file.metadata,
            chunk_index: results.length - 1,
            generated_metadata: {
              start_line: i,
              num_lines: 1,
            },
          } as unknown as ChunkType);
          if (results.length >= limit) break;
        }
      }
      if (results.length >= limit) break;
    }

    return { data: results };
  }

  async retrieve(_storeId: string): Promise<unknown> {
    const db = await this.load();
    return db.info;
  }

  async create(options: CreateStoreOptions): Promise<unknown> {
    return await this.synchronized(async () => {
      const db = await this.load();
      db.info.name = options.name;
      db.info.description = options.description || "";
      await this.save(db);
      return db.info;
    });
  }

  async ask(
    storeIds: string[],
    question: string,
    top_k?: number,
    search_options?: SearchOptions,
    filters?: SearchFilter,
  ): Promise<AskResponse> {
    const searchRes = await this.search(
      storeIds,
      question,
      top_k,
      search_options,
      filters,
    );
    return {
      answer: 'This is a mock answer from TestStore.<cite i="0" />',
      sources: searchRes.data,
    };
  }

  async getInfo(_storeId: string): Promise<StoreInfo> {
    const db = await this.load();
    return db.info;
  }
}
