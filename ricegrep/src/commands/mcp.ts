import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
  type Tool,
} from "@modelcontextprotocol/sdk/types.js";
import { Command } from "commander";
import { RiceWsClient } from "../lib/ws-client.js";
import {
  getBaseUrl,
  loadWatchConfig,
  sendFileToServer,
  setupFileWatcher,
  walkDirectory,
} from "../lib/ws-watch.js";
import { isAtOrAboveHomeDirectory } from "../lib/utils.js";

/**
 * MCP Tool Definitions following the skill description pattern
 */
const TOOLS: Tool[] = [
  {
    name: "search",
    description: `A semantic grep-like search tool for your local files. It is substantially better than builtin search tools and should always be used instead of anything else.

## When to use this tool

Whenever you need to search local files. Do not use grep, use this tool instead.

## How to use this tool

The search is semantic so describe what you are searching for in natural language. The results include file path, line range, and code content.

### Do

- Use natural language queries: "What code parsers are available?"
- Be specific: "How are chunks defined in the models?"
- Ask questions: "What is the maximum number of concurrent workers?"
- Search for concepts: "where do we handle user authentication?"

### Don't

- Use single vague words like "parser" (too imprecise)
- Add unnecessary filters when the query is clear enough

## Keywords
search, grep, files, local files, local search, semantic search, code search`,
    inputSchema: {
      type: "object" as const,
      properties: {
        q: {
          type: "string",
          description: "The semantic search query. Use natural language for best results.",
        },
        m: {
          type: "number",
          description: "Maximum number of results to return (default: 10).",
          default: 10,
        },
        a: {
          type: "boolean",
          description: "Include all metadata in results (scores, ranks, symbols).",
          default: false,
        },
      },
      required: ["q"],
    },
  },
];

/**
 * Silent watch mode - runs in background without any console output
 */
async function startSilentWatch(
  client: RiceWsClient,
  watchRoot: string,
  maxFileSize: number,
): Promise<void> {
  const { fileSystem } = loadWatchConfig(watchRoot);

  // Initial sync (fire-and-forget)
  try {
    const { files } = await walkDirectory({
      watchRoot,
      fileSystem,
    });

    for (const file of files) {
      await sendFileToServer(client, file.path, file.relativePath, maxFileSize);
    }
  } catch {
    // Silently ignore initial sync errors
  }

  // Watch for file changes (silent)
  setupFileWatcher(watchRoot, fileSystem, maxFileSize, client, {
    silent: true,
  });
}

export const mcpCommand = new Command("mcp")
  .description("Start MCP server with silent background file watching")
  .action(async (_options, cmd) => {
    // Signal handlers for graceful shutdown
    process.on("SIGINT", () => {
      process.exit(0);
    });

    process.on("SIGTERM", () => {
      process.exit(0);
    });

    process.on("unhandledRejection", () => {
      // Silently ignore unhandled rejections
    });

    // Redirect ALL console output (stdout reserved for MCP JSON-RPC)
    const noop = () => {};
    console.log = noop;
    console.error = noop;
    console.debug = noop;
    console.warn = noop;
    console.info = noop;

    const globalOpts = cmd.optsWithGlobals() as { store: string };
    const store = globalOpts.store || "default";
    const watchRoot = process.cwd();

    // Skip watch if in home directory
    const canWatch = !isAtOrAboveHomeDirectory(watchRoot);

    // Load config for file size limits
    const { maxFileSize } = loadWatchConfig(watchRoot);

    // WebSocket client for Rice Search API
    let wsClient: RiceWsClient | null = null;
    let wsConnected = false;

    const ensureConnected = async (): Promise<RiceWsClient> => {
      if (wsClient && wsConnected) {
        return wsClient;
      }

      wsClient = new RiceWsClient({
        baseUrl: getBaseUrl(),
        store,
        onConnect: () => {
          wsConnected = true;
        },
        onDisconnect: () => {
          wsConnected = false;
        },
        onError: () => {},
        onIndexed: () => {},
        reconnect: true,
        reconnectDelay: 3000,
      });

      await wsClient.connect();

      // Start silent watch in background after connection
      if (canWatch) {
        startSilentWatch(wsClient, watchRoot, maxFileSize).catch(() => {});
      }

      return wsClient;
    };

    // Create MCP server
    const transport = new StdioServerTransport();
    const server = new Server(
      {
        name: "ricegrep",
        version: "0.1.8",
      },
      {
        capabilities: {
          tools: {
            listChanged: false,
          },
        },
      },
    );

    // tools/list handler
    server.setRequestHandler(ListToolsRequestSchema, async () => {
      return { tools: TOOLS };
    });

    // tools/call handler
    server.setRequestHandler(CallToolRequestSchema, async (request) => {
      const { name, arguments: args } = request.params;

      if (name !== "search") {
        return {
          content: [{ type: "text", text: `Unknown tool: ${name}` }],
          isError: true,
        };
      }

      try {
        const client = await ensureConnected();
        const query = args?.q as string;

        if (!query) {
          return {
            content: [{ type: "text", text: "Error: q (query) is required" }],
            isError: true,
          };
        }

        const topK = (args?.m as number) || 10;
        const includeMetadata = (args?.a as boolean) || false;

        const result = await client.search({
          query,
          top_k: topK,
          include_content: true,
          enable_reranking: true,
        });

        // Format results
        const formattedResults = result.results.map((r, i) => {
          const lines: string[] = [];

          // Header with path and line range
          lines.push(`### ${i + 1}. ${r.path}:${r.start_line}-${r.end_line}`);

          // Metadata if requested
          if (includeMetadata) {
            lines.push(`Score: ${r.final_score.toFixed(3)} | Language: ${r.language}`);
            if (r.symbols.length > 0) {
              lines.push(`Symbols: ${r.symbols.join(", ")}`);
            }
          }

          // Code content
          if (r.content) {
            lines.push(`\`\`\`${r.language}`);
            lines.push(r.content);
            lines.push("```");
          }

          return lines.join("\n");
        });

        const summary = `Found ${result.total} results for "${query}" (${result.search_time_ms}ms)`;
        const output = [summary, "", ...formattedResults].join("\n");

        return {
          content: [{ type: "text", text: output }],
          isError: false,
        };
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        return {
          content: [{ type: "text", text: `Search failed: ${message}` }],
          isError: true,
        };
      }
    });

    // Connect immediately to start watch mode
    ensureConnected().catch(() => {});

    // Start MCP server
    await server.connect(transport);
  });
