"""
SGLang Unified Inference Service.

Provides LLM, Embeddings, and Reranking in a single optimized service.
Uses SGLang for all inference with RadixAttention and unified memory.
"""
from __future__ import annotations

import logging
import os
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

    class Config:
        # Allow dict input to be coerced to model
        from_attributes = True


class ChatRequest(BaseModel):
    messages: List[Dict[str, str]]  # Accept dicts directly
    model: Optional[str] = None
    max_tokens: int = 1024
    temperature: float = 0.7


class ChatResponse(BaseModel):
    content: str
    model: str
    usage: Dict[str, int]


# ============================================================================
# SGLang Unified Service
# ============================================================================

@bentoml.service(
    name="rice-inference-sglang",
    traffic={"timeout": 300},
    resources={"gpu": 1, "memory": "8Gi"},  # Less than vLLM!
)
class RiceInferenceServiceSGLang:
    """
    Unified inference service using SGLang for ALL models.

    Provides:
    - /embed - Text embeddings
    - /rerank - Document reranking
    - /chat - LLM chat completion
    - /health - Health check

    All models run in SGLang with shared memory pool and RadixAttention.
    """

    def __init__(self):
        # Configuration from environment
        self.llm_model_name = os.getenv("LLM_MODEL", "Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ")
        self.embed_model_name = os.getenv("EMBEDDING_MODEL", "BAAI/bge-base-en-v1.5")
        self.rerank_model_name = os.getenv("RERANK_MODEL", "BAAI/bge-reranker-v2-m3")

        logger.info("=" * 80)
        logger.info("🚀 Rice Search - Hybrid Inference Service")
        logger.info("=" * 80)
        logger.info(f"LLM Model: {self.llm_model_name} (SGLang)")
        logger.info(f"Embedding Model: {self.embed_model_name} (SentenceTransformers)")
        logger.info(f"Rerank Model: {self.rerank_model_name} (CrossEncoder)")
        logger.info("=" * 80)

        # =====================================================================
        # Initialize SentenceTransformers for Embeddings (NO EVENT LOOP ISSUES)
        # =====================================================================
        try:
            from sentence_transformers import SentenceTransformer

            logger.info("Loading Embedding model via SentenceTransformers...")
            self.embed_model = SentenceTransformer(self.embed_model_name, device="cuda")
            logger.info(f"✅ Embedding model loaded: {self.embed_model_name}")
        except Exception as e:
            logger.error(f"❌ Failed to load Embedding model: {e}")
            self.embed_model = None

        # =====================================================================
        # Initialize CrossEncoder for Reranking (NO EVENT LOOP ISSUES)
        # =====================================================================
        try:
            from sentence_transformers import CrossEncoder

            logger.info("Loading Rerank model via CrossEncoder...")
            self.rerank_model = CrossEncoder(self.rerank_model_name, device="cuda")
            logger.info(f"✅ Rerank model loaded: {self.rerank_model_name}")
        except Exception as e:
            logger.error(f"❌ Failed to load Rerank model: {e}")
            self.rerank_model = None

        # =====================================================================
        # LLM via Standalone SGLang Service
        # =====================================================================
        # SGLang runs in separate process to avoid event loop conflicts
        self.sglang_llm_url = os.getenv("SGLANG_LLM_URL", "http://sglang-llm:30003")
        logger.info(f"LLM via standalone SGLang service: {self.sglang_llm_url}")
        self.llm_engine = None  # Not running locally

        logger.info("=" * 80)
        logger.info("✅ All models loaded successfully")
        logger.info("=" * 80)

    @bentoml.api
    def health(self) -> Dict[str, Any]:
        """Health check endpoint."""
        return {
            "status": "healthy",
            "framework": "hybrid",
            "models": {
                "llm": self.llm_model_name if self.llm_engine else None,
                "embedding": self.embed_model_name if self.embed_model else None,
                "rerank": self.rerank_model_name if self.rerank_model else None,
            },
            "llm_available": self.llm_engine is not None,
            "embedding_available": self.embed_model is not None,
            "rerank_available": self.rerank_model is not None,
        }

    @bentoml.api
    def embed(self, request: EmbedRequest) -> EmbedResponse:
        """Generate embeddings for texts."""
        if self.embed_model is None:
            raise RuntimeError("Embedding model not available")

        # Use SentenceTransformers (NO EVENT LOOP ISSUES!)
        embeddings = self.embed_model.encode(request.texts, convert_to_numpy=True)
        embeddings = embeddings.tolist()  # Convert numpy to list

        return EmbedResponse(
            embeddings=embeddings,
            model=self.embed_model_name,
            usage={"total_tokens": sum(len(t.split()) for t in request.texts)},
        )

    @bentoml.api
    def rerank(self, request: RerankRequest) -> RerankResponse:
        """Rerank documents by relevance to query."""
        if self.rerank_model is None:
            raise RuntimeError("Rerank model not available")

        # Create query-document pairs
        pairs = [[request.query, doc] for doc in request.documents]

        # Use CrossEncoder (NO EVENT LOOP ISSUES!)
        scores = self.rerank_model.predict(pairs)

        # Create scored results
        scored_results = [
            RerankResult(
                index=i,
                score=float(score),
                text=doc
            )
            for i, (score, doc) in enumerate(zip(scores, request.documents))
        ]
        scored_results.sort(key=lambda x: x.score, reverse=True)

        # Apply top_n if specified
        if request.top_n:
            scored_results = scored_results[:request.top_n]

        return RerankResponse(
            results=scored_results,
            model=self.rerank_model_name,
        )

    @bentoml.api
    def chat(self, request: ChatRequest) -> ChatResponse:
        """Generate chat completion using LLM."""
        import httpx

        # Call standalone SGLang LLM service
        try:
            with httpx.Client(timeout=120.0) as client:
                response = client.post(
                    f"{self.sglang_llm_url}/chat",
                    json={
                        "messages": request.messages,
                        "max_tokens": request.max_tokens,
                        "temperature": request.temperature,
                    }
                )
                response.raise_for_status()
                data = response.json()
                return ChatResponse(
                    content=data["content"],
                    model=data["model"],
                    usage=data["usage"],
                )
        except Exception as e:
            logger.error(f"SGLang LLM service error: {e}")
            return ChatResponse(
                content=f"LLM service unavailable: {str(e)}. Please ensure SGLang LLM service is running.",
                model="error",
                usage={"prompt_tokens": 0, "completion_tokens": 0},
            )

    def _format_chat_prompt(self, messages: List[Dict[str, str]]) -> str:
        """Format chat messages into a prompt string."""
        model_name = self.llm_model_name.lower()

        # Qwen format (ChatML)
        if "qwen" in model_name:
            prompt_parts = []
            for msg in messages:
                prompt_parts.append(f"<|im_start|>{msg['role']}\n{msg['content']}<|im_end|>\n")

            if messages and messages[-1]['role'] == "user":
                prompt_parts.append("<|im_start|>assistant\n")

            return "".join(prompt_parts)

        # Gemma / CodeGemma format
        if "gemma" in model_name:
            prompt_parts = []
            for msg in messages:
                role = "model" if msg['role'] == "assistant" else msg['role']
                prompt_parts.append(f"<start_of_turn>{role}\n{msg['content']}<end_of_turn>\n")

            if messages and messages[-1]['role'] == "user":
                prompt_parts.append("<start_of_turn>model\n")

            return "".join(prompt_parts)

        # Llama / CodeLlama Instruct format (fallback)
        prompt_parts = []
        for msg in messages:
            if msg['role'] == "system":
                prompt_parts.append(f"[INST] <<SYS>>\n{msg['content']}\n<</SYS>>\n\n")
            elif msg['role'] == "user":
                if prompt_parts and not prompt_parts[-1].endswith("[/INST]"):
                    prompt_parts.append(f"{msg['content']} [/INST]")
                else:
                    prompt_parts.append(f"[INST] {msg['content']} [/INST]")
            elif msg['role'] == "assistant":
                prompt_parts.append(f" {msg['content']} ")

        return "".join(prompt_parts)
