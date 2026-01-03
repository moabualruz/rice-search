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
    
    # Authentication
    AUTH_ENABLED: bool = False

    # CORS
    BACKEND_CORS_ORIGINS: List[str] = ["http://localhost:3000", "http://localhost:8000"]

    # Embedding Model (V3 Spec: Code-optimized)
    EMBEDDING_MODEL: str = "jina-embeddings-v3"  # Multilingual, code-aware embedding model
    EMBEDDING_DIM: int = 1024  # Jina v3 embedding dimension

    # Hybrid Search (Phase 9)
    SPARSE_MODEL: str = "naver/splade-cocondenser-ensembledistil"
    SPARSE_ENABLED: bool = True
    RRF_K: int = 60

    # MCP Protocol (Phase 10)
    MCP_ENABLED: bool = False
    MCP_TRANSPORT: str = "stdio"  # stdio, tcp, sse
    MCP_TCP_HOST: str = "0.0.0.0"
    MCP_TCP_PORT: int = 9090
    MCP_SSE_PORT: int = 9091

    # AST Parsing (Phase 12)
    AST_PARSING_ENABLED: bool = True
    AST_LANGUAGES: List[str] = ["py", "js", "ts", "go", "rs", "java", "cpp"]
    AST_MAX_CHUNK_LINES: int = 200

    # Reranking (REQ-SRCH-03 - V3 Spec)
    RERANK_ENABLED: bool = True
    RERANK_MODE: str = "llm"  # "tei" = dedicated TEI reranker, "llm" = use vLLM for reranking
    RERANK_MODEL: str = "jinaai/jina-reranker-v2-base-multilingual"
    QUERY_UNDERSTANDING_MODEL: str = "microsoft/codebert-base"
    RERANK_TOP_K: int = 10

    # Query Understanding (REQ-SRCH-01)
    QUERY_ANALYSIS_ENABLED: bool = True
    QUERY_MODEL: str = "microsoft/codebert-base"

    # Hardware Acceleration
    FORCE_GPU: bool = True  # Force GPU usage for all models/services if available
    # System Optimization (Phase 5)
    MODEL_TTL_SECONDS: int = 300  # Unload unused models after 5 minutes
    MODEL_AUTO_UNLOAD: bool = True
    
    # BentoML - Unified Model Inference Server
    # Single service for embeddings, reranking, and LLM chat
    BENTOML_URL: str = "http://localhost:3001"
    
    # Model names (configured in BentoML service)
    LLM_MODEL: str = "codellama/CodeLlama-7b-Instruct-hf"  # For RAG chat

    class Config:
        env_file = ".env"

settings = Settings()

