"""
FastAPI application for unified-inference.
"""
import logging
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from config.settings import settings
from config.models import ModelRegistry
from lifecycle.manager import LifecycleManager
from router.selector import ModelSelector
from router.proxy import RequestProxy
from router.offload import OffloadPolicy
from api import routes

# Configure logging
logging.basicConfig(
    level=settings.log_level,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


# Global state
model_registry = None
lifecycle_manager = None
model_selector = None
request_proxy = None
offload_policy = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifespan context manager for startup and shutdown."""
    global model_registry, lifecycle_manager, model_selector, request_proxy, offload_policy

    logger.info("=" * 60)
    logger.info("Starting unified-inference service")
    logger.info(f"Execution mode: {settings.execution_mode}")
    logger.info(f"Router: {settings.router_host}:{settings.router_port}")
    logger.info("=" * 60)

    try:
        # Load model registry
        logger.info(f"Loading model registry from {settings.models_config_path}")
        model_registry = ModelRegistry(settings.models_config_path)
        logger.info(f"Loaded {len(model_registry.models)} models")

        # Initialize managers
        lifecycle_manager = LifecycleManager(model_registry)
        model_selector = ModelSelector(model_registry, lifecycle_manager)
        request_proxy = RequestProxy()
        offload_policy = OffloadPolicy(model_registry, lifecycle_manager)

        # Initialize routes with dependencies
        routes.init_routes(
            lifecycle_manager,
            model_selector,
            request_proxy,
            offload_policy,
            model_registry,
        )

        # Start background tasks
        await lifecycle_manager.start_background_tasks()

        logger.info("unified-inference ready")

        yield

    except Exception as e:
        logger.error(f"Startup error: {e}", exc_info=True)
        raise

    finally:
        # Shutdown
        logger.info("Shutting down unified-inference")

        if lifecycle_manager:
            await lifecycle_manager.stop_background_tasks()
            await lifecycle_manager.stop_all_models()

        if request_proxy:
            await request_proxy.close()

        logger.info("Shutdown complete")


# Create FastAPI app
app = FastAPI(
    title="unified-inference",
    description="Multi-model inference orchestrator for SGLang backends",
    version="1.0.0",
    lifespan=lifespan,
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routes
app.include_router(routes.router)


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "api.app:app",
        host=settings.router_host,
        port=settings.router_port,
        log_level=settings.log_level.lower(),
    )
