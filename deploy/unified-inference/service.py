"""
Unified Inference Service - SGLang Router

Runs 3 SGLang servers in one container:
- Embedding server (port 30001)
- Reranking server (port 30002)
- LLM server (port 30003)

Flask router on port 3001 routes requests to appropriate SGLang server.
"""
import os
import sys
import time
import logging
import subprocess
import signal
from flask import Flask, request, jsonify, Response
import httpx

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# SGLang server processes
sglang_processes = []

# Model configuration from environment
EMBEDDING_MODEL = os.getenv("EMBEDDING_MODEL", "BAAI/bge-base-en-v1.5")
RERANK_MODEL = os.getenv("RERANK_MODEL", "BAAI/bge-reranker-v2-m3")
LLM_MODEL = os.getenv("LLM_MODEL", "Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ")
MAX_TOTAL_TOKENS = int(os.getenv("MAX_TOTAL_TOKENS", "4096"))

# SGLang server ports
EMBED_PORT = 30001
RERANK_PORT = 30002
LLM_PORT = 30003

def launch_sglang_server(model_path: str, port: int, is_embedding: bool = False, is_rerank: bool = False):
    """Launch a SGLang server as a background process."""
    cmd = [
        "python3.12", "-m", "sglang.launch_server",
        "--model-path", model_path,
        "--host", "0.0.0.0",
        "--port", str(port),
        "--trust-remote-code",
    ]

    if is_embedding or is_rerank:
        cmd.append("--is-embedding")

    if is_rerank:
        # Reranking models need specific flags
        cmd.extend([
            "--disable-radix-cache",
            "--chunked-prefill-size", "-1",
            "--attention-backend", "triton"
        ])

    if not is_embedding and not is_rerank:
        # LLM-specific flags
        cmd.extend([
            "--mem-fraction-static", "0.5",
            "--max-total-tokens", str(MAX_TOTAL_TOKENS),
        ])
        if "awq" in model_path.lower():
            cmd.extend(["--quantization", "awq"])

    model_type = "Rerank" if is_rerank else ("Embedding" if is_embedding else "LLM")
    logger.info(f"🚀 Launching {model_type} server: {model_path} on port {port}")
    logger.info(f"Command: {' '.join(cmd)}")

    process = subprocess.Popen(
        cmd,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        bufsize=1
    )

    sglang_processes.append(process)
    return process

def wait_for_server(port: int, timeout: int = 300):
    """Wait for SGLang server to be ready."""
    start = time.time()
    while time.time() - start < timeout:
        try:
            with httpx.Client(timeout=2.0) as client:
                response = client.get(f"http://localhost:{port}/health")
                if response.status_code == 200:
                    logger.info(f"✅ Server on port {port} is ready")
                    return True
        except:
            pass
        time.sleep(2)
    logger.error(f"❌ Server on port {port} failed to start within {timeout}s")
    return False

def init_sglang_servers():
    """Initialize all 3 SGLang servers."""
    logger.info("=" * 80)
    logger.info("🚀 Rice Search - Unified SGLang Inference Service")
    logger.info("=" * 80)

    # Launch embedding server
    embed_process = launch_sglang_server(EMBEDDING_MODEL, EMBED_PORT, is_embedding=True)

    # Launch reranking server
    rerank_process = launch_sglang_server(RERANK_MODEL, RERANK_PORT, is_rerank=True)

    # Launch LLM server
    llm_process = launch_sglang_server(LLM_MODEL, LLM_PORT, is_embedding=False)

    logger.info("=" * 80)
    logger.info("⏳ Waiting for SGLang servers to start...")
    logger.info("=" * 80)

    # Wait for all servers to be ready
    embed_ready = wait_for_server(EMBED_PORT)
    rerank_ready = wait_for_server(RERANK_PORT)
    llm_ready = wait_for_server(LLM_PORT)

    if not (embed_ready and rerank_ready and llm_ready):
        logger.error("❌ Not all servers started successfully")
        shutdown_servers()
        sys.exit(1)

    logger.info("=" * 80)
    logger.info("✅ All SGLang servers ready!")
    logger.info(f"   - Embedding: localhost:{EMBED_PORT}")
    logger.info(f"   - Reranking: localhost:{RERANK_PORT}")
    logger.info(f"   - LLM: localhost:{LLM_PORT}")
    logger.info("=" * 80)

def shutdown_servers():
    """Shutdown all SGLang servers."""
    logger.info("Shutting down SGLang servers...")
    for process in sglang_processes:
        try:
            process.terminate()
            process.wait(timeout=10)
        except:
            process.kill()

# Signal handlers
def signal_handler(signum, frame):
    logger.info(f"Received signal {signum}, shutting down...")
    shutdown_servers()
    sys.exit(0)

signal.signal(signal.SIGTERM, signal_handler)
signal.signal(signal.SIGINT, signal_handler)

# ============================================================================
# Flask Routes - Proxy to SGLang servers
# ============================================================================

@app.route("/health", methods=["POST", "GET"])
def health():
    """Health check."""
    return jsonify({
        "status": "healthy",
        "framework": "sglang-unified",
        "models": {
            "embedding": EMBEDDING_MODEL,
            "rerank": RERANK_MODEL,
            "llm": LLM_MODEL,
        },
        "servers": {
            "embedding": f"localhost:{EMBED_PORT}",
            "rerank": f"localhost:{RERANK_PORT}",
            "llm": f"localhost:{LLM_PORT}",
        }
    })

@app.route("/embed", methods=["POST"])
def embed():
    """Proxy to SGLang embedding server."""
    try:
        data = request.json
        req_data = data.get("request", data)

        # Forward to SGLang embedding server (OpenAI compatible API)
        with httpx.Client(timeout=120.0) as client:
            response = client.post(
                f"http://localhost:{EMBED_PORT}/v1/embeddings",
                json={
                    "model": EMBEDDING_MODEL,
                    "input": req_data.get("texts", [])
                }
            )
            response.raise_for_status()
            sglang_response = response.json()

            # Convert to our format
            embeddings = [item["embedding"] for item in sglang_response.get("data", [])]
            return jsonify({
                "embeddings": embeddings,
                "model": EMBEDDING_MODEL,
                "usage": sglang_response.get("usage", {})
            })
    except Exception as e:
        logger.error(f"Embedding failed: {e}")
        return jsonify({"error": str(e)}), 500

@app.route("/rerank", methods=["POST"])
def rerank():
    """Proxy to SGLang reranking server."""
    try:
        data = request.json
        req_data = data.get("request", data)
        query = req_data.get("query", "")
        documents = req_data.get("documents", [])
        top_n = req_data.get("top_n")

        # Forward to SGLang rerank server
        with httpx.Client(timeout=120.0) as client:
            response = client.post(
                f"http://localhost:{RERANK_PORT}/v1/rerank",
                json={
                    "model": RERANK_MODEL,
                    "query": query,
                    "documents": documents,
                    "top_n": top_n
                }
            )
            response.raise_for_status()
            return jsonify(response.json())
    except Exception as e:
        logger.error(f"Rerank failed: {e}")
        return jsonify({"error": str(e)}), 500

@app.route("/chat", methods=["POST"])
def chat():
    """Proxy to SGLang LLM server."""
    try:
        data = request.json
        req_data = data.get("request", data)

        # Forward to SGLang LLM server (OpenAI compatible API)
        with httpx.Client(timeout=120.0) as client:
            response = client.post(
                f"http://localhost:{LLM_PORT}/v1/chat/completions",
                json={
                    "model": LLM_MODEL,
                    "messages": req_data.get("messages", []),
                    "max_tokens": req_data.get("max_tokens", 1024),
                    "temperature": req_data.get("temperature", 0.7),
                }
            )
            response.raise_for_status()
            sglang_response = response.json()

            # Convert to our format
            return jsonify({
                "content": sglang_response["choices"][0]["message"]["content"],
                "model": LLM_MODEL,
                "usage": sglang_response.get("usage", {})
            })
    except Exception as e:
        logger.error(f"Chat failed: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == "__main__":
    # Initialize SGLang servers
    init_sglang_servers()

    # Start Flask router
    logger.info("Starting Flask router on port 3001...")
    app.run(host="0.0.0.0", port=3001, threaded=True)
