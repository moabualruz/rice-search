from typing import List
from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    PROJECT_NAME: str = "Rice Search"
    API_V1_STR: str = "/api/v1"
    
    # Infrastructure
    QDRANT_URL: str = "http://localhost:6333"
    REDIS_URL: str = "redis://localhost:6379/0"
    MINIO_ENDPOINT: str = "localhost:9000"
    MINIO_ACCESS_KEY: str = "minioadmin"
    MINIO_SECRET_KEY: str = "minioadmin"

    # CORS
    BACKEND_CORS_ORIGINS: List[str] = ["http://localhost:3000", "http://localhost:8000"]

    # Hybrid Search (Phase 9)
    SPARSE_MODEL: str = "naver/splade-v3"
    SPARSE_ENABLED: bool = True
    RRF_K: int = 60
    EMBEDDING_MODEL: str = "all-MiniLM-L6-v2"

    # MCP Protocol (Phase 10)
    MCP_ENABLED: bool = False
    MCP_TRANSPORT: str = "stdio"  # stdio, tcp, sse
    MCP_TCP_HOST: str = "0.0.0.0"
    MCP_TCP_PORT: int = 9090
    MCP_SSE_PORT: int = 9091

    class Config:
        env_file = ".env"

settings = Settings()
