"""
HTTP proxying to backend model servers.
"""
import httpx
import logging
from typing import Dict, Any, Optional
from fastapi import Request, Response, HTTPException

from backends.base import Backend
from config.settings import settings

logger = logging.getLogger(__name__)


class RequestProxy:
    """Proxies HTTP requests to backend servers."""

    def __init__(self):
        self.client = httpx.AsyncClient(
            timeout=settings.proxy_timeout,
            follow_redirects=True,
        )

    async def proxy_request(
        self,
        request: Request,
        backend: Backend,
        path: str,
        body: Optional[Dict[str, Any]] = None,
    ) -> Response:
        """
        Proxy an HTTP request to a backend server.

        Args:
            request: Original FastAPI request
            backend: Target backend
            path: API path to proxy to
            body: Request body (if already parsed)

        Returns:
            FastAPI Response with backend's response

        Raises:
            HTTPException: If proxying fails
        """
        try:
            # Mark backend as handling a request
            backend.mark_request()

            # Build target URL
            target_url = f"{backend.url}{path}"

            # Get request body if not provided
            if body is None and request.method in ("POST", "PUT", "PATCH"):
                body = await request.json()

            # Prepare headers (exclude hop-by-hop headers)
            headers = {
                k: v for k, v in request.headers.items()
                if k.lower() not in (
                    "host", "connection", "keep-alive",
                    "proxy-authenticate", "proxy-authorization",
                    "te", "trailers", "transfer-encoding", "upgrade"
                )
            }

            logger.debug(f"Proxying {request.method} {path} to {target_url}")

            # Make request to backend
            response = await self.client.request(
                method=request.method,
                url=target_url,
                headers=headers,
                json=body if request.method in ("POST", "PUT", "PATCH") else None,
                params=request.query_params,
            )

            # Return response
            return Response(
                content=response.content,
                status_code=response.status_code,
                headers=dict(response.headers),
                media_type=response.headers.get("content-type"),
            )

        except httpx.TimeoutException as e:
            logger.error(f"Timeout proxying to {backend.model_config.name}: {e}")
            raise HTTPException(
                status_code=504,
                detail={
                    "error": {
                        "code": "BACKEND_TIMEOUT",
                        "message": f"Request to model '{backend.model_config.name}' timed out",
                    }
                }
            )

        except httpx.ConnectError as e:
            logger.error(f"Connection error to {backend.model_config.name}: {e}")
            raise HTTPException(
                status_code=503,
                detail={
                    "error": {
                        "code": "BACKEND_UNAVAILABLE",
                        "message": f"Could not connect to model '{backend.model_config.name}'",
                    }
                }
            )

        except Exception as e:
            logger.error(f"Error proxying to {backend.model_config.name}: {e}", exc_info=True)
            raise HTTPException(
                status_code=500,
                detail={
                    "error": {
                        "code": "PROXY_ERROR",
                        "message": f"Error proxying request: {str(e)}",
                    }
                }
            )

    async def close(self):
        """Close the HTTP client."""
        await self.client.aclose()
