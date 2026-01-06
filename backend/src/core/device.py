
import torch
from src.core.config import settings
import logging

logger = logging.getLogger(__name__)

def get_device() -> str:
    """
    Get the compute device (cuda or cpu) based on availability and settings.
    Respects settings.FORCE_GPU:
    - If True, will try to use CUDA even if not explicitly checked before, or fallback to CPU with warning.
    - Standard logic: check torch.cuda.is_available()
    """
    if settings.FORCE_GPU:
        if torch.cuda.is_available():
            logger.info("GPU enforced and available. Using CUDA.")
            return "cuda"
        else:
            logger.warning("GPU enforced but CUDA not available. Falling back to CPU.")
            return "cpu"
    
    # Default behavior if not forced
    return "cuda" if torch.cuda.is_available() else "cpu"
