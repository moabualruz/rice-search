"""
MCP Tool Handlers for Rice Search.

Implements the three core tools:
- search: Query indexed content
- read_file: Retrieve full file content
- list_files: List indexed files
"""

import logging
from typing import List, Dict, Any, Optional
from qdrant_client.models import Filter, FieldCondition, MatchValue

from src.services.search.retriever import Retriever
from src.db.qdrant import get_qdrant_client
from src.core.config import settings

logger = logging.getLogger(__name__)

COLLECTION_NAME = "rice_chunks"


async def handle_search(
    query: str,
    limit: int = 10,
    org_id: str = "public",
    hybrid: Optional[bool] = None
) -> List[Dict[str, Any]]:
    """
    Handle search tool request.
    
    Args:
        query: Search query
        limit: Max results
        org_id: Organization filter
        hybrid: Enable hybrid search
        
    Returns:
        List of search results
    """
    try:
        results = Retriever.search(
            query=query,
            limit=limit,
            org_id=org_id,
            hybrid=hybrid
        )
        return results
    except Exception as e:
        logger.error(f"Search error: {e}")
        return []


async def handle_read_file(
    file_path: str,
    org_id: str = "public"
) -> str:
    """
    Handle read_file tool request.
    
    Retrieves the full content of an indexed file by aggregating all chunks.
    
    Args:
        file_path: Path to file
        org_id: Organization filter
        
    Returns:
        File content as string
    """
    try:
        qdrant = get_qdrant_client()
        
        # Build filter
        query_filter = Filter(
            must=[
                FieldCondition(
                    key="org_id",
                    match=MatchValue(value=org_id)
                ),
                FieldCondition(
                    key="file_path",
                    match=MatchValue(value=file_path)
                )
            ]
        )
        
        # Scroll through all matching points
        results = qdrant.scroll(
            collection_name=COLLECTION_NAME,
            scroll_filter=query_filter,
            limit=1000,  # Should be enough for most files
            with_payload=True,
            with_vectors=False
        )
        
        if not results or not results[0]:
            return f"File not found: {file_path}"
        
        # Sort chunks by chunk_index and concatenate
        chunks = sorted(
            results[0],
            key=lambda x: x.payload.get("chunk_index", 0)
        )
        
        content = "\n".join([
            chunk.payload.get("text", "")
            for chunk in chunks
        ])
        
        return content
        
    except Exception as e:
        logger.error(f"Read file error: {e}")
        return f"Error reading file: {str(e)}"


async def handle_list_files(
    org_id: str = "public",
    pattern: Optional[str] = None
) -> List[str]:
    """
    Handle list_files tool request.
    
    Lists all unique file paths in the index.
    
    Args:
        org_id: Organization filter
        pattern: Optional glob pattern for filtering
        
    Returns:
        List of file paths
    """
    try:
        qdrant = get_qdrant_client()
        
        # Build filter
        query_filter = Filter(
            must=[
                FieldCondition(
                    key="org_id",
                    match=MatchValue(value=org_id)
                )
            ]
        )
        
        # Scroll through all points
        results = qdrant.scroll(
            collection_name=COLLECTION_NAME,
            scroll_filter=query_filter,
            limit=10000,
            with_payload=["file_path"],
            with_vectors=False
        )
        
        if not results or not results[0]:
            return []
        
        # Extract unique file paths
        file_paths = set()
        for point in results[0]:
            path = point.payload.get("file_path")
            if path:
                # Apply pattern filter if provided
                if pattern:
                    import fnmatch
                    if fnmatch.fnmatch(path, pattern):
                        file_paths.add(path)
                else:
                    file_paths.add(path)
        
        return sorted(list(file_paths))
        
    except Exception as e:
        logger.error(f"List files error: {e}")
        return []
