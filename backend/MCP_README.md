# Rice Search MCP Server

Rice Search implements the Model Context Protocol (MCP), allowing AI agents to use it as a tool for searching and reading indexed code.

## Available Tools

| Tool         | Description              | Parameters                                      |
| ------------ | ------------------------ | ----------------------------------------------- |
| `search`     | Search indexed code/docs | `query` (required), `limit`, `org_id`, `hybrid` |
| `read_file`  | Read full file content   | `file_path` (required), `org_id`                |
| `list_files` | List indexed files       | `org_id`, `pattern` (glob)                      |

## Usage

### StdIO Transport (for IDEs, CLI)

```bash
# From backend directory
python -m src.cli.mcp_stdio
```

This runs the MCP server using stdio, suitable for integration with:

- Claude Desktop
- Continue.dev
- Other MCP-compatible IDEs

### TCP Transport (for networked agents)

```bash
# From backend directory
python -m src.services.mcp.mcp_daemon
```

Default: `tcp://0.0.0.0:9090`

### Configuration

Environment variables:

```bash
MCP_ENABLED=true          # Enable MCP server
MCP_TRANSPORT=stdio        # stdio, tcp, or sse
MCP_TCP_HOST=0.0.0.0      # TCP host
MCP_TCP_PORT=9090          # TCP port
MCP_SSE_PORT=9091          # SSE port (future)
```

## Claude Desktop Integration

Add to Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "rice-search": {
      "command": "python",
      "args": ["-m", "src.cli.mcp_stdio"],
      "cwd": "/path/to/rice-search/backend"
    }
  }
}
```

Restart Claude Desktop. You can now ask Claude to search your indexed code!

## Example Queries

Once configured, you can ask Claude:

- "Search for authentication logic in my code"
- "Read the contents of src/main.py"
- "List all Python files in the backend"
- "Find where we handle database connections"

## Admin API

Monitor MCP server status:

```bash
GET /api/v1/admin/mcp/status
PUT /api/v1/admin/mcp/enable
PUT /api/v1/admin/mcp/disable
GET /api/v1/admin/mcp/connections
```

All endpoints require admin authentication.
