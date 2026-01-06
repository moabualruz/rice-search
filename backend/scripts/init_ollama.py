"""
Initialize Ollama by pre-pulling required models.

Runs on backend startup to ensure all models are available.
"""
import asyncio
import httpx
import logging
import sys

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

OLLAMA_URL = "http://ollama:11434"
REQUIRED_MODELS = [
    "qwen3-embedding:4b",  # Code-focused embeddings (100+ langs, 32k context)
    "qwen2.5-coder:1.5b",  # LLM for chat - optimized for code
]


async def pull_model(model_name: str):
    """Pull a model from Ollama."""
    async with httpx.AsyncClient(timeout=600.0) as client:
        try:
            logger.info(f"Pulling Ollama model: {model_name}")
            response = await client.post(
                f"{OLLAMA_URL}/api/pull",
                json={"name": model_name},
            )
            response.raise_for_status()
            logger.info(f"✓ Model {model_name} pulled successfully")
            return True
        except Exception as e:
            logger.error(f"✗ Failed to pull model {model_name}: {e}")
            return False


async def check_ollama_health():
    """Check if Ollama is running."""
    async with httpx.AsyncClient(timeout=5.0) as client:
        try:
            response = await client.get(f"{OLLAMA_URL}/api/tags")
            response.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"Ollama health check failed: {e}")
            return False


async def main():
    """Pre-pull all required Ollama models."""
    logger.info("=" * 60)
    logger.info("Initializing Ollama models")
    logger.info("=" * 60)

    # Wait for Ollama to be ready
    logger.info("Waiting for Ollama to be ready...")
    for i in range(30):
        if await check_ollama_health():
            logger.info("✓ Ollama is ready")
            break
        await asyncio.sleep(2)
    else:
        logger.error("✗ Ollama failed to start")
        sys.exit(1)

    # Pull all required models
    success_count = 0
    for model in REQUIRED_MODELS:
        if await pull_model(model):
            success_count += 1

    logger.info("=" * 60)
    logger.info(f"Models pulled: {success_count}/{len(REQUIRED_MODELS)}")
    logger.info("=" * 60)

    if success_count < len(REQUIRED_MODELS):
        logger.warning("Some models failed to pull, but continuing...")

    logger.info("Ollama initialization complete")


if __name__ == "__main__":
    asyncio.run(main())
