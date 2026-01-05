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

        # Memory configuration (tunable via env vars)
        # max_total_tokens determines KV cache size - keep reasonable to save memory
        self.max_total_tokens = int(os.getenv("MAX_TOTAL_TOKENS", "4096"))

        logger.info("=" * 80)
        logger.info("🚀 Rice Search - SGLang Unified Inference Service")
        logger.info("=" * 80)
        logger.info(f"Memory Config:")
        logger.info(f"  - Mode: DYNAMIC allocation (on-demand KV cache growth)")
        logger.info(f"  - CUDA Graphs: DISABLED (required for dynamic memory)")
        logger.info(f"  - Max context window: {self.max_total_tokens} tokens")
        logger.info(f"  - Memory will grow as needed, not pre-allocated!")
        logger.info("=" * 80)
        logger.info(f"LLM Model: {self.llm_model_name}")
        logger.info(f"Embedding Model: {self.embed_model_name}")
        logger.info(f"Rerank Model: {self.rerank_model_name}")
        logger.info("=" * 80)

        # =====================================================================
        # Initialize SGLang Engine for LLM
        # =====================================================================
        try:
            import sglang as sgl

            logger.info("Loading LLM via SGLang...")
            self.llm_engine = sgl.Engine(
                model_path=self.llm_model_name,
                trust_remote_code=True,

                # Memory optimization
                max_total_tokens=self.max_total_tokens,  # Max context window
                mem_fraction_static=0.5,         # 50% of VRAM for model+KV cache (~8GB)

                # Prefill optimization
                chunked_prefill_size=512,        # Process long sequences in chunks

                # CUDA graph - disable for dynamic memory growth
                disable_cuda_graph=True,         # Allows flexible batch sizes

                # Quantization
                quantization="awq" if "awq" in self.llm_model_name.lower() else None,
            )
            logger.info(f"✅ LLM loaded: {self.llm_model_name}")
        except Exception as e:
            logger.error(f"❌ Failed to load LLM: {e}")
            self.llm_engine = None

        # =====================================================================
        # Initialize SGLang for Embeddings
        # =====================================================================
        try:
            import sglang as sgl

            logger.info("Loading Embedding model via SGLang...")
            self.embed_engine = sgl.Engine(
                model_path=self.embed_model_name,
                trust_remote_code=True,
                is_embedding=True,              # Mark as embedding model
                mem_fraction_static=0.05,       # 5% VRAM (~800MB)
                disable_cuda_graph=True,
            )
            logger.info(f"✅ Embedding model loaded: {self.embed_model_name}")
        except Exception as e:
            logger.error(f"❌ Failed to load Embedding model: {e}")
            self.embed_engine = None

        # =====================================================================
        # Initialize SGLang for Reranking
        # =====================================================================
        try:
            import sglang as sgl

            logger.info("Loading Rerank model via SGLang...")
            self.rerank_engine = sgl.Engine(
                model_path=self.rerank_model_name,
                trust_remote_code=True,
                is_embedding=True,              # Rerankers use embedding mode
                mem_fraction_static=0.05,       # 5% VRAM (~800MB)
                disable_cuda_graph=True,
            )
            logger.info(f"✅ Rerank model loaded: {self.rerank_model_name}")
        except Exception as e:
            logger.error(f"❌ Failed to load Rerank model: {e}")
            self.rerank_engine = None

        logger.info("=" * 80)
        logger.info("✅ All models loaded successfully via SGLang unified service")
        logger.info("🎯 RadixAttention enabled for automatic KV cache reuse")
        logger.info("⚡ Unified memory pool - better GPU utilization")
        logger.info("=" * 80)

    @bentoml.api
    def health(self) -> Dict[str, Any]:
        """Health check endpoint."""
        return {
            "status": "healthy",
            "framework": "sglang",
            "models": {
                "llm": self.llm_model_name if self.llm_engine else None,
                "embedding": self.embed_model_name if self.embed_engine else None,
                "rerank": self.rerank_model_name if self.rerank_engine else None,
            },
            "llm_available": self.llm_engine is not None,
            "embedding_available": self.embed_engine is not None,
            "rerank_available": self.rerank_engine is not None,
        }

    @bentoml.api
    def embed(self, request: EmbedRequest) -> EmbedResponse:
        """Generate embeddings for texts."""
        if self.embed_engine is None:
            raise RuntimeError("Embedding model not available")

        # Use SGLang embedding engine
        results = self.embed_engine.encode(
            request.texts,
            normalize=True,
        )

        embeddings = [r["embedding"] for r in results]

        return EmbedResponse(
            embeddings=embeddings,
            model=self.embed_model_name,
            usage={"total_tokens": sum(len(t.split()) for t in request.texts)},
        )

    @bentoml.api
    def rerank(self, request: RerankRequest) -> RerankResponse:
        """Rerank documents by relevance to query."""
        if self.rerank_engine is None:
            raise RuntimeError("Rerank model not available")

        # Create query-document pairs
        pairs = [[request.query, doc] for doc in request.documents]

        # Use SGLang rerank engine
        results = self.rerank_engine.encode(
            pairs,
            normalize=False,
        )

        # Extract scores and sort
        scored_results = [
            RerankResult(
                index=i,
                score=float(r.get("score", 0.0)),
                text=doc
            )
            for i, (r, doc) in enumerate(zip(results, request.documents))
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
        if self.llm_engine is None:
            return ChatResponse(
                content="LLM not available. Please check server configuration.",
                model="none",
                usage={"prompt_tokens": 0, "completion_tokens": 0},
            )

        from sglang.srt.sampling.sampling_params import SamplingParams

        # Format messages into prompt
        prompt = self._format_chat_prompt(request.messages)

        # Generate with SGLang
        sampling_params = SamplingParams(
            max_new_tokens=request.max_tokens,
            temperature=request.temperature,
            stop=["</s>", "<|im_end|>"],
        )

        outputs = self.llm_engine.generate([prompt], sampling_params)
        generated_text = outputs[0].text

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

        # Qwen format (ChatML)
        if "qwen" in model_name:
            prompt_parts = []
            for msg in messages:
                prompt_parts.append(f"<|im_start|>{msg.role}\n{msg.content}<|im_end|>\n")

            if messages and messages[-1].role == "user":
                prompt_parts.append("<|im_start|>assistant\n")

            return "".join(prompt_parts)

        # Gemma / CodeGemma format
        if "gemma" in model_name:
            prompt_parts = []
            for msg in messages:
                role = "model" if msg.role == "assistant" else msg.role
                prompt_parts.append(f"<start_of_turn>{role}\n{msg.content}<end_of_turn>\n")

            if messages and messages[-1].role == "user":
                prompt_parts.append("<start_of_turn>model\n")

            return "".join(prompt_parts)

        # Llama / CodeLlama Instruct format (fallback)
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
