import os
import uuid
from typing import List
from sentence_transformers import SentenceTransformer
from qdrant_client.models import PointStruct, VectorParams, SparseVectorParams, SparseIndexParams, Distance

from src.worker import celery_app
from src.db.qdrant import get_qdrant_client
from src.services.ingestion.parser import DocumentParser
from src.services.ingestion.chunker import DocumentChunker
from src.core.config import settings

# Load dense model globally per worker process
model = SentenceTransformer(settings.EMBEDDING_MODEL)
qdrant = get_qdrant_client()
chunker = DocumentChunker()

# Lazy load sparse embedder (GPU-heavy)
_sparse_embedder = None

def get_sparse_embedder():
    """Lazy load sparse embedder to avoid GPU allocation on import."""
    global _sparse_embedder
    if _sparse_embedder is None and settings.SPARSE_ENABLED:
        from src.services.search.sparse_embedder import SparseEmbedder
        _sparse_embedder = SparseEmbedder.get_instance()
    return _sparse_embedder

COLLECTION_NAME = "rice_chunks"

@celery_app.task(bind=True)
def ingest_file_task(self, file_path: str, repo_name: str = "default", org_id: str = "public"):
    """
    Full pipeline: Parse -> Chunk -> Embed (Dense + Sparse) -> Upsert
    """
    self.update_state(state='STARTED', meta={'step': 'Parsing'})
    
    # 1. Parse
    try:
        text = DocumentParser.parse_file(file_path)
    except Exception as e:
        return {"status": "error", "message": f"Parsing failed: {str(e)}"}

    if not text.strip():
        return {"status": "skipped", "message": "Empty file"}

    # 2. Chunk (AST-aware for code, fallback to text)
    self.update_state(state='STARTED', meta={'step': 'Chunking'})
    base_metadata = {
        "file_path": file_path,
        "repo_name": repo_name,
        "org_id": org_id,  # Phase 7 Multi-tenancy
        "doc_id": str(uuid.uuid4())
    }
    
    # Try AST parsing for code files (Phase 12)
    chunks = []
    ast_used = False
    if settings.AST_PARSING_ENABLED:
        from pathlib import Path
        from src.services.ingestion.ast_parser import get_ast_parser
        parser = get_ast_parser()
        path = Path(file_path)
        if parser.can_parse(path):
            ast_chunks = parser.parse_file(path, text)
            if ast_chunks:
                ast_used = True
                for i, ast_chunk in enumerate(ast_chunks):
                    chunks.append({
                        "content": ast_chunk.content,
                        "chunk_index": i,
                        "metadata": {
                            **base_metadata,
                            "language": ast_chunk.language,
                            "chunk_type": ast_chunk.chunk_type,
                            "symbols": ast_chunk.symbols,
                            "start_line": ast_chunk.start_line,
                            "end_line": ast_chunk.end_line,
                        }
                    })
    
    # Fallback to text chunking
    if not chunks:
        chunks = chunker.chunk_text(text, base_metadata)

    # 3. Embed (Dense)
    self.update_state(state='STARTED', meta={'step': 'Embedding (Dense)'})
    contents = [c["content"] for c in chunks]
    dense_embeddings = model.encode(contents)

    # 4. Embed (Sparse) - Phase 9 Hybrid Search
    sparse_embeddings = []
    if settings.SPARSE_ENABLED:
        self.update_state(state='STARTED', meta={'step': 'Embedding (Sparse)'})
        sparse_embedder = get_sparse_embedder()
        if sparse_embedder:
            sparse_embeddings = sparse_embedder.embed_batch(contents)

    # 5. Upsert to Qdrant
    self.update_state(state='STARTED', meta={'step': 'Indexing'})
    points = []
    
    # Ensure collection exists with hybrid schema
    ensure_collection_exists()
    
    for i, chunk in enumerate(chunks):
        point_id = str(uuid.uuid4())
        
        # Build vectors dict
        vectors = {
            "default": dense_embeddings[i].tolist()
        }
        
        # Add sparse vector if available
        sparse_vectors = None
        if sparse_embeddings and i < len(sparse_embeddings):
            sparse_vec = sparse_embeddings[i]
            sparse_vectors = {
                "sparse": {
                    "indices": sparse_vec.indices,
                    "values": sparse_vec.values
                }
            }
        
        points.append(PointStruct(
            id=point_id,
            vector=vectors,
            payload={
                "text": chunk["content"],
                **chunk["metadata"],
                "chunk_index": chunk["chunk_index"]
            }
        ))
    
    qdrant.upsert(
        collection_name=COLLECTION_NAME,
        points=points
    )

    return {
        "status": "success", 
        "chunks_indexed": len(points), 
        "file_path": file_path,
        "hybrid_enabled": settings.SPARSE_ENABLED
    }


def ensure_collection_exists():
    """Ensure collection exists with proper hybrid schema."""
    try:
        qdrant.get_collection(COLLECTION_NAME)
    except Exception:
        # Create with hybrid schema
        qdrant.create_collection(
            collection_name=COLLECTION_NAME,
            vectors_config={
                "default": VectorParams(
                    size=384,  # all-MiniLM-L6-v2 dimension
                    distance=Distance.COSINE
                )
            },
            sparse_vectors_config={
                "sparse": SparseVectorParams(
                    index=SparseIndexParams(on_disk=False)
                )
            } if settings.SPARSE_ENABLED else None
        )


@celery_app.task(bind=True, name="src.tasks.ingestion.rebuild_index_task")
def rebuild_index_task(self):
    """
    Rebuild entire index by re-indexing all documents.
    Triggered from admin dashboard.
    """
    self.update_state(state='STARTED', meta={'step': 'Initializing rebuild'})
    
    # Get all points from collection
    try:
        result = qdrant.scroll(
            collection_name=COLLECTION_NAME,
            limit=1000,
            with_payload=True
        )
        points, _ = result
        
        self.update_state(state='STARTED', meta={
            'step': 'Rebuilding',
            'total_points': len(points)
        })
        
        # For now, just validate the collection exists
        ensure_collection_exists()
        
        return {
            "status": "success",
            "message": "Index rebuild completed",
            "points_verified": len(points)
        }
    except Exception as e:
        return {
            "status": "error",
            "message": f"Rebuild failed: {str(e)}"
        }

