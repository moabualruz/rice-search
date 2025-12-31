"""
Rice Search Client watch command implementation.

Monitors directories for changes and indexes files via Rice Search backend.
"""

import time
import hashlib
from pathlib import Path
from typing import Optional, Set
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler, FileSystemEvent
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn

from src.cli.ricesearch.api_client import get_api_client
from src.cli.ricesearch.config import get_config
from src.cli.ricesearch.ignore import IgnoreRules

console = Console()


def compute_hash(file_path: Path) -> str:
    """Compute SHA256 hash of file content."""
    try:
        with open(file_path, 'rb') as f:
            return hashlib.sha256(f.read()).hexdigest()
    except Exception:
        return ""


class IndexHandler(FileSystemEventHandler):
    """Handle filesystem events and trigger indexing."""
    
    def __init__(
        self,
        root_path: Path,
        org_id: str,
        ignore_rules: IgnoreRules
    ):
        self.root_path = root_path
        self.org_id = org_id
        self.ignore_rules = ignore_rules
        self.client = get_api_client()
        self.file_hashes: dict = {}  # path -> hash
        self.pending_files: Set[Path] = set()
        self.debounce_time = 0.5  # seconds
        self.last_event_time = 0.0
    
    def _should_process(self, path: Path) -> bool:
        """Check if file should be processed."""
        # Must be a file
        if not path.is_file():
            return False
        
        # Not ignored
        if self.ignore_rules.is_ignored(path):
            return False
        
        # Not binary (simple check)
        try:
            with open(path, 'r', encoding='utf-8') as f:
                f.read(1024)
            return True
        except (UnicodeDecodeError, PermissionError):
            return False
    
    def _index_file(self, path: Path):
        """Index a single file."""
        # Compute hash
        new_hash = compute_hash(path)
        old_hash = self.file_hashes.get(str(path))
        
        # Skip if unchanged
        if new_hash == old_hash:
            console.print(f"[dim]Skipped (unchanged): {path}[/dim]")
            return
        
        # Index via API
        console.print(f"[blue]Indexing:[/blue] {path}")
        result = self.client.index_file(path, self.org_id)
        
        if result.get("status") == "success":
            self.file_hashes[str(path)] = new_hash
            chunks = result.get("chunks_indexed", 0)
            console.print(f"[green]✓ Indexed {chunks} chunks[/green]")
        else:
            msg = result.get("message", "Unknown error")
            console.print(f"[red]✗ Failed: {msg}[/red]")
    
    def on_modified(self, event: FileSystemEvent):
        """Handle file modification."""
        if event.is_directory:
            return
        
        path = Path(event.src_path)
        if self._should_process(path):
            self.pending_files.add(path)
            self.last_event_time = time.time()
    
    def on_created(self, event: FileSystemEvent):
        """Handle file creation."""
        self.on_modified(event)
    
    def on_deleted(self, event: FileSystemEvent):
        """Handle file deletion."""
        path = Path(event.src_path)
        if str(path) in self.file_hashes:
            del self.file_hashes[str(path)]
            console.print(f"[yellow]Removed from index: {path}[/yellow]")
    
    def process_pending(self):
        """Process pending files after debounce period."""
        if not self.pending_files:
            return
        
        if time.time() - self.last_event_time < self.debounce_time:
            return
        
        # Process all pending files
        files_to_process = list(self.pending_files)
        self.pending_files.clear()
        
        for path in files_to_process:
            if path.exists():
                self._index_file(path)


def watch_command(
    path: str,
    org_id: Optional[str] = None,
    initial_scan: bool = True
):
    """
    Watch directory and index changes.
    
    Args:
        path: Directory to watch
        org_id: Organization ID
        initial_scan: Perform initial scan of all files
    """
    config = get_config()
    org_id = org_id or config.org_id
    
    root_path = Path(path).resolve()
    
    if not root_path.exists():
        console.print(f"[red]Error:[/red] Path does not exist: {root_path}")
        return
    
    if not root_path.is_dir():
        console.print(f"[red]Error:[/red] Path is not a directory: {root_path}")
        return
    
    # Setup
    client = get_api_client()
    if not client.health_check():
        console.print("[red]Error:[/red] Cannot connect to Rice Search backend")
        return
    
    console.print(f"[green]Watching:[/green] {root_path}")
    console.print(f"[dim]Org ID: {org_id}[/dim]")
    
    ignore_rules = IgnoreRules(root_path)
    handler = IndexHandler(root_path, org_id, ignore_rules)
    
    # Initial scan
    if initial_scan:
        console.print("\n[blue]Initial scan...[/blue]")
        file_count = 0
        for file_path in root_path.rglob("*"):
            if handler._should_process(file_path):
                handler._index_file(file_path)
                file_count += 1
        console.print(f"[green]Scanned {file_count} files[/green]\n")
    
    # Start watching
    observer = Observer()
    observer.schedule(handler, str(root_path), recursive=True)
    observer.start()
    
    console.print("[green]Watching for changes... (Ctrl+C to stop)[/green]")
    
    try:
        while True:
            handler.process_pending()
            time.sleep(0.1)
    except KeyboardInterrupt:
        console.print("\n[yellow]Stopping...[/yellow]")
        observer.stop()
    
    observer.join()
    console.print("[green]Done.[/green]")
