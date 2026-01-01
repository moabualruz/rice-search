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
        self.sparse_embedder = sparse_embedder
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
                    "default": VectorParams(size=384, distance=Distance.COSINE)
                },
                sparse_vectors_config={
                    "sparse": SparseVectorParams(index=SparseIndexParams(on_disk=False))
                } if settings.SPARSE_ENABLED else None
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
        # (Simplified AST logic for now, add back if needed)
        
        # 3. Embed
        contents = [c["content"] for c in chunks]
        dense_embeddings = self.model.encode(contents)
        
        sparse_embeddings = []
        if self.sparse_embedder:
             sparse_embeddings = self.sparse_embedder.embed_batch(contents)

        # 4. Upsert
        self.ensure_collection()
        points = []
        
        for i, chunk in enumerate(chunks):
            vectors = {"default": dense_embeddings[i].tolist()}
            if sparse_embeddings:
                sparse_vec = sparse_embeddings[i]
                vectors["sparse"] = {
                     "indices": sparse_vec.indices.tolist() if hasattr(sparse_vec.indices, 'tolist') else sparse_vec.indices,
                     "values": sparse_vec.values.tolist() if hasattr(sparse_vec.values, 'tolist') else sparse_vec.values
                }
            
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
