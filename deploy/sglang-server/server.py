"""
Standalone SGLang HTTP Server.

Runs in its own process with its own event loop to avoid conflicts with BentoML.
"""
import os
import logging
from flask import Flask, request, jsonify

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Global engines
embed_engine = None
rerank_engine = None
llm_engine = None

def init_models():
    """Initialize SGLang models."""
    global embed_engine, rerank_engine, llm_engine

    import sglang as sgl

    embed_model = os.getenv("EMBEDDING_MODEL", "BAAI/bge-base-en-v1.5")
    rerank_model = os.getenv("RERANK_MODEL", "BAAI/bge-reranker-v2-m3")
    llm_model = os.getenv("LLM_MODEL", "Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ")

    logger.info("=" * 80)
    logger.info("Loading Embedding model...")
    embed_engine = sgl.Engine(
        model_path=embed_model,
        trust_remote_code=True,
        is_embedding=True,
        mem_fraction_static=0.05,
        disable_cuda_graph=True,
    )
    logger.info(f"✅ Embedding model loaded: {embed_model}")

    logger.info("Loading Rerank model...")
    rerank_engine = sgl.Engine(
        model_path=rerank_model,
        trust_remote_code=True,
        is_embedding=True,
        mem_fraction_static=0.05,
        disable_cuda_graph=True,
    )
    logger.info(f"✅ Rerank model loaded: {rerank_model}")

    logger.info("Loading LLM model...")
    llm_engine = sgl.Engine(
        model_path=llm_model,
        trust_remote_code=True,
        max_total_tokens=int(os.getenv("MAX_TOTAL_TOKENS", "4096")),
        mem_fraction_static=0.5,
        chunked_prefill_size=512,
        disable_cuda_graph=True,
        quantization="awq" if "awq" in llm_model.lower() else None,
    )
    logger.info(f"✅ LLM loaded: {llm_model}")
    logger.info("=" * 80)

@app.route("/health", methods=["GET"])
def health():
    """Health check."""
    return jsonify({
        "status": "healthy",
        "embed_ready": embed_engine is not None,
        "rerank_ready": rerank_engine is not None,
        "llm_ready": llm_engine is not None,
    })

@app.route("/embed", methods=["POST"])
def embed():
    """Generate embeddings."""
    if embed_engine is None:
        return jsonify({"error": "Embedding model not loaded"}), 503

    data = request.json
    texts = data.get("texts", [])

    try:
        results = embed_engine.encode(texts)
        embeddings = [r["embedding"] for r in results]
        return jsonify({"embeddings": embeddings})
    except Exception as e:
        logger.error(f"Embedding failed: {e}")
        return jsonify({"error": str(e)}), 500

@app.route("/rerank", methods=["POST"])
def rerank():
    """Rerank documents."""
    if rerank_engine is None:
        return jsonify({"error": "Rerank model not loaded"}), 503

    data = request.json
    query = data.get("query", "")
    documents = data.get("documents", [])

    try:
        pairs = [[query, doc] for doc in documents]
        results = rerank_engine.encode(pairs)

        scored_results = [
            {"index": i, "score": float(r.get("score", 0.0)), "text": doc}
            for i, (r, doc) in enumerate(zip(results, documents))
        ]
        scored_results.sort(key=lambda x: x["score"], reverse=True)

        top_n = data.get("top_n")
        if top_n:
            scored_results = scored_results[:top_n]

        return jsonify({"results": scored_results})
    except Exception as e:
        logger.error(f"Rerank failed: {e}")
        return jsonify({"error": str(e)}), 500

@app.route("/generate", methods=["POST"])
def generate():
    """Generate text with LLM."""
    if llm_engine is None:
        return jsonify({"error": "LLM not loaded"}), 503

    data = request.json
    prompt = data.get("prompt", "")
    max_tokens = data.get("max_tokens", 1024)
    temperature = data.get("temperature", 0.7)
    stop = data.get("stop", ["</s>", "<|im_end|>"])

    try:
        from sglang.srt.sampling.sampling_params import SamplingParams

        sampling_params = SamplingParams(
            max_new_tokens=max_tokens,
            temperature=temperature,
            stop=stop,
        )

        outputs = llm_engine.generate([prompt], sampling_params)
        text = outputs[0].text

        return jsonify({"text": text})
    except Exception as e:
        logger.error(f"Generation failed: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == "__main__":
    logger.info("Initializing SGLang models...")
    init_models()
    logger.info("Starting Flask server on port 30002...")
    app.run(host="0.0.0.0", port=30002, threaded=True)
