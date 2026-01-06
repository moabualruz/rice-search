from src.db.qdrant import get_qdrant_client
from qdrant_client.models import Filter, FieldCondition, MatchValue

def debug_search():
    client = get_qdrant_client()
    try:
        # 1. Check collection info
        param = client.get_collection("rice_chunks")
        print(f"Collection: {param}")
        print(f"Points Count: {param.points_count}")
        
        # 2. Scroll some points to see payload
        print("\n--- Sample Points ---")
        points, next_page = client.scroll(
            collection_name="rice_chunks",
            limit=5,
            with_payload=True,
            with_vectors=False
        )
        for p in points:
            print(f"ID: {p.id}")
            print(f"Org ID: {p.payload.get('org_id')}")
            # print(f"Payload: {p.payload}")

    except Exception as e:
        print(f"Error: {e}")

if __name__ == "__main__":
    debug_search()
