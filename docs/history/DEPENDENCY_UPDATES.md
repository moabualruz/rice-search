# Dependency Updates - Latest Stable Versions (January 2025)

## Python Dependencies (Backend & BentoML)

| Package | Current | Latest Stable | Notes |
|---------|---------|---------------|-------|
| **PyTorch** | 2.3.0 | **2.5.1** | Latest for CUDA 12.1 (2.9.1 requires newer CUDA) |
| **Transformers** | >=4.40.0 | **4.57.3** | Latest HuggingFace |
| **Accelerate** | >=0.20.0 | **1.12.0** | Latest HuggingFace |
| **Pydantic** | >=2.0.0 | **2.12.5** | Latest validation library |
| **BentoML** | >=1.2.0 | **1.3.15** | Latest stable |
| **SGLang** | 0.5.7 | **0.5.7** | ✅ Already latest |
| **FlashInfer** | N/A | **Platform-specific** | Installed via SGLang |

### SGLang Installation Notes

- SGLang `[all]` extra handles FlashInfer automatically for your CUDA version
- FlashInfer requires SM75+ GPUs (Turing+: T4, A10, A100, L4, L40S, H100)
- For CUDA 12.1: FlashInfer is included in SGLang's dependencies
- **DO NOT** install FlashInfer separately - let SGLang handle it

## Frontend Dependencies (Node.js)

| Package | Current | Latest Stable | Notes |
|---------|---------|---------------|-------|
| **Node.js** | Unknown | **22.x LTS** ("Jod") | Active LTS until Oct 2025 |
| **React** | 18.x | **19.2.3** | Just released stable Dec 2024 |
| **Next.js** | 14.x | **15.1** | Released Dec 10, 2024 |
| **TypeScript** | Check | **5.7.x** | Latest stable |

⚠️ **Breaking Changes**: React 19 + Next.js 15 have breaking changes from 18/14

## Rust Dependencies

| Package | Current | Latest Stable | Notes |
|---------|---------|---------------|-------|
| **Tantivy** | Unknown | **0.25.0** | Latest full-text search engine |
| **Rust** | Unknown | **1.85.0** | Latest stable (Jan 2026) |

## Recommended Update Strategy

### Phase 1: Backend Python (Lower Risk)
1. ✅ Update PyTorch 2.3.0 → 2.9.1 (CUDA 12.1 compatible)
2. ✅ Update Transformers → 4.57.3
3. ✅ Update Accelerate → 1.12.0
4. ✅ Update Pydantic → 2.12.5
5. ✅ Update BentoML → 1.3.15
6. ✅ Fix SGLang installation (remove flashinfer, use sglang[all])

### Phase 2: Frontend (Higher Risk - Breaking Changes)
⚠️ **DEFER** - React 19 + Next.js 15 have breaking changes
- Stick with React 18 + Next.js 14 for now
- Upgrade after backend is stable

### Phase 3: Rust (Low Risk)
- Update Tantivy to 0.25.0
- Check rust-tantivy service compatibility

## Updated Dockerfile

```dockerfile
# Install PyTorch 2.9.1 with CUDA 12.1
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install torch==2.9.1 --index-url https://download.pytorch.org/whl/cu121

# Install latest stable ML dependencies
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install \
    bentoml==1.3.15 \
    transformers==4.57.3 \
    accelerate==1.12.0 \
    pydantic==2.12.5 \
    einops \
    httpx

# Install SGLang with all extras (includes FlashInfer for CUDA 12.1)
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install "sglang[all]==0.5.7"
```

## Sources

**Python Libraries:**
- [PyTorch 2.9.1 Release](https://github.com/pytorch/pytorch/releases)
- [Transformers 4.57.3](https://pypi.org/project/transformers/)
- [Accelerate 1.12.0](https://pypi.org/project/accelerate/)
- [Pydantic 2.12.5](https://pypi.org/project/pydantic/)
- [SGLang Installation Docs](https://deepwiki.com/sgl-project/sglang/6.1-installation-and-setup)
- [SGLang Dependencies](https://github.com/sgl-project/sglang/releases)

**Frontend:**
- [Next.js 15.1](https://nextjs.org/blog/next-16-1)
- [React 19 Release](https://www.npmjs.com/package/react?activeTab=versions)
- [Node.js 22 LTS](https://nodejs.org/en/blog/release/v22.11.0)

**Rust:**
- [Tantivy 0.25.0](https://crates.io/crates/tantivy)
- [Tantivy Releases](https://github.com/quickwit-oss/tantivy/releases)
