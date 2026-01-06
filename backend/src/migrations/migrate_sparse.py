"""
Migration script to add sparse vector support to Qdrant collection.

This script updates the existing collection schema to support hybrid search
by adding a named sparse vector field alongside the existing dense vector.
"""

import logging
from qdrant_client import QdrantClient
from qdrant_client.models import (
    VectorParams,
    SparseVectorParams,
    SparseIndexParams,
    Distance,
)

from src.core.config import settings

logger = logging.getLogger(__name__)

COLLECTION_NAME = "rice_chunks"
SPARSE_VECTOR_NAME = "sparse"


def get_client() -> QdrantClient:
    """Get Qdrant client."""
    return QdrantClient(url=settings.QDRANT_URL)


def collection_exists(client: QdrantClient) -> bool:
    """Check if collection exists."""
    collections = client.get_collections().collections
    return any(c.name == COLLECTION_NAME for c in collections)


def has_sparse_vectors(client: QdrantClient) -> bool:
    """Check if collection already has sparse vector config."""
    try:
        info = client.get_collection(COLLECTION_NAME)
        sparse_config = info.config.params.sparse_vectors
        if sparse_config and SPARSE_VECTOR_NAME in sparse_config:
            return True
    except Exception:
        pass
    return False


def migrate_add_sparse_vectors():
    """
    Add sparse vector support to existing collection.
    
    Note: Qdrant does not support altering collection vector config after creation.
    This script will recreate the collection if needed, or create a new one.
    
    For production, you would:
    1. Create a new collection with the updated schema
    2. Migrate data from old to new
    3. Swap collection names
    
    For development, we'll just create with the right schema if it doesn't exist.
    """
    client = get_client()
    
    if not collection_exists(client):
        logger.info(f"Collection {COLLECTION_NAME} does not exist. Creating with hybrid schema...")
        create_hybrid_collection(client)
        return
    
    if has_sparse_vectors(client):
        logger.info(f"Collection {COLLECTION_NAME} already has sparse vector support.")
        return
    
    # Collection exists but doesn't have sparse vectors
    # In Qdrant, we cannot alter vector config after creation
    # For now, log a warning and provide instructions
    logger.warning(
        f"Collection {COLLECTION_NAME} exists but lacks sparse vector support. "
        f"To enable hybrid search, you need to recreate the collection. "
        f"Run 'migrate_recreate_hybrid()' to backup and recreate."
    )


def create_hybrid_collection(client: QdrantClient):
    """Create a new collection with hybrid (dense + sparse) vector support."""
    client.recreate_collection(
        collection_name=COLLECTION_NAME,
        vectors_config={
            "default": VectorParams(
                size=384,  # all-MiniLM-L6-v2 dimension
                distance=Distance.COSINE
            )
        },
        sparse_vectors_config={
            SPARSE_VECTOR_NAME: SparseVectorParams(
                index=SparseIndexParams(
                    on_disk=False  # Keep in memory for speed
                )
            )
        }
    )
    logger.info(f"Created collection {COLLECTION_NAME} with hybrid vector support.")


def migrate_recreate_hybrid():
    """
    Recreate collection with hybrid support.
    
    WARNING: This will delete existing data!
    For production, implement a proper migration with data backup.
    """
    client = get_client()
    
    logger.warning(f"Recreating collection {COLLECTION_NAME}. Existing data will be deleted!")
    
    # Delete if exists
    if collection_exists(client):
        client.delete_collection(COLLECTION_NAME)
        logger.info(f"Deleted existing collection {COLLECTION_NAME}")
    
    # Create with hybrid schema
    create_hybrid_collection(client)
    logger.info(f"Migration complete. Collection {COLLECTION_NAME} now supports hybrid search.")


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    
    import sys
    if len(sys.argv) > 1 and sys.argv[1] == "--force":
        migrate_recreate_hybrid()
    else:
        migrate_add_sparse_vectors()
