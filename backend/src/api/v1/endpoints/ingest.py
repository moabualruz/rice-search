import shutil
import os
import uuid
from fastapi import APIRouter, UploadFile, File, HTTPException, Depends, Form
from typing import Dict, Optional
from src.tasks.ingestion import ingest_file_task
from src.api.v1.dependencies import verify_admin

router = APIRouter()

TEMP_DIR = "/tmp/ingest" 
os.makedirs(TEMP_DIR, exist_ok=True)

@router.post("/file", status_code=202)
async def upload_file(
    file: UploadFile = File(...),
    org_id: Optional[str] = Form("public"),
    admin: dict = Depends(verify_admin)
) -> Dict:
    """
    Upload a file to ingest into the Vector DB.
    """
    try:
        # Original path from client (sent as filename in multipart)
        original_path = file.filename or "unknown"
        
        # Create unique temp path for processing
        file_id = str(uuid.uuid4())
        ext = os.path.splitext(original_path)[1]
        
        # Use shared temp dir accessible by both API and Worker
        base_tmp = os.getenv("SHARED_TMP_DIR", "/tmp/ingest")
        os.makedirs(base_tmp, exist_ok=True)
        temp_path = os.path.join(base_tmp, f"{file_id}{ext}")
        
        # Save file to temp location
        with open(temp_path, "wb") as buffer:
            shutil.copyfileobj(file.file, buffer)

        # Get org_id from form or authenticated user
        effective_org_id = org_id or admin.get("org_id", "public")

        # Dispatch Celery Task with ORIGINAL path for metadata
        task = ingest_file_task.delay(
            temp_path,           # actual file location for reading
            original_path,       # original client path for metadata
            repo_name="default",
            org_id=effective_org_id
        )
        
        return {"status": "queued", "task_id": str(task.id), "file": original_path}

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

