"""
BGE-M3 Embedding Service - FastAPI wrapper for FlagEmbedding library
Provides dense, sparse, and ColBERT vector embeddings with optional reranking
"""

import logging
import os
import time
from contextlib import asynccontextmanager
from typing import Dict, List, Literal, Optional, Union

import numpy as np
import torch
from fastapi import FastAPI, HTTPException, status
from fastapi.responses import JSONResponse
from pydantic import BaseModel, Field, field_validator

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)

# Global model instance
model = None

# Configuration from environment variables
MODEL_NAME = os.getenv("MODEL_NAME", "BAAI/bge-m3")
DEVICE = os.getenv("DEVICE", "cuda" if torch.cuda.is_available() else "cpu")
USE_FP16 = os.getenv("USE_FP16", "true").lower() == "true"
BATCH_SIZE = int(os.getenv("BATCH_SIZE", "32"))
MAX_LENGTH = int(os.getenv("MAX_LENGTH", "8192"))


# Pydantic models for request/response
class EncodeRequest(BaseModel):
    """Request model for encoding texts into embeddings"""

    texts: List[str] = Field(
        ..., description="List of texts to encode", min_length=1, max_length=1000
    )
    return_dense: bool = Field(True, description="Return dense embeddings")
    return_sparse: bool = Field(
        False, description="Return sparse embeddings (lexical weights)"
    )
    return_colbert: bool = Field(
        False, description="Return ColBERT multi-vector embeddings"
    )
    batch_size: Optional[int] = Field(
        None, description="Override default batch size", gt=0, le=128
    )
    max_length: Optional[int] = Field(
        None, description="Override default max length", gt=0, le=8192
    )
    normalize: bool = Field(True, description="Normalize dense embeddings")

    @field_validator("texts")
    @classmethod
    def validate_texts(cls, v: List[str]) -> List[str]:
        if not all(isinstance(text, str) and text.strip() for text in v):
            raise ValueError("All texts must be non-empty strings")
        return v


class RerankRequest(BaseModel):
    """Request model for reranking documents"""

    query: str = Field(..., description="Query text", min_length=1)
    documents: List[str] = Field(
        ..., description="Documents to rerank", min_length=1, max_length=1000
    )
    top_k: Optional[int] = Field(None, description="Return top-k results", gt=0)

    @field_validator("query")
    @classmethod
    def validate_query(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("Query must be non-empty")
        return v

    @field_validator("documents")
    @classmethod
    def validate_documents(cls, v: List[str]) -> List[str]:
        if not all(isinstance(doc, str) and doc.strip() for doc in v):
            raise ValueError("All documents must be non-empty strings")
        return v


class DenseEmbedding(BaseModel):
    """Dense embedding vector"""

    embedding: List[float]
    index: int


class SparseEmbedding(BaseModel):
    """Sparse embedding (lexical weights)"""

    weights: Dict[str, float]
    index: int


class ColbertEmbedding(BaseModel):
    """ColBERT multi-vector embedding"""

    vectors: List[List[float]]
    index: int


class EncodeResponse(BaseModel):
    """Response model for encoding"""

    dense: Optional[List[DenseEmbedding]] = None
    sparse: Optional[List[SparseEmbedding]] = None
    colbert: Optional[List[ColbertEmbedding]] = None
    model: str
    usage: Dict[str, int]


class RerankResult(BaseModel):
    """Single rerank result"""

    document: str
    score: float
    index: int


class RerankResponse(BaseModel):
    """Response model for reranking"""

    results: List[RerankResult]
    model: str
    usage: Dict[str, int]


class HealthResponse(BaseModel):
    """Health check response"""

    status: Literal["healthy", "unhealthy"]
    model: str
    device: str
    use_fp16: bool


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Lifespan context manager for model loading/cleanup"""
    global model

    logger.info(
        f"Starting BGE-M3 service - Model: {MODEL_NAME}, Device: {DEVICE}, FP16: {USE_FP16}"
    )

    try:
        # Import here to catch errors during startup
        from FlagEmbedding import BGEM3FlagModel

        start_time = time.time()
        logger.info(f"Loading model {MODEL_NAME}...")

        model = BGEM3FlagModel(
            MODEL_NAME,
            use_fp16=USE_FP16 and DEVICE != "cpu",
            devices=[DEVICE] if DEVICE != "cpu" else None,
            batch_size=BATCH_SIZE,
            query_max_length=MAX_LENGTH,
            passage_max_length=MAX_LENGTH,
        )

        load_time = time.time() - start_time
        logger.info(f"Model loaded successfully in {load_time:.2f}s")

        # Warmup
        logger.info("Warming up model...")
        _ = model.encode(
            ["warmup text"],
            return_dense=True,
            return_sparse=False,
            return_colbert_vecs=False,
        )
        logger.info("Warmup complete")

    except Exception as e:
        logger.error(f"Failed to load model: {e}", exc_info=True)
        raise

    yield

    # Cleanup
    logger.info("Shutting down BGE-M3 service")
    model = None


# Create FastAPI app
app = FastAPI(
    title="BGE-M3 Embedding Service",
    description="FastAPI wrapper for BGE-M3 embedding model with multi-vector support",
    version="1.0.0",
    lifespan=lifespan,
)


@app.get("/health", response_model=HealthResponse, tags=["Health"])
async def health_check():
    """Health check endpoint"""
    if model is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Model not loaded"
        )

    return HealthResponse(
        status="healthy",
        model=MODEL_NAME,
        device=DEVICE,
        use_fp16=USE_FP16 and DEVICE != "cpu",
    )


@app.post("/encode", response_model=EncodeResponse, tags=["Embeddings"])
async def encode(request: EncodeRequest):
    """
    Encode texts into embeddings (dense, sparse, and/or ColBERT vectors)

    Returns:
    - dense: Single embedding vector per text (if return_dense=True)
    - sparse: Lexical weights (token -> weight mapping) per text (if return_sparse=True)
    - colbert: Multi-vector embeddings per text (if return_colbert=True)
    """
    if model is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Model not loaded"
        )

    try:
        start_time = time.time()

        # Encode with requested outputs
        batch_size = request.batch_size or BATCH_SIZE
        max_length = request.max_length or MAX_LENGTH

        logger.info(
            f"Encoding {len(request.texts)} texts "
            f"(dense={request.return_dense}, sparse={request.return_sparse}, colbert={request.return_colbert})"
        )

        output = model.encode(
            request.texts,
            batch_size=batch_size,
            max_length=max_length,
            return_dense=request.return_dense,
            return_sparse=request.return_sparse,
            return_colbert_vecs=request.return_colbert,
        )

        # Build response
        response = EncodeResponse(
            model=MODEL_NAME,
            usage={
                "texts": len(request.texts),
                "time_ms": int((time.time() - start_time) * 1000),
            },
        )

        # Dense embeddings
        if request.return_dense and "dense_vecs" in output:
            dense_vecs = output["dense_vecs"]
            if request.normalize:
                # Normalize if not already normalized
                norms = np.linalg.norm(dense_vecs, axis=1, keepdims=True)
                dense_vecs = dense_vecs / np.maximum(norms, 1e-10)

            response.dense = [
                DenseEmbedding(embedding=vec.tolist(), index=i)
                for i, vec in enumerate(dense_vecs)
            ]

        # Sparse embeddings (lexical weights)
        if request.return_sparse and "lexical_weights" in output:
            lexical_weights = output["lexical_weights"]
            # Convert token IDs to tokens - must call per-item, not on entire list
            response.sparse = [
                SparseEmbedding(
                    weights={
                        k: float(v)
                        for k, v in model.convert_id_to_token(weights).items()
                    },
                    index=i,
                )
                for i, weights in enumerate(lexical_weights)
            ]

        # ColBERT embeddings
        if request.return_colbert and "colbert_vecs" in output:
            colbert_vecs = output["colbert_vecs"]

            response.colbert = [
                ColbertEmbedding(vectors=vecs.tolist(), index=i)
                for i, vecs in enumerate(colbert_vecs)
            ]

        logger.info(f"Encoding completed in {response.usage['time_ms']}ms")
        return response

    except Exception as e:
        logger.error(f"Encoding failed: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Encoding failed: {str(e)}",
        )


@app.post("/rerank", response_model=RerankResponse, tags=["Reranking"])
async def rerank(request: RerankRequest):
    """
    Rerank documents based on query using ColBERT-based scoring

    Returns documents sorted by relevance score (highest first)
    """
    if model is None:
        raise HTTPException(
            status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail="Model not loaded"
        )

    try:
        start_time = time.time()

        logger.info(f"Reranking {len(request.documents)} documents for query")

        # Encode query and documents with ColBERT vectors
        query_output = model.encode(
            [request.query],
            return_dense=False,
            return_sparse=False,
            return_colbert_vecs=True,
        )

        doc_output = model.encode(
            request.documents,
            return_dense=False,
            return_sparse=False,
            return_colbert_vecs=True,
        )

        # Compute ColBERT scores
        query_vecs = query_output["colbert_vecs"][0]
        scores = []

        for doc_vecs in doc_output["colbert_vecs"]:
            score = model.colbert_score(query_vecs, doc_vecs)
            scores.append(float(score))

        # Sort by score (descending)
        ranked_indices = np.argsort(scores)[::-1]

        # Apply top_k if specified
        if request.top_k is not None:
            ranked_indices = ranked_indices[: request.top_k]

        results = [
            RerankResult(
                document=request.documents[idx],
                score=scores[idx],
                index=int(idx),
            )
            for idx in ranked_indices
        ]

        response = RerankResponse(
            results=results,
            model=MODEL_NAME,
            usage={
                "query": 1,
                "documents": len(request.documents),
                "time_ms": int((time.time() - start_time) * 1000),
            },
        )

        logger.info(f"Reranking completed in {response.usage['time_ms']}ms")
        return response

    except Exception as e:
        logger.error(f"Reranking failed: {e}", exc_info=True)
        raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail=f"Reranking failed: {str(e)}",
        )


@app.get("/", tags=["Info"])
async def root():
    """Root endpoint with service information"""
    return {
        "service": "BGE-M3 Embedding Service",
        "model": MODEL_NAME,
        "device": DEVICE,
        "endpoints": {
            "health": "/health",
            "encode": "/encode",
            "rerank": "/rerank",
            "docs": "/docs",
        },
    }


# Error handlers
@app.exception_handler(Exception)
async def global_exception_handler(request, exc):
    """Global exception handler"""
    logger.error(f"Unhandled exception: {exc}", exc_info=True)
    return JSONResponse(
        status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
        content={"detail": "Internal server error"},
    )


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=80)
