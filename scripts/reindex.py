#!/usr/bin/env python3
"""
Reindex a directory or repository into the Local Code Search platform.

Supports incremental indexing - only changed files are re-indexed.

Usage:
    python reindex.py /path/to/repo [--store default] [--api-url http://localhost:8080]

    # Force re-index all files
    python reindex.py /path/to/repo --force

    # Sync deleted files (remove from index files that no longer exist)
    python reindex.py /path/to/repo --sync
"""

import argparse
import json
import sys
from pathlib import Path
from typing import Generator
import urllib.request
import urllib.error

# Default ignore patterns
IGNORE_PATTERNS = {
    # Directories
    ".git",
    ".svn",
    ".hg",
    "node_modules",
    "vendor",
    ".venv",
    "venv",
    "__pycache__",
    ".tox",
    ".nox",
    "dist",
    "build",
    "target",
    ".next",
    "out",
    "_build",
    ".vscode",
    ".idea",
    ".cache",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    "logs",
    "coverage",
    ".nyc_output",
}

# File extensions to include
INCLUDE_EXTENSIONS = {
    ".py",
    ".pyi",
    ".js",
    ".mjs",
    ".cjs",
    ".ts",
    ".mts",
    ".tsx",
    ".jsx",
    ".rs",
    ".go",
    ".java",
    ".c",
    ".h",
    ".cpp",
    ".cc",
    ".hpp",
    ".cs",
    ".rb",
    ".php",
    ".swift",
    ".kt",
    ".kts",
    ".scala",
    ".sh",
    ".bash",
    ".yaml",
    ".yml",
    ".json",
    ".toml",
    ".md",
    ".mdx",
    ".html",
    ".htm",
    ".css",
    ".scss",
    ".sass",
    ".sql",
    ".graphql",
    ".gql",
    ".vue",
    ".svelte",
}

# Max file size (10MB)
MAX_FILE_SIZE = 10 * 1024 * 1024


def should_include_file(path: Path) -> bool:
    """Check if file should be included in indexing."""
    # Check extension
    if path.suffix.lower() not in INCLUDE_EXTENSIONS:
        return False

    # Check size
    try:
        if path.stat().st_size > MAX_FILE_SIZE:
            return False
    except OSError:
        return False

    return True


def should_ignore_dir(name: str) -> bool:
    """Check if directory should be ignored."""
    return name in IGNORE_PATTERNS or name.startswith(".")


def walk_directory(root: Path) -> Generator[Path, None, None]:
    """Walk directory and yield files to index."""
    for entry in root.iterdir():
        try:
            if entry.is_dir():
                if not should_ignore_dir(entry.name):
                    yield from walk_directory(entry)
            elif entry.is_file():
                if should_include_file(entry):
                    yield entry
        except PermissionError:
            print(f"  Warning: Permission denied: {entry}", file=sys.stderr)
        except Exception as e:
            print(f"  Warning: Error accessing {entry}: {e}", file=sys.stderr)


def read_file_content(path: Path) -> str | None:
    """Read file content, handling encoding errors."""
    try:
        return path.read_text(encoding="utf-8", errors="replace")
    except Exception as e:
        print(f"  Warning: Could not read {path}: {e}", file=sys.stderr)
        return None


def index_files(
    api_url: str, store: str, files: list[dict], force: bool = False
) -> dict:
    """Send files to API for indexing."""
    url = f"{api_url}/v1/stores/{store}/index"
    data = json.dumps({"files": files, "force": force}).encode("utf-8")

    req = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json"}, method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=120) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8") if e.fp else str(e)
        raise RuntimeError(f"HTTP {e.code}: {error_body}")


def sync_deleted_files(api_url: str, store: str, current_paths: list[str]) -> dict:
    """Remove deleted files from index."""
    url = f"{api_url}/v1/stores/{store}/index/sync"
    data = json.dumps({"current_paths": current_paths}).encode("utf-8")

    req = urllib.request.Request(
        url, data=data, headers={"Content-Type": "application/json"}, method="POST"
    )

    try:
        with urllib.request.urlopen(req, timeout=120) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8") if e.fp else str(e)
        raise RuntimeError(f"HTTP {e.code}: {error_body}")


def get_stats(api_url: str, store: str) -> dict:
    """Get indexing statistics."""
    url = f"{api_url}/v1/stores/{store}/index/stats"

    req = urllib.request.Request(url, headers={"Content-Type": "application/json"})

    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8") if e.fp else str(e)
        raise RuntimeError(f"HTTP {e.code}: {error_body}")


def main():
    parser = argparse.ArgumentParser(
        description="Reindex a directory into Local Code Search (supports incremental indexing)"
    )
    parser.add_argument("path", help="Directory path to index")
    parser.add_argument(
        "--store", default="default", help="Store name (default: default)"
    )
    parser.add_argument("--api-url", default="http://localhost:8080", help="API URL")
    parser.add_argument(
        "--batch-size", type=int, default=50, help="Files per batch (default: 50)"
    )
    parser.add_argument(
        "--dry-run", action="store_true", help="Show files without indexing"
    )
    parser.add_argument(
        "--force", action="store_true", help="Force re-index all files (ignore cache)"
    )
    parser.add_argument(
        "--sync", action="store_true", help="Remove deleted files from index"
    )
    parser.add_argument(
        "--stats", action="store_true", help="Show indexing statistics and exit"
    )

    args = parser.parse_args()

    # Stats-only mode
    if args.stats:
        try:
            stats = get_stats(args.api_url, args.store)
            print(f"Store: {args.store}")
            print(f"  Tracked files: {stats.get('tracked_files', 0)}")
            print(f"  Total size: {stats.get('total_size', 0) / 1024 / 1024:.1f} MB")
            print(f"  Last updated: {stats.get('last_updated', 'N/A')}")
        except Exception as e:
            print(f"Error getting stats: {e}", file=sys.stderr)
            sys.exit(1)
        return

    root = Path(args.path).resolve()
    if not root.is_dir():
        print(f"Error: {args.path} is not a directory", file=sys.stderr)
        sys.exit(1)

    print(f"Scanning: {root}")
    print(f"Store: {args.store}")
    print(f"API: {args.api_url}")
    if args.force:
        print(f"Mode: FORCE (re-index all files)")
    else:
        print(f"Mode: INCREMENTAL (only changed files)")
    print()

    # Collect files
    files_to_index = []
    all_paths = []
    total_size = 0

    for file_path in walk_directory(root):
        rel_path = file_path.relative_to(root)
        path_str = str(rel_path).replace("\\", "/")
        all_paths.append(path_str)

        content = read_file_content(file_path)

        if content is not None:
            files_to_index.append({"path": path_str, "content": content})
            total_size += len(content)

    print(f"Found {len(files_to_index)} files ({total_size / 1024 / 1024:.1f} MB)")

    # Sync deleted files if requested
    if args.sync:
        print("\nSyncing deleted files...", end=" ", flush=True)
        try:
            result = sync_deleted_files(args.api_url, args.store, all_paths)
            deleted = result.get("deleted", 0)
            if deleted > 0:
                print(f"removed {deleted} files from index")
            else:
                print("no deleted files to remove")
        except Exception as e:
            print(f"ERROR: {e}")

    if args.dry_run:
        print("\nDry run - files that would be indexed:")
        for f in files_to_index[:20]:
            print(f"  {f['path']}")
        if len(files_to_index) > 20:
            print(f"  ... and {len(files_to_index) - 20} more")
        return

    if not files_to_index:
        print("No files to index.")
        return

    # Index in batches
    total_chunks = 0
    total_skipped = 0
    total_errors = 0
    batch_num = 0

    for i in range(0, len(files_to_index), args.batch_size):
        batch = files_to_index[i : i + args.batch_size]
        batch_num += 1

        print(
            f"Indexing batch {batch_num} ({len(batch)} files)...", end=" ", flush=True
        )

        try:
            result = index_files(args.api_url, args.store, batch, force=args.force)
            chunks = result.get("chunks_indexed", 0)
            skipped = result.get("skipped_unchanged", 0)
            errors = len(result.get("errors", []))
            total_chunks += chunks
            total_skipped += skipped
            total_errors += errors

            # Show result with incremental stats
            if skipped > 0:
                print(f"OK ({chunks} chunks, {skipped} unchanged)")
            else:
                print(f"OK ({chunks} chunks)")

            if errors > 0:
                for err in result.get("errors", [])[:3]:
                    print(f"  Warning: {err}")
        except Exception as e:
            print(f"ERROR: {e}")
            total_errors += len(batch)

    print()
    print(f"Indexing complete!")
    print(f"  Files processed: {len(files_to_index)}")
    print(f"  Chunks indexed: {total_chunks}")
    if total_skipped > 0:
        print(f"  Unchanged (skipped): {total_skipped}")
    if total_errors > 0:
        print(f"  Errors: {total_errors}")


if __name__ == "__main__":
    main()
