/**
 * WebSocket Message DTOs
 * All communication is non-blocking via message types
 */

// ============================================================================
// Client → Server Messages
// ============================================================================

export interface WsFileMessage {
  type: 'file';
  path: string;
  content: string;
}

export interface WsSearchMessage {
  type: 'search';
  req_id: string;
  query: string;
  top_k?: number;
  filters?: {
    path_prefix?: string;
    languages?: string[];
  };
  include_content?: boolean;
  enable_reranking?: boolean;
}

export interface WsDeleteMessage {
  type: 'delete';
  req_id: string;
  paths?: string[];
  path_prefix?: string;
}

export interface WsStatsMessage {
  type: 'stats';
  req_id: string;
}

export interface WsPingMessage {
  type: 'ping';
}

export type WsClientMessage =
  | WsFileMessage
  | WsSearchMessage
  | WsDeleteMessage
  | WsStatsMessage
  | WsPingMessage;

// ============================================================================
// Server → Client Messages
// ============================================================================

export interface WsAckMessage {
  type: 'ack';
  conn_id: string;
  store: string;
}

export interface WsIndexedMessage {
  type: 'indexed';
  chunks_queued: number;
  files_count: number;
  batch_id: string;
}

export interface WsSearchResultMessage {
  type: 'results';
  req_id: string;
  query: string;
  results: Array<{
    doc_id: string;
    path: string;
    language: string;
    start_line: number;
    end_line: number;
    content?: string;
    symbols: string[];
    final_score: number;
  }>;
  total: number;
  search_time_ms: number;
}

export interface WsDeletedMessage {
  type: 'deleted';
  req_id: string;
  sparse_deleted: number;
  dense_deleted: number;
}

export interface WsStatsResultMessage {
  type: 'stats_result';
  req_id: string;
  tracked_files: number;
  total_size: number;
  last_updated: string;
}

export interface WsPongMessage {
  type: 'pong';
}

export interface WsErrorMessage {
  type: 'error';
  req_id?: string;
  code: string;
  message: string;
}

export type WsServerMessage =
  | WsAckMessage
  | WsIndexedMessage
  | WsSearchResultMessage
  | WsDeletedMessage
  | WsStatsResultMessage
  | WsPongMessage
  | WsErrorMessage;
