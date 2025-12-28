import { Injectable, Logger } from '@nestjs/common';
import { SearchService } from '../search/search.service';
import { IndexService } from '../index/index.service';
import { StoresService } from '../stores/stores.service';
import { FileTrackerService } from '../services/file-tracker.service';
import {
  MCP_PROTOCOL_VERSION,
  McpErrorCodes,
  type McpToolDefinition,
  type McpResponse,
  type McpResourceDefinition,
  type McpResourceTemplate,
  type McpResourceContent,
  type McpPromptDefinition,
  type McpPromptResult,
  type McpServerCapabilities,
  type McpInitializeResult,
  type CodeSearchInput,
  type IndexFilesInput,
  type DeleteFilesInput,
  type McpSearchResult,
} from './dto/mcp.dto';

@Injectable()
export class McpService {
  private readonly logger = new Logger(McpService.name);
  private initialized = false;

  constructor(
    private searchService: SearchService,
    private indexService: IndexService,
    private storesService: StoresService,
    private fileTracker: FileTrackerService,
  ) {}

  // ============================================================================
  // Server Info & Capabilities
  // ============================================================================

  getServerInfo(): McpInitializeResult {
    return {
      protocolVersion: MCP_PROTOCOL_VERSION,
      capabilities: this.getCapabilities(),
      serverInfo: {
        name: 'rice-search',
        version: '1.0.0',
      },
    };
  }

  getCapabilities(): McpServerCapabilities {
    return {
      tools: { listChanged: false },
      resources: { subscribe: false, listChanged: false },
      prompts: { listChanged: false },
      logging: {},
    };
  }

  // ============================================================================
  // Tools
  // ============================================================================

  getTools(): McpToolDefinition[] {
    return [
      {
        name: 'code_search',
        description:
          'Search code across indexed repositories using hybrid BM25 + semantic search with neural reranking. Returns relevant code snippets with file paths, line numbers, and symbols.',
        inputSchema: {
          type: 'object',
          properties: {
            query: {
              type: 'string',
              description: 'Search query - can be natural language or code patterns',
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
              description: 'Filter by programming languages (e.g., ["python", "typescript"])',
              items: { type: 'string' },
            },
            include_content: {
              type: 'boolean',
              description: 'Include full code content in results (default: true)',
              default: true,
            },
          },
          required: ['query'],
        },
      },
      {
        name: 'index_files',
        description:
          'Index code files for search. Supports incremental indexing - unchanged files are skipped automatically.',
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
              description: 'Files to index, each with "path" and "content" fields',
              items: { type: 'object' },
            },
          },
          required: ['files'],
        },
      },
      {
        name: 'delete_files',
        description: 'Remove files from the search index.',
        inputSchema: {
          type: 'object',
          properties: {
            store: {
              type: 'string',
              description: 'Store name (default: "default")',
              default: 'default',
            },
            paths: {
              type: 'array',
              description: 'Specific file paths to delete',
              items: { type: 'string' },
            },
            path_prefix: {
              type: 'string',
              description: 'Delete all files with this path prefix',
            },
          },
          required: [],
        },
      },
      {
        name: 'list_stores',
        description: 'List all available code search stores with their statistics.',
        inputSchema: {
          type: 'object',
          properties: {},
          required: [],
        },
      },
      {
        name: 'get_store_stats',
        description: 'Get detailed statistics for a specific store.',
        inputSchema: {
          type: 'object',
          properties: {
            store: {
              type: 'string',
              description: 'Store name (default: "default")',
              default: 'default',
            },
          },
          required: [],
        },
      },
    ];
  }

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

        case 'delete_files':
          return await this.handleDeleteFiles(args as unknown as DeleteFilesInput);

        case 'list_stores':
          return await this.handleListStores();

        case 'get_store_stats':
          return await this.handleGetStoreStats(args.store as string);

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

  private async handleIndexFiles(
    input: IndexFilesInput,
  ): Promise<{ content: { indexed: number; errors?: string[] } }> {
    const store = input.store || 'default';
    const result = await this.indexService.indexFiles(store, input.files, false, false);
    const indexed = 'chunks_indexed' in result ? result.chunks_indexed : result.chunks_queued;

    return {
      content: {
        indexed,
        errors: result.errors,
      },
    };
  }

  private async handleDeleteFiles(
    input: DeleteFilesInput,
  ): Promise<{ content: { deleted: number } }> {
    const store = input.store || 'default';
    const result = await this.indexService.deleteFiles(
      store,
      input.paths,
      input.path_prefix,
    );

    return {
      content: {
        deleted: result.sparse_deleted + result.dense_deleted,
      },
    };
  }

  private async handleListStores(): Promise<{ content: unknown[] }> {
    const { stores } = await this.storesService.listStores();
    const storeInfos = await Promise.all(
      stores.map(async (storeName) => {
        try {
          return await this.storesService.getStore(storeName);
        } catch {
          return { name: storeName };
        }
      }),
    );
    return { content: storeInfos };
  }

  private async handleGetStoreStats(
    store: string = 'default',
  ): Promise<{ content: unknown }> {
    const stats = await this.storesService.getStoreStats(store);
    const fileStats = this.fileTracker.getStoreStats(store);
    return {
      content: {
        ...stats,
        files: fileStats,
      },
    };
  }

  // ============================================================================
  // Resources
  // ============================================================================

  getResourceTemplates(): McpResourceTemplate[] {
    return [
      {
        uriTemplate: 'store://{store}/files',
        name: 'Indexed Files List',
        description: 'List all indexed files in a store',
        mimeType: 'application/json',
      },
      {
        uriTemplate: 'store://{store}/file/{path}',
        name: 'Indexed File Content',
        description: 'Read content of an indexed file',
        mimeType: 'text/plain',
      },
      {
        uriTemplate: 'store://{store}/stats',
        name: 'Store Statistics',
        description: 'Get statistics for a store',
        mimeType: 'application/json',
      },
    ];
  }

  async getResources(): Promise<McpResourceDefinition[]> {
    const resources: McpResourceDefinition[] = [];
    const { stores } = await this.storesService.listStores();

    for (const storeName of stores) {
      resources.push({
        uri: `store://${storeName}/files`,
        name: `${storeName} - Files`,
        description: `List of indexed files in store '${storeName}'`,
        mimeType: 'application/json',
      });
      resources.push({
        uri: `store://${storeName}/stats`,
        name: `${storeName} - Stats`,
        description: `Statistics for store '${storeName}'`,
        mimeType: 'application/json',
      });
    }

    return resources;
  }

  async readResource(uri: string): Promise<McpResourceContent[]> {
    // Parse URI: store://{store}/files or store://{store}/file/{path} or store://{store}/stats
    const match = uri.match(/^store:\/\/([^/]+)\/(.+)$/);
    if (!match) {
      throw new Error(`Invalid resource URI: ${uri}`);
    }

    const [, store, resourcePath] = match;

    if (resourcePath === 'files') {
      const files = this.fileTracker.getTrackedFiles(store);
      return [
        {
          uri,
          mimeType: 'application/json',
          text: JSON.stringify(
            files.map((f) => ({
              path: f.path,
              size: f.size,
              indexed_at: f.indexed_at,
            })),
            null,
            2,
          ),
        },
      ];
    }

    if (resourcePath === 'stats') {
      const stats = await this.storesService.getStoreStats(store);
      const fileStats = this.fileTracker.getStoreStats(store);
      return [
        {
          uri,
          mimeType: 'application/json',
          text: JSON.stringify({ ...stats, files: fileStats }, null, 2),
        },
      ];
    }

    if (resourcePath.startsWith('file/')) {
      const filePath = resourcePath.slice(5); // Remove 'file/' prefix
      const fileInfo = this.fileTracker.getFileInfo(store, filePath);
      if (!fileInfo) {
        throw new Error(`File not found: ${filePath}`);
      }
      // Note: We don't store file content, just metadata
      return [
        {
          uri,
          mimeType: 'application/json',
          text: JSON.stringify(fileInfo, null, 2),
        },
      ];
    }

    throw new Error(`Unknown resource path: ${resourcePath}`);
  }

  // ============================================================================
  // Prompts
  // ============================================================================

  getPrompts(): McpPromptDefinition[] {
    return [
      {
        name: 'code_review',
        description: 'Review code for bugs, security issues, and improvements',
        arguments: [
          {
            name: 'code',
            description: 'The code to review',
            required: true,
          },
          {
            name: 'language',
            description: 'Programming language of the code',
            required: false,
          },
          {
            name: 'focus',
            description: 'Focus area: security, performance, readability, or all',
            required: false,
          },
        ],
      },
      {
        name: 'explain_code',
        description: 'Explain what a piece of code does',
        arguments: [
          {
            name: 'code',
            description: 'The code to explain',
            required: true,
          },
          {
            name: 'language',
            description: 'Programming language of the code',
            required: false,
          },
          {
            name: 'detail_level',
            description: 'Level of detail: brief, normal, or detailed',
            required: false,
          },
        ],
      },
      {
        name: 'find_similar',
        description: 'Find similar code patterns in the indexed codebase',
        arguments: [
          {
            name: 'code',
            description: 'The code pattern to search for',
            required: true,
          },
          {
            name: 'store',
            description: 'Store to search in (default: "default")',
            required: false,
          },
        ],
      },
      {
        name: 'search_and_summarize',
        description: 'Search the codebase and summarize findings',
        arguments: [
          {
            name: 'query',
            description: 'What to search for',
            required: true,
          },
          {
            name: 'store',
            description: 'Store to search in (default: "default")',
            required: false,
          },
        ],
      },
    ];
  }

  async getPrompt(
    name: string,
    args: Record<string, string>,
  ): Promise<McpPromptResult> {
    switch (name) {
      case 'code_review':
        return this.buildCodeReviewPrompt(args);
      case 'explain_code':
        return this.buildExplainCodePrompt(args);
      case 'find_similar':
        return await this.buildFindSimilarPrompt(args);
      case 'search_and_summarize':
        return await this.buildSearchAndSummarizePrompt(args);
      default:
        throw new Error(`Unknown prompt: ${name}`);
    }
  }

  private buildCodeReviewPrompt(args: Record<string, string>): McpPromptResult {
    const { code, language, focus } = args;
    const lang = language || 'the detected language';
    const focusArea = focus || 'all aspects';

    return {
      description: `Code review focusing on ${focusArea}`,
      messages: [
        {
          role: 'user',
          content: {
            type: 'text',
            text: `Please review the following ${lang} code for ${focusArea}. Look for:
- Bugs and potential errors
- Security vulnerabilities
- Performance issues
- Code style and readability improvements
- Best practices violations

Code to review:
\`\`\`${language || ''}
${code}
\`\`\`

Provide specific, actionable feedback with line references where applicable.`,
          },
        },
      ],
    };
  }

  private buildExplainCodePrompt(args: Record<string, string>): McpPromptResult {
    const { code, language, detail_level } = args;
    const lang = language || 'the detected language';
    const level = detail_level || 'normal';

    let instruction = 'Explain what this code does.';
    if (level === 'brief') {
      instruction = 'Give a brief one-paragraph explanation of what this code does.';
    } else if (level === 'detailed') {
      instruction = 'Provide a detailed explanation of this code, including how each part works and why.';
    }

    return {
      description: `${level} code explanation`,
      messages: [
        {
          role: 'user',
          content: {
            type: 'text',
            text: `${instruction}

\`\`\`${language || ''}
${code}
\`\`\``,
          },
        },
      ],
    };
  }

  private async buildFindSimilarPrompt(
    args: Record<string, string>,
  ): Promise<McpPromptResult> {
    const { code, store } = args;
    const storeName = store || 'default';

    // Search for similar code
    const result = await this.searchService.search(storeName, {
      query: code,
      top_k: 5,
      include_content: true,
    });

    const similarCode = result.results
      .map((r, i) => `### ${i + 1}. ${r.path}:${r.start_line}-${r.end_line} (score: ${r.final_score.toFixed(2)})\n\`\`\`${r.language}\n${r.content}\n\`\`\``)
      .join('\n\n');

    return {
      description: 'Similar code patterns found in the codebase',
      messages: [
        {
          role: 'user',
          content: {
            type: 'text',
            text: `Find code similar to this pattern and explain the similarities:

\`\`\`
${code}
\`\`\``,
          },
        },
        {
          role: 'assistant',
          content: {
            type: 'text',
            text: `I found ${result.results.length} similar code patterns in the codebase:\n\n${similarCode}\n\nWould you like me to analyze the similarities and differences between these patterns?`,
          },
        },
      ],
    };
  }

  private async buildSearchAndSummarizePrompt(
    args: Record<string, string>,
  ): Promise<McpPromptResult> {
    const { query, store } = args;
    const storeName = store || 'default';

    // Search the codebase
    const result = await this.searchService.search(storeName, {
      query,
      top_k: 10,
      include_content: true,
    });

    const searchResults = result.results
      .map((r, i) => `${i + 1}. **${r.path}:${r.start_line}-${r.end_line}** (${r.language})\n   Symbols: ${r.symbols.join(', ') || 'none'}\n   Score: ${r.final_score.toFixed(2)}`)
      .join('\n');

    return {
      description: `Search results for: ${query}`,
      messages: [
        {
          role: 'user',
          content: {
            type: 'text',
            text: `Search the codebase for "${query}" and summarize what you find.`,
          },
        },
        {
          role: 'assistant',
          content: {
            type: 'text',
            text: `I searched for "${query}" and found ${result.results.length} relevant results:\n\n${searchResults}\n\nWould you like me to dive deeper into any of these results or provide a summary of the patterns I found?`,
          },
        },
      ],
    };
  }

  // ============================================================================
  // JSON-RPC Handler
  // ============================================================================

  async handleRequest(
    method: string,
    params: Record<string, unknown> | undefined,
    id: number | string | null,
  ): Promise<McpResponse> {
    try {
      switch (method) {
        // Lifecycle
        case 'initialize':
          this.initialized = true;
          return {
            jsonrpc: '2.0',
            id,
            result: this.getServerInfo(),
          };

        case 'ping':
          return {
            jsonrpc: '2.0',
            id,
            result: {},
          };

        // Tools
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
                code: McpErrorCodes.INVALID_PARAMS,
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

        // Resources
        case 'resources/list':
          return {
            jsonrpc: '2.0',
            id,
            result: {
              resources: await this.getResources(),
            },
          };

        case 'resources/templates/list':
          return {
            jsonrpc: '2.0',
            id,
            result: {
              resourceTemplates: this.getResourceTemplates(),
            },
          };

        case 'resources/read':
          if (!params || typeof params.uri !== 'string') {
            return {
              jsonrpc: '2.0',
              id,
              error: {
                code: McpErrorCodes.INVALID_PARAMS,
                message: 'Invalid params: missing resource uri',
              },
            };
          }
          const contents = await this.readResource(params.uri);
          return {
            jsonrpc: '2.0',
            id,
            result: {
              contents,
            },
          };

        // Prompts
        case 'prompts/list':
          return {
            jsonrpc: '2.0',
            id,
            result: {
              prompts: this.getPrompts(),
            },
          };

        case 'prompts/get':
          if (!params || typeof params.name !== 'string') {
            return {
              jsonrpc: '2.0',
              id,
              error: {
                code: McpErrorCodes.INVALID_PARAMS,
                message: 'Invalid params: missing prompt name',
              },
            };
          }
          const promptResult = await this.getPrompt(
            params.name,
            (params.arguments as Record<string, string>) || {},
          );
          return {
            jsonrpc: '2.0',
            id,
            result: promptResult,
          };

        default:
          return {
            jsonrpc: '2.0',
            id,
            error: {
              code: McpErrorCodes.METHOD_NOT_FOUND,
              message: `Method not found: ${method}`,
            },
          };
      }
    } catch (error) {
      this.logger.error(`MCP request failed: ${method}`, error);
      return {
        jsonrpc: '2.0',
        id,
        error: {
          code: McpErrorCodes.INTERNAL_ERROR,
          message: String(error),
        },
      };
    }
  }

  /**
   * Handle notifications (no response expected)
   */
  handleNotification(method: string, _params?: Record<string, unknown>): void {
    switch (method) {
      case 'notifications/initialized':
        this.logger.log('Client initialized');
        break;
      case 'notifications/cancelled':
        this.logger.log('Request cancelled');
        break;
      default:
        this.logger.warn(`Unknown notification: ${method}`);
    }
  }
}
