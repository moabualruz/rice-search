// ============================================================================
// Search Types
// ============================================================================

export interface SearchFilters {
  path_prefix?: string;
  languages?: string[];
}

export interface SearchRequest {
  query: string;
  top_k?: number;
  filters?: SearchFilters;
  include_content?: boolean;
  enable_reranking?: boolean;
  rerank_candidates?: number;
  sparse_weight?: number;
  dense_weight?: number;
  enable_dedup?: boolean;
  dedup_threshold?: number;
  enable_diversity?: boolean;
  diversity_lambda?: number;
  group_by_file?: boolean;
  max_chunks_per_file?: number;
  enable_expansion?: boolean;
}

export interface AggregationInfo {
  is_representative: boolean;
  related_chunks: number;
  file_score: number;
  chunk_rank_in_file: number;
}

export interface SearchResult {
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

export interface IntelligenceInfo {
  intent: 'navigational' | 'factual' | 'exploratory' | 'analytical';
  difficulty: 'easy' | 'medium' | 'hard';
  strategy: 'sparse-only' | 'balanced' | 'dense-heavy' | 'deep-rerank';
  confidence: number;
}

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

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  total: number;
  store: string;
  search_time_ms: number;
  intelligence?: IntelligenceInfo;
  reranking?: RerankingInfo;
  postrank?: PostrankInfo;
}

// ============================================================================
// Store Types
// ============================================================================

export interface StoreInfo {
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  doc_count?: number;
}

export interface StoreStats {
  store: string;
  sparse_index: {
    doc_count: number;
    segment_count: number;
  };
  dense_index: {
    doc_count: number;
    exists: boolean;
  };
  last_updated: string;
}

export interface IndexStats {
  tracked_files: number;
  total_size: number;
  last_updated: string;
}

// ============================================================================
// File Types
// ============================================================================

export interface TrackedFile {
  path: string;
  size: number;
  hash: string;
  indexed_at: string;
  chunk_count: number;
  language?: string;
}

export interface ListFilesResponse {
  files: TrackedFile[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

// ============================================================================
// Observability Types
// ============================================================================

export interface TelemetryStats {
  totalQueries: number;
  avgLatencyMs: number;
  cacheHitRate: number;
  rerankSkipRate: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
}

export interface ObservabilityStats {
  telemetry: TelemetryStats;
  strategies: Record<string, number>;
  intents: Record<string, number>;
  judgments: {
    totalQueries: number;
    totalJudgments: number;
    avgJudgmentsPerQuery: number;
  };
}

export interface QueryLogEntry {
  timestamp: string;
  query: string;
  intent: string;
  strategy: string;
  resultCount: number;
  latencyMs: number;
}

export interface QueryStats {
  store: string;
  period: {
    since: string;
    until: string;
  };
  totalQueries: number;
  uniqueQueries: number;
  avgLatencyMs: number;
  avgResultCount: number;
  intentDistribution: Record<string, number>;
  strategyDistribution: Record<string, number>;
}

export interface TelemetryRecord {
  requestId: string;
  timestamp: string;
  store: string;
  query: string;
  intent: string;
  strategy: string;
  resultCount: number;
  totalLatencyMs: number;
  sparse: {
    count: number;
    latencyMs: number;
  };
  dense: {
    count: number;
    latencyMs: number;
  };
  rerank: {
    enabled: boolean;
    skipped: boolean;
    latencyMs: number;
  };
}

export interface RecentQueriesResponse {
  store: string;
  count: number;
  queries: QueryLogEntry[];
}

export interface TelemetryResponse {
  count: number;
  records: TelemetryRecord[];
}

// ============================================================================
// UI Types
// ============================================================================

export type SortField = 'path' | 'size' | 'indexed_at';
export type SortOrder = 'asc' | 'desc';
