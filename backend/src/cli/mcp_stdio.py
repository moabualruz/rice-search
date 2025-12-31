"""
MCP StdIO Entry Point.

Runs Rice Search as an MCP server using stdio transport.
This allows IDEs and CLI tools to use Rice Search as an agent tool.

Usage:
    python -m src.cli.mcp_stdio
"""

import asyncio
import logging
import sys

from src.services.mcp.mcp_server import mcp_server

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    stream=sys.stderr  # Log to stderr to keep stdout clean for MCP messages
)

logger = logging.getLogger(__name__)


async def main():
    """Run MCP server with stdio transport."""
    logger.info("Starting Rice Search MCP server (stdio)")
    try:
        await mcp_server.run_stdio()
    except KeyboardInterrupt:
        logger.info("Server stopped by user")
    except Exception as e:
        logger.error(f"Server error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
