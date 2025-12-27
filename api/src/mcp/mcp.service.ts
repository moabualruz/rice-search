import { Injectable, Logger } from '@nestjs/common';
import { SearchService } from '../search/search.service';
import { IndexService } from '../index/index.service';
import type {
  McpToolDefinition,
  McpResponse,
  CodeSearchInput,
  IndexFilesInput,
  McpSearchResult,
} from './dto/mcp.dto';

@Injectable()
export class McpService {
  private readonly logger = new Logger(McpService.name);

  constructor(
    private searchService: SearchService,
    private indexService: IndexService,
  ) {}

  /**
   * Get list of available MCP tools
   */
  getTools(): McpToolDefinition[] {
    return [
      {
        name: 'code_search',
        description:
          'Search code across indexed repositories using hybrid BM25 + semantic search. Returns relevant code snippets with file paths, line numbers, and symbols.',
        inputSchema: {
          type: 'object',
          properties: {
            query: {
              type: 'string',
              description:
                'Search query - can be natural language or code patterns',
            },
            store: {
              type: 'string',
              description: 'Store name (default: "default")',
              default: 'default',
            },
            top_k: {
              type: 'number',
              description: 'Maximum results to return (default: 10)',
              default: 10,
            },
            path_prefix: {
              type: 'string',
              description: 'Filter by path prefix (e.g., "src/")',
            },
            languages: {
              type: 'array',
              description:
                'Filter by programming languages (e.g., ["python", "typescript"])',
            },
            include_content: {
              type: 'boolean',
              description:
                'Include full code content in results (default: true)',
              default: true,
            },
          },
          required: ['query'],
        },
      },
      {
        name: 'index_files',
        description:
          'Index code files for search. Supports incremental indexing.',
        inputSchema: {
          type: 'object',
          properties: {
            store: {
              type: 'string',
              description: 'Store name (default: "default")',
              default: 'default',
            },
            files: {
              type: 'array',
              description:
                'Files to index, each with "path" and "content" fields',
            },
          },
          required: ['files'],
        },
      },
      {
        name: 'list_stores',
        description: 'List all available code search stores',
        inputSchema: {
          type: 'object',
          properties: {},
          required: [],
        },
      },
    ];
  }

  /**
   * Execute a tool call
   */
  async callTool(
    name: string,
    args: Record<string, unknown>,
  ): Promise<{ content: unknown; isError?: boolean }> {
    try {
      switch (name) {
        case 'code_search':
          return await this.handleCodeSearch(args as unknown as CodeSearchInput);

        case 'index_files':
          return await this.handleIndexFiles(args as unknown as IndexFilesInput);

        case 'list_stores':
          return await this.handleListStores();

        default:
          return {
            content: { error: `Unknown tool: ${name}` },
            isError: true,
          };
      }
    } catch (error) {
      this.logger.error(`Tool call failed: ${name}`, error);
      return {
        content: { error: String(error) },
        isError: true,
      };
    }
  }

  /**
   * Handle code_search tool
   */
  private async handleCodeSearch(
    input: CodeSearchInput,
  ): Promise<{ content: McpSearchResult[] }> {
    const store = input.store || 'default';
    const topK = input.top_k || 10;
    const includeContent = input.include_content !== false;

    const result = await this.searchService.search(store, {
      query: input.query,
      top_k: topK,
      filters: {
        path_prefix: input.path_prefix,
        languages: input.languages,
      },
      include_content: includeContent,
    });

    const mcpResults: McpSearchResult[] = result.results.map((r) => ({
      path: r.path,
      language: r.language,
      start_line: r.start_line,
      end_line: r.end_line,
      content: r.content,
      symbols: r.symbols,
      score: r.final_score,
    }));

    return { content: mcpResults };
  }

  /**
   * Handle index_files tool
   */
  private async handleIndexFiles(
    input: IndexFilesInput,
  ): Promise<{ content: { indexed: number; errors?: string[] } }> {
    const store = input.store || 'default';

    // MCP always uses sync mode
    const result = await this.indexService.indexFiles(store, input.files, false, false);

    // Type guard for sync response
    const indexed = 'chunks_indexed' in result ? result.chunks_indexed : result.chunks_queued;

    return {
      content: {
        indexed,
        errors: result.errors,
      },
    };
  }

  /**
   * Handle list_stores tool
   */
  private async handleListStores(): Promise<{ content: string[] }> {
    // This would need access to StoreManagerService
    // For now, return default
    return { content: ['default'] };
  }

  /**
   * Handle MCP JSON-RPC request
   */
  async handleRequest(
    method: string,
    params: Record<string, unknown> | undefined,
    id: number,
  ): Promise<McpResponse> {
    try {
      switch (method) {
        case 'initialize':
          return {
            jsonrpc: '2.0',
            id,
            result: {
              protocolVersion: '2024-11-05',
              serverInfo: {
                name: 'local-code-search',
                version: '1.0.0',
              },
              capabilities: {
                tools: {},
              },
            },
          };

        case 'tools/list':
          return {
            jsonrpc: '2.0',
            id,
            result: {
              tools: this.getTools(),
            },
          };

        case 'tools/call':
          if (!params || typeof params.name !== 'string') {
            return {
              jsonrpc: '2.0',
              id,
              error: {
                code: -32602,
                message: 'Invalid params: missing tool name',
              },
            };
          }
          const toolResult = await this.callTool(
            params.name,
            (params.arguments as Record<string, unknown>) || {},
          );
          return {
            jsonrpc: '2.0',
            id,
            result: toolResult,
          };

        default:
          return {
            jsonrpc: '2.0',
            id,
            error: {
              code: -32601,
              message: `Method not found: ${method}`,
            },
          };
      }
    } catch (error) {
      return {
        jsonrpc: '2.0',
        id,
        error: {
          code: -32603,
          message: String(error),
        },
      };
    }
  }
}
