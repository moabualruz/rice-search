"""
Centralized Settings Manager with Redis Backend.

Provides a single source of truth for all application settings with:
- YAML file as base configuration
- Environment variable overrides
- Redis for runtime storage and updates
- File persistence for runtime changes
- Dot-notation access (settings.get("models.embedding.dimension"))
"""
import os
import logging
import json
import yaml
from typing import Any, Dict, Optional
from pathlib import Path
from redis import Redis
from threading import Lock

logger = logging.getLogger(__name__)


class SettingsManager:
    """
    Centralized settings manager with Redis backend.

    Load order:
    1. Read settings.yaml file
    2. Apply environment variable overrides
    3. Store in Redis
    4. Provide runtime read/write access
    5. Persist changes back to YAML file
    """

    REDIS_KEY_PREFIX = "settings:"
    SETTINGS_VERSION_KEY = "settings:version"
    SETTINGS_FILE = "settings.yaml"

    def __init__(self, redis_client: Redis, settings_path: Optional[str] = None):
        """
        Initialize settings manager.

        Args:
            redis_client: Redis client for storage
            settings_path: Path to settings.yaml file (defaults to backend/settings.yaml)
        """
        self.redis = redis_client
        self._lock = Lock()

        # Determine settings file path
        if settings_path:
            self.settings_path = Path(settings_path)
        else:
            # Default: backend/settings.yaml
            backend_dir = Path(__file__).parent.parent.parent
            self.settings_path = backend_dir / self.SETTINGS_FILE

        # In-memory cache for faster access
        self._cache: Dict[str, Any] = {}
        self._cache_dirty = False

        # Load settings
        self._load_settings()

    def _load_settings(self):
        """Load settings from YAML, apply env overrides, store in Redis."""
        logger.info(f"Loading settings from {self.settings_path}")

        # 1. Load from YAML file
        if not self.settings_path.exists():
            raise FileNotFoundError(f"Settings file not found: {self.settings_path}")

        with open(self.settings_path, 'r') as f:
            yaml_settings = yaml.safe_load(f)

        # 2. Flatten settings for easier access and env override
        flat_settings = self._flatten_dict(yaml_settings)

        # 3. Apply environment variable overrides
        # Convention: RICE_SEARCH_<SECTION>_<KEY> maps to section.key
        for env_key, env_value in os.environ.items():
            if env_key.startswith('RICE_SEARCH_'):
                # Convert RICE_SEARCH_MODELS_EMBEDDING_DIMENSION -> models.embedding.dimension
                setting_key = env_key[12:].lower().replace('_', '.')

                # Type conversion
                flat_settings[setting_key] = self._convert_type(env_value, flat_settings.get(setting_key))
                logger.info(f"Override from env: {setting_key} = {env_value}")

        # 4. Store in Redis
        with self._lock:
            for key, value in flat_settings.items():
                redis_key = f"{self.REDIS_KEY_PREFIX}{key}"
                self.redis.set(redis_key, json.dumps(value))

            # Update cache
            self._cache = flat_settings

            # Increment version
            self.redis.incr(self.SETTINGS_VERSION_KEY)

        logger.info(f"Loaded {len(flat_settings)} settings into Redis")

    def get(self, key: str, default: Any = None) -> Any:
        """
        Get setting value by dot-notation key.

        Args:
            key: Setting key in dot notation (e.g., "models.embedding.dimension")
            default: Default value if key not found

        Returns:
            Setting value
        """
        # Check cache first
        if key in self._cache:
            return self._cache[key]

        # Check Redis
        redis_key = f"{self.REDIS_KEY_PREFIX}{key}"
        value = self.redis.get(redis_key)

        if value is None:
            return default

        # Deserialize and cache
        try:
            deserialized = json.loads(value)
            self._cache[key] = deserialized
            return deserialized
        except json.JSONDecodeError:
            logger.error(f"Failed to deserialize setting {key}")
            return default

    def set(self, key: str, value: Any, persist: bool = True):
        """
        Set setting value at runtime.

        Args:
            key: Setting key in dot notation
            value: New value
            persist: If True, persist change to YAML file
        """
        with self._lock:
            # Update Redis
            redis_key = f"{self.REDIS_KEY_PREFIX}{key}"
            self.redis.set(redis_key, json.dumps(value))

            # Update cache
            self._cache[key] = value
            self._cache_dirty = True

            # Increment version
            self.redis.incr(self.SETTINGS_VERSION_KEY)

            logger.info(f"Setting updated: {key} = {value}")

            # Persist to file if requested
            if persist:
                self._persist_to_file()

    def get_all(self, prefix: Optional[str] = None) -> Dict[str, Any]:
        """
        Get all settings or settings with a specific prefix.

        Args:
            prefix: Optional prefix filter (e.g., "models" returns all model settings)

        Returns:
            Dictionary of settings
        """
        if prefix:
            return {k: v for k, v in self._cache.items() if k.startswith(f"{prefix}.")}
        return self._cache.copy()

    def get_nested(self, prefix: str) -> Dict[str, Any]:
        """
        Get settings as nested dictionary.

        Args:
            prefix: Prefix to filter (e.g., "models")

        Returns:
            Nested dictionary
        """
        flat = self.get_all(prefix)
        return self._unflatten_dict(flat, prefix)

    def delete(self, key: str, persist: bool = True):
        """
        Delete a setting.

        Args:
            key: Setting key
            persist: If True, persist change to YAML file
        """
        with self._lock:
            # Delete from Redis
            redis_key = f"{self.REDIS_KEY_PREFIX}{key}"
            self.redis.delete(redis_key)

            # Delete from cache
            if key in self._cache:
                del self._cache[key]
                self._cache_dirty = True

            logger.info(f"Setting deleted: {key}")

            if persist:
                self._persist_to_file()

    def reload(self):
        """Reload settings from YAML file (discards runtime changes)."""
        logger.warning("Reloading settings from file - runtime changes will be lost")
        self._cache.clear()
        self._load_settings()

    def _persist_to_file(self):
        """Persist current settings to YAML file."""
        try:
            # Convert flat cache to nested structure
            nested = self._unflatten_dict(self._cache)

            # Write to file
            with open(self.settings_path, 'w') as f:
                yaml.dump(nested, f, default_flow_style=False, sort_keys=False)

            logger.info(f"Settings persisted to {self.settings_path}")
            self._cache_dirty = False
        except Exception as e:
            logger.error(f"Failed to persist settings to file: {e}")

    def _flatten_dict(self, d: Dict, parent_key: str = '', sep: str = '.') -> Dict[str, Any]:
        """Flatten nested dictionary with dot notation keys."""
        items = []
        for k, v in d.items():
            new_key = f"{parent_key}{sep}{k}" if parent_key else k
            if isinstance(v, dict):
                items.extend(self._flatten_dict(v, new_key, sep=sep).items())
            else:
                items.append((new_key, v))
        return dict(items)

    def _unflatten_dict(self, flat_dict: Dict[str, Any], prefix: str = '') -> Dict[str, Any]:
        """Convert flat dictionary back to nested structure."""
        result = {}

        for flat_key, value in flat_dict.items():
            # Remove prefix if present
            if prefix and flat_key.startswith(f"{prefix}."):
                key = flat_key[len(prefix)+1:]
            else:
                key = flat_key

            # Build nested structure
            keys = key.split('.')
            current = result
            for part in keys[:-1]:
                if part not in current:
                    current[part] = {}
                current = current[part]
            current[keys[-1]] = value

        return result

    def _convert_type(self, value: str, reference: Any) -> Any:
        """Convert string environment variable to appropriate type."""
        if reference is None:
            return value

        ref_type = type(reference)

        if ref_type == bool:
            return value.lower() in ('true', '1', 'yes', 'on')
        elif ref_type == int:
            try:
                return int(value)
            except ValueError:
                return value
        elif ref_type == float:
            try:
                return float(value)
            except ValueError:
                return value
        elif ref_type == list:
            # Comma-separated list
            return [item.strip() for item in value.split(',')]
        else:
            return value

    def get_version(self) -> int:
        """Get current settings version (increments on each change)."""
        version = self.redis.get(self.SETTINGS_VERSION_KEY)
        return int(version) if version else 0


# Singleton instance
_settings_manager: Optional[SettingsManager] = None
_manager_lock = Lock()


def get_settings_manager(redis_client: Optional[Redis] = None) -> SettingsManager:
    """Get or create singleton settings manager."""
    global _settings_manager

    with _manager_lock:
        if _settings_manager is None:
            if redis_client is None:
                # Create default Redis client
                from src.db.redis_client import get_redis_client
                redis_client = get_redis_client()

            _settings_manager = SettingsManager(redis_client)

        return _settings_manager


def reload_settings():
    """Reload settings from file (discards runtime changes)."""
    manager = get_settings_manager()
    manager.reload()
