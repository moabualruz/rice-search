"""
SPLADE Neural Sparse Encoder.

GPU-first encoder with runtime device/model switching.
Settings are persisted via Redis for live updates.

Models:
- naver/splade-cocondenser-ensembledistil (default, high quality)
- naver/splade-cocondenser-distil (lightweight)
"""

import logging
from typing import List, Dict, Optional, NamedTuple
from functools import lru_cache

import torch
from transformers import AutoTokenizer, AutoModelForMaskedLM

from src.core.config import settings

logger = logging.getLogger(__name__)


class SparseVector(NamedTuple):
    """Sparse vector representation for Qdrant."""
    indices: List[int]
    values: List[float]


class SpladeEncoder:
    """
    SPLADE neural sparse encoder with GPU/CPU switching.

    Features:
    - GPU-first with CPU fallback
    - Runtime model switching
    - Runtime device switching
    - Batched inference
    - fp16 support on GPU
    """

    def __init__(
        self,
        model_id: str = None,
        device: str = None,
        use_fp16: bool = True,
        max_length: int = None,
        batch_size: int = None
    ):
        # Get defaults from settings
        default_model = settings.SPARSE_MODEL
        lightweight_model = settings.SPARSE_LIGHTWEIGHT_MODEL

        self.model_id = model_id or settings.SPARSE_MODEL or default_model
        self.max_length = max_length if max_length is not None else settings.SPLADE_MAX_TOKENS
        self.batch_size = batch_size if batch_size is not None else settings.SPLADE_BATCH_SIZE
        self.use_fp16 = use_fp16
        
        # Determine device
        if device is None:
            self.device = "cuda" if torch.cuda.is_available() and settings.FORCE_GPU else "cpu"
        else:
            self.device = device
        
        self.model = None
        self.tokenizer = None
        self._load_model()
    
    def _load_model(self):
        """Load SPLADE model and tokenizer."""
        logger.info(f"Loading SPLADE model: {self.model_id} on {self.device}")
        
        try:
            self.tokenizer = AutoTokenizer.from_pretrained(self.model_id)
            self.model = AutoModelForMaskedLM.from_pretrained(self.model_id)
            
            # Move to device
            self.model = self.model.to(self.device)
            
            # Enable fp16 on CUDA
            if self.device == "cuda" and self.use_fp16:
                self.model = self.model.half()
                logger.info("SPLADE using fp16 precision")
            
            self.model.eval()
            logger.info(f"SPLADE model loaded successfully on {self.device}")
            
        except Exception as e:
            logger.error(f"Failed to load SPLADE model: {e}")
            raise
    
    def switch_model(self, model_id: str):
        """Switch to a different SPLADE model at runtime."""
        if model_id == self.model_id:
            logger.info(f"Model {model_id} already loaded")
            return
        
        logger.info(f"Switching SPLADE model: {self.model_id} -> {model_id}")
        
        # Free current model
        self._free_model()
        
        # Load new model
        self.model_id = model_id
        self._load_model()
    
    def switch_device(self, device: str):
        """Switch between GPU and CPU at runtime."""
        if device == self.device:
            logger.info(f"Already on device {device}")
            return
        
        if device == "cuda" and not torch.cuda.is_available():
            logger.warning("CUDA not available, staying on CPU")
            return
        
        logger.info(f"Switching SPLADE device: {self.device} -> {device}")
        
        self.device = device
        self.model = self.model.to(device)
        
        # Handle precision
        if device == "cuda" and self.use_fp16:
            self.model = self.model.half()
        elif device == "cpu":
            self.model = self.model.float()
    
    def _free_model(self):
        """Free GPU memory from current model."""
        if self.model is not None:
            del self.model
            self.model = None
        if self.tokenizer is not None:
            del self.tokenizer
            self.tokenizer = None
        
        if torch.cuda.is_available():
            torch.cuda.empty_cache()
    
    def encode(self, texts: List[str]) -> List[SparseVector]:
        """
        Encode texts to SPLADE sparse vectors.
        
        Args:
            texts: List of texts to encode
            
        Returns:
            List of SparseVector namedtuples
        """
        if not texts:
            return []
        
        if self.model is None:
            raise RuntimeError("SPLADE model not loaded")
        
        results = []
        
        # Process in batches
        for i in range(0, len(texts), self.batch_size):
            batch = texts[i:i + self.batch_size]
            batch_results = self._encode_batch(batch)
            results.extend(batch_results)
        
        return results
    
    def _encode_batch(self, texts: List[str]) -> List[SparseVector]:
        """Encode a batch of texts."""
        # Tokenize
        inputs = self.tokenizer(
            texts,
            return_tensors="pt",
            padding=True,
            truncation=True,
            max_length=self.max_length
        )
        
        # Move to device
        inputs = {k: v.to(self.device) for k, v in inputs.items()}
        
        # Forward pass
        with torch.no_grad():
            outputs = self.model(**inputs)
        
        # SPLADE aggregation: max over sequence length, then ReLU + log(1+x)
        # Shape: (batch_size, seq_len, vocab_size) -> (batch_size, vocab_size)
        logits = outputs.logits
        
        # Apply attention mask to ignore padding
        attention_mask = inputs["attention_mask"].unsqueeze(-1)
        logits = logits * attention_mask
        
        # Max pooling over sequence
        max_logits, _ = torch.max(logits, dim=1)
        
        # SPLADE transform: ReLU + log(1 + x)
        splade_vectors = torch.log1p(torch.relu(max_logits))
        
        # Convert to sparse format
        results = []
        for vec in splade_vectors:
            # Get non-zero indices and values
            nonzero_mask = vec > 0
            indices = nonzero_mask.nonzero(as_tuple=True)[0].cpu().tolist()
            values = vec[nonzero_mask].cpu().tolist()
            
            results.append(SparseVector(indices=indices, values=values))
        
        return results
    
    def encode_single(self, text: str) -> SparseVector:
        """Encode a single text."""
        return self.encode([text])[0]
    
    @property
    def vocab_size(self) -> int:
        """Get vocabulary size."""
        return self.tokenizer.vocab_size if self.tokenizer else 0
    
    @property
    def is_gpu(self) -> bool:
        """Check if running on GPU."""
        return self.device == "cuda"


# Singleton instance
_splade_encoder: Optional[SpladeEncoder] = None


def get_splade_encoder() -> SpladeEncoder:
    """Get or create the singleton SPLADE encoder."""
    global _splade_encoder
    
    if _splade_encoder is None:
        _splade_encoder = SpladeEncoder()
    
    return _splade_encoder


def reload_splade_encoder(
    model_id: str = None,
    device: str = None
) -> SpladeEncoder:
    """Reload SPLADE encoder with new settings."""
    global _splade_encoder
    
    if _splade_encoder is not None:
        _splade_encoder._free_model()
    
    _splade_encoder = SpladeEncoder(model_id=model_id, device=device)
    return _splade_encoder
