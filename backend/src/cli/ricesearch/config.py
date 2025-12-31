"""
Rice Search Client CLI configuration management.

Handles loading/saving CLI configuration from ~/.ricesearch/config.yaml
"""

import os
from pathlib import Path
from typing import Optional
import yaml


class RicesearchConfig:
    """CLI configuration manager."""
    
    CONFIG_DIR = Path.home() / ".ricesearch"
    CONFIG_FILE = CONFIG_DIR / "config.yaml"
    
    DEFAULT_CONFIG = {
        "backend_url": "http://localhost:8000",
        "org_id": "public",
        "default_limit": 10,
        "hybrid_search": True
    }
    
    def __init__(self):
        self.config = self._load()
    
    def _load(self) -> dict:
        """Load config from file or create with defaults."""
        if not self.CONFIG_FILE.exists():
            self.CONFIG_DIR.mkdir(parents=True, exist_ok=True)
            self._save(self.DEFAULT_CONFIG)
            return self.DEFAULT_CONFIG.copy()
        
        try:
            with open(self.CONFIG_FILE, 'r') as f:
                return yaml.safe_load(f) or self.DEFAULT_CONFIG.copy()
        except Exception:
            return self.DEFAULT_CONFIG.copy()
    
    def _save(self, config: dict):
        """Save config to file."""
        self.CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        with open(self.CONFIG_FILE, 'w') as f:
            yaml.safe_dump(config, f, default_flow_style=False)
    
    def get(self, key: str, default=None):
        """Get config value."""
        return self.config.get(key, default)
    
    def set(self, key: str, value):
        """Set config value and persist."""
        self.config[key] = value
        self._save(self.config)
    
    def show(self) -> dict:
        """Return full config."""
        return self.config.copy()
    
    @property
    def backend_url(self) -> str:
        return self.get("backend_url")
    
    @property
    def org_id(self) -> str:
        return self.get("org_id")
    
    @property
    def default_limit(self) -> int:
        return self.get("default_limit", 10)
    
    @property
    def hybrid_search(self) -> bool:
        return self.get("hybrid_search", True)


# Singleton instance
_config: Optional[RicesearchConfig] = None

def get_config() -> RicesearchConfig:
    """Get global config instance."""
    global _config
    if _config is None:
        _config = RicesearchConfig()
    return _config
