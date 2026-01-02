"""
Indexing Service.
Handles document processing and Qdrant operations.
"""
import uuid
from typing import Dict, List, Optional
from qdrant_client.models import PointStruct, VectorParams, SparseVectorParams, SparseIndexParams, Distance, SparseVector

import hashlib
from src.core.config import settings
from src.services.ingestion.parser import DocumentParser
from src.services.ingestion.chunker import DocumentChunker
from src.services.ingestion.ast_parser import get_ast_parser
from src.services.search.sparse import get_sparse_embedder



class Indexer:
    """Core indexing logic, decoupled from Celery."""
    
    def __init__(self, qdrant_client, dense_model, sparse_embedder=None):
        self.qdrant = qdrant_client
        self.model = dense_model
        self.sparse_embedder = sparse_embedder or get_sparse_embedder()
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
                    "default": VectorParams(size=settings.EMBEDDING_DIM, distance=Distance.COSINE)
                },
                sparse_vectors_config={
                    "sparse": SparseVectorParams(
                        index=SparseIndexParams(
                            on_disk=False
                        )
                    )
                }
            )

    def ingest_file(self, file_path: str, repo_name: str, org_id: str) -> Dict:
        """Ingest a single file."""
        import pathlib
        
        path_obj = pathlib.Path(file_path)
        ast_parser = get_ast_parser()
        
        chunks = []
        is_ast = False
        
        # 1. Try AST Parsing
        if ast_parser.can_parse(path_obj):
            try:
                ast_chunks = ast_parser.parse_file(path_obj)
                # Convert AST chunks to dicts compatible with ingestion
                for i, c in enumerate(ast_chunks):
                    chunks.append({
                        "content": c.content,
                        "metadata": {
                            "file_path": file_path,
                            "repo_name": repo_name,
                            "org_id": org_id,
                            "doc_id": str(uuid.uuid4()), # Doc ID can be random or hashed
                            "language": c.language,
                            "chunk_type": c.chunk_type,
                            "symbols": c.symbols,
                            "start_line": c.start_line,
                            "end_line": c.end_line
                        },
                        "chunk_index": i
                    })
                is_ast = True
            except Exception as e:
                return {"status": "error", "message": f"AST Parsing failed: {str(e)}"}
        
        # 2. Fallback to Standard Parsing
        if not chunks:
            try:
                text = DocumentParser.parse_file(file_path)
            except Exception as e:
                return {"status": "error", "message": f"Parsing failed: {str(e)}"}

            if not text.strip():
                return {"status": "skipped", "message": "Empty file"}

            base_metadata = {
                "file_path": file_path,
                "repo_name": repo_name,
                "org_id": org_id,
                "doc_id": str(uuid.uuid4()),
                "chunk_type": "text", # Default for fallback
                "symbols": [],
                "start_line": 0,
                "end_line": 0
            }
            
            chunks = self.chunker.chunk_text(text, base_metadata)
        
        if not chunks:
             print("Indexer: No chunks generated.")
             return {"status": "skipped", "message": "No chunks generated"}

        print(f"Indexer: Generated {len(chunks)} chunks (AST={is_ast})")

        # 3. Embed
        contents = [c["content"] for c in chunks]
        
        # Dense
        print("Indexer: Embedding dense...")
        dense_embeddings = self.model.encode(contents)
        print("Indexer: Dense embedding done.")
        
        # Upsert
        self.ensure_collection()
        points = []
        
        for i, chunk in enumerate(chunks):
            # Dense
            vectors = {"default": dense_embeddings[i].tolist()}
            
            # Sparse
            try:
                if self.sparse_embedder:
                    sp_vec = self.sparse_embedder.embed(chunk["content"])
                    vectors["sparse"] = SparseVector(
                        indices=sp_vec.indices,
                        values=sp_vec.values
                    )
            except Exception as e:
                # Log error but proceed? Or fail? STRICT TDD implies fail if requirement.
                # But typically we log warning for partial failure in batch.
                print(f"Sparse embedding failed: {e}") 
                pass

            # Deterministic ID generation
            # Hash: file_path + content + chunk_index
            content_hash = hashlib.sha256(
                f"{file_path}:{chunk['chunk_index']}:{chunk['content']}".encode()
            ).hexdigest()
            # Convert hash to UUID
            chunk_id = str(uuid.UUID(content_hash[:32]))
            
            points.append(PointStruct(
                id=chunk_id,
                vector=vectors,
                payload={
                    "text": chunk["content"],
                    **chunk["metadata"],
                    "chunk_index": chunk["chunk_index"],
                    "content_hash": content_hash
                }
            ))
            
        print(f"Indexer: Upserting {len(points)} points to {self.collection_name}...")
        self.qdrant.upsert(
            collection_name=self.collection_name,
            points=points
        )
        print("Indexer: Upsert complete.")
        
        return {"status": "success", "chunks_indexed": len(points), "mode": "ast" if is_ast else "fallback"}

