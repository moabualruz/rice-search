import shutil
import os
import uuid
from fastapi import APIRouter, UploadFile, File, HTTPException, Depends
from typing import Dict
from src.tasks.ingestion import ingest_file_task
from src.api.v1.dependencies import verify_admin

router = APIRouter()

TEMP_DIR = "/tmp/ingest" 
os.makedirs(TEMP_DIR, exist_ok=True)

@router.post("/file", status_code=202)
async def upload_file(
    file: UploadFile = File(...),
    admin: dict = Depends(verify_admin)
) -> Dict:
    """
    Upload a file to ingest into the Vector DB.
    """
    try:
        # Create unique temp path
        file_id = str(uuid.uuid4())
        ext = os.path.splitext(file.filename)[1]
        save_path = os.path.join(TEMP_DIR, f"{file_id}{ext}")
        
        # Save to disk (In prod, upload to MinIO here)
        with open(save_path, "wb") as buffer:
            shutil.copyfileobj(file.file, buffer)
            
        # Dispatch Celery Task
        # Note: Worker needs access to this path. 
        # In Docker Compose, backend-api and backend-worker mount the same volume.
        # But /tmp is usually ephemeral. For specific implementation here, 
        # we should use a shared volume or MinIO.
        # For Phase 2 iron core, let's assume shared volume for simplicity
        # or just pass content if small. 
        
        # FIX: Ensure path is accessible by both API and Worker
        # Use simple local tmp relative to valid CWD or /tmp fallback
        base_tmp = os.getenv("SHARED_TMP_DIR", os.path.join(os.getcwd(), "tmp"))
        os.makedirs(base_tmp, exist_ok=True)
        shared_path = os.path.join(base_tmp, f"{file_id}{ext}")
        # Re-save to shared path since /tmp might not be shared
        with open(shared_path, "wb") as buffer:
             file.file.seek(0)
             shutil.copyfileobj(file.file, buffer)

        # Extract org_id from authenticated user (injected by verify_admin -> get_current_user)
        # Default to 'public' if somehow missing (though dependency ensures it)
        org_id = admin.get("org_id", "public")

        task = ingest_file_task.delay(shared_path, repo_name="upload", org_id=org_id)
        
        return {"status": "queued", "task_id": str(task.id), "file": file.filename}

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
