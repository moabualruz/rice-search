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
            if device_map:
                model = AutoModelForMaskedLM.from_pretrained(
                    self.model_name,
                    device_map=device_map,
                    low_cpu_mem_usage=True  # Avoid CPU copy
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
        
        Args:
            text: Input text to embed
            
        Returns:
            SparseVector with indices (token IDs) and values (weights)
        """
        self._load_model()
        with torch.no_grad():
            # Tokenize
            inputs = self.tokenizer(
                text,
                return_tensors="pt",
                truncation=True,
                max_length=512
            ).to(self.device)
            
            # Forward pass
            outputs = self.model(**inputs)
            
            # SPLADE aggregation: max pooling over sequence, then ReLU + log
            # This produces sparse activations
            logits = outputs.logits  # (batch, seq_len, vocab_size)
            
            # Max pooling over sequence dimension
            max_logits, _ = torch.max(logits, dim=1)  # (batch, vocab_size)
            
            # SPLADE activation: log(1 + ReLU(x))
            sparse_vec = torch.log(1 + torch.relu(max_logits))  # (batch, vocab_size)
            
            # Get non-zero indices and values
            sparse_vec = sparse_vec.squeeze(0)  # (vocab_size,)
            non_zero_mask = sparse_vec > 0
            indices = non_zero_mask.nonzero().squeeze(-1).cpu().tolist()
            values = sparse_vec[non_zero_mask].cpu().tolist()
            
            # Ensure indices is always a list
            if isinstance(indices, int):
                indices = [indices]
            if isinstance(values, float):
                values = [values]
            
            return SparseVector(indices=indices, values=values)
    
    def embed_batch(self, texts: List[str]) -> List[SparseVector]:
        """
        Generate sparse embeddings for a batch of texts.
        
        Args:
            texts: List of input texts
            
        Returns:
            List of SparseVector objects
        """
        results = []
        for text in texts:
            results.append(self.embed(text))
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
