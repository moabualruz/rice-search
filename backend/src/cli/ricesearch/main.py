"""
ricesearch - Rice Search Client CLI.

A command-line tool for indexing and searching code with Rice Search backend.
"""

import typer
from typing import Optional
from rich.console import Console

from src.cli.ricesearch.config import get_config
from src.cli.ricesearch.search import search_command
from src.cli.ricesearch.watch import watch_command

app = typer.Typer(
    name="ricesearch",
    help="Rice Search Client - local code search and indexing",
    add_completion=False
)
console = Console()


@app.command()
def search(
    query: str = typer.Argument(..., help="Search query"),
    limit: int = typer.Option(10, "--limit", "-n", help="Max number of results"),
    org_id: Optional[str] = typer.Option(None, "--org-id", "-o", help="Organization ID"),
    hybrid: bool = typer.Option(True, "--hybrid/--no-hybrid", help="Use hybrid search"),
    no_color: bool = typer.Option(False, "--no-color", help="Disable colored output")
):
    """
    Search indexed code and documents.
    
    Output format: file:line:content
    """
    search_command(
        query=query,
        limit=limit,
        org_id=org_id,
        hybrid=hybrid,
        no_color=no_color
    )


@app.command()
def watch(
    path: str = typer.Argument(".", help="Directory to watch"),
    org_id: Optional[str] = typer.Option(None, "--org-id", "-o", help="Organization ID"),
    no_initial: bool = typer.Option(False, "--no-initial", help="Skip initial scan")
):
    """
    Watch directory and automatically index changes.
    
    Monitors files for changes and indexes them via Rice Search backend.
    Respects .gitignore and .riceignore patterns.
    """
    watch_command(
        path=path,
        org_id=org_id,
        initial_scan=not no_initial
    )


@app.command()
def config(
    action: str = typer.Argument("show", help="Action: show, set"),
    key: Optional[str] = typer.Argument(None, help="Config key"),
    value: Optional[str] = typer.Argument(None, help="Config value")
):
    """
    View or edit CLI configuration.
    
    Examples:
        ricesearch config show
        ricesearch config set backend_url http://localhost:8000
        ricesearch config set org_id myorg
    """
    cfg = get_config()
    
    if action == "show":
        console.print("\n[bold]Rice Search Client configuration:[/bold]\n")
        for k, v in cfg.show().items():
            console.print(f"  {k}: {v}")
        console.print(f"\n[dim]Config file: {cfg.CONFIG_FILE}[/dim]")
    
    elif action == "set":
        if not key or value is None:
            console.print("[red]Error:[/red] Usage: ricesearch config set <key> <value>")
            return
        
        cfg.set(key, value)
        console.print(f"[green]Set {key} = {value}[/green]")
    
    else:
        console.print(f"[red]Unknown action:[/red] {action}")


@app.command()
def version():
    """Show version information."""
    console.print("Rice Search Client v0.1.0")
    console.print("[dim]Part of the Rice Search platform[/dim]")


def main():
    """CLI entry point."""
    app()


if __name__ == "__main__":
    main()
