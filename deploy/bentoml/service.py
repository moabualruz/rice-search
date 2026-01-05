"""
BentoML Unified Inference Service.

Provides LLM, Embeddings, and Reranking in a single service.
Uses vLLM for LLM, Sentence Transformers for embeddings, Cross-Encoder for reranking.
"""
from __future__ import annotations

import logging
from typing import List, Dict, Any, Optional

import bentoml
from pydantic import BaseModel

logger = logging.getLogger(__name__)


# ============================================================================
# Request/Response Models
# ============================================================================

class EmbedRequest(BaseModel):
    texts: List[str]
    model: Optional[str] = None


class EmbedResponse(BaseModel):
    embeddings: List[List[float]]
    model: str
    usage: Dict[str, int]


class RerankRequest(BaseModel):
    query: str
    documents: List[str]
    top_n: Optional[int] = None
    model: Optional[str] = None


class RerankResult(BaseModel):
    index: int
    score: float
    text: Optional[str] = None


class RerankResponse(BaseModel):
    results: List[RerankResult]
    model: str


class ChatMessage(BaseModel):
    role: str
    content: str


class ChatRequest(BaseModel):
    messages: List[ChatMessage]
    model: Optional[str] = None
    max_tokens: int = 1024
    temperature: float = 0.7


class ChatResponse(BaseModel):
    content: str
    model: str
    usage: Dict[str, int]


# ============================================================================
# BentoML Service
# ============================================================================

@bentoml.service(
    name="rice-inference",
    traffic={"timeout": 300},
    resources={"gpu": 1, "memory": "16Gi"},
)
class RiceInferenceService:
    """
    Unified inference service for Rice Search.
    
    Provides:
    - /embed - Text embeddings via Sentence Transformers
    - /rerank - Document reranking via Cross-Encoder
    - /chat - LLM chat completion via vLLM
    - /health - Health check
    """
    
    def __init__(self):
        import os
        import torch
        from sentence_transformers import SentenceTransformer, CrossEncoder
        
        # Configuration from environment with reliable defaults
        # Use BAAI/bge-base-en-v1.5 as default - it's well-tested and reliable
        self.embed_model_name = os.getenv("EMBEDDING_MODEL", "BAAI/bge-base-en-v1.5")
        self.rerank_model_name = os.getenv("RERANK_MODEL", "BAAI/bge-reranker-base")
        self.llm_model_name = os.getenv("LLM_MODEL", "codellama/CodeLlama-7b-Instruct-hf")
        
        # Debug log environment
        logger.info(f"EMBEDDING_MODEL env: {os.getenv('EMBEDDING_MODEL', 'NOT SET')}")
        logger.info(f"RERANK_MODEL env: {os.getenv('RERANK_MODEL', 'NOT SET')}")
        logger.info(f"LLM_MODEL env: {os.getenv('LLM_MODEL', 'NOT SET')}")
        
        # Device selection
        self.device = "cuda" if torch.cuda.is_available() else "cpu"
        logger.info(f"Using device: {self.device}")
        
        # Load embedding model
        logger.info(f"Loading embedding model: {self.embed_model_name}")
        self.embed_model = SentenceTransformer(
            self.embed_model_name,
            device=self.device,
            trust_remote_code=True
        )
        logger.info(f"Embedding model loaded: {self.embed_model_name}")
        
        # Load reranking model
        logger.info(f"Loading rerank model: {self.rerank_model_name}")
        self.rerank_model = CrossEncoder(
            self.rerank_model_name,
            device=self.device,
            trust_remote_code=True
        )
        logger.info(f"Rerank model loaded: {self.rerank_model_name}")
        
        # Load LLM via vLLM
        self.llm = None
        try:
            from vllm import LLM, SamplingParams
            logger.info(f"Loading LLM: {self.llm_model_name}")
            self.llm = LLM(
                model=self.llm_model_name,
                trust_remote_code=True,

                # ⚠️ CRITICAL: Reduce memory pre-allocation to save ~2-3GB
                gpu_memory_utilization=0.50,      # Was 0.90 - use only 50% of VRAM
                max_model_len=4096,                # Was 8192 - realistic context for code RAG

                # Enable dynamic allocation features to reduce waste
                enable_chunked_prefill=True,       # Chunk large prefills
                max_num_batched_tokens=2048,       # Lower = less memory (default 8192)
                max_num_seqs=4,                    # Limit concurrent requests

                disable_log_stats=True,
                quantization="awq",
            )
            self.SamplingParams = SamplingParams
            logger.info("LLM loaded successfully with optimized memory settings")
        except Exception as e:
            logger.warning(f"Failed to load LLM: {e}. Chat endpoint will be disabled.")
    
    @bentoml.api
    def health(self) -> Dict[str, Any]:
        """Health check endpoint."""
        return {
            "status": "healthy",
            "models": {
                "embedding": self.embed_model_name,
                "rerank": self.rerank_model_name,
                "llm": self.llm_model_name if self.llm else None,
            },
            "llm_available": self.llm is not None,
        }
    
    @bentoml.api
    def embed(self, request: EmbedRequest) -> EmbedResponse:
        """Generate embeddings for texts."""
        embeddings = self.embed_model.encode(
            request.texts,
            convert_to_numpy=True,
            normalize_embeddings=True,
        )
        
        return EmbedResponse(
            embeddings=embeddings.tolist(),
            model=self.embed_model_name,
            usage={"total_tokens": sum(len(t.split()) for t in request.texts)},
        )
    
    @bentoml.api
    def rerank(self, request: RerankRequest) -> RerankResponse:
        """Rerank documents by relevance to query."""
        # Create query-document pairs
        pairs = [[request.query, doc] for doc in request.documents]
        
        # Get scores
        scores = self.rerank_model.predict(pairs)
        
        # Build results sorted by score
        results = [
            RerankResult(index=i, score=float(score), text=doc)
            for i, (score, doc) in enumerate(zip(scores, request.documents))
        ]
        results.sort(key=lambda x: x.score, reverse=True)
        
        # Apply top_n if specified
        if request.top_n:
            results = results[:request.top_n]
        
        return RerankResponse(
            results=results,
            model=self.rerank_model_name,
        )
    
    @bentoml.api
    def chat(self, request: ChatRequest) -> ChatResponse:
        """Generate chat completion using LLM."""
        if self.llm is None:
            return ChatResponse(
                content="LLM not available. Please check server configuration.",
                model="none",
                usage={"prompt_tokens": 0, "completion_tokens": 0},
            )
        
        # Format messages into prompt
        prompt = self._format_chat_prompt(request.messages)
        
        # Generate with proper stop sequences for Llama/CodeLlama
        sampling_params = self.SamplingParams(
            max_tokens=request.max_tokens,
            temperature=request.temperature,
            stop=["</s>"],  # Stop sequences
            repetition_penalty=1.1,  # Prevent repetitive output
        )
        
        outputs = self.llm.generate([prompt], sampling_params)
        generated_text = outputs[0].outputs[0].text
        
        return ChatResponse(
            content=generated_text,
            model=self.llm_model_name,
            usage={
                "prompt_tokens": len(prompt.split()),
                "completion_tokens": len(generated_text.split()),
            },
        )
    
    def _format_chat_prompt(self, messages: List[ChatMessage]) -> str:
        """Format chat messages into a prompt string."""
        model_name = self.llm_model_name.lower()
        
        # Gemma / CodeGemma format
        if "gemma" in model_name:
            prompt_parts = []
            for msg in messages:
                role = "model" if msg.role == "assistant" else msg.role
                prompt_parts.append(f"<start_of_turn>{role}\n{msg.content}<end_of_turn>\n")
            
            # Start model turn if last was user
            if messages and messages[-1].role == "user":
                prompt_parts.append("<start_of_turn>model\n")
            
            return "".join(prompt_parts)

        # Qwen format (ChatML)
        if "qwen" in model_name:
            prompt_parts = []
            for msg in messages:
                prompt_parts.append(f"<|im_start|>{msg.role}\n{msg.content}<|im_end|>\n")
            
            # Start assistant turn
            if messages and messages[-1].role == "user":
                prompt_parts.append("<|im_start|>assistant\n")
            
            return "".join(prompt_parts)

        # Llama / CodeLlama Instruct format
        prompt_parts = []
        for msg in messages:
            if msg.role == "system":
                prompt_parts.append(f"[INST] <<SYS>>\n{msg.content}\n<</SYS>>\n\n")
            elif msg.role == "user":
                if prompt_parts and not prompt_parts[-1].endswith("[/INST]"):
                    prompt_parts.append(f"{msg.content} [/INST]")
                else:
                    prompt_parts.append(f"[INST] {msg.content} [/INST]")
            elif msg.role == "assistant":
                prompt_parts.append(f" {msg.content} ")
        
        return "".join(prompt_parts)
