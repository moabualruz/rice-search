# Model Optimization Recommendations

## Current Model Analysis

### Current Configuration

| Component | Current Model | Size/Memory | Issues |
|-----------|---------------|-------------|---------|
| **Embedding** | jina-embeddings-v3 OR BAAI/bge-base-en-v1.5 | 1024 dim / 768 dim | Inconsistent config, larger than needed |
| **Reranker** | BAAI/bge-reranker-base OR jina-reranker-v2-base-multilingual | ~278M params | Good but can be lighter |
| **LLM** | CodeLlama-7b-Instruct OR codegemma-7b-it-AWQ | **7B params (~4-5GB VRAM)** | **TOO LARGE - main bottleneck** |
| **Sparse** | naver/splade-cocondenser-ensembledistil | ~300M params | Can be optimized |
| **BM42** | Qdrant/bm42-all-minilm-l6-v2-attentions | 0.09GB | Already lightweight ✓ |

### Key Problems

1. **Configuration Inconsistency**: docker-compose.yml uses different models than config.py
   - docker-compose: `BAAI/bge-base-en-v1.5`
   - bentofile.yaml: `jinaai/jina-embeddings-v3`
   - config.py: `jina-embeddings-v3`

2. **LLM Memory Bottleneck**: 7B model consumes 4-5GB VRAM even with AWQ quantization
   - This is the **PRIMARY** memory issue
   - Limits ability to run other models simultaneously
   - Slow inference on GPU

3. **Over-dimensioned Embeddings**: 1024 dims is overkill for code search
   - Research shows 384-512 dims offer best performance/cost tradeoff
   - Smaller dims = faster search, lower memory, faster indexing

## Critical Discovery: vLLM Context Pre-allocation Issue

### The Problem

**vLLM pre-allocates GPU memory for the FULL context window**, not dynamically as needed:
- If model supports 128K tokens, vLLM reserves memory for 128K upfront
- Even if your prompts are only 1K tokens, memory for 128K is wasted
- This causes 60-80% memory waste through fragmentation and over-allocation
- Can lead to OOM errors even when actual usage is low

### Solutions (Multiple Approaches)

#### Option A: Optimize vLLM Configuration (Keep vLLM)

**1. Reduce `max_model_len` to actual needs**
```python
# deploy/bentoml/service.py - Line 132-139
self.llm = LLM(
    model=self.llm_model_name,
    trust_remote_code=True,
    gpu_memory_utilization=0.50,  # Reduced from 0.90
    max_model_len=4096,           # ⚠️ SET TO ACTUAL MAX (not model's 128K!)
    disable_log_stats=True,
    quantization="awq",
    # New parameters for dynamic allocation:
    enable_chunked_prefill=True,  # Enable chunked prefill
    max_num_batched_tokens=2048,  # Lower = less memory, more latency
    max_num_seqs=4,               # Reduce concurrent sequences
)
```

**Key vLLM Parameters:**
- `max_model_len`: Set to realistic max (e.g., 4096 or 8192, NOT 128K)
- `gpu_memory_utilization`: Lower from 0.90 to 0.50-0.60
- `enable_chunked_prefill=True`: Splits large prefills into smaller chunks
- `max_num_batched_tokens`: Lower values = less memory (default 8192)
- `max_num_seqs`: Reduce concurrent requests (default varies)

**Estimated Savings with vLLM tuning:**
- Setting `max_model_len=4096` instead of 128K: **~2-3GB saved**
- Combined with Qwen 1.5B: Total LLM memory **~1.5-2GB** (vs 4-5GB)

#### Option B: Switch to SGLang (Best Balance) ⭐ RECOMMENDED

**Why SGLang is better than vLLM for your use case:**
- ✅ **RadixAttention** - Advanced KV cache reuse via radix tree
- ✅ **Concurrent batching** - Like vLLM, handles parallel requests
- ✅ **Lower memory** - Better KV cache management than vLLM
- ✅ **10% faster** on multi-turn conversations
- ✅ **Up to 6.4× higher throughput** on structured workloads
- ✅ **Dynamic prefix caching** - Automatically discovers caching opportunities
- ✅ **ALL model types** - LLM, embeddings, reranking in ONE framework!

**CRITICAL DISCOVERY: SGLang Supports Everything!**

SGLang can replace your **entire BentoML service**:
- ✅ LLM (text generation)
- ✅ Embedding models (E5, GTE, BGE, CLIP)
- ✅ Reranker models (BGE-Reranker cross-encoders)
- ✅ Multimodal models

**Single unified framework = better optimization, less complexity!**

**Memory comparison:**
- Current BentoML (all models): ~8GB
- SGLang unified (all models optimized): **~2-3GB**

**Performance comparison:**
- Current: 3 separate model loads, different memory management
- SGLang: Unified memory pool, shared KV cache, RadixAttention across all

#### Option C: Use TGI v3 (Best for Long Contexts)

**Hugging Face Text Generation Inference v3:**
- ✅ **3× more tokens** in same memory as vLLM
- ✅ **13× faster** on long prompts (>200K tokens)
- ✅ **Prefix caching** built-in
- ✅ **Concurrent batching** like vLLM
- ⚠️ More complex setup than SGLang

Good if you have very long code contexts, but overkill for most use cases.

#### Option D: llama.cpp (NOT Recommended - Serializes Requests)

**Critical limitation discovered:**
- ❌ While llama.cpp supports continuous batching with `--cont-batching`
- ❌ It queues requests and processes them **serially**, not truly concurrently
- ❌ Much lower throughput than vLLM/SGLang for parallel requests
- ✅ Only good for: Single-user, CPU-heavy, or development/testing

**When to use llama.cpp:**
- Single concurrent user (no parallelism needed)
- CPU-only or heavy CPU offloading required
- Development/local testing only

## Recommended Model Stack (Optimized)

### 🎯 High Priority: Replace LLM

**Current**: CodeLlama-7b (7B params, ~4-5GB VRAM)

**Recommended**: **Qwen 2.5 Coder 1.5B** (GGUF Q4_K_M quantization)
- **Size**: ~1.5GB RAM (3x smaller!)
- **Format**: GGUF Q4_K_M (best balance of quality/speed)
- **Specialization**: Specifically trained for coding tasks
- **Context**: 128K tokens
- **Performance**: Outperforms larger general models on code tasks

**Alternative**: DeepSeek Coder 1.3B Instruct
- Size: ~2.69GB at FP16
- Also code-specialized

**Implementation**:
```yaml
# docker-compose.yml
environment:
  - LLM_MODEL=Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF
  # Or if using AWQ:
  - LLM_MODEL=Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ
```

### 🔧 Medium Priority: Optimize Embeddings (SGLang-Compatible)

**Current**: jina-embeddings-v3 (1024 dims) OR BAAI/bge-base-en-v1.5 (768 dims)

**Recommended for SGLang**: **GTE-Qwen2-1.5B-instruct** or **E5-mistral-7b-instruct**
- **Benefits**: Native SGLang support, optimized for RadixAttention
- **Dims**: 768-1024 (configurable with Matryoshka)
- **Performance**: SOTA on MTEB benchmarks
- **SGLang optimization**: Shares memory pool with LLM

**Budget Option**: **all-MiniLM-L6-v2** (384 dims)
- **Size**: ~22M params (tiny!)
- **Dims**: 384 (vs 1024 current)
- **Performance**: Surprisingly good for code despite being general-purpose
- **Speed**: 2-3x faster encoding
- **Memory**: 50% less storage in Qdrant
- **SGLang**: Works via transformers fallback

**Why SGLang-native models are better:**
- Share GPU memory pool with LLM
- RadixAttention caching for repeated prompts
- Unified batch processing
- Better GPU utilization

**Implementation**:
```yaml
# docker-compose.yml
environment:
  - EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2

# config.py
EMBEDDING_MODEL: str = "all-MiniLM-L6-v2"
EMBEDDING_DIM: int = 384  # Update from 1024!
```

**⚠️ IMPORTANT**: Changing embedding dims requires **re-indexing all data**
- Qdrant collections must be recreated with new dimensions
- Plan migration strategy before switching

### 🎨 Low Priority: Lighter Reranker (SGLang-Native)

**Current**: BAAI/bge-reranker-base (~278M params)

**Recommended for SGLang**: **BAAI/bge-reranker-v2-m3** (600M params)
- **Benefits**: Native SGLang support with optimized serving
- **Performance**: SOTA reranking performance
- **SGLang optimization**: Unified memory pool, RadixAttention
- **Multilingual**: 100+ languages

**Budget Option**: **cross-encoder/ms-marco-MiniLM-L-6-v2** (90M params)
- **Size**: ~90M params (3x smaller than current!)
- **Performance**: Competitive with larger models
- **Speed**: 2-3x faster inference
- **SGLang**: Works via transformers fallback

**Why keep bge-reranker-v2-m3 in SGLang:**
- Already BAAI family (consistent with embeddings)
- Native SGLang optimization
- Shares memory with other models
- Better batching efficiency

**Implementation**:
```yaml
# docker-compose.yml
environment:
  - RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-6-v2
```

### ✅ Keep: Sparse Models

**BM42**: Already optimized at 0.09GB via FastEmbed ✓

**SPLADE**: Consider switching to **FastEmbed implementation**
- Current: `naver/splade-cocondenser-ensembledistil` (direct PyTorch)
- Recommended: `prithivida/Splade_PP_en_v1` via FastEmbed (ONNX)
- Benefits: No GPU needed, smaller dependencies, faster CPU inference

**Implementation**:
```python
# Use FastEmbed for SPLADE
from fastembed import SparseTextEmbedding

sparse_model = SparseTextEmbedding(
    model_name="prithivida/Splade_PP_en_v1"
)
```

## Memory Savings Summary

### Scenario 1: vLLM Optimized (Quick Fix)

| Component | Current Memory | Optimized Memory | Savings |
|-----------|----------------|------------------|---------|
| LLM (7B vLLM) | 4-5GB | 2-2.5GB (Qwen 1.5B + tuning) | **~2.5GB saved** |
| Embedding | ~2GB | ~100MB | ~1.9GB saved |
| Reranker | ~600MB | ~200MB | ~400MB saved |
| Sparse | ~600MB | ~300MB (ONNX) | ~300MB saved |
| **TOTAL** | **~8GB** | **~3.1GB** | **~5GB saved** |

### Scenario 2: SGLang (Best Fix) ⭐

| Component | Current Memory | Optimized Memory | Savings |
|-----------|----------------|------------------|---------|
| LLM (SGLang) | 4-5GB | **1.5-2GB** (RadixAttention!) | **~3GB saved** ✨ |
| Embedding | ~2GB | ~100MB | ~1.9GB saved |
| Reranker | ~600MB | ~200MB | ~400MB saved |
| Sparse | ~600MB | ~300MB (ONNX) | ~300MB saved |
| **TOTAL** | **~8GB** | **~2.2GB** | **~5.8GB saved!** |

**Key difference**: SGLang has better KV cache management + concurrent batching!

## Implementation Priority

### Phase 1A: Critical - Fix Context Pre-allocation (Do FIRST!)

**Choose ONE approach:**

**Option 1: Quick Fix (Stay with vLLM)**
- Update vLLM config in `service.py`
- Set `max_model_len=4096` (not 128K)
- Add `enable_chunked_prefill=True`
- Reduce `gpu_memory_utilization=0.50`
- **Time**: 10 minutes
- **Memory saved**: ~2-3GB

**Option 2: Better Fix (Switch to SGLang)** ⭐ RECOMMENDED
- Replace vLLM with SGLang
- RadixAttention for automatic KV cache reuse
- Concurrent request batching (like vLLM)
- Better memory efficiency
- **Time**: 30-60 minutes
- **Memory saved**: ~2-3GB
- **Throughput**: Up to 6.4× better on structured workloads

**Option 3: Best Fix for Long Contexts (TGI v3)**
- Replace vLLM with Text Generation Inference v3
- 3× more tokens in same memory
- 13× faster on long prompts
- **Time**: 1-2 hours
- **Memory saved**: ~2-3GB
- **Best for**: Very long code files (>50K tokens)

### Phase 1B: Replace Model
1. **Switch to Qwen 2.5 Coder 1.5B**
   - Smaller base model (1.5B vs 7B)
   - Better code specialization
   - Faster inference

### Phase 2: Optimize (After LLM works)
2. **Standardize embedding config**
   - Fix docker-compose.yml vs config.py inconsistency
   - Choose one model across all configs

3. **Test with current embeddings**
   - Verify system functionality
   - Establish baseline metrics

### Phase 3: Fine-tune (Optional)
4. **Switch to MiniLM-L6 embeddings**
   - Requires full re-indexing
   - Test thoroughly first
   - 2x faster, 50% less storage

5. **Lighter reranker**
   - Easy drop-in replacement
   - Test quality vs speed tradeoff

## Detailed Implementation Guides

### Implementation: vLLM Optimization (Quick Fix)

**deploy/bentoml/service.py** (Lines 128-144):

```python
# Load LLM via vLLM with optimized settings
self.llm = None
try:
    from vllm import LLM, SamplingParams
    logger.info(f"Loading LLM: {self.llm_model_name}")
    self.llm = LLM(
        model=self.llm_model_name,
        trust_remote_code=True,

        # ⚠️ CRITICAL: Reduce memory pre-allocation
        gpu_memory_utilization=0.50,      # Was 0.90 - use only 50% of VRAM
        max_model_len=4096,                # Was 8192 or default 128K - BE REALISTIC!

        # Enable dynamic allocation features
        enable_chunked_prefill=True,       # NEW: Chunk large prefills
        max_num_batched_tokens=2048,       # NEW: Lower = less memory
        max_num_seqs=4,                    # NEW: Limit concurrent requests

        disable_log_stats=True,
        quantization="awq",                # Keep AWQ quantization
    )
    self.SamplingParams = SamplingParams
    logger.info("LLM loaded successfully")
except Exception as e:
    logger.warning(f"Failed to load LLM: {e}. Chat endpoint will be disabled.")
```

**Key points:**
- `max_model_len=4096`: For code RAG, 4K context is usually enough
- If you need 8K, set it to 8192 (still much better than 128K default)
- `enable_chunked_prefill`: Allows batching with decode requests
- `max_num_seqs=4`: Most code search queries are 1-2 concurrent max

### Implementation: SGLang Switch (Best Fix)

**Step 1: Update dependencies**

`deploy/bentoml/bentofile.yaml`:
```yaml
python:
  packages:
    - torch>=2.0.0
    - sentence-transformers>=2.2.0
    # Replace vllm with sglang
    # - vllm>=0.4.0
    - sglang[all]>=0.3.0
    - flashinfer>=0.1.0  # Required for SGLang
    - transformers>=4.40.0
    - accelerate>=0.20.0
    - pydantic>=2.0.0
```

**Step 2: Update service code**

`deploy/bentoml/service.py` (Replace vLLM section):

```python
def __init__(self):
    import os
    import torch
    from sentence_transformers import SentenceTransformer, CrossEncoder

    # ... (embedding and reranker code stays same)

    # Load LLM via SGLang instead of vLLM
    self.llm = None
    try:
        import sglang as sgl
        from sglang.srt.sampling.sampling_params import SamplingParams

        logger.info(f"Loading LLM via SGLang: {self.llm_model_name}")

        self.llm = sgl.Engine(
            model_path=self.llm_model_name,
            trust_remote_code=True,

            # ⚠️ CRITICAL: Memory-efficient settings
            max_total_tokens=4096,        # Total KV cache budget
            mem_fraction_static=0.7,      # Use 70% of VRAM (vs vLLM's 90%)

            # Enable RadixAttention for automatic KV cache reuse
            enable_radix_cache=True,
            radix_cache_size_gb=0.5,      # Cache frequently used prefixes

            # Quantization (if AWQ model)
            quantization="awq" if "awq" in self.llm_model_name.lower() else None,
        )

        self.SamplingParams = SamplingParams
        logger.info("LLM loaded successfully via SGLang")

    except Exception as e:
        logger.warning(f"Failed to load LLM: {e}. Chat endpoint will be disabled.")

@bentoml.api
def chat(self, request: ChatRequest) -> ChatResponse:
    """Generate chat completion using LLM (SGLang version)."""
    if self.llm is None:
        return ChatResponse(
            content="LLM not available.",
            model="none",
            usage={"prompt_tokens": 0, "completion_tokens": 0},
        )

    # Format messages into prompt
    prompt = self._format_chat_prompt(request.messages)

    # Generate with SGLang
    sampling_params = self.SamplingParams(
        max_new_tokens=request.max_tokens,
        temperature=request.temperature,
        stop=["</s>", "<|im_end|>"],  # Stop sequences
    )

    outputs = self.llm.generate([prompt], sampling_params)
    generated_text = outputs[0].text

    return ChatResponse(
        content=generated_text,
        model=self.llm_model_name,
        usage={
            "prompt_tokens": len(prompt.split()),  # Approximate
            "completion_tokens": len(generated_text.split()),
        },
    )
```

**Step 3: RadixAttention Benefits**

SGLang automatically caches and reuses common prefixes:
- System prompts cached once, reused across requests
- Few-shot examples shared across similar queries
- Multi-turn conversations reuse prior context
- No manual optimization needed!

## Configuration File Updates Needed

### 1. Fix Config Consistency

**backend/src/core/config.py**:
```python
# Phase 1: LLM only
LLM_MODEL: str = "Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF"

# Phase 3: Full optimization
EMBEDDING_MODEL: str = "all-MiniLM-L6-v2"
EMBEDDING_DIM: int = 384  # Changed from 1024
RERANK_MODEL: str = "cross-encoder/ms-marco-MiniLM-L-6-v2"
```

**deploy/docker-compose.yml**:
```yaml
bentoml:
  environment:
    - EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2  # Was BAAI/bge-base-en-v1.5
    - RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-6-v2  # Was BAAI/bge-reranker-base
    - LLM_MODEL=Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF  # Was TechxGenus/codegemma-7b-it-AWQ
```

**deploy/bentoml/bentofile.yaml**:
```yaml
docker:
  env:
    - EMBEDDING_MODEL=sentence-transformers/all-MiniLM-L6-v2  # Was jinaai/jina-embeddings-v3
    - RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-6-v2  # Was BAAI/bge-reranker-base
    - LLM_MODEL=Qwen/Qwen2.5-Coder-1.5B-Instruct-GGUF  # Was codellama/CodeLlama-7b-Instruct-hf
```

### 2. Update BentoML Service

**deploy/bentoml/service.py** needs vLLM quantization update:
```python
# Line 138-139: Update quantization config
self.llm = LLM(
    model=self.llm_model_name,
    trust_remote_code=True,
    gpu_memory_utilization=0.50,  # Reduced from 0.90 (smaller model)
    max_model_len=32768,  # Increased from 8192 (Qwen supports 128K)
    disable_log_stats=True,
    # For GGUF models, use different quantization approach
    # quantization="awq",  # Remove this for GGUF
)
```

## Testing Strategy (Once System is Functional)

After fixing the current issues and getting to 100% functional state:

1. **Baseline Metrics** (with current/optimized LLM)
   - Search latency (p50, p95, p99)
   - Indexing throughput
   - Memory usage (peak, average)
   - Search quality (MRR, Recall@10)

2. **A/B Testing** (embedding changes)
   - Compare 1024-dim vs 384-dim embeddings
   - Measure quality vs speed tradeoffs
   - User feedback on relevance

3. **Load Testing**
   - Concurrent search queries
   - Large file indexing
   - Memory pressure scenarios

## Next Steps

1. ✅ **Fix LLM immediately**
   - Update all 3 config files with Qwen 2.5 Coder 1.5B
   - Rebuild BentoML service
   - Test basic functionality

2. ⏸️ **Get system to 100% functional**
   - Address current blocking issues
   - Verify all endpoints work
   - Confirm worker tasks complete

3. 📊 **Establish baselines**
   - Measure current performance
   - Document metrics
   - Create test suite

4. 🔧 **Phase 2 optimizations**
   - Switch to lighter embeddings (requires re-indexing)
   - Update reranker
   - Optimize sparse models

## Decision Matrix: Which Approach?

| Factor | vLLM Optimized | SGLang | TGI v3 | llama.cpp |
|--------|----------------|--------|--------|-----------|
| **Memory Savings** | ~5GB total | **~5.8GB total** ⭐ | ~5.5GB total | ~6.4GB total |
| **Implementation Time** | 10 mins | **30-60 mins** ⭐ | 1-2 hours | 1-2 hours |
| **Complexity** | Low | **Low** ⭐ | Medium | Medium |
| **Concurrent Requests** | **Yes** ⭐ | **Yes** ⭐ | **Yes** ⭐ | ❌ Serialized |
| **KV Cache Reuse** | Basic | **RadixAttention** ⭐ | Prefix caching | Limited |
| **Throughput** | High | **6.4× higher** ⭐ | 3× on long | Low (serial) |
| **Dynamic Allocation** | Limited | **Better** ⭐ | **Best** ⭐ | Yes |
| **CPU Offloading** | No | No | No | **Yes** ⭐ |
| **Best For** | Quick fix | **Production** ⭐ | Long contexts | Single user |

**Recommendation**: Use **SGLang** for production. It combines the best of both worlds:
- ✅ **Better memory** than vLLM (RadixAttention KV cache reuse)
- ✅ **Concurrent batching** (unlike llama.cpp which serializes)
- ✅ **6.4× higher throughput** on structured workloads (code search patterns)
- ✅ **Easy migration** from vLLM (similar API)
- ✅ **Automatic prefix caching** (no manual optimization needed)

**Why NOT llama.cpp:**
- ❌ Serializes requests (one at a time, even with continuous batching enabled)
- ❌ Lower throughput for multi-user scenarios
- ✅ Only use if: Single user, CPU-only, or development/testing

## Sources

**Model Research:**
- [Best Open-Source Embedding Models Benchmarked and Ranked](https://supermemory.ai/blog/best-open-source-embedding-models-benchmarked-and-ranked/)
- [6 Best Code Embedding Models Compared](https://modal.com/blog/6-best-code-embedding-models-compared)
- [Best Sub-3B GGUF Models for Mid-Range CPUs](https://ggufloader.github.io/2025-07-07-top-10-gguf-models-i5-16gb/)
- [LLM Quantization Guide 2025](https://local-ai-zone.github.io/guides/what-is-ai-quantization-q4-k-m-q8-gguf-guide-2025.html)
- [Top 7 Rerankers for RAG](https://www.analyticsvidhya.com/blog/2025/06/top-rerankers-for-rag/)

**Sparse Embeddings:**
- [BM42: New Baseline for Hybrid Search - Qdrant](https://qdrant.tech/articles/bm42/)
- [FastEmbed: Lightweight Embeddings](https://github.com/qdrant/fastembed)
- [Modern Sparse Neural Retrieval - Qdrant](https://qdrant.tech/articles/modern-sparse-neural-retrieval/)

**vLLM Memory Management:**
- [How PagedAttention Resolves Memory Waste](https://developers.redhat.com/articles/2025/07/24/how-pagedattention-resolves-memory-waste-llm-systems)
- [vLLM Optimization and Tuning](https://docs.vllm.ai/en/stable/configuration/optimization/)
- [Default vLLM KV Cache Allocation Issue](https://github.com/vllm-project/vllm/discussions/9525)

**Inference Engine Alternatives:**
- [When to Choose SGLang Over vLLM](https://www.runpod.io/blog/sglang-vs-vllm-kv-cache)
- [Comparing Top 6 Inference Runtimes 2025](https://www.marktechpost.com/2025/11/07/comparing-the-top-6-inference-runtimes-for-llm-serving-in-2025/)
- [vLLM vs TensorRT-LLM vs TGI vs LMDeploy](https://www.marktechpost.com/2025/11/19/vllm-vs-tensorrt-llm-vs-hf-tgi-vs-lmdeploy-a-deep-technical-comparison-for-production-llm-inference/)
- [PagedAttention vs Continuous Batching vs SGLang](https://python.plainenglish.io/pagedattention-vs-continuous-batching-vs-vllm-vs-sglang-a-practical-breakdown-4c19cc9e21c0)

**llama.cpp Limitations:**
- [llama.cpp Concurrent Requests Issue](https://github.com/ggml-org/llama.cpp/issues/4666)
- [Parallelization/Batching Explanation](https://github.com/ggml-org/llama.cpp/discussions/4130)
- [Why NOT llama.cpp on Multi-GPU](https://www.ahmadosman.com/blog/do-not-use-llama-cpp-or-ollama-on-multi-gpus-setups-use-vllm-or-exllamav2/)
