from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from src.core.config import settings
from src.db.qdrant import get_qdrant_client
from src.worker import celery_app, echo_task

app = FastAPI(
    title=settings.PROJECT_NAME,
    openapi_url=f"{settings.API_V1_STR}/openapi.json"
)

from src.api.v1.endpoints import ingest, search
from src.api.v1.endpoints.admin import config as admin_config

app.include_router(ingest.router, prefix=f"{settings.API_V1_STR}/ingest", tags=["ingestion"])
app.include_router(search.router, prefix=f"{settings.API_V1_STR}/search", tags=["search"])
app.include_router(admin_config.router, prefix=f"{settings.API_V1_STR}/admin", tags=["admin"])

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
