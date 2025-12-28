import WebSocket from "ws";
import { EventEmitter } from "node:events";

// ============================================================================
// Message Types (must match server DTOs)
// ============================================================================

// Client → Server
export interface WsFileMessage {
  type: "file";
  path: string;
  content: string;
}

export interface WsSearchMessage {
  type: "search";
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
  type: "delete";
  req_id: string;
  paths?: string[];
  path_prefix?: string;
}

export interface WsStatsMessage {
  type: "stats";
  req_id: string;
}

export interface WsPingMessage {
  type: "ping";
}

export type WsClientMessage =
  | WsFileMessage
  | WsSearchMessage
  | WsDeleteMessage
  | WsStatsMessage
  | WsPingMessage;

// Server → Client
export interface WsAckMessage {
  type: "ack";
  conn_id: string;
  store: string;
}

export interface WsIndexedMessage {
  type: "indexed";
  chunks_queued: number;
  files_count: number;
  batch_id: string;
}

export interface WsSearchResult {
  doc_id: string;
  path: string;
  language: string;
  start_line: number;
  end_line: number;
  content?: string;
  symbols: string[];
  final_score: number;
}

export interface WsSearchResultMessage {
  type: "results";
  req_id: string;
  query: string;
  results: WsSearchResult[];
  total: number;
  search_time_ms: number;
}

export interface WsDeletedMessage {
  type: "deleted";
  req_id: string;
  sparse_deleted: number;
  dense_deleted: number;
}

export interface WsStatsResultMessage {
  type: "stats_result";
  req_id: string;
  tracked_files: number;
  total_size: number;
  last_updated: string;
}

export interface WsPongMessage {
  type: "pong";
}

export interface WsErrorMessage {
  type: "error";
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

// ============================================================================
// WebSocket Client
// ============================================================================

export interface RiceWsClientOptions {
  baseUrl: string;
  store: string;
  onIndexed?: (msg: WsIndexedMessage) => void;
  onError?: (msg: WsErrorMessage) => void;
  onDisconnect?: () => void;
  onConnect?: (connId: string) => void;
  reconnect?: boolean;
  reconnectDelay?: number;
}

type PendingRequest = {
  resolve: (value: WsServerMessage) => void;
  reject: (error: Error) => void;
  timeout: ReturnType<typeof setTimeout>;
};

/**
 * Non-blocking WebSocket client for Rice Search
 * 
 * - File sends are fire-and-forget (no response expected)
 * - Search/delete/stats use req_id for response matching
 * - Server pushes 'indexed' notifications when batches complete
 */
export class RiceWsClient extends EventEmitter {
  private ws: WebSocket | null = null;
  private connId: string | null = null;
  private connected = false;
  private pendingRequests = new Map<string, PendingRequest>();
  private reqCounter = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  private readonly baseUrl: string;
  private readonly store: string;
  private readonly reconnect: boolean;
  private readonly reconnectDelay: number;

  // Callbacks
  private onIndexed?: (msg: WsIndexedMessage) => void;
  private onError?: (msg: WsErrorMessage) => void;
  private onDisconnect?: () => void;
  private onConnectCb?: (connId: string) => void;

  constructor(options: RiceWsClientOptions) {
    super();
    // Convert http(s) to ws(s)
    this.baseUrl = options.baseUrl
      .replace(/^http:/, "ws:")
      .replace(/^https:/, "wss:");
    this.store = options.store;
    this.reconnect = options.reconnect ?? true;
    this.reconnectDelay = options.reconnectDelay ?? 3000;
    this.onIndexed = options.onIndexed;
    this.onError = options.onError;
    this.onDisconnect = options.onDisconnect;
    this.onConnectCb = options.onConnect;
  }

  /**
   * Connect to WebSocket server
   */
  async connect(): Promise<string> {
    return new Promise((resolve, reject) => {
      const wsUrl = `${this.baseUrl}/v1/ws?store=${encodeURIComponent(this.store)}`;
      
      // Debug: log connection URL
      if (process.env.DEBUG) {
        console.error(`[ws-client] Connecting to: ${wsUrl}`);
      }
      
      this.ws = new WebSocket(wsUrl);

      const timeout = setTimeout(() => {
        if (!this.connected) {
          this.ws?.close();
          reject(new Error("WebSocket connection timeout"));
        }
      }, 10000);

      this.ws.on("open", () => {
        // Wait for ack message with conn_id
      });

      this.ws.on("message", (data: Buffer) => {
        const msg = JSON.parse(data.toString()) as WsServerMessage;
        this.handleMessage(msg, resolve, clearTimeout.bind(null, timeout));
      });

      this.ws.on("error", (err: Error) => {
        if (!this.connected) {
          clearTimeout(timeout);
          reject(err);
        } else {
          console.error("WebSocket error:", err.message);
        }
      });

      this.ws.on("close", (code: number, reason: Buffer) => {
        const wasConnected = this.connected;
        this.connected = false;
        this.connId = null;
        
        // If closed during initial connection, reject the promise
        if (!wasConnected) {
          clearTimeout(timeout);
          const reasonStr = reason?.toString() || `code ${code}`;
          reject(new Error(`WebSocket closed during connect: ${reasonStr}`));
          return;
        }
        
        this.onDisconnect?.();
        this.emit("disconnect");

        // Reject all pending requests
        for (const [reqId, pending] of this.pendingRequests) {
          clearTimeout(pending.timeout);
          pending.reject(new Error("WebSocket disconnected"));
          this.pendingRequests.delete(reqId);
        }

        // Attempt reconnect
        if (this.reconnect && !this.reconnectTimer) {
          this.reconnectTimer = setTimeout(() => {
            this.reconnectTimer = null;
            this.connect().catch(() => {
              // Reconnect failed, will retry
            });
          }, this.reconnectDelay);
        }
      });
    });
  }

  /**
   * Handle incoming server message
   */
  private handleMessage(
    msg: WsServerMessage,
    onFirstConnect?: (connId: string) => void,
    clearConnectTimeout?: () => void,
  ): void {
    switch (msg.type) {
      case "ack":
        this.connected = true;
        this.connId = msg.conn_id;
        clearConnectTimeout?.();
        this.onConnectCb?.(msg.conn_id);
        this.emit("connect", msg.conn_id);
        onFirstConnect?.(msg.conn_id);
        break;

      case "indexed":
        this.onIndexed?.(msg);
        this.emit("indexed", msg);
        break;

      case "results":
      case "deleted":
      case "stats_result":
        this.resolveRequest(msg.req_id, msg);
        break;

      case "pong":
        this.emit("pong");
        break;

      case "error":
        if (msg.req_id) {
          this.rejectRequest(msg.req_id, new Error(`${msg.code}: ${msg.message}`));
        } else {
          this.onError?.(msg);
          this.emit("error", msg);
        }
        break;
    }
  }

  /**
   * Resolve a pending request
   */
  private resolveRequest(reqId: string, msg: WsServerMessage): void {
    const pending = this.pendingRequests.get(reqId);
    if (pending) {
      clearTimeout(pending.timeout);
      this.pendingRequests.delete(reqId);
      pending.resolve(msg);
    }
  }

  /**
   * Reject a pending request
   */
  private rejectRequest(reqId: string, error: Error): void {
    const pending = this.pendingRequests.get(reqId);
    if (pending) {
      clearTimeout(pending.timeout);
      this.pendingRequests.delete(reqId);
      pending.reject(error);
    }
  }

  /**
   * Generate unique request ID
   */
  private nextReqId(): string {
    return `req_${++this.reqCounter}_${Date.now()}`;
  }

  /**
   * Send message (low-level)
   */
  private send(msg: WsClientMessage): void {
    if (!this.ws || !this.connected) {
      throw new Error("WebSocket not connected");
    }
    this.ws.send(JSON.stringify(msg));
  }

  /**
   * Send message and wait for response
   */
  private async sendWithResponse<T extends WsServerMessage>(
    msg: WsClientMessage & { req_id: string },
    timeoutMs = 30000,
  ): Promise<T> {
    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        this.pendingRequests.delete(msg.req_id);
        reject(new Error(`Request ${msg.req_id} timeout`));
      }, timeoutMs);

      this.pendingRequests.set(msg.req_id, {
        resolve: resolve as (value: WsServerMessage) => void,
        reject,
        timeout,
      });

      try {
        this.send(msg);
      } catch (err) {
        clearTimeout(timeout);
        this.pendingRequests.delete(msg.req_id);
        reject(err);
      }
    });
  }

  // ==========================================================================
  // Public API
  // ==========================================================================

  /**
   * Send file for indexing (fire-and-forget)
   * No response - server will push 'indexed' when batch completes
   */
  sendFile(path: string, content: string): void {
    this.send({ type: "file", path, content });
  }

  /**
   * Search (waits for response)
   */
  async search(options: {
    query: string;
    top_k?: number;
    filters?: { path_prefix?: string; languages?: string[] };
    include_content?: boolean;
    enable_reranking?: boolean;
  }): Promise<WsSearchResultMessage> {
    const reqId = this.nextReqId();
    return this.sendWithResponse<WsSearchResultMessage>({
      type: "search",
      req_id: reqId,
      query: options.query,
      top_k: options.top_k,
      filters: options.filters,
      include_content: options.include_content,
      enable_reranking: options.enable_reranking,
    });
  }

  /**
   * Delete files (waits for response)
   */
  async deleteFiles(options: {
    paths?: string[];
    path_prefix?: string;
  }): Promise<WsDeletedMessage> {
    const reqId = this.nextReqId();
    return this.sendWithResponse<WsDeletedMessage>({
      type: "delete",
      req_id: reqId,
      paths: options.paths,
      path_prefix: options.path_prefix,
    });
  }

  /**
   * Get store stats (waits for response)
   */
  async getStats(): Promise<WsStatsResultMessage> {
    const reqId = this.nextReqId();
    return this.sendWithResponse<WsStatsResultMessage>({
      type: "stats",
      req_id: reqId,
    });
  }

  /**
   * Send ping (for keepalive)
   */
  ping(): void {
    this.send({ type: "ping" });
  }

  /**
   * Check if connected
   */
  isConnected(): boolean {
    return this.connected;
  }

  /**
   * Get connection ID
   */
  getConnId(): string | null {
    return this.connId;
  }

  /**
   * Close connection
   */
  close(): void {
    this.reconnect && (this.reconnectTimer = null); // Disable reconnect
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.connected = false;
    this.connId = null;
  }
}
