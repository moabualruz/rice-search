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
        self.model = None
        self.tokenizer = None
        self._loaded = False
        
    @classmethod
    def get_instance(cls) -> "SparseEmbedder":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
        
    def _load_model(self):
        """Lazy load model via ModelManager."""
        if self._loaded:
            return
            
        manager = get_model_manager()
        
        def loader():
            import gc
            logger.info(f"Loading sparse model: {self.model_name}")
            
            # Determine device (CPU preferred for sparse usually, unless batch)
            # But prompt says "GPU support via Docker".
            # Check availability
            import torch
            from transformers import AutoTokenizer, AutoModelForMaskedLM
            
            device = "cuda" if torch.cuda.is_available() and settings.FORCE_GPU else "cpu"
            
            tokenizer = AutoTokenizer.from_pretrained(self.model_name)
            model = AutoModelForMaskedLM.from_pretrained(self.model_name)
            model.to(device)
            model.eval()
            
            return {"model": model, "tokenizer": tokenizer, "device": device}
            
        # We store both model and tokenizer in the manager instance
        manager.load_model("sparse", loader)
        status = manager.get_model_status("sparse")
        
        if status["loaded"]:
            data = manager._models["sparse"]["instance"]
            self.model = data["model"]
            self.tokenizer = data["tokenizer"]
            self.device = data["device"]
            self._loaded = True
        else:
            raise RuntimeError("Failed to load sparse model")

    def embed(self, text: str) -> SparseEmbedding:
        """
        Generate sparse embedding for text.
        Returns object with .indices (List[int]) and .values (List[float]).
        """
        self._load_model()
        import torch
        
        with torch.no_grad():
            # tokenize
            tokens = self.tokenizer(
                text, 
                return_tensors="pt", 
                truncation=True, 
                max_length=512,
                padding=True
            )
            tokens = {k: v.to(self.device) for k, v in tokens.items()}
            
            # inference
            output = self.model(**tokens)
            logits = output.logits # (1, seq_len, vocab_size)
            
            # SPLADE formula: max(log(1 + relu(logits)), dim=0)
            # But for query, usually just relu and aggregate
            # We use the standard SPLADE max pooling
            
            # max over sequence length
            relu_logits = torch.relu(logits)
            # log1p is strictly speaking part of SPLADE, but often omitted in simple implementations if model is trained for it
            # The prompt mentions `naver/splade-cocondenser-ensembledistil`.
            # Standard inference: values, _ = torch.max(torch.log(1 + torch.relu(logits)) * attention_mask.unsqueeze(-1), dim=1)
            
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
