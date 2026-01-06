"""
Indexing Service.

Handles document processing and multi-representation storage:
1. Dense embeddings (BentoML)
2. SPLADE sparse vectors
3. BM42 sparse vectors
4. BM25 index (Tantivy)

All representations computed at INDEX TIME and persisted.
"""

import uuid
import hashlib
import logging
from typing import Dict, List, Optional

from qdrant_client.models import (
    PointStruct,
    VectorParams,
    SparseVectorParams,
    SparseIndexParams,
    Distance,
    SparseVector,
)

from src.core.config import settings
from src.services.ingestion.parser import DocumentParser
from src.services.ingestion.chunker import DocumentChunker
from src.services.ingestion.ast_parser import get_ast_parser
from src.services.search.retriever import embed_texts

logger = logging.getLogger(__name__)


class Indexer:
    """
    Core indexing logic for triple retrieval system.
    
    Generates four representations for each chunk:
    1. Dense embeddings (for semantic search)
    2. SPLADE sparse vectors (neural sparse)
    3. BM42 sparse vectors (Qdrant hybrid)
    4. BM25 index (Tantivy lexical)
    """
    
    def __init__(self, qdrant_client):
        self.qdrant = qdrant_client
        self.chunker = DocumentChunker()
        self.collection_name = settings.COLLECTION_PREFIX
        
        # Lazy-loaded encoders
        self._splade_encoder = None
        self._bm42_encoder = None
        self._tantivy_client = None
    
    @property
    def splade_encoder(self):
        """Lazy-load SPLADE encoder."""
        if self._splade_encoder is None:
            try:
                from src.services.retrieval.splade_encoder import get_splade_encoder
                self._splade_encoder = get_splade_encoder()
            except Exception as e:
                logger.warning(f"SPLADE encoder not available: {e}")
        return self._splade_encoder
    
    @property
    def bm42_encoder(self):
        """Lazy-load BM42 encoder."""
        if self._bm42_encoder is None:
            try:
                from src.services.retrieval.bm42_encoder import get_bm42_encoder
                self._bm42_encoder = get_bm42_encoder()
            except Exception as e:
                logger.warning(f"BM42 encoder not available: {e}")
        return self._bm42_encoder
    
    @property
    def tantivy_client(self):
        """Lazy-load Tantivy client."""
        if self._tantivy_client is None:
            try:
                from src.services.retrieval.tantivy_client import get_tantivy_client
                self._tantivy_client = get_tantivy_client()
            except Exception as e:
                logger.warning(f"Tantivy client not available: {e}")
        return self._tantivy_client
    
    def ensure_collection(self):
        """
        Ensure collection exists with proper schema for triple retrieval.
        
        Schema:
        - dense: Dense vector (768 dims for bge-base, cosine)
        - splade: Sparse vector
        - bm42: Sparse vector
        """
        try:
            self.qdrant.get_collection(self.collection_name)
            logger.info(f"Collection {self.collection_name} exists")
        except Exception:
            logger.info(f"Creating collection {self.collection_name}")
            embedding_dim = settings.get("models.embedding.fallback_dimension", settings.EMBEDDING_DIM)
            self.qdrant.create_collection(
                collection_name=self.collection_name,
                vectors_config={
                    "dense": VectorParams(
                        size=embedding_dim,
                        distance=Distance.COSINE
                    )
                },
                sparse_vectors_config={
                    "splade": SparseVectorParams(
                        index=SparseIndexParams(on_disk=False)
                    ),
                    "bm42": SparseVectorParams(
                        index=SparseIndexParams(on_disk=False)
                    )
                }
            )
            logger.info(f"Collection {self.collection_name} created with triple vector schema")
    
    def ingest_file(
        self,
        file_path: str,
        display_path: str,
        repo_name: str,
        org_id: str,
        minio_bucket: str = None,
        minio_object_name: str = None,
    ) -> Dict:
        """
        Ingest a single file with all representations.
        
        Args:
            file_path: Actual path to read file from
            display_path: Client-side path for metadata (client_system_path)
            repo_name: Repository name
            org_id: Organization ID
            minio_bucket: MinIO bucket (if stored)
            minio_object_name: MinIO object key (if stored)
            
        Returns:
            Dict with status and statistics
        """
        import pathlib
        
        path_obj = pathlib.Path(file_path)
        ast_parser = get_ast_parser()
        doc_id = str(uuid.uuid4())
        
        chunks = []
        is_ast = False
        
        # 1. Try AST Parsing
        if ast_parser.can_parse(path_obj):
            try:
                ast_chunks = ast_parser.parse_file(path_obj)
                for i, c in enumerate(ast_chunks):
                    chunks.append({
                        "content": c.content,
                        "metadata": {
                            "client_system_path": display_path,
                            "file_path": display_path,  # Legacy compatibility
                            "repo_name": repo_name,
                            "org_id": org_id,
                            "doc_id": doc_id,
                            "language": c.language,
                            "chunk_type": c.chunk_type,
                            "symbols": c.symbols,
                            "start_line": c.start_line,
                            "end_line": c.end_line,
                            "minio_bucket": minio_bucket,
                            "minio_object_name": minio_object_name,
                        },
                        "chunk_index": i
                    })
                is_ast = True
            except Exception as e:
                logger.error(f"AST Parsing failed: {e}")
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
                "client_system_path": display_path,
                "file_path": display_path,
                "repo_name": repo_name,
                "org_id": org_id,
                "doc_id": doc_id,
                "chunk_type": "text",
                "symbols": [],
                "start_line": 0,
                "end_line": 0,
                "minio_bucket": minio_bucket,
                "minio_object_name": minio_object_name,
            }
            
            chunks = self.chunker.chunk_text(text, base_metadata)
        
        if not chunks:
            logger.info("No chunks generated")
            return {"status": "skipped", "message": "No chunks generated"}

        logger.info(f"Generated {len(chunks)} chunks (AST={is_ast})")
        
        # 3. Generate all representations
        # Extract file name for enhanced indexing
        import os
        file_name = os.path.basename(display_path)

        # Enhance content with file path/name for better searchability
        # This allows searches for file names to work properly
        enhanced_contents = []
        for c in chunks:
            # Prepend file metadata to make file names searchable
            enhanced = f"File: {file_name}\nPath: {display_path}\n\n{c['content']}"
            enhanced_contents.append(enhanced)

        # Use enhanced contents for embedding (file path is now searchable)
        contents = enhanced_contents

        # 3a. Dense embeddings (BentoML)
        logger.info("Generating dense embeddings...")
        try:
            dense_embeddings = embed_texts(contents)
        except Exception as e:
            logger.error(f"Dense embedding failed: {e}")
            return {"status": "error", "message": f"Dense embedding failed: {e}"}
        
        # 3b. SPLADE sparse vectors
        splade_vectors = []
        if self.splade_encoder:
            logger.info("Generating SPLADE vectors...")
            try:
                splade_vectors = self.splade_encoder.encode(contents)
            except Exception as e:
                logger.warning(f"SPLADE encoding failed: {e}")
        
        # 3c. BM42 sparse vectors
        bm42_vectors = []
        if self.bm42_encoder:
            logger.info("Generating BM42 vectors...")
            try:
                bm42_vectors = self.bm42_encoder.encode(contents)
            except Exception as e:
                logger.warning(f"BM42 encoding failed: {e}")
        
        # 4. Prepare Qdrant points
        self.ensure_collection()
        points = []
        chunk_ids = []
        
        for i, chunk in enumerate(chunks):
            # Deterministic chunk ID
            content_hash = hashlib.sha256(
                f"{file_path}:{chunk['chunk_index']}:{chunk['content']}".encode()
            ).hexdigest()
            chunk_id = str(uuid.UUID(content_hash[:32]))
            chunk_ids.append(chunk_id)
            
            # Build vector dict
            vectors = {"dense": dense_embeddings[i]}
            
            # Add SPLADE if available
            if splade_vectors and i < len(splade_vectors):
                sv = splade_vectors[i]
                vectors["splade"] = SparseVector(
                    indices=sv.indices,
                    values=sv.values
                )
            
            # Add BM42 if available
            if bm42_vectors and i < len(bm42_vectors):
                bv = bm42_vectors[i]
                vectors["bm42"] = SparseVector(
                    indices=bv.indices,
                    values=bv.values
                )
            
            points.append(PointStruct(
                id=chunk_id,
                vector=vectors,
                payload={
                    "text": chunk["content"],  # Original chunk content (not enhanced)
                    **chunk["metadata"],
                    "chunk_id": chunk_id,
                    "chunk_index": chunk["chunk_index"],
                    "content_hash": content_hash,
                    # Add separate fields for filtering and display
                    "full_path": display_path,  # Full path for filtering
                    "filename": file_name,  # Just filename for quick access
                }
            ))
        
        # 5. Upsert to Qdrant
        logger.info(f"Upserting {len(points)} points to Qdrant...")
        self.qdrant.upsert(
            collection_name=self.collection_name,
            points=points
        )
        
        # 6. Index in Tantivy (BM25)
        tantivy_indexed = 0
        if self.tantivy_client:
            logger.info("Indexing in Tantivy (BM25)...")
            try:
                tantivy_chunks = [
                    (chunk_ids[i], chunks[i]["content"])
                    for i in range(len(chunks))
                ]
                if self.tantivy_client.batch_index(tantivy_chunks):
                    tantivy_indexed = len(tantivy_chunks)
            except Exception as e:
                logger.warning(f"Tantivy indexing failed: {e}")
        
        logger.info("Indexing complete")
        
        return {
            "status": "success",
            "chunks_indexed": len(points),
            "mode": "ast" if is_ast else "fallback",
            "representations": {
                "dense": len(points),
                "splade": len(splade_vectors) if splade_vectors else 0,
                "bm42": len(bm42_vectors) if bm42_vectors else 0,
                "bm25": tantivy_indexed
            }
        }
    
    def delete_document(self, doc_id: str) -> Dict:
        """Delete all chunks for a document."""
        from qdrant_client.models import Filter, FieldCondition, MatchValue
        
        # Get chunk IDs for Tantivy deletion
        points = self.qdrant.scroll(
            collection_name=self.collection_name,
            scroll_filter=Filter(
                must=[FieldCondition(key="doc_id", match=MatchValue(value=doc_id))]
            ),
            limit=10000,
            with_payload=False
        )[0]
        
        chunk_ids = [str(p.id) for p in points]
        
        # Delete from Tantivy
        if self.tantivy_client and chunk_ids:
            for cid in chunk_ids:
                self.tantivy_client.delete(cid)
        
        # Delete from Qdrant
        self.qdrant.delete(
            collection_name=self.collection_name,
            points_selector=Filter(
                must=[FieldCondition(key="doc_id", match=MatchValue(value=doc_id))]
            )
        )
        
        return {"status": "deleted", "chunks_removed": len(chunk_ids)}
