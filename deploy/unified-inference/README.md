# unified-inference

Python-only router/orchestrator service for multi-model inference using SGLang backends.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ UNIFIED-INFERENCE (FastAPI Router)                         │
│ Port: 3001                                                  │
│                                                             │
│ ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐ │
│ │  Selector   │  │    Proxy     │  │  Offload Policy    │ │
│ │  (Routes)   │──│  (Forward)   │──│  (GPU → CPU)       │ │
│ └─────────────┘  └──────────────┘  └────────────────────┘ │
│                                                             │
│ ┌───────────────────────────────────────────────────────┐  │
│ │  Lifecycle Manager                                    │  │
│ │  - Start/stop models on demand                        │  │
│ │  - Health monitoring                                  │  │
│ │  - Idle timeout                                       │  │
│ └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
┌───────▼──────┐   ┌──────▼──────┐   ┌──────▼──────┐
│ SGLang GPU   │   │ SGLang GPU  │   │ CPU Backend │
│ Embedding    │   │ LLM         │   │ (Optional)  │
│ Port: 30001  │   │ Port: 30003 │   │ Port: 30010 │
└──────────────┘   └─────────────┘   └─────────────┘
```

## Key Features

### Execution Modes

**GPU FULL MODE** (default)
- Models run on GPU via SGLang
- Elastic memory: `--max-total-tokens 0`
- Bounded parallelism: `--max-running-requests 3`
- Optional CPU offload when GPU is overloaded

**CPU FULL MODE**
- Models run entirely on CPU
- No GPU usage
- Placeholder implementation (needs llama.cpp integration)

### Dynamic Model Orchestration

- Starts with ZERO models loaded
- Models auto-start on first request
- Models auto-stop after idle timeout
- No hot-swapping (each model = separate process)

### Explicit Model Selection

ALL requests MUST include `"model"` parameter:

```bash
# POST example
curl -X POST http://localhost:3001/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen-coder-1.5b",
    "prompt": "def fibonacci(n):",
    "max_tokens": 100
  }'

# GET example
curl http://localhost:3001/v1/models?model=qwen-coder-1.5b
```

### CPU Offload Policy

In GPU mode, when backend queue exceeds threshold:
1. Router checks if model has `cpu_offload_model` configured
2. If yes, request is routed to CPU backend
3. If no, request is rejected with 429 (Too Many Requests)

## Configuration

### models.yaml

```yaml
models:
  - name: qwen-coder-1.5b
    type: llm
    execution_mode: gpu
    backend: sglang
    model_path: Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ
    format: awq
    gpu_id: 0
    port: 30003
    idle_timeout: 600
    cpu_offload_model: qwen-coder-1.5b-cpu  # Optional
```

### Environment Variables

```bash
# Execution mode (gpu or cpu)
EXECUTION_MODE=gpu

# Router configuration
ROUTER_HOST=0.0.0.0
ROUTER_PORT=3001

# SGLang configuration (ENFORCED)
SGLANG_MAX_RUNNING_REQUESTS=3
SGLANG_MAX_TOTAL_TOKENS=0

# Model registry
MODELS_CONFIG_PATH=/app/config/models.yaml

# Lifecycle
DEFAULT_IDLE_TIMEOUT=300
HEALTH_CHECK_INTERVAL=10
STARTUP_TIMEOUT=120

# CPU offload (GPU mode only)
ENABLE_CPU_OFFLOAD=true
OFFLOAD_QUEUE_THRESHOLD=3
```

## API Endpoints

### Core Endpoints

- `GET /health` - Health check
- `GET /v1/models` - List available models
- `GET /v1/models?model=<name>` - Get specific model info
- `GET /v1/status` - Get backend status

### Installation Guidance

- `GET /v1/models/install` - Model installation documentation
  - DOES NOT download models
  - ONLY provides guidance

### Admin Endpoints

- `POST /v1/admin/models/{model_name}/start` - Manually start model
- `POST /v1/admin/models/{model_name}/stop` - Manually stop model

### Proxy Endpoint

- `/{path:path}` - Proxies all other requests to appropriate backend
  - Preserves OpenAI-compatible API shape
  - Injects model selection logic
  - Applies offload policy

## Error Responses

Structured error format:

```json
{
  "error": {
    "code": "MODEL_NOT_FOUND",
    "message": "Model 'foo' is not registered",
    "available_models": ["qwen-coder-1.5b", "bge-base-en"],
    "install_endpoint": "/v1/models/install"
  }
}
```

Error codes:
- `MODEL_REQUIRED` - Missing model parameter
- `MODEL_NOT_FOUND` - Model not in registry
- `FORMAT_NOT_SUPPORTED` - Incompatible format (e.g., GGUF on SGLang)
- `MODEL_UNAVAILABLE` - Model failed to start
- `GPU_OVERLOADED` - GPU at capacity, no CPU offload
- `BACKEND_TIMEOUT` - Request timeout
- `BACKEND_UNAVAILABLE` - Connection error

## Supported Formats

### GPU Mode (SGLang)
- `hf` - HuggingFace format (fp16, bf16)
- `awq` - AWQ quantization

### CPU Mode (Placeholder)
- `gguf` - GGUF format (requires llama.cpp)

GGUF is **NOT** supported by SGLang (GPU mode).

## Development

### Directory Structure

```
unified-inference/
├── config/           # Configuration (settings, models)
├── backends/         # Backend managers (SGLang, CPU)
├── lifecycle/        # Process lifecycle management
├── router/           # Routing logic (selector, proxy, offload)
├── api/              # FastAPI app and routes
├── main.py           # Entry point
├── Dockerfile
├── requirements.txt
└── README.md
```

### Running Locally

```bash
# Install dependencies
pip install -r requirements.txt

# Run service
python main.py

# Or with uvicorn
uvicorn api.app:app --host 0.0.0.0 --port 3001
```

### Running with Docker

```bash
# Build
docker build -t unified-inference .

# Run (GPU mode)
docker run --gpus all \
  -p 3001:3001 \
  -v $(pwd)/config:/app/config \
  -e EXECUTION_MODE=gpu \
  unified-inference

# Run (CPU mode)
docker run \
  -p 3001:3001 \
  -v $(pwd)/config:/app/config \
  -e EXECUTION_MODE=cpu \
  unified-inference
```

## Design Philosophy

This system prioritizes:
- **Correct memory semantics** over throughput
- **Full utilization** of available resources
- **Explicit behavior** over magic
- **Elastic context** over fixed limits
- **Transparency** over convenience

The result behaves like:
**"vLLM-style routing + SGLang-style elastic memory"**

## Limitations

1. **CPU backend is a PLACEHOLDER**
   - Needs llama.cpp or ggml integration
   - Currently returns "not implemented" errors

2. **No in-process GPU → CPU KV paging**
   - SGLang doesn't support dynamic KV offloading
   - CPU offload is router-level spillover only

3. **Execution mode requires restart to change**
   - GPU ↔ CPU mode changes require restarting model instance
   - Router process stays running

4. **No model auto-downloading**
   - Models must be pre-downloaded or accessible from HuggingFace
   - Service does NOT download models automatically

## Future Enhancements

- [ ] Implement CPU backend (llama.cpp integration)
- [ ] Add request validation endpoint (`POST /v1/models/validate`)
- [ ] Add metrics endpoint (Prometheus format)
- [ ] Add request cancellation on client disconnect
- [ ] Add soft/hard timeout policies
- [ ] Add model warmup strategies
- [ ] Add A/B testing support for model variants
