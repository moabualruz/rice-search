"""
Sparse Embedder Service using SPLADE v3.

This module provides sparse embeddings for hybrid search using the SPLADE v3 model.
SPLADE generates high-dimensional sparse vectors for lexical/keyword matching.
"""

import logging
from typing import Dict, List, Tuple, Optional
from dataclasses import dataclass

import torch
from transformers import AutoTokenizer, AutoModelForMaskedLM

from src.core.config import settings

logger = logging.getLogger(__name__)


@dataclass
class SparseVector:
    """Represents a sparse vector with indices and values."""
    indices: List[int]
    values: List[float]


class SparseEmbedder:
    """
    Sparse embedding service using SPLADE v3.
    
    SPLADE (Sparse Lexical and Expansion) generates sparse vectors where:
    - Non-zero dimensions correspond to vocabulary token IDs
    - Values represent term importance weights
    
    The vectors are compatible with Qdrant's sparse vector storage.
    """
    
from src.services.model_manager import get_model_manager

class SparseEmbedder:
    """
    Sparse embedding service using SPLADE v3.
    """
    
    _instance: Optional["SparseEmbedder"] = None
    
    def __init__(self, model_name: str = None, device: str = None):
        """Initialize wrapper. Model is lazy-loaded via ModelManager."""
        self.model_name = model_name or settings.SPARSE_MODEL
        
        # Auto-detect device
        if device is None:
            from src.core.device import get_device
            self.device = get_device()
        else:
            self.device = device
            
        self.tokenizer = None
        # Model is managed externally
        
    def _load_model(self):
        """Load model via ModelManager."""
        manager = get_model_manager()
        
        def loader():
            import torch
            import gc
            from transformers import AutoTokenizer, AutoModelForMaskedLM
            from src.services.admin.admin_store import get_admin_store
            from src.core.device import get_device
            
            # Check if GPU is enabled for sparse model
            store = get_admin_store()
            models = store.get_models()
            gpu_enabled = models.get("sparse", {}).get("gpu_enabled", True)
            
            if gpu_enabled and torch.cuda.is_available():
                device = "cuda"
                device_map = "cuda:0"  # Load directly on GPU
            else:
                device = "cpu"
                device_map = None
            
            logger.info(f"Loading SPLADE model: {self.model_name} on {device}")
            
            # Load tokenizer (always CPU, small memory footprint)
            tokenizer = AutoTokenizer.from_pretrained(self.model_name)
            
            # Load model directly on target device
            # Load model directly on target device
            if device_map:
                model = AutoModelForMaskedLM.from_pretrained(
                    self.model_name,
                    device_map=device_map,
                    low_cpu_mem_usage=True,  # Avoid CPU copy
                    torch_dtype=torch.float16
                )
            else:
                model = AutoModelForMaskedLM.from_pretrained(self.model_name)
            
            model.eval()
            
            return {"model": model, "tokenizer": tokenizer}
            
        manager.load_model("sparse", loader)
        status = manager.get_model_status("sparse")
        
        if status["loaded"]:
            data = manager._models["sparse"]["instance"]
            self.model = data["model"]
            self.tokenizer = data["tokenizer"]
        else:
            raise RuntimeError("Failed to load SPLADE model")

    @classmethod
    def get_instance(cls) -> "SparseEmbedder":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
    
    def embed(self, text: str) -> SparseVector:
        """
        Generate a sparse embedding for a single text.
        Delegates to embed_batch.
        """
        return self.embed_batch([text])[0]
    
    def embed_batch(self, texts: List[str]) -> List[SparseVector]:
        """
        Generate sparse embeddings for a batch of texts using vectorized inference.
        """
        self._load_model()
        with torch.no_grad():
            # Tokenize Batch (Auto padding)
            inputs = self.tokenizer(
                texts,
                return_tensors="pt",
                truncation=True,
                padding=True,
                max_length=512
            ).to(self.device)
            
            # Forward pass
            outputs = self.model(**inputs)
            logits = outputs.logits  # (batch, seq_len, vocab_size)
            
            # Apply attention mask to ignore padding tokens during max pooling
            # attention_mask is (batch, seq_len), unsqueeze to (batch, seq_len, 1)
            attention_mask = inputs["attention_mask"].unsqueeze(-1)
            # Set logits of padded tokens to negative infinity so they don't affect max
            logits = logits + (1.0 - attention_mask) * -10000.0
            
            # Max pooling over sequence dimension
            max_logits, _ = torch.max(logits, dim=1)  # (batch, vocab_size)
            
            # SPLADE activation: log(1 + ReLU(x))
            sparse_vecs = torch.log(1 + torch.relu(max_logits))  # (batch, vocab_size)
            
            # Extract sparse vectors (CPU side)
            results = []
            sparse_vecs_cpu = sparse_vecs.cpu()
            
            for i in range(len(texts)):
                vec = sparse_vecs_cpu[i]
                non_zero_mask = vec > 0
                
                # Get indices and values
                indices = non_zero_mask.nonzero().squeeze(-1).tolist()
                values = vec[non_zero_mask].tolist()
                
                # Ensure types
                if isinstance(indices, int):
                    indices = [indices]
                if isinstance(values, float):
                    values = [values]
                
                results.append(SparseVector(indices=indices, values=values))
            
            return results
    
    def to_qdrant_format(self, sparse_vec: SparseVector) -> Dict:
        """
        Convert SparseVector to Qdrant sparse vector format.
        
        Args:
            sparse_vec: SparseVector object
            
        Returns:
            Dict with 'indices' and 'values' keys for Qdrant
        """
        return {
            "indices": sparse_vec.indices,
            "values": sparse_vec.values
        }


# Convenience function for one-off embeddings
def get_sparse_embedding(text: str) -> SparseVector:
    """
    Generate a sparse embedding using the global embedder instance.
    
    Args:
        text: Input text
        
    Returns:
        SparseVector
    """
    if not settings.SPARSE_ENABLED:
        return SparseVector(indices=[], values=[])
    
    embedder = SparseEmbedder.get_instance()
    return embedder.embed(text)
