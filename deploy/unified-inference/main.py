"""
Main entry point for unified-inference service.
"""
import uvicorn
from config.settings import settings

if __name__ == "__main__":
    uvicorn.run(
        "api.app:app",
        host=settings.router_host,
        port=settings.router_port,
        log_level=settings.log_level.lower(),
        reload=False,
    )
