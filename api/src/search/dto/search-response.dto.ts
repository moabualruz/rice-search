import { ApiProperty, ApiPropertyOptional } from "@nestjs/swagger";

/**
 * Aggregation info for grouped results
 */
export class AggregationInfoDto {
  @ApiProperty({ description: "Whether this is the representative chunk for the file" })
  is_representative: boolean;

  @ApiProperty({ description: "Number of related chunks from the same file" })
  related_chunks: number;

  @ApiProperty({ description: "Aggregated file score" })
  file_score: number;

  @ApiProperty({ description: "Rank of this chunk within the file" })
  chunk_rank_in_file: number;
}

/**
 * Individual search result
 */
export class SearchResultDto {
  @ApiProperty({ description: "Unique document/chunk ID" })
  doc_id: string;

  @ApiProperty({ description: "File path" })
  path: string;

  @ApiProperty({ description: "Programming language" })
  language: string;

  @ApiProperty({ description: "Start line number" })
  start_line: number;

  @ApiProperty({ description: "End line number" })
  end_line: number;

  @ApiPropertyOptional({ description: "Code content (if include_content=true)" })
  content?: string;

  @ApiProperty({ description: "Extracted symbols (functions, classes, etc.)", type: [String] })
  symbols: string[];

  @ApiProperty({ description: "Final hybrid score (0-1)" })
  final_score: number;

  @ApiPropertyOptional({ description: "BM25 sparse score" })
  sparse_score?: number;

  @ApiPropertyOptional({ description: "Semantic dense score" })
  dense_score?: number;

  @ApiPropertyOptional({ description: "Rank in sparse results" })
  sparse_rank?: number;

  @ApiPropertyOptional({ description: "Rank in dense results" })
  dense_rank?: number;

  @ApiPropertyOptional({ description: "Aggregation info (when group_by_file=true)", type: AggregationInfoDto })
  aggregation?: AggregationInfoDto;
}

/**
 * Intelligence info - query understanding
 */
export class IntelligenceInfoDto {
  @ApiProperty({
    description: "Detected query intent",
    enum: ["navigational", "factual", "exploratory", "analytical"],
  })
  intent: string;

  @ApiProperty({
    description: "Query difficulty estimate",
    enum: ["easy", "medium", "hard"],
  })
  difficulty: string;

  @ApiProperty({
    description: "Selected retrieval strategy",
    enum: ["sparse-only", "balanced", "dense-heavy", "deep-rerank"],
  })
  strategy: string;

  @ApiProperty({ description: "Classification confidence (0-1)" })
  confidence: number;
}

/**
 * Reranking stats
 */
export class RerankingInfoDto {
  @ApiProperty({ description: "Whether reranking was enabled" })
  enabled: boolean;

  @ApiProperty({ description: "Number of candidates sent to reranker" })
  candidates: number;

  @ApiProperty({ description: "Whether pass 1 reranking was applied" })
  pass1_applied: boolean;

  @ApiProperty({ description: "Pass 1 latency in milliseconds" })
  pass1_latency_ms: number;

  @ApiProperty({ description: "Whether pass 2 reranking was applied" })
  pass2_applied: boolean;

  @ApiProperty({ description: "Pass 2 latency in milliseconds" })
  pass2_latency_ms: number;

  @ApiProperty({ description: "Whether early exit was triggered" })
  early_exit: boolean;

  @ApiPropertyOptional({ description: "Reason for early exit" })
  early_exit_reason?: string;
}

/**
 * Deduplication stats
 */
export class DedupStatsDto {
  @ApiProperty({ description: "Input count before dedup" })
  input_count: number;

  @ApiProperty({ description: "Output count after dedup" })
  output_count: number;

  @ApiProperty({ description: "Number of duplicates removed" })
  removed: number;

  @ApiProperty({ description: "Dedup latency in milliseconds" })
  latency_ms: number;
}

/**
 * Diversity stats
 */
export class DiversityStatsDto {
  @ApiProperty({ description: "Whether diversity was enabled" })
  enabled: boolean;

  @ApiProperty({ description: "Average diversity score (0-1)" })
  avg_diversity: number;

  @ApiProperty({ description: "Diversity processing latency in milliseconds" })
  latency_ms: number;
}

/**
 * Aggregation stats
 */
export class AggregationStatsDto {
  @ApiProperty({ description: "Number of unique files in results" })
  unique_files: number;

  @ApiProperty({ description: "Number of chunks dropped due to max_chunks_per_file" })
  chunks_dropped: number;
}

/**
 * PostRank processing stats
 */
export class PostrankInfoDto {
  @ApiProperty({ description: "Deduplication stats", type: DedupStatsDto })
  dedup: DedupStatsDto;

  @ApiProperty({ description: "Diversity stats", type: DiversityStatsDto })
  diversity: DiversityStatsDto;

  @ApiProperty({ description: "Aggregation stats", type: AggregationStatsDto })
  aggregation: AggregationStatsDto;

  @ApiProperty({ description: "Total postrank processing latency in milliseconds" })
  total_latency_ms: number;
}

/**
 * Complete search response
 */
export class SearchResponseDto {
  @ApiProperty({ description: "Original search query" })
  query: string;

  @ApiProperty({ description: "Search results", type: [SearchResultDto] })
  results: SearchResultDto[];

  @ApiProperty({ description: "Total number of results returned" })
  total: number;

  @ApiProperty({ description: "Store name" })
  store: string;

  @ApiProperty({ description: "Total search time in milliseconds" })
  search_time_ms: number;

  @ApiPropertyOptional({
    description: "Query intelligence info (intent, strategy, difficulty)",
    type: IntelligenceInfoDto,
  })
  intelligence?: IntelligenceInfoDto;

  @ApiPropertyOptional({
    description: "Reranking stats",
    type: RerankingInfoDto,
  })
  reranking?: RerankingInfoDto;

  @ApiPropertyOptional({
    description: "Post-rank processing stats (dedup, diversity, aggregation)",
    type: PostrankInfoDto,
  })
  postrank?: PostrankInfoDto;
}
