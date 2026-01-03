"""
Indexing Service.
Handles document processing and Qdrant operations.
All model inference via Xinference.
"""
import uuid
from typing import Dict, List, Optional
from qdrant_client.models import PointStruct, VectorParams, SparseVectorParams, SparseIndexParams, Distance, SparseVector

import hashlib
from src.core.config import settings
from src.services.ingestion.parser import DocumentParser
from src.services.ingestion.chunker import DocumentChunker
from src.services.ingestion.ast_parser import get_ast_parser
from src.services.search.sparse import sparse_embed
from src.services.search.retriever import embed_texts


class Indexer:
    """Core indexing logic, decoupled from Celery."""
    
    def __init__(self, qdrant_client):
        self.qdrant = qdrant_client
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

    def ingest_file(self, file_path: str, display_path: str, repo_name: str, org_id: str) -> Dict:
        """Ingest a single file.
        
        Args:
            file_path: Actual path to read file from
            display_path: Client-side path for metadata storage
        """
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
                            "file_path": display_path,
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
                "file_path": display_path,
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
        
        # Dense embeddings via Xinference
        print("Indexer: Embedding dense via Xinference...")
        dense_embeddings = embed_texts(contents)
        print("Indexer: Dense embedding done.")
        
        # Upsert
        self.ensure_collection()
        points = []
        
        for i, chunk in enumerate(chunks):
            # Dense (already list format from Xinference)
            vectors = {"default": dense_embeddings[i]}
            
            # Sparse
            try:
                sp_vec = sparse_embed(chunk["content"])
                vectors["sparse"] = SparseVector(
                    indices=sp_vec.indices,
                    values=sp_vec.values
                )
            except Exception as e:
                print(f"Sparse embedding failed: {e}")

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

