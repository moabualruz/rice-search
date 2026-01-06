"""
Redis client singleton for application-wide use.
"""
import logging
from redis import Redis
from typing import Optional

logger = logging.getLogger(__name__)

# Singleton instance
_redis_client: Optional[Redis] = None


def get_redis_client() -> Redis:
    """
    Get or create singleton Redis client.

    Returns:
        Redis client instance
    """
    global _redis_client

    if _redis_client is None:
        # Get URL from environment or use default
        import os
        redis_url = os.getenv("REDIS_URL", "redis://localhost:6379/0")

        logger.info(f"Creating Redis client: {redis_url}")

        _redis_client = Redis.from_url(
            redis_url,
            decode_responses=True,
            socket_timeout=5,
            socket_connect_timeout=5
        )

        # Test connection
        try:
            _redis_client.ping()
            logger.info("Redis client connected successfully")
        except Exception as e:
            logger.error(f"Redis connection failed: {e}")
            raise

    return _redis_client


__all__ = ["get_redis_client"]
