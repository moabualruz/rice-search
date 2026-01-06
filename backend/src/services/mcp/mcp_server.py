"""
MCP Server for Rice Search.

Implements the Model Context Protocol server with three tools:
- search: Query indexed content
- read_file: Retrieve full file content  
- list_files: List indexed files
"""

import asyncio
import logging
from typing import Any, Sequence
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp.types import (
    Tool,
    TextContent,
    CallToolRequest,
    CallToolResult,
)

from src.services.mcp.tools import handle_search, handle_read_file, handle_list_files
from src.core.config import settings

logger = logging.getLogger(__name__)


# Define MCP tools
TOOLS: Sequence[Tool] = [
    Tool(
        name="search",
        description="Search indexed code and documents using semantic or hybrid search",
        inputSchema={
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "The search query"
                },
                "limit": {
                    "type": "number",
                    "description": "Maximum number of results (default: 10)",
                    "default": 10
                },
                "org_id": {
                    "type": "string",
                    "description": "Organization ID for multi-tenancy (default: 'public')",
                    "default": "public"
                },
                "hybrid": {
                    "type": "boolean",
                    "description": "Enable hybrid search (dense + sparse) (default: from config)"
                }
            },
            "required": ["query"]
        }
    ),
    Tool(
        name="read_file",
        description="Read the full content of an indexed file",
        inputSchema={
            "type": "object",
            "properties": {
                "file_path": {
                    "type": "string",
                    "description": "Path to the file to read"
                },
                "org_id": {
                    "type": "string",
                    "description": "Organization ID (default: 'public')",
                    "default": "public"
                }
            },
            "required": ["file_path"]
        }
    ),
    Tool(
        name="list_files",
        description="List all indexed files, optionally filtered by pattern",
        inputSchema={
            "type": "object",
            "properties": {
                "org_id": {
                    "type": "string",
                    "description": "Organization ID (default: 'public')",
                    "default": "public"
                },
                "pattern": {
                    "type": "string",
                    "description": "Glob pattern to filter files (e.g., '*.py')"
                }
            }
        }
    )
]


class RiceSearchMCPServer:
    """MCP Server for Rice Search."""
    
    def __init__(self):
        self.server = Server("rice-search")
        self._register_handlers()
    
    def _register_handlers(self):
        """Register MCP request handlers."""
        
        @self.server.list_tools()
        async def list_tools() -> list[Tool]:
            """List available tools."""
            return TOOLS
        
        @self.server.call_tool()
        async def call_tool(name: str, arguments: dict[str, Any]) -> Sequence[TextContent]:
            """Handle tool calls."""
            try:
                if name == "search":
                    results = await handle_search(
                        query=arguments["query"],
                        limit=arguments.get("limit", 10),
                        org_id=arguments.get("org_id", "public"),
                        hybrid=arguments.get("hybrid")
                    )
                    # Format results as text
                    if not results:
                        content = "No results found."
                    else:
                        lines = []
                        for i, result in enumerate(results, 1):
                            score = result.get("score", 0.0)
                            text = result.get("text", "")
                            metadata = result.get("metadata", {})
                            file_path = metadata.get("file_path", "unknown")
                            
                            lines.append(f"{i}. {file_path} (score: {score:.3f})")
                            lines.append(f"   {text[:200]}...")
                            lines.append("")
                        
                        content = "\n".join(lines)
                    
                    return [TextContent(type="text", text=content)]
                
                elif name == "read_file":
                    content = await handle_read_file(
                        file_path=arguments["file_path"],
                        org_id=arguments.get("org_id", "public")
                    )
                    return [TextContent(type="text", text=content)]
                
                elif name == "list_files":
                    files = await handle_list_files(
                        org_id=arguments.get("org_id", "public"),
                        pattern=arguments.get("pattern")
                    )
                    if not files:
                        content = "No files found."
                    else:
                        content = "\n".join(files)
                    
                    return [TextContent(type="text", text=content)]
                
                else:
                    return [TextContent(
                        type="text",
                        text=f"Unknown tool: {name}"
                    )]
                    
            except Exception as e:
                logger.error(f"Tool execution error: {e}")
                return [TextContent(
                    type="text",
                    text=f"Error: {str(e)}"
                )]
    
    async def run_stdio(self):
        """Run server with stdio transport."""
        async with stdio_server() as (read_stream, write_stream):
            await self.server.run(
                read_stream,
                write_stream,
                self.server.create_initialization_options()
            )


# Create singleton instance
mcp_server = RiceSearchMCPServer()
