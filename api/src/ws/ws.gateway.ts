import {
  WebSocketGateway,
  WebSocketServer,
  OnGatewayConnection,
  OnGatewayDisconnect,
  SubscribeMessage,
  MessageBody,
  ConnectedSocket,
} from '@nestjs/websockets';
import { Logger } from '@nestjs/common';
import { Server, WebSocket } from 'ws';
import { IncomingMessage } from 'http';
import { URL } from 'url';

import { FileBufferService } from './file-buffer.service';
import { SearchService } from '../search/search.service';
import { IndexService } from '../index/index.service';
import type {
  WsClientMessage,
  WsServerMessage,
  WsAckMessage,
  WsIndexedMessage,
  WsSearchResultMessage,
  WsDeletedMessage,
  WsStatsResultMessage,
  WsPongMessage,
  WsErrorMessage,
} from './dto/ws-messages.dto';

interface ClientData {
  connId: string;
  store: string;
}

const clientDataMap = new WeakMap<WebSocket, ClientData>();

/**
 * WebSocket Gateway for non-blocking communication
 * 
 * All handlers are fire-and-forget - they don't await results,
 * instead push responses when ready.
 */
@WebSocketGateway({
  path: '/v1/ws',
})
export class WsGateway implements OnGatewayConnection, OnGatewayDisconnect {
  private readonly logger = new Logger(WsGateway.name);

  @WebSocketServer()
  server: Server;

  constructor(
    private readonly fileBuffer: FileBufferService,
    private readonly searchService: SearchService,
    private readonly indexService: IndexService,
  ) {}

  /**
   * Handle new WebSocket connection
   * Query params: ?store=default
   */
  handleConnection(client: WebSocket, request: IncomingMessage): void {
    // Parse store from query string
    const url = new URL(request.url || '/', `http://${request.headers.host}`);
    const store = url.searchParams.get('store') || 'default';
    const connId = this.generateConnId();

    // Store client data
    clientDataMap.set(client, { connId, store });

    // Register file buffer for this connection
    this.fileBuffer.register(connId, store, (result) => {
      this.send(client, {
        type: 'indexed',
        chunks_queued: result.chunks_queued,
        files_count: result.files_count,
        batch_id: result.batch_id,
      });
    });

    // Send ack with connection ID
    this.send(client, {
      type: 'ack',
      conn_id: connId,
      store,
    });

    this.logger.log(`Client connected: ${connId}, store: ${store}`);
  }

  /**
   * Handle WebSocket disconnection
   */
  handleDisconnect(client: WebSocket): void {
    const data = clientDataMap.get(client);
    if (data) {
      this.fileBuffer.unregister(data.connId).catch((err) => {
        this.logger.error(`Error unregistering buffer: ${err}`);
      });
      this.logger.log(`Client disconnected: ${data.connId}`);
      clientDataMap.delete(client);
    }
  }

  /**
   * Handle incoming messages
   * All handlers are non-blocking
   */
  @SubscribeMessage('message')
  handleMessage(
    @MessageBody() data: string,
    @ConnectedSocket() client: WebSocket,
  ): void {
    const clientData = clientDataMap.get(client);
    if (!clientData) {
      this.sendError(client, undefined, 'NOT_CONNECTED', 'Client not registered');
      return;
    }

    let message: WsClientMessage;
    try {
      message = JSON.parse(data);
    } catch {
      this.sendError(client, undefined, 'INVALID_JSON', 'Failed to parse message');
      return;
    }

    // Route to appropriate handler (all non-blocking)
    switch (message.type) {
      case 'file':
        this.handleFile(client, clientData, message.path, message.content);
        break;

      case 'search':
        this.handleSearch(client, clientData, message);
        break;

      case 'delete':
        this.handleDelete(client, clientData, message);
        break;

      case 'stats':
        this.handleStats(client, clientData, message.req_id);
        break;

      case 'ping':
        this.send(client, { type: 'pong' });
        break;

      default:
        this.sendError(client, undefined, 'UNKNOWN_TYPE', `Unknown message type: ${(message as { type: string }).type}`);
    }
  }

  /**
   * Handle file message (non-blocking)
   * Adds to buffer, doesn't wait for indexing
   */
  private handleFile(
    client: WebSocket,
    clientData: ClientData,
    path: string,
    content: string,
  ): void {
    this.fileBuffer.addFile(clientData.connId, path, content);
    // No response - fire and forget
    // Client will receive 'indexed' message when batch completes
  }

  /**
   * Handle search message (non-blocking)
   * Fires search, pushes results when ready
   */
  private handleSearch(
    client: WebSocket,
    clientData: ClientData,
    message: {
      req_id: string;
      query: string;
      top_k?: number;
      filters?: { path_prefix?: string; languages?: string[] };
      include_content?: boolean;
      enable_reranking?: boolean;
    },
  ): void {
    // Fire and forget - don't await
    this.searchService
      .search(clientData.store, {
        query: message.query,
        top_k: message.top_k || 20,
        filters: message.filters,
        include_content: message.include_content !== false,
        enable_reranking: message.enable_reranking !== false,
      })
      .then((result) => {
        const response: WsSearchResultMessage = {
          type: 'results',
          req_id: message.req_id,
          query: result.query,
          results: result.results.map((r) => ({
            doc_id: r.doc_id,
            path: r.path,
            language: r.language,
            start_line: r.start_line,
            end_line: r.end_line,
            content: r.content,
            symbols: r.symbols,
            final_score: r.final_score,
          })),
          total: result.total,
          search_time_ms: result.search_time_ms,
        };
        this.send(client, response);
      })
      .catch((err) => {
        this.sendError(client, message.req_id, 'SEARCH_ERROR', String(err));
      });
  }

  /**
   * Handle delete message (non-blocking)
   */
  private handleDelete(
    client: WebSocket,
    clientData: ClientData,
    message: {
      req_id: string;
      paths?: string[];
      path_prefix?: string;
    },
  ): void {
    this.indexService
      .deleteFiles(clientData.store, message.paths, message.path_prefix)
      .then((result) => {
        const response: WsDeletedMessage = {
          type: 'deleted',
          req_id: message.req_id,
          sparse_deleted: result.sparse_deleted,
          dense_deleted: result.dense_deleted,
        };
        this.send(client, response);
      })
      .catch((err) => {
        this.sendError(client, message.req_id, 'DELETE_ERROR', String(err));
      });
  }

  /**
   * Handle stats request (non-blocking)
   */
  private handleStats(
    client: WebSocket,
    clientData: ClientData,
    reqId: string,
  ): void {
    // Stats is sync, but we still don't block the message loop
    Promise.resolve()
      .then(() => {
        const stats = this.indexService.getStoreStats(clientData.store);
        const bufferStats = this.fileBuffer.getBufferStats(clientData.connId);
        
        const response: WsStatsResultMessage = {
          type: 'stats_result',
          req_id: reqId,
          tracked_files: stats.tracked_files,
          total_size: stats.total_size,
          last_updated: stats.last_updated,
        };
        this.send(client, response);
      })
      .catch((err) => {
        this.sendError(client, reqId, 'STATS_ERROR', String(err));
      });
  }

  /**
   * Send message to client
   */
  private send(client: WebSocket, message: WsServerMessage): void {
    if (client.readyState === WebSocket.OPEN) {
      client.send(JSON.stringify(message));
    }
  }

  /**
   * Send error message to client
   */
  private sendError(
    client: WebSocket,
    reqId: string | undefined,
    code: string,
    message: string,
  ): void {
    const error: WsErrorMessage = {
      type: 'error',
      req_id: reqId,
      code,
      message,
    };
    this.send(client, error);
  }

  /**
   * Generate unique connection ID
   */
  private generateConnId(): string {
    return `conn_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
  }
}
