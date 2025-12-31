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
    
    _instance: Optional["SparseEmbedder"] = None
    
    def __init__(self, model_name: str = None, device: str = None):
        """
        Initialize the SPLADE embedder.
        
        Args:
            model_name: HuggingFace model ID (default: from settings)
            device: Device to run on ("cuda", "cpu", or None for auto)
        """
        self.model_name = model_name or settings.SPARSE_MODEL
        
        # Auto-detect device
        if device is None:
            self.device = "cuda" if torch.cuda.is_available() else "cpu"
        else:
            self.device = device
        
        logger.info(f"Loading SPLADE model: {self.model_name} on {self.device}")
        
        self.tokenizer = AutoTokenizer.from_pretrained(self.model_name)
        self.model = AutoModelForMaskedLM.from_pretrained(self.model_name)
        self.model.to(self.device)
        self.model.eval()
        
        logger.info(f"SPLADE model loaded successfully")
    
    @classmethod
    def get_instance(cls) -> "SparseEmbedder":
        """Get or create singleton instance for memory efficiency."""
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
