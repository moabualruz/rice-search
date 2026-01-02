
import sys
import os
import logging
import torch
from transformers import AutoTokenizer, AutoModelForMaskedLM

sys.path.insert(0, '/app')
from src.core.config import settings
from src.services.admin.admin_store import get_admin_store
from src.core.device import get_device

logging.basicConfig(level=logging.INFO)

print(f"SPARSE_MODEL: {settings.SPARSE_MODEL}")

try:
    print("Attempting to load SPLADE model DIRECTLY...")
    
    # Logic from SparseEmbedder._load_model loader
    store = get_admin_store()
    # Mock models config if needed, or rely on defaults
    # gpu_enabled = True
    
    device = "cpu"
    if torch.cuda.is_available():
        print("CUDA available, trying CUDA...")
        device = "cuda"
    else:
        print("CUDA NOT available, using CPU...")

    model_name = settings.SPARSE_MODEL
    
    print(f"Loading tokenizer: {model_name}")
    tokenizer = AutoTokenizer.from_pretrained(model_name)
    
    print(f"Loading model: {model_name}")
    model = AutoModelForMaskedLM.from_pretrained(model_name)
    
    print("SUCCESS: Model and Tokenizer loaded.")

except Exception as e:
    print(f"FAILURE: {e}")
    import traceback
    traceback.print_exc()
