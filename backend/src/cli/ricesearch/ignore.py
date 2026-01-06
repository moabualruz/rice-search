"""
Ignore rules parser for .gitignore and .riceignore files.

Uses pathspec library for gitignore-style pattern matching.
"""

from pathlib import Path
from typing import List, Set
import pathspec


# Default patterns to always ignore
DEFAULT_IGNORES = [
    ".git/",
    ".git",
    "node_modules/",
    "__pycache__/",
    "*.pyc",
    ".venv/",
    "venv/",
    ".env",
    "dist/",
    "build/",
    ".next/",
    "target/",
    ".idea/",
    ".vscode/",
    "*.egg-info/",
    ".pytest_cache/",
    ".mypy_cache/",
    "*.log",
    ".DS_Store",
]


class IgnoreRules:
    """
    Parser for .gitignore and .riceignore files.
    
    Supports hierarchical rules - parent directory rules apply to children.
    """
    
    def __init__(self, root_path: Path):
        self.root_path = Path(root_path).resolve()
        self.specs: List[pathspec.PathSpec] = []
        self._load_rules()
    
    def _load_rules(self):
        """Load all ignore rules from root and subdirectories."""
        # Start with default ignores
        patterns = DEFAULT_IGNORES.copy()
        
        # Load .gitignore from root
        gitignore = self.root_path / ".gitignore"
        if gitignore.exists():
            patterns.extend(self._parse_file(gitignore))
        
        # Load .riceignore from root (takes precedence)
        riceignore = self.root_path / ".riceignore"
        if riceignore.exists():
            patterns.extend(self._parse_file(riceignore))
        
        # Create pathspec
        self.specs.append(pathspec.PathSpec.from_lines('gitwildmatch', patterns))
    
    def _parse_file(self, path: Path) -> List[str]:
        """Parse an ignore file and return patterns."""
        patterns = []
        try:
            with open(path, 'r', encoding='utf-8') as f:
                for line in f:
                    line = line.strip()
                    # Skip empty lines and comments
                    if line and not line.startswith('#'):
                        patterns.append(line)
        except Exception:
            pass
        return patterns
    
    def is_ignored(self, path: Path) -> bool:
        """
        Check if a path should be ignored.
        
        Args:
            path: Absolute or relative path to check
            
        Returns:
            True if path should be ignored
        """
        try:
            # Make path relative to root
            if path.is_absolute():
                rel_path = path.relative_to(self.root_path)
            else:
                rel_path = path
            
            # Check against all specs
            path_str = str(rel_path).replace('\\', '/')
            for spec in self.specs:
                if spec.match_file(path_str):
                    return True
            
            return False
        except ValueError:
            # Path not relative to root
            return False
    
    def filter_paths(self, paths: List[Path]) -> List[Path]:
        """Filter list of paths, removing ignored ones."""
        return [p for p in paths if not self.is_ignored(p)]
