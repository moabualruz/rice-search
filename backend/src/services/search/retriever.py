from typing import List, Dict
from sentence_transformers import SentenceTransformer
from src.db.qdrant import get_qdrant_client
from qdrant_client.models import Filter

model = SentenceTransformer('all-MiniLM-L6-v2')
qdrant = get_qdrant_client()
COLLECTION_NAME = "rice_codebase"

class Retriever:
    @staticmethod
    def search(query: str, limit: int = 5) -> List[Dict]:
        """
        Semantic search. Encodes query and searches Qdrant.
        """
        # 1. Encode query
        vector = model.encode(query).tolist()

        # 2. Search Qdrant
        # We assume collection exists (created by ingestion). 
        # If not, return empty list safely.
        try:
             results = qdrant.search(
                collection_name=COLLECTION_NAME,
                query_vector=vector,
                limit=limit
            )
        except Exception:
            # Collection might not exist yet
            return []

        # 3. Format results
        output = []
        for hit in results:
            payload = hit.payload
            output.append({
                "score": hit.score,
                "text": payload.get("text", ""),
                "metadata": {k:v for k,v in payload.items() if k != "text"}
            })
        
        return output
