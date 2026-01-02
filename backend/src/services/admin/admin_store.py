"""
Redis-backed admin store for persistent state.

Provides persistence for admin configuration, models, users, and audit logging.
"""

import json
import logging
from datetime import datetime
from typing import Dict, List, Any, Optional
import redis

from src.core.config import settings

logger = logging.getLogger(__name__)


class AdminStore:
    """Redis-backed storage for admin state."""
    
    # Redis key prefixes
    MODELS_KEY = "rice:admin:models"
    CONFIG_KEY = "rice:admin:config"
    USERS_KEY = "rice:admin:users"
    STORES_KEY = "rice:admin:stores"
    CONNECTIONS_KEY = "rice:admin:connections"
    AUDIT_KEY = "rice:admin:audit"
    METRICS_KEY = "rice:admin:metrics"
    
    def __init__(self):
        self._redis: Optional[redis.Redis] = None
        self._initialized = False
    
    @property
    def redis(self) -> redis.Redis:
        """Lazy Redis connection."""
        if self._redis is None:
            self._redis = redis.from_url(settings.REDIS_URL, decode_responses=True)
        return self._redis
    
    def _ensure_defaults(self):
        """Ensure default data exists on first access."""
        if self._initialized:
            return
        
        try:
            # Initialize models if not exist
            if not self.redis.exists(self.MODELS_KEY):
                # Helper to slugify
                def slugify(name: str) -> str:
                    return name.replace("/", "-").lower()

                default_models = {
                    slugify(settings.EMBEDDING_MODEL): {
                        "id": slugify(settings.EMBEDDING_MODEL),
                        "name": settings.EMBEDDING_MODEL,
                        "type": "embedding",
                        "active": True,
                        "gpu_enabled": True
                    },
                    slugify(settings.SPARSE_MODEL): {
                        "id": slugify(settings.SPARSE_MODEL),
                        "name": settings.SPARSE_MODEL,
                        "type": "sparse_embedding",
                        "active": settings.SPARSE_ENABLED,
                        "gpu_enabled": True
                    },
                    slugify(settings.RERANK_MODEL): {
                        "id": slugify(settings.RERANK_MODEL),
                        "name": settings.RERANK_MODEL,
                        "type": "reranker",
                        "active": settings.RERANK_ENABLED,
                        "gpu_enabled": True
                    },
                    slugify(settings.QUERY_UNDERSTANDING_MODEL): {
                        "id": slugify(settings.QUERY_UNDERSTANDING_MODEL),
                        "name": settings.QUERY_UNDERSTANDING_MODEL,
                        "type": "classification",
                        "active": getattr(settings, "QUERY_ANALYSIS_ENABLED", True),
                        "gpu_enabled": True
                    }
                }
                self.redis.set(self.MODELS_KEY, json.dumps(default_models))
            
            # Initialize config if not exist
            if not self.redis.exists(self.CONFIG_KEY):
                default_config = {
                    "sparse_enabled": settings.SPARSE_ENABLED,
                    "rrf_k": settings.RRF_K,
                    "ast_parsing_enabled": settings.AST_PARSING_ENABLED,
                    "query_analysis_enabled": settings.QUERY_ANALYSIS_ENABLED,
                    "mcp_enabled": settings.MCP_ENABLED, # Added explicit default
                    "worker_pool": "threads",
                    "worker_concurrency": 10
                }
                self.redis.set(self.CONFIG_KEY, json.dumps(default_config))
            
            # Initialize users if not exist
            if not self.redis.exists(self.USERS_KEY):
                default_users = {
                    "admin-1": {
                        "id": "admin-1",
                        "email": "admin@rice.local",
                        "role": "admin",
                        "org_id": "default",
                        "active": True,
                        "created_at": datetime.now().isoformat()
                    }
                }
                self.redis.set(self.USERS_KEY, json.dumps(default_users))
            
            # Initialize stores if not exist
            if not self.redis.exists(self.STORES_KEY):
                default_stores = {
                    "public": {
                        "id": "public",
                        "name": "Public Index",
                        "type": "production",
                        "description": "Default public index",
                        "created_at": datetime.now().isoformat()
                    }
                }
                self.redis.set(self.STORES_KEY, json.dumps(default_stores))
            
            self._initialized = True
        except Exception as e:
            logger.error(f"Failed to initialize admin store: {e}")
    
    # ============== Models ==============
    
    def get_models(self) -> Dict[str, dict]:
        """Get all models."""
        # Ensure defaults first (lazy init)
        self._ensure_defaults()
        
        try:
            data = self.redis.get(self.MODELS_KEY)
            models = json.loads(data) if data else {}
            
            # Resilience: If models key exists but is empty/corrupt, or if flushed
            if not models:
                 # Check if we should re-seed defaults (e.g. after a flush)
                 # Force re-init check by setting flag to False
                 self._initialized = False
                 self._ensure_defaults()
                 # Fetch again
                 data = self.redis.get(self.MODELS_KEY)
                 models = json.loads(data) if data else {}
                 
            return models
        except Exception as e:
            logger.error(f"Failed to get models: {e}")
            return {}
    
    def set_model(self, model_id: str, model: dict) -> bool:
        """Set a model."""
        try:
            models = self.get_models()
            models[model_id] = model
            self.redis.set(self.MODELS_KEY, json.dumps(models))
            self.log_audit("model_updated", f"Model {model_id} updated")
            return True
        except Exception as e:
            logger.error(f"Failed to set model: {e}")
            return False
    
    def delete_model(self, model_id: str) -> bool:
        """Delete a model."""
        try:
            models = self.get_models()
            if model_id in models:
                del models[model_id]
                self.redis.set(self.MODELS_KEY, json.dumps(models))
                self.log_audit("model_deleted", f"Model {model_id} deleted")
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to delete model: {e}")
            return False
    
    # ============== Config ==============
    
    def get_config(self) -> Dict[str, Any]:
        """Get config overrides."""
        self._ensure_defaults()
        try:
            data = self.redis.get(self.CONFIG_KEY)
            config = json.loads(data) if data else {}
            
            if not config:
                 self._initialized = False
                 self._ensure_defaults()
                 data = self.redis.get(self.CONFIG_KEY)
                 config = json.loads(data) if data else {}
                 
            return config
        except Exception as e:
            logger.error(f"Failed to get config: {e}")
            return {}
    
    def set_config(self, key: str, value: Any) -> bool:
        """Set a config value."""
        try:
            config = self.get_config()
            config[key] = value
            self.redis.set(self.CONFIG_KEY, json.dumps(config))
            self.log_audit("config_updated", f"Config {key}={value}")
            return True
        except Exception as e:
            logger.error(f"Failed to set config: {e}")
            return False
    
    def get_effective_config(self) -> Dict[str, Any]:
        """Get merged config (defaults + overrides)."""
        overrides = self.get_config()
        return {
            "sparse_enabled": overrides.get("sparse_enabled", settings.SPARSE_ENABLED),
            "sparse_model": settings.SPARSE_MODEL,
            "embedding_model": settings.EMBEDDING_MODEL,
            "rrf_k": overrides.get("rrf_k", settings.RRF_K),
            "ast_parsing_enabled": overrides.get("ast_parsing_enabled", settings.AST_PARSING_ENABLED),
            "mcp_enabled": overrides.get("mcp_enabled", settings.MCP_ENABLED),
            "mcp_transport": overrides.get("mcp_transport", settings.MCP_TRANSPORT),
            "mcp_tcp_port": overrides.get("mcp_tcp_port", settings.MCP_TCP_PORT),
            "rerank_enabled": overrides.get("rerank_enabled", settings.RERANK_ENABLED),
            "rerank_model": settings.RERANK_MODEL,
            "query_analysis_enabled": overrides.get("query_analysis_enabled", getattr(settings, "QUERY_ANALYSIS_ENABLED", False)),
            "worker_pool": overrides.get("worker_pool", "threads"),
            "worker_concurrency": overrides.get("worker_concurrency", 10),
            "model_ttl_seconds": overrides.get("model_ttl_seconds", settings.MODEL_TTL_SECONDS),
            "model_auto_unload": overrides.get("model_auto_unload", settings.MODEL_AUTO_UNLOAD),
            "qdrant_url": settings.QDRANT_URL,
            "redis_url": settings.REDIS_URL
        }
    
    def save_config_snapshot(self, label: str = None):
        """Save current config as a snapshot for rollback."""
        try:
            config = self.get_config()
            snapshot = {
                "timestamp": datetime.now().isoformat(),
                "label": label or f"snapshot_{datetime.now().strftime('%Y%m%d_%H%M%S')}",
                "config": config
            }
            self.redis.lpush(f"{self.CONFIG_KEY}:history", json.dumps(snapshot))
            # Keep only last 20 snapshots
            self.redis.ltrim(f"{self.CONFIG_KEY}:history", 0, 19)
            self.log_audit("config_snapshot", f"Saved snapshot: {snapshot['label']}")
            return snapshot
        except Exception as e:
            logger.error(f"Failed to save config snapshot: {e}")
            return None
    
    def list_config_history(self, limit: int = 10) -> List[dict]:
        """List config snapshots."""
        try:
            entries = self.redis.lrange(f"{self.CONFIG_KEY}:history", 0, limit - 1)
            return [json.loads(e) for e in entries]
        except Exception as e:
            logger.error(f"Failed to list config history: {e}")
            return []
    
    def rollback_config(self, index: int = 0) -> bool:
        """Rollback to a previous config snapshot."""
        try:
            history = self.list_config_history(limit=index + 1)
            if index >= len(history):
                return False
            
            snapshot = history[index]
            old_config = snapshot.get("config", {})
            
            # Restore config
            self.redis.set(self.CONFIG_KEY, json.dumps(old_config))
            self.log_audit("config_rollback", f"Rolled back to: {snapshot.get('label')}")
            return True
        except Exception as e:
            logger.error(f"Failed to rollback config: {e}")
            return False
    
    # ============== Users ==============
    
    def get_users(self) -> Dict[str, dict]:
        """Get all users."""
        self._ensure_defaults()
        try:
            data = self.redis.get(self.USERS_KEY)
            users = json.loads(data) if data else {}
            
            if not users:
                 self._initialized = False
                 self._ensure_defaults()
                 data = self.redis.get(self.USERS_KEY)
                 users = json.loads(data) if data else {}

            return users
        except Exception as e:
            logger.error(f"Failed to get users: {e}")
            return {}
    
    def set_user(self, user_id: str, user: dict) -> bool:
        """Set a user."""
        try:
            users = self.get_users()
            users[user_id] = user
            self.redis.set(self.USERS_KEY, json.dumps(users))
            self.log_audit("user_updated", f"User {user_id} updated")
            return True
        except Exception as e:
            logger.error(f"Failed to set user: {e}")
            return False
    
    def delete_user(self, user_id: str) -> bool:
        """Delete a user."""
        try:
            users = self.get_users()
            if user_id in users:
                del users[user_id]
                self.redis.set(self.USERS_KEY, json.dumps(users))
                self.log_audit("user_deleted", f"User {user_id} deleted")
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to delete user: {e}")
            return False
    
    # ============== Stores ==============

    def get_stores(self) -> Dict[str, dict]:
        """Get all stores."""
        self._ensure_defaults()
        try:
            data = self.redis.get(self.STORES_KEY)
            stores = json.loads(data) if data else {}
            
            if not stores:
                 self._initialized = False
                 self._ensure_defaults()
                 data = self.redis.get(self.STORES_KEY)
                 stores = json.loads(data) if data else {}

            return stores
        except Exception as e:
            logger.error(f"Failed to get stores: {e}")
            return {}

    def set_store(self, store_id: str, store: dict) -> bool:
        """Set a store."""
        try:
            stores = self.get_stores()
            stores[store_id] = store
            self.redis.set(self.STORES_KEY, json.dumps(stores))
            self.log_audit("store_updated", f"Store {store_id} updated")
            return True
        except Exception as e:
            logger.error(f"Failed to set store: {e}")
            return False

    def delete_store(self, store_id: str) -> bool:
        """Delete a store."""
        try:
            stores = self.get_stores()
            if store_id in stores:
                del stores[store_id]
                self.redis.set(self.STORES_KEY, json.dumps(stores))
                self.log_audit("store_deleted", f"Store {store_id} deleted")
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to delete store: {e}")
            return False
            
    # ============== Connections ==============

    def get_connections(self) -> Dict[str, dict]:
        """Get all active connections."""
        self._ensure_defaults()
        try:
            data = self.redis.get(self.CONNECTIONS_KEY)
            return json.loads(data) if data else {}
        except Exception as e:
            logger.error(f"Failed to get connections: {e}")
            return {}

    def set_connection(self, connection_id: str, connection: dict) -> bool:
        """Register or update a connection."""
        try:
            connections = self.get_connections()
            connections[connection_id] = connection
            self.redis.set(self.CONNECTIONS_KEY, json.dumps(connections))
            # No audit log for heartbeat updates to avoid noise, only new/changes roughly?
            # actually let's log only if new
            if connection_id not in connections:
                self.log_audit("connection_registered", f"Connection {connection_id} registered")
            return True
        except Exception as e:
            logger.error(f"Failed to set connection: {e}")
            return False

    def delete_connection(self, connection_id: str) -> bool:
        """Delete a connection."""
        try:
            connections = self.get_connections()
            if connection_id in connections:
                del connections[connection_id]
                self.redis.set(self.CONNECTIONS_KEY, json.dumps(connections))
                self.log_audit("connection_deleted", f"Connection {connection_id} deleted")
                return True
            return False
        except Exception as e:
            logger.error(f"Failed to delete connection: {e}")
            return False
    
    # ============== Audit Log ==============
    
    def log_audit(self, action: str, details: str = None, user: str = "system"):
        """Log an audit event."""
        try:
            entry = {
                "timestamp": datetime.now().isoformat(),
                "action": action,
                "user": user,
                "details": details
            }
            # Push to list (most recent first)
            self.redis.lpush(self.AUDIT_KEY, json.dumps(entry))
            # Keep only last 1000 entries
            self.redis.ltrim(self.AUDIT_KEY, 0, 999)
        except Exception as e:
            logger.error(f"Failed to log audit: {e}")
    
    def get_audit_log(self, limit: int = 20) -> List[dict]:
        """Get recent audit log entries."""
        try:
            entries = self.redis.lrange(self.AUDIT_KEY, 0, limit - 1)
            return [json.loads(e) for e in entries]
        except Exception as e:
            logger.error(f"Failed to get audit log: {e}")
            return []
    
    # ============== Metrics ==============
    
    def record_request_latency(self, latency_ms: float):
        """Record a request latency."""
        try:
            # Store in a sorted set for percentile calculation
            timestamp = datetime.now().timestamp()
            self.redis.zadd(f"{self.METRICS_KEY}:latency", {str(timestamp): latency_ms})
            # Keep only last 1000 entries
            self.redis.zremrangebyrank(f"{self.METRICS_KEY}:latency", 0, -1001)
        except Exception as e:
            logger.error(f"Failed to record latency: {e}")
    
    def get_latency_percentiles(self) -> Dict[str, float]:
        """Get latency percentiles."""
        try:
            # Get all latencies
            entries = self.redis.zrange(f"{self.METRICS_KEY}:latency", 0, -1, withscores=True)
            if not entries:
                return {"p50": 0, "p95": 0, "p99": 0}
            
            latencies = sorted([score for _, score in entries])
            n = len(latencies)
            
            return {
                "p50": latencies[int(n * 0.5)] if n > 0 else 0,
                "p95": latencies[int(n * 0.95)] if n > 0 else 0,
                "p99": latencies[int(n * 0.99)] if n > 0 else 0
            }
        except Exception as e:
            logger.error(f"Failed to get latency percentiles: {e}")
            return {"p50": 0, "p95": 0, "p99": 0}
    
    def increment_counter(self, name: str, amount: int = 1):
        """Increment a counter."""
        try:
            self.redis.incrby(f"{self.METRICS_KEY}:counter:{name}", amount)
        except Exception as e:
            logger.error(f"Failed to increment counter: {e}")
    
    def get_counter(self, name: str) -> int:
        """Get a counter value."""
        try:
            val = self.redis.get(f"{self.METRICS_KEY}:counter:{name}")
            return int(val) if val else 0
        except Exception as e:
            logger.error(f"Failed to get counter: {e}")
            return 0
    
    # ============== Cache Operations ==============
    
    def clear_cache(self) -> int:
        """Clear all cache keys (not admin state)."""
        try:
            # Only clear non-admin keys
            cursor = 0
            deleted = 0
            while True:
                cursor, keys = self.redis.scan(cursor, match="rice:cache:*", count=100)
                if keys:
                    deleted += self.redis.delete(*keys)
                if cursor == 0:
                    break
            self.log_audit("cache_cleared", f"Deleted {deleted} cache keys")
            return deleted
        except Exception as e:
            logger.error(f"Failed to clear cache: {e}")
            return 0


# Singleton instance
_store: Optional[AdminStore] = None

def get_admin_store() -> AdminStore:
    """Get global admin store instance."""
    global _store
    if _store is None:
        _store = AdminStore()
    return _store
