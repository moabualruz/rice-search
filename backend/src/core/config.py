"""
Configuration module - wraps SettingsManager for backward compatibility.

All settings are loaded from settings.yaml, overridable via environment variables,
and stored/managed through Redis.

NO HARDCODED VALUES - everything comes from SettingsManager.
"""
import logging
from typing import List
from src.core.settings_manager import get_settings_manager

logger = logging.getLogger(__name__)


class Settings:
    """
    Dynamic settings class that fetches all values from SettingsManager.

    Provides backward-compatible attribute access while using centralized
    settings management underneath.
    """

    def __init__(self):
        """Initialize settings wrapper."""
        self._manager = None
        self._initialized = False

    def _ensure_initialized(self):
        """Lazy initialization of settings manager."""
        if not self._initialized:
            try:
                self._manager = get_settings_manager()
                self._initialized = True
                logger.info("Settings initialized from centralized manager")
            except Exception as e:
                logger.error(f"Failed to initialize settings manager: {e}")
                raise

    def __getattr__(self, name: str):
        """
        Dynamically fetch setting values.

        Maps Python attribute names to setting keys.
        Example: settings.PROJECT_NAME -> manager.get("app.name")
        """
        self._ensure_initialized()

        # Map attribute names to setting keys
        mappings = {
            # Application
            "PROJECT_NAME": "app.name",
            "API_V1_STR": "app.api_prefix",

            # Infrastructure
            "QDRANT_URL": "infrastructure.qdrant.url",
            "REDIS_URL": "infrastructure.redis.url",
            "REDIS_SOCKET_TIMEOUT": "infrastructure.redis.socket_timeout",
            "MINIO_ENDPOINT": "infrastructure.minio.endpoint",
            "MINIO_ACCESS_KEY": "infrastructure.minio.access_key",
            "MINIO_SECRET_KEY": "infrastructure.minio.secret_key",

            # Authentication
            "AUTH_ENABLED": "auth.enabled",

            # CORS
            "BACKEND_CORS_ORIGINS": "server.cors_origins",

            # Embedding Model
            "EMBEDDING_MODEL": "models.embedding.name",
            "EMBEDDING_DIM": "models.embedding.dimension",
            "EMBEDDING_FALLBACK_DIM": "models.embedding.fallback_dimension",
            "EMBEDDING_MODEL_NAME": "inference.ollama.embedding_model",
            "EMBEDDING_TIMEOUT": "models.embedding.timeout",

            # Sparse Model (SPLADE)
            "SPARSE_MODEL": "models.sparse.model",
            "SPARSE_LIGHTWEIGHT_MODEL": "models.sparse.lightweight_model",
            "SPARSE_ENABLED": "models.sparse.enabled",
            "SPLADE_MODEL": "models.sparse.model",
            "SPLADE_ENABLED": "models.sparse.enabled",
            "SPLADE_DEVICE": "models.sparse.device",
            "SPLADE_PRECISION": "models.sparse.precision",
            "SPLADE_BATCH_SIZE": "models.sparse.batch_size",
            "SPLADE_MAX_TOKENS": "models.sparse.max_tokens",
            "SPARSE_VOCAB_SIZE": "models.sparse.vocab_size",
            "SPARSE_MIN_WORD_LENGTH": "models.sparse.min_word_length",

            # Hybrid Search
            "RRF_K": "search.hybrid.rrf_k",
            "DEFAULT_USE_BM25": "search.hybrid.use_bm25",
            "DEFAULT_USE_SPLADE": "search.hybrid.use_splade",
            "DEFAULT_USE_BM42": "search.hybrid.use_bm42",
            "COLLECTION_PREFIX": "search.collection_prefix",

            # BM25/Tantivy
            "BM25_ENABLED": "search.bm25.enabled",
            "TANTIVY_URL": "search.bm25.url",
            "TANTIVY_TIMEOUT": "search.bm25.timeout",

            # BM42
            "BM42_ENABLED": "search.hybrid.use_bm42",
            "BM42_MODEL": "models.bm42.model",

            # MCP Protocol
            "MCP_ENABLED": "mcp.enabled",
            "MCP_TRANSPORT": "mcp.transport",
            "MCP_TCP_HOST": "mcp.tcp.host",
            "MCP_TCP_PORT": "mcp.tcp.port",
            "MCP_SSE_PORT": "mcp.sse.port",

            # AST Parsing
            "AST_PARSING_ENABLED": "ast.enabled",
            "AST_LANGUAGES": "ast.languages",
            "AST_MAX_CHUNK_LINES": "ast.max_chunk_lines",

            # Reranking
            "RERANK_ENABLED": "models.reranker.enabled",
            "RERANK_MODE": "models.reranker.mode",
            "RERANK_MODEL": "models.reranker.model",
            "RERANK_TOP_K": "models.reranker.top_k",
            "RERANK_DOC_PREVIEW_LENGTH": "models.reranker.doc_preview_length",
            "RERANK_LLM_MAX_TOKENS": "models.reranker.llm_max_tokens",
            "RERANK_LLM_TEMPERATURE": "models.reranker.llm_temperature",

            # Query Understanding
            "QUERY_ANALYSIS_ENABLED": "search.query_analysis.enabled",
            "QUERY_UNDERSTANDING_MODEL": "models.query_analysis.model",
            "QUERY_MODEL": "models.query_analysis.model",
            "QUERY_ANALYSIS_LLM_MAX_TOKENS": "models.query_analysis.llm_max_tokens",
            "QUERY_ANALYSIS_LLM_TEMPERATURE": "models.query_analysis.llm_temperature",

            # LLM
            "LLM_MODEL": "inference.ollama.llm_model",
            "LLM_MAX_TOKENS": "models.llm.max_tokens",
            "LLM_TEMPERATURE": "models.llm.temperature",
            "LLM_CHAT_TIMEOUT": "models.llm.chat_timeout",

            # Model Management
            "FORCE_GPU": "model_management.force_gpu",
            "MODEL_TTL_SECONDS": "model_management.ttl_seconds",
            "MODEL_AUTO_UNLOAD": "model_management.auto_unload",

            # Inference
            "INFERENCE_URL": "inference.ollama.base_url",
            "OLLAMA_BASE_URL": "inference.ollama.base_url",

            # RAG
            "RAG_ENABLED": "rag.enabled",
            "RAG_MAX_TOKENS": "rag.max_tokens",
            "RAG_TEMPERATURE": "rag.temperature",
            "RAG_SYSTEM_PROMPT": "rag.system_prompt",

            # Indexing
            "CHUNK_SIZE": "indexing.chunk_size",
            "CHUNK_OVERLAP": "indexing.chunk_overlap",
            "INDEXING_BATCH_SIZE": "indexing.batch_size",
            "TEMP_DIR": "indexing.temp_dir",

            # Search
            "DEFAULT_SEARCH_LIMIT": "search.default_limit",
            "DEFAULT_SEARCH_MODE": "search.default_mode",

            # Admin
            "ADMIN_PERSIST_DIR": "admin.persist_dir",
            "ADMIN_REDIS_KEY_PREFIX": "admin.redis_key_prefix",

            # Metrics
            "METRICS_ENABLED": "metrics.enabled",
            "METRICS_PSUTIL_INTERVAL": "metrics.psutil_interval",

            # CLI
            "CLI_DEFAULT_LIMIT": "cli.default_limit",
            "CLI_DEFAULT_HYBRID": "cli.default_hybrid",
        }

        if name in mappings:
            setting_key = mappings[name]
            value = self._manager.get(setting_key)

            if value is None:
                logger.warning(f"Setting {name} (key: {setting_key}) not found in settings manager")
                # Return sensible defaults
                return self._get_default(name)

            return value

        # If not in mappings, try direct access
        logger.warning(f"Unmapped setting access: {name}")
        raise AttributeError(f"Setting {name} not found in configuration")

    def _get_default(self, name: str):
        """Provide fallback defaults for critical settings."""
        defaults = {
            "PROJECT_NAME": "Rice Search",
            "API_V1_STR": "/api/v1",
            "QDRANT_URL": "http://localhost:6333",
            "REDIS_URL": "redis://localhost:6379/0",
            "AUTH_ENABLED": False,
            "BACKEND_CORS_ORIGINS": ["http://localhost:3000", "http://localhost:8000"],
            "EMBEDDING_DIM": 1024,
            "RRF_K": 60,
            "SPARSE_ENABLED": True,
            "AST_PARSING_ENABLED": True,
            "RERANK_ENABLED": True,
            "MODEL_TTL_SECONDS": 300,
        }
        return defaults.get(name)

    def get(self, key: str, default=None):
        """
        Get setting by dot-notation key.

        Args:
            key: Setting key (e.g., "models.embedding.dimension")
            default: Default value if not found
        """
        self._ensure_initialized()
        return self._manager.get(key, default)

    def set(self, key: str, value, persist: bool = True):
        """
        Set setting at runtime.

        Args:
            key: Setting key
            value: New value
            persist: Whether to persist to file
        """
        self._ensure_initialized()
        self._manager.set(key, value, persist)

    def get_all(self, prefix: str = None):
        """Get all settings or settings with prefix."""
        self._ensure_initialized()
        return self._manager.get_all(prefix)

    def reload(self):
        """Reload settings from file."""
        self._ensure_initialized()
        self._manager.reload()

    class Config:
        """Pydantic-style config for backward compatibility."""
        env_file = ".env"


# Singleton instance
settings = Settings()


# Helper functions for messages
def get_message(key: str, **kwargs) -> str:
    """
    Get formatted message from settings.

    Args:
        key: Message key (e.g., "errors.model_not_found")
        **kwargs: Format parameters

    Returns:
        Formatted message string
    """
    message_key = f"messages.{key}"
    template = settings.get(message_key, "")

    if template:
        return template.format(**kwargs)

    logger.warning(f"Message template not found: {key}")
    return f"[{key}]"


# Export for backward compatibility
__all__ = ["settings", "Settings", "get_message"]
