"""
MCP TCP Daemon.

Runs Rice Search MCP server as a TCP daemon for networked agents.

Usage:
    python -m src.services.mcp.mcp_daemon
"""

import asyncio
import logging
import sys
from typing import Optional

from mcp.server.stdio import stdio_server
from src.services.mcp.mcp_server import mcp_server
from src.core.config import settings

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)

logger = logging.getLogger(__name__)


class MCPTCPServer:
    """TCP server for MCP protocol."""
    
    def __init__(self, host: str = "0.0.0.0", port: int = 9090):
        self.host = host
        self.port = port
        self.server: Optional[asyncio.Server] = None
    
    async def handle_client(
        self,
        reader: asyncio.StreamReader,
        writer: asyncio.StreamWriter
    ):
        """Handle a single client connection."""
        addr = writer.get_extra_info('peername')
        logger.info(f"Client connected: {addr}")
        
        try:
            # Run MCP server for this connection
            await mcp_server.server.run(
                reader,
                writer,
                mcp_server.server.create_initialization_options()
            )
        except Exception as e:
            logger.error(f"Client error: {e}", exc_info=True)
        finally:
            logger.info(f"Client disconnected: {addr}")
            writer.close()
            await writer.wait_closed()
    
    async def start(self):
        """Start the TCP server."""
        self.server = await asyncio.start_server(
            self.handle_client,
            self.host,
            self.port
        )
        
        addr = self.server.sockets[0].getsockname()
        logger.info(f"MCP TCP server running on {addr}")
        
        async with self.server:
            await self.server.serve_forever()
    
    async def stop(self):
        """Stop the TCP server."""
        if self.server:
            self.server.close()
            await self.server.wait_closed()
            logger.info("MCP TCP server stopped")


async def main():
    """Run MCP TCP daemon."""
    server = MCPTCPServer(
        host=settings.MCP_TCP_HOST,
        port=settings.MCP_TCP_PORT
    )
    
    try:
        await server.start()
    except KeyboardInterrupt:
        logger.info("Server stopped by user")
        await server.stop()
    except Exception as e:
        logger.error(f"Server error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
