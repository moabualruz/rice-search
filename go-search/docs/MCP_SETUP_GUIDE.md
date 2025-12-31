# Rice Search MCP Setup Guide (Windows)

This guide explains how to configure **Claude Desktop** to use Rice Search via the Model Context Protocol (MCP) on Windows.

## Prerequisite: Start the Server

Before using MCP, the Rice Search Server must be running and listening for MCP connections.

1. Open a terminal (PowerShell or Command Prompt).
2. Start the server with the `--mcp-addr` flag to enable TCP listening:

```powershell
./build/rice-search-server.exe --mcp-addr localhost:50053
```

> **Note**: You can choose any available port (e.g., 50053).

## Configure Claude Desktop

1. Open your Claude Desktop configuration file.

   - Typically located at: `%APPDATA%\Claude\claude_desktop_config.json`
   - (e.g., `C:\Users\YourUser\AppData\Roaming\Claude\claude_desktop_config.json`)

2. Add the `rice-search` entry to the `mcpServers` object:

```json
{
  "mcpServers": {
    "rice-search": {
      "command": "C:\\path\\to\\rice-search.exe",
      "args": ["mcp", "--addr", "localhost:50053"]
    }
  }
}
```

**Important**:

- Replace `C:\\path\\to\\rice-search.exe` with the absolute path to your `rice-search.exe` client binary (e.g., inside the `build` folder).
- Ensure the port in `--addr` matches the port you started the server with.

## Verification

1. Restart Claude Desktop.
2. In a chat, look for the "connection" icon or ask Claude: "What tools are available?".
3. Claude should see tools like `search`, `read_file`, `list_files` provided by Rice Search.

## Troubleshooting

- **Connection Refused**: Ensure the server is running and the `--mcp-addr` flag was used.
- **Path Issues**: Use double backslashes `\\` in the JSON config path.
- **Firewall**: Ensure Windows Firewall allows the server to listen on the specified port (usually prompted on first run).
