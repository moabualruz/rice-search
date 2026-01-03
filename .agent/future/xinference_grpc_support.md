# Future: Native gRPC Support for Xinference

> **Status:** DEFERRED  
> **Priority:** Low  
> **Estimated Effort:** 2-4 weeks  
> **Created:** 2026-01-03

## Overview

This document outlines a future enhancement to add native gRPC APIs to Xinference for lower-latency inference calls.

## Current State

Rice Search uses Xinference via REST API:

- Embeddings: `POST /v1/embeddings`
- Reranking: `POST /v1/rerank`
- Chat: `POST /v1/chat/completions`

## Proposed Enhancement

Create a custom Xinference overlay that exposes all APIs over gRPC:

- True gRPC streaming for chat tokens
- Binary protobuf serialization
- HTTP/2 multiplexing
- Auto-generated protos from Xinference schemas

## Why Deferred

| Factor                | Current State                                                     |
| --------------------- | ----------------------------------------------------------------- |
| **Latency breakdown** | Model inference: 50-500ms, Network: 1-5ms                         |
| **Bottleneck**        | Model inference dominates (99% of time)                           |
| **REST performance**  | httpx with connection pooling is sufficient                       |
| **Complexity**        | gRPC overlay requires ongoing maintenance with Xinference updates |
| **ROI**               | Low - optimizing 1% of latency doesn't justify weeks of work      |

## When to Reconsider

Revisit this enhancement when:

1. **Scale increases** - Thousands of requests/second
2. **Latency requirements tighten** - Sub-10ms responses needed
3. **Xinference adds native gRPC** - Adopt upstream support
4. **Rust client needs streaming** - True bidirectional streaming required

## Alternative Optimizations (Now)

Instead of gRPC, focus on:

- Connection pooling (already using httpx)
- Request batching where possible
- Model optimization (quantization, pruning)
- Caching frequent queries

## Technical Approach (When Ready)

```
┌─────────────────────────────────────────────┐
│           Custom Xinference Image           │
├─────────────────────────────────────────────┤
│  gRPC Server (grpcio)  │  REST Server       │
│  :50051                │  :9997             │
├─────────────────────────────────────────────┤
│          Shared Xinference Core             │
│  (vLLM, schedulers, batching, etc.)         │
└─────────────────────────────────────────────┘
```

### Implementation Steps (Future)

1. Proto generation script from Pydantic/OpenAPI schemas
2. gRPC server implementation calling Xinference internals
3. Dockerfile extending `xprobe/xinference`
4. Client updates (Python + Rust)

## References

- [Xinference GitHub](https://github.com/xorbitsai/inference)
- [gRPC Python](https://grpc.io/docs/languages/python/)
- [vLLM gRPC discussion](https://github.com/vllm-project/vllm/discussions)
