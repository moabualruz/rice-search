import os
import uuid
from typing import List
from sentence_transformers import SentenceTransformer
from qdrant_client.models import PointStruct

from src.worker import celery_app
from src.db.qdrant import get_qdrant_client
from src.services.ingestion.parser import DocumentParser
from src.services.ingestion.chunker import DocumentChunker

# Load model globally per worker process (Lazy loading recommended but this is simple)
# We use 'all-MiniLM-L6-v2' for speed/quality balance locally
model = SentenceTransformer('all-MiniLM-L6-v2') 
qdrant = get_qdrant_client()
chunker = DocumentChunker()

COLLECTION_NAME = "rice_codebase"

@celery_app.task(bind=True)
def ingest_file_task(self, file_path: str, repo_name: str = "default", org_id: str = "public"):
    """
    Full pipeline: Parse -> Chunk -> Embed -> Upsert
    """
    self.update_state(state='STARTED', meta={'step': 'Parsing'})
    
    # 1. Parse
    try:
        text = DocumentParser.parse_file(file_path)
    except Exception as e:
        return {"status": "error", "message": f"Parsing failed: {str(e)}"}

    if not text.strip():
        return {"status": "skipped", "message": "Empty file"}

    # 2. Chunk
    self.update_state(state='STARTED', meta={'step': 'Chunking'})
    base_metadata = {
        "file_path": file_path,
        "repo_name": repo_name,
        "org_id": org_id, # Phase 7 Multi-tenancy
        "doc_id": str(uuid.uuid4())
    }
    chunks = chunker.chunk_text(text, base_metadata)

    # 3. Embed
    self.update_state(state='STARTED', meta={'step': 'Embedding'})
    contents = [c["content"] for c in chunks]
    embeddings = model.encode(contents)

    # 4. Upsert to Qdrant
    self.update_state(state='STARTED', meta={'step': 'Indexing'})
    points = []
    
    # Ensure collection exists
    try:
        qdrant.get_collection(COLLECTION_NAME)
    except Exception:
        qdrant.create_collection(
            collection_name=COLLECTION_NAME,
            vectors_config={"size": 384, "distance": "Cosine"}
        )
    
    for i, chunk in enumerate(chunks):
        points.append(PointStruct(
            id=str(uuid.uuid4()),
            vector=embeddings[i].tolist(),
            payload={
                "text": chunk["content"], # Store text in payload for retrieval
                **chunk["metadata"],
                "chunk_index": chunk["chunk_index"]
            }
        ))
    
    qdrant.upsert(
        collection_name=COLLECTION_NAME,
        points=points
    )

    # Cleanup temp file if needed (skipping for now)
    
    return {
        "status": "success", 
        "chunks_indexed": len(points), 
        "file_path": file_path
    }
