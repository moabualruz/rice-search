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
        
        # FIX: We will save to 'backend/tmp' which is mounted in both at /app/tmp
        shared_path = f"/app/tmp/{file_id}{ext}"
        os.makedirs("/app/tmp", exist_ok=True)
        # Re-save to shared path since /tmp might not be shared
        with open(shared_path, "wb") as buffer:
             file.file.seek(0)
             shutil.copyfileobj(file.file, buffer)

        task = ingest_file_task.delay(shared_path, repo_name="upload")
        
        return {"status": "queued", "task_id": str(task.id), "file": file.filename}

    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
