import logging
# import torch # Lazy
from typing import Dict, Any, List, Optional
# from transformers import AutoModelForMaskedLM, AutoTokenizer # Lazy
from collections import namedtuple

from src.core.config import settings
from src.services.model_manager import get_model_manager

logger = logging.getLogger(__name__)

SparseEmbedding = namedtuple("SparseEmbedding", ["indices", "values"])

class SparseEmbedder:
    """
    SPLADE (Sparse Lexical and Expansion) Embedder.
    Generates sparse vectors for hybrid search.
    """
    _instance: Optional["SparseEmbedder"] = None
    
    def __init__(self, model_name: str = None):
        self.model_name = model_name or settings.SPARSE_MODEL
        # No persistent checking
        
    @classmethod
    def get_instance(cls) -> "SparseEmbedder":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
        
    def _get_model(self):
        """Get model tuple (model, tokenizer, device) from manager."""
        from src.services.model_manager import get_model_manager
        manager = get_model_manager()
        
        def loader():
            import gc
            logger.info(f"Loading sparse model: {self.model_name}")
            import torch
            from transformers import AutoTokenizer, AutoModelForMaskedLM
            
            device = "cuda" if torch.cuda.is_available() and settings.FORCE_GPU else "cpu"
            
            # Explicitly load on CPU (default) then move
            tokenizer = AutoTokenizer.from_pretrained(self.model_name)
            model = AutoModelForMaskedLM.from_pretrained(self.model_name)
            model.to(device)
            model.eval()
            
            return {"model": model, "tokenizer": tokenizer, "device": device}
            
        manager.load_model("sparse", loader)
        return manager.get_model_instance("sparse") # Returns the dict

    def embed(self, text: str) -> SparseEmbedding:
        """
        Generate sparse embedding for text.
        """
        instance_data = self._get_model()
        if not instance_data:
             logger.error("Sparse model not available")
             # Return empty or raise
             return SparseEmbedding(indices=[], values=[])

        model = instance_data["model"]
        tokenizer = instance_data["tokenizer"]
        device = instance_data["device"]
        
        import torch
        
        with torch.no_grad():
            # tokenize
            tokens = tokenizer(
                text, 
                return_tensors="pt", 
                truncation=True, 
                max_length=512,
                padding=True
            )
            tokens = {k: v.to(device) for k, v in tokens.items()}
            
            # inference
            output = model(**tokens)
            logits = output.logits # (1, seq_len, vocab_size)
            
            # SPLADE formula: max(log(1 + relu(logits)), dim=0)
            
            # max over sequence length
            relu_logits = torch.relu(logits)
            
            attention_mask = tokens["attention_mask"].unsqueeze(-1)
            # (1, seq_len, vocab)
            weighted_logits = torch.log(1 + relu_logits) * attention_mask
            
            # Max pooling over tokens -> (1, vocab)
            max_val, _ = torch.max(weighted_logits, dim=1)
            vector = max_val.squeeze() # (vocab_size,)
            
            # Extract non-zero
            indices = torch.nonzero(vector).squeeze().cpu().tolist()
            if isinstance(indices, int):
                indices = [indices]
                
            values = vector[indices].cpu().tolist()
            if isinstance(values, float):
                values = [values]
            
            return SparseEmbedding(indices=indices, values=values)

def get_sparse_embedder() -> SparseEmbedder:
    return SparseEmbedder.get_instance()
