"""
Rice Search Client search command implementation.

Provides grep-like search output from Rice Search backend.
"""

from typing import Optional
from rich.console import Console
from rich.text import Text

from src.cli.ricesearch.api_client import get_api_client
from src.cli.ricesearch.config import get_config

console = Console()


def search_command(
    query: str,
    limit: int = 10,
    org_id: Optional[str] = None,
    hybrid: Optional[bool] = None,
    no_color: bool = False
):
    """
    Search indexed content with grep-like output.
    
    Args:
        query: Search query
        limit: Max results
        org_id: Organization ID (default from config)
        hybrid: Use hybrid search (default from config)
        no_color: Disable colored output
    """
    config = get_config()
    
    # Use config defaults if not specified
    org_id = org_id or config.org_id
    hybrid = hybrid if hybrid is not None else config.hybrid_search
    
    # Get API client
    client = get_api_client()
    
    # Check backend health
    if not client.health_check():
        console.print("[red]Error:[/red] Cannot connect to Rice Search backend")
        console.print(f"[dim]Backend URL: {client.base_url}[/dim]")
        return
    
    # Search
    console.print(f"[dim]Searching for:[/dim] {query}")
    results = client.search(
        query=query,
        limit=limit,
        org_id=org_id,
        hybrid=hybrid
    )
    
    if not results:
        console.print("[yellow]No results found.[/yellow]")
        return
    
    # Output in grep-like format
    console.print()
    for result in results:
        text = result.get("text", "")
        metadata = result.get("metadata", {})
        score = result.get("score", 0.0)
        
        file_path = metadata.get("file_path", "unknown")
        chunk_index = metadata.get("chunk_index", 0)
        
        # Truncate and clean text
        text_preview = text.strip().replace('\n', ' ')[:200]
        
        if no_color:
            print(f"{file_path}:{chunk_index}: {text_preview}")
        else:
            # Colorized output
            line = Text()
            line.append(file_path, style="cyan")
            line.append(":", style="dim")
            line.append(str(chunk_index), style="green")
            line.append(":", style="dim")
            line.append(" ")
            line.append(text_preview, style="white")
            line.append(f" ({score:.3f})", style="dim")
            console.print(line)
    
    console.print()
    console.print(f"[dim]Found {len(results)} results (hybrid={hybrid})[/dim]")
