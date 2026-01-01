from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
import time
from src.core.config import settings
from src.db.qdrant import get_qdrant_client
from src.worker import celery_app, echo_task

app = FastAPI(
    title=settings.PROJECT_NAME,
    openapi_url=f"{settings.API_V1_STR}/openapi.json"
)

from src.api.v1.endpoints import ingest, search, files, stores
from src.api.v1.endpoints import metrics
from src.api.v1.endpoints.admin import config as admin_config
from src.api.v1.endpoints.admin import public as admin_public

app.include_router(ingest.router, prefix=f"{settings.API_V1_STR}/ingest", tags=["ingestion"])
app.include_router(search.router, prefix=f"{settings.API_V1_STR}/search", tags=["search"])
app.include_router(files.router, prefix=f"{settings.API_V1_STR}/files", tags=["files"])
app.include_router(stores.router, prefix=f"{settings.API_V1_STR}/stores", tags=["stores"])
app.include_router(admin_config.router, prefix=f"{settings.API_V1_STR}/admin", tags=["admin"])
app.include_router(admin_public.router, prefix=f"{settings.API_V1_STR}/admin/public", tags=["admin-public"])
app.include_router(metrics.router, tags=["metrics"])

# Request timing middleware
@app.middleware("http")
async def timing_middleware(request: Request, call_next):
    start = time.time()
    response = await call_next(request)
    duration_ms = (time.time() - start) * 1000
    
    # Record latency to Redis for dashboard
    try:
        from src.services.admin.admin_store import get_admin_store
        store = get_admin_store()
        store.record_request_latency(duration_ms)
        store.increment_counter("requests_total")
        
        # Track specific request types
        if "/search" in request.url.path:
            store.increment_counter("search_requests")
        elif "/ingest" in request.url.path:
            store.increment_counter("ingest_requests")
    except:
        pass  # Don't fail request if metrics fail
    
    response.headers["X-Response-Time-Ms"] = f"{duration_ms:.2f}"
    return response

# CORS
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.BACKEND_CORS_ORIGINS,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/health")
def health_check():
    """
    Checks connectivity to Qdrant and Celery.
    """
    status = {"status": "ok", "components": {}}
    
    # Check Qdrant
    try:
        q_client = get_qdrant_client()
        collections = q_client.get_collections()
        status["components"]["qdrant"] = {"status": "up", "collections": len(collections.collections)}
    except Exception as e:
        status["components"]["qdrant"] = {"status": "down", "error": str(e)}
        status["status"] = "degraded"

    # Check Celery/Redis
    try:
        # Simple ping task (async)
        task = echo_task.delay("ping")
        status["components"]["celery"] = {"status": "up", "last_task_id": str(task.id)}
    except Exception as e:
        status["components"]["celery"] = {"status": "down", "error": str(e)}
        status["status"] = "degraded"

    return status

@app.get("/")
def root():
    return {"message": "Welcome to Rice Search API"}

