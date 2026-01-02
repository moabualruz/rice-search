"""
Indexing Service.
Handles document processing and Qdrant operations.
"""
import uuid
from typing import Dict, List, Optional
from qdrant_client.models import PointStruct, VectorParams, SparseVectorParams, SparseIndexParams, Distance

from src.core.config import settings
from src.services.ingestion.parser import DocumentParser
from src.services.ingestion.chunker import DocumentChunker

class Indexer:
    """Core indexing logic, decoupled from Celery."""
    
    def __init__(self, qdrant_client, dense_model, sparse_embedder=None):
        self.qdrant = qdrant_client
        self.model = dense_model
        # User requested removal of client-side sparse embedder
        self.sparse_embedder = None
        self.chunker = DocumentChunker()
        self.collection_name = "rice_chunks"
        
    def ensure_collection(self):
        """Ensure collection exists."""
        try:
            self.qdrant.get_collection(self.collection_name)
        except Exception:
            self.qdrant.create_collection(
                collection_name=self.collection_name,
                vectors_config={
                    "default": VectorParams(size=768, distance=Distance.COSINE)
                },
                # Sparse config removed as per user request (server-side managed)
            )

    def ingest_file(self, file_path: str, repo_name: str, org_id: str) -> Dict:
        """Ingest a single file."""
        # 1. Parse
        try:
            text = DocumentParser.parse_file(file_path)
        except Exception as e:
            return {"status": "error", "message": f"Parsing failed: {str(e)}"}

        if not text.strip():
            return {"status": "skipped", "message": "Empty file"}

        # 2. Chunk
        base_metadata = {
            "file_path": file_path,
            "repo_name": repo_name,
            "org_id": org_id,
            "doc_id": str(uuid.uuid4())
        }
        
        chunks = self.chunker.chunk_text(text, base_metadata)
        
        # 3. Embed (Dense Only)
        contents = [c["content"] for c in chunks]
        dense_embeddings = self.model.encode(contents)
        
        # 4. Upsert
        self.ensure_collection()
        points = []
        
        for i, chunk in enumerate(chunks):
            vectors = {"default": dense_embeddings[i].tolist()}
            # Sparse vectors removed
            
            points.append(PointStruct(
                id=str(uuid.uuid4()),
                vector=vectors,
                payload={
                    "text": chunk["content"],
                    **chunk["metadata"],
                    "chunk_index": chunk["chunk_index"]
                }
            ))
            
        self.qdrant.upsert(
            collection_name=self.collection_name,
            points=points
        )
        
        return {"status": "success", "chunks_indexed": len(points)}
