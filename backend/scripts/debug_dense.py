
from sentence_transformers import SentenceTransformer
import torch
import logging

logging.basicConfig(level=logging.INFO)

MODEL_NAME = "jinaai/jina-embeddings-v2-base-code"

try:
    print(f"Loading {MODEL_NAME}...")
    device = "cuda" if torch.cuda.is_available() else "cpu"
    print(f"Device: {device}")
    
    model = SentenceTransformer(MODEL_NAME, device=device, trust_remote_code=True)
    print("Model loaded successfully.")
    
    embedding = model.encode("test string")
    print(f"Embedding shape: {embedding.shape}")

except Exception as e:
    print(f"FAILURE: {e}")
    import traceback
    traceback.print_exc()
