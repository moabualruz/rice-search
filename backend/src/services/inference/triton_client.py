"""
Triton Inference Server gRPC Client.
Provides low-latency access to SPLADE sparse embedding model.
"""
import logging
from typing import List, Tuple, Optional
import numpy as np

from src.core.config import settings

logger = logging.getLogger(__name__)


class SparseEmbedding:
    """Sparse embedding representation."""
    
    def __init__(self, indices: List[int], values: List[float]):
        self.indices = indices
        self.values = values


class TritonClient:
    """
    Client for NVIDIA Triton Inference Server.
    Used for SPLADE sparse embeddings.
    """
    
    def __init__(self, url: str):
        """
        Initialize Triton client.
        
        Args:
            url: Triton gRPC endpoint (e.g., "localhost:8001")
        """
        self.url = url
        self._client = None
        self._connected = False
    
    def _ensure_connected(self):
        """Lazy connection to Triton server."""
        if self._connected:
            return
        
        try:
            # Import tritonclient lazily
            import tritonclient.grpc as grpcclient
            
            self._client = grpcclient.InferenceServerClient(
                url=self.url,
                verbose=False
            )
            
            # Check if server is ready
            if self._client.is_server_live():
                self._connected = True
                logger.info(f"Triton gRPC connected: {self.url}")
            else:
                raise ConnectionError("Triton server not live")
                
        except ImportError:
            logger.error("tritonclient not installed. Run: pip install tritonclient[grpc]")
            raise
        except Exception as e:
            logger.error(f"Triton connection failed: {e}")
            raise
    
    def sparse_embed(self, text: str) -> SparseEmbedding:
        """
        Generate sparse embedding for text using SPLADE.
        
        Args:
            text: Text to embed
            
        Returns:
            SparseEmbedding with indices and values
        """
        self._ensure_connected()
        
        try:
            import tritonclient.grpc as grpcclient
            
            # Prepare input
            input_data = np.array([[text]], dtype=object)
            inputs = [
                grpcclient.InferInput("text", input_data.shape, "BYTES")
            ]
            inputs[0].set_data_from_numpy(input_data)
            
            # Request outputs
            outputs = [
                grpcclient.InferRequestedOutput("indices"),
                grpcclient.InferRequestedOutput("values"),
            ]
            
            # Inference
            result = self._client.infer(
                model_name="splade",
                inputs=inputs,
                outputs=outputs
            )
            
            indices = result.as_numpy("indices").tolist()
            values = result.as_numpy("values").tolist()
            
            return SparseEmbedding(indices=indices, values=values)
            
        except Exception as e:
            logger.error(f"Triton sparse embed failed: {e}")
            raise
    
    def batch_sparse_embed(self, texts: List[str]) -> List[SparseEmbedding]:
        """
        Generate sparse embeddings for multiple texts.
        
        Args:
            texts: List of texts to embed
            
        Returns:
            List of SparseEmbedding objects
        """
        # For simplicity, process one at a time
        # TODO: Implement proper batching with dynamic batching on server
        return [self.sparse_embed(text) for text in texts]
    
    def health_check(self) -> bool:
        """Check if Triton server is healthy."""
        try:
            self._ensure_connected()
            return self._client.is_server_live() and self._client.is_model_ready("splade")
        except Exception:
            return False
    
    def close(self):
        """Close Triton client."""
        self._client = None
        self._connected = False


# Singleton instance with lazy loading
_triton_client: Optional[TritonClient] = None


def get_triton_client() -> TritonClient:
    """Get singleton Triton client (lazy loaded)."""
    global _triton_client
    if _triton_client is None:
        _triton_client = TritonClient(url=settings.TRITON_URL)
    return _triton_client
