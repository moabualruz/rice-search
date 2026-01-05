"""
Standalone SGLang LLM Server (LLM ONLY).

Runs in its own process with its own event loop to avoid conflicts with BentoML.
"""
import os
import logging
from flask import Flask, request, jsonify

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Global LLM engine
llm_engine = None

def init_llm():
    """Initialize SGLang LLM."""
    global llm_engine

    import sglang as sgl

    llm_model = os.getenv("LLM_MODEL", "Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ")

    logger.info("=" * 80)
    logger.info("🚀 SGLang LLM-Only Server")
    logger.info("=" * 80)
    logger.info("Loading LLM model...")
    llm_engine = sgl.Engine(
        model_path=llm_model,
        trust_remote_code=True,
        max_total_tokens=int(os.getenv("MAX_TOTAL_TOKENS", "4096")),
        mem_fraction_static=0.8,  # Use more GPU mem since it's the only model
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
        "llm_ready": llm_engine is not None,
    })

@app.route("/chat", methods=["POST"])
def chat():
    """Generate chat completion."""
    if llm_engine is None:
        return jsonify({"error": "LLM not loaded"}), 503

    data = request.json
    messages = data.get("messages", [])
    max_tokens = data.get("max_tokens", 1024)
    temperature = data.get("temperature", 0.7)

    try:
        from sglang.srt.sampling.sampling_params import SamplingParams

        # Format messages into prompt (Qwen ChatML format)
        prompt_parts = []
        for msg in messages:
            prompt_parts.append(f"<|im_start|>{msg['role']}\n{msg['content']}<|im_end|>\n")

        if messages and messages[-1]['role'] == "user":
            prompt_parts.append("<|im_start|>assistant\n")

        prompt = "".join(prompt_parts)

        sampling_params = SamplingParams(
            max_new_tokens=max_tokens,
            temperature=temperature,
            stop=["</s>", "<|im_end|>"],
        )

        outputs = llm_engine.generate([prompt], sampling_params)
        text = outputs[0].text

        return jsonify({
            "content": text,
            "model": os.getenv("LLM_MODEL", "Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ"),
            "usage": {
                "prompt_tokens": len(prompt.split()),
                "completion_tokens": len(text.split())
            }
        })
    except Exception as e:
        logger.error(f"Generation failed: {e}")
        return jsonify({"error": str(e)}), 500

if __name__ == "__main__":
    logger.info("Initializing SGLang LLM...")
    init_llm()
    logger.info("Starting Flask server on port 30003...")
    app.run(host="0.0.0.0", port=30003, threaded=True)
