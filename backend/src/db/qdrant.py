from qdrant_client import QdrantClient
from src.core.config import settings

class QdrantConnector:
    _instance = None

    @classmethod
    def get_client(cls) -> QdrantClient:
        if cls._instance is None:
            cls._instance = QdrantClient(url=settings.QDRANT_URL)
        return cls._instance

def get_qdrant_client():
    return QdrantConnector.get_client()
