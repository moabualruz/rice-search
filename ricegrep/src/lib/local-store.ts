/**
 * LocalStore - Store implementation for Rice Search backend
 *
 * This store connects to a local Rice Search API.
 * Configure with RICEGREP_BASE_URL (defaults to http://localhost:8088)
 */


import type {
  AskResponse,
  ChunkType,
  CreateStoreOptions,
  IntelligenceInfo,
  ListFilesOptions,
  PostrankInfo,
  RerankingInfo,
  SearchFilter,
  SearchOptions,
  SearchResponse,
  Store,
  StoreFile,
  StoreInfo,
  UploadFileOptions,
} from "./store.js";
import debug from "debug";

const log = debug("ricegrep:local-store");

export const LOCAL_API_URL =
  process.env.RICEGREP_BASE_URL || "http://localhost:8088";

export function isLocalMode(): boolean {
  // Rice Search is always local - no cloud mode
  return true;
}

interface LocalSearchResult {
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
  aggregation?: {
    is_representative: boolean;
    related_chunks: number;
    file_score: number;
    chunk_rank_in_file: number;
  };
}

interface LocalSearchResponse {
  query: string;
  results: LocalSearchResult[];
  total: number;
  store: string;
  search_time_ms: number;
  // Phase 1-5 metadata from server
  intelligence?: IntelligenceInfo;
  reranking?: RerankingInfo;
  postrank?: PostrankInfo;
}

interface LocalIndexResponse {
  files_processed: number;
  chunks_indexed: number;
  time_ms: number;
  errors?: string[];
}

interface LocalStoreInfo {
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}



/**
 * Local backend implementation of the Store interface
 */
export class LocalStore implements Store {
  private baseUrl: string;

  constructor(baseUrl: string = LOCAL_API_URL) {
    this.baseUrl = baseUrl.replace(/\/$/, ""); // Remove trailing slash
    log("LocalStore initialized with baseUrl:", this.baseUrl);
  }

  private async fetch<T>(
    path: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    log("Fetching:", url, options.method || "GET");

    const response = await fetch(url, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`HTTP ${response.status}: ${errorText}`);
    }

    return response.json() as Promise<T>;
  }

  async *listFiles(
    _storeId: string,
    _options?: ListFilesOptions
  ): AsyncGenerator<StoreFile> {
    // Local API doesn't have a file listing endpoint yet
    // This would need to be implemented in unified-api
    // For now, return empty - the sync will just re-upload everything
    log("listFiles not fully implemented for local mode");
    return;
  }

  async uploadFile(
    storeId: string,
    file: File | ReadableStream,
    options: UploadFileOptions
  ): Promise<void> {
    // Read file content
    let content: string;
    if (file instanceof File) {
      content = await file.text();
    } else if ("getReader" in file) {
      const reader = (file as ReadableStream).getReader();
      const decoder = new TextDecoder();
      let result = "";
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        result += decoder.decode(value, { stream: true });
      }
      result += decoder.decode();
      content = result;
    } else {
      throw new Error("Unsupported file type");
    }

    // Index the file via unified-api
    await this.fetch<LocalIndexResponse>(`/v1/stores/${storeId}/index`, {
      method: "POST",
      body: JSON.stringify({
        files: [
          {
            path: options.metadata?.path || options.external_id,
            content,
          },
        ],
        force: options.overwrite,
      }),
    });

    log("Uploaded file:", options.external_id);
  }

  async deleteFile(storeId: string, externalId: string): Promise<void> {
    await this.fetch(`/v1/stores/${storeId}/index`, {
      method: "DELETE",
      body: JSON.stringify({
        paths: [externalId],
      }),
    });
    log("Deleted file:", externalId);
  }

  async search(
    storeIds: string[],
    query: string,
    top_k?: number,
    search_options?: SearchOptions,
    filters?: SearchFilter
  ): Promise<SearchResponse> {
    // Use first store (local API doesn't support multi-store search yet)
    const storeId = storeIds[0] || "default";

    // Extract path prefix from filters if present
    let pathPrefix: string | undefined;
    if (filters?.all) {
      const pathFilter = filters.all.find(
        (f) => "key" in f && f.key === "path" && f.operator === "starts_with"
      );
      if (pathFilter && "value" in pathFilter) {
        pathPrefix = pathFilter.value as string;
      }
    }

    // Build request with all Phase 1-5 options
    // ricegrep is a thin client - server makes all decisions
    const requestBody: Record<string, unknown> = {
      query,
      top_k: top_k || 10,
      include_content: search_options?.includeContent ?? true,
      // Reranking (Phase 1)
      enable_reranking: search_options?.rerank ?? true,
      // Retrieval weights (Phase 1 - server may adjust based on query analysis)
      sparse_weight: search_options?.sparseWeight,
      dense_weight: search_options?.denseWeight,
      // Post-processing (Phase 2)
      enable_dedup: search_options?.enableDedup,
      dedup_threshold: search_options?.dedupThreshold,
      enable_diversity: search_options?.enableDiversity,
      diversity_lambda: search_options?.diversityLambda,
      group_by_file: search_options?.groupByFile,
      max_chunks_per_file: search_options?.maxChunksPerFile,
      // Query processing (Phase 5)
      enable_expansion: search_options?.enableExpansion,
      // Filters
      filters: pathPrefix ? { path_prefix: pathPrefix } : undefined,
    };

    // Remove undefined values to use server defaults
    for (const key of Object.keys(requestBody)) {
      if (requestBody[key] === undefined) {
        delete requestBody[key];
      }
    }

    const response = await this.fetch<LocalSearchResponse>(
      `/v1/stores/${storeId}/search`,
      {
        method: "POST",
        body: JSON.stringify(requestBody),
      }
    );

    // Convert local format to Rice Search format
    const chunks: ChunkType[] = response.results.map((result, index) => ({
      type: "text" as const,
      text: result.content || "",
      score: result.final_score,
      chunk_index: index,
      metadata: {
        path: result.path,
        hash: "",
      },
      generated_metadata: {
        start_line: result.start_line - 1, // Convert 1-indexed to 0-indexed
        num_lines: result.end_line - result.start_line + 1,
      },
    })) as unknown as ChunkType[];

    // Return full response including server metadata
    return {
      data: chunks,
      query: response.query,
      total: response.total,
      store: response.store,
      search_time_ms: response.search_time_ms,
      intelligence: response.intelligence,
      reranking: response.reranking,
      postrank: response.postrank,
    };
  }

  async retrieve(storeId: string): Promise<unknown> {
    return this.fetch<LocalStoreInfo>(`/v1/stores/${storeId}`);
  }

  async create(options: CreateStoreOptions): Promise<unknown> {
    return this.fetch(`/v1/stores`, {
      method: "POST",
      body: JSON.stringify({
        name: options.name,
        description: options.description,
      }),
    });
  }

  async ask(
    storeIds: string[],
    question: string,
    top_k?: number,
    search_options?: SearchOptions,
    filters?: SearchFilter
  ): Promise<AskResponse> {
    // Local API doesn't have RAG/ask endpoint yet
    // Fall back to search and return results without an answer
    const searchRes = await this.search(
      storeIds,
      question,
      top_k,
      search_options,
      filters
    );

    return {
      answer:
        "Note: Answer generation is not available in local mode. See search results below.",
      sources: searchRes.data,
    };
  }

  async getInfo(storeId: string): Promise<StoreInfo> {
    try {
      const info = await this.fetch<LocalStoreInfo>(`/v1/stores/${storeId}`);
      return {
        name: info.name,
        description: info.description || "",
        created_at: info.created_at,
        updated_at: info.updated_at,
        counts: {
          pending: 0, // Local mode processes synchronously
          in_progress: 0,
        },
      };
    } catch (error) {
      // Store might not exist yet
      return {
        name: storeId,
        description: "",
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
        counts: { pending: 0, in_progress: 0 },
      };
    }
  }

  async refreshClient(): Promise<void> {
    // No-op for local mode (no auth tokens to refresh)
  }
}
