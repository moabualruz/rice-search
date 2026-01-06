from fastapi import APIRouter, HTTPException, Query
from typing import List, Optional
from pydantic import BaseModel

from src.services.mcp.tools import handle_list_files, handle_read_file

router = APIRouter()

class FileListResponse(BaseModel):
    files: List[str]
    count: int

class FileContentResponse(BaseModel):
    path: str
    content: str
    language: str = "text"

@router.get("/list", response_model=FileListResponse)
async def list_files(
    org_id: str = "public",
    pattern: Optional[str] = None
):
    """
    List all indexed files.
    """
    files = await handle_list_files(org_id=org_id, pattern=pattern)
    return {
        "files": files,
        "count": len(files)
    }

@router.get("/content", response_model=FileContentResponse)
async def get_file_content(
    path: str = Query(..., description="Full path to file"),
    org_id: str = "public"
):
    """
    Get content of a specific file.
    """
    content = await handle_read_file(file_path=path, org_id=org_id)
    
    if content.startswith("File not found") or content.startswith("Error reading"):
        raise HTTPException(status_code=404, detail=content)
        
    # Simple extension-based language detection
    lang = "text"
    if "." in path:
        ext = path.split(".")[-1].lower()
        lang_map = {
            "py": "python", "js": "javascript", "ts": "typescript", 
            "go": "go", "rs": "rust", "java": "java", "cpp": "cpp",
            "c": "c", "h": "c", "json": "json", "md": "markdown",
            "yaml": "yaml", "yml": "yaml", "html": "html", "css": "css"
        }
        lang = lang_map.get(ext, "text")
    
    return {
        "path": path,
        "content": content,
        "language": lang
    }
