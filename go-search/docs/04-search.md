# Search Algorithm

## Overview

Hybrid search combining sparse (SPLADE) and dense (semantic) retrieval with RRF fusion and optional reranking. Includes intelligent query understanding for better keyword extraction and intent classification.

---

## Search Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              SEARCH FLOW                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. QUERY INPUT                                                             │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  "find authentication handler in golang"                         │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  1b. QUERY UNDERSTANDING (CodeBERT)                                         │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  Intent: navigational                                            │    │
│     │  Keywords: [authentication, handler, golang, go]                 │    │
│     │  Expanded: [auth, authenticate, login, http.Handler]             │    │
│     │  Confidence: 0.92                                                │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  2. ENCODE QUERY (parallel)                                                 │
│     ┌─────────────────────┐    ┌─────────────────────┐                     │
│     │   SPLADE Encode     │    │   Dense Embed       │                     │
│     │                     │    │                     │                     │
│     │   → Sparse Vector   │    │   → Dense Vector    │                     │
│     │   {auth: 0.9,       │    │   [0.12, 0.34, ...] │                     │
│     │    handler: 0.8,    │    │   (1536 dims)       │                     │
│     │    golang: 0.7}     │    │                     │                     │
│     └──────────┬──────────┘    └──────────┬──────────┘                     │
│                │                          │                                 │
│                ▼                          ▼                                 │
│  3. RETRIEVE (parallel)                                                     │
│     ┌─────────────────────┐    ┌─────────────────────┐                     │
│     │   Sparse Search     │    │   Dense Search      │                     │
│     │   (Qdrant)          │    │   (Qdrant)          │                     │
│     │                     │    │                     │                     │
│     │   Top 100 by        │    │   Top 100 by        │                     │
│     │   sparse score      │    │   cosine similarity │                     │
│     └──────────┬──────────┘    └──────────┬──────────┘                     │
│                │                          │                                 │
│                └────────────┬─────────────┘                                 │
│                             ▼                                               │
│  4. FUSION (RRF)                                                            │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │   RRF Score = Σ (weight_i / (k + rank_i))                       │    │
│     │                                                                  │    │
│     │   For each document:                                             │    │
│     │   - Get rank in sparse results (or infinity if not present)     │    │
│     │   - Get rank in dense results (or infinity if not present)      │    │
│     │   - Compute weighted RRF score                                   │    │
│     │   - Sort by RRF score descending                                 │    │
│     └──────────┬──────────────────────────────────────────────────────┘    │
│                │                                                            │
│                ▼                                                            │
│  5. RERANK (optional)                                                       │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │   Cross-encoder reranking (Jina Reranker)                       │    │
│     │                                                                  │    │
│     │   For top 30 candidates:                                         │    │
│     │   - Score = CrossEncoder(query, document)                        │    │
│     │   - Sort by reranker score descending                           │    │
│     └──────────┬──────────────────────────────────────────────────────┘    │
│                │                                                            │
│                ▼                                                            │
│  6. RETURN TOP K                                                            │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │   Return top 20 results with scores                              │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Step 1: Query Encoding

Query encoding uses the event bus for ML operations:

```
Search Service                     Event Bus                      ML Service
      │                               │                               │
      │  ml.embed.request             │                               │
      │  {texts: [query]}             │                               │
      │──────────────────────────────>│──────────────────────────────>│
      │                               │                               │
      │  ml.embed.response            │   (generate dense embedding)  │
      │  {embeddings: [[...]]}        │                               │
      │<──────────────────────────────│<──────────────────────────────│
      │                               │                               │
      │  ml.sparse.request            │                               │
      │  {texts: [query]}             │                               │
      │──────────────────────────────>│──────────────────────────────>│
      │                               │                               │
      │  ml.sparse.response           │   (generate sparse vector)    │
      │  {vectors: [{...}]}           │                               │
      │<──────────────────────────────│<──────────────────────────────│
```

**Fallback**: If the event bus is unavailable, the service falls back to direct ML calls.

### Sparse Encoding (SPLADE)

SPLADE produces a sparse vector where each dimension = a token from BERT vocabulary.

```
Input:  "authentication handler golang"
Output: {
    "authentication": 0.92,
    "auth": 0.65,
    "handler": 0.81,
    "golang": 0.73,
    "go": 0.45,
    "function": 0.23,
    ...
}
```

Properties:
- Expands query with related terms
- Handles synonyms implicitly
- ~50-200 non-zero values

### Dense Encoding (Jina)

Produces fixed-size semantic vector.

```
Input:  "authentication handler golang"
Output: [0.12, -0.34, 0.56, ...] (1536 floats)
```

Properties:
- Captures semantic meaning
- Works across languages
- L2 normalized

---

## Step 2: Retrieval

### Sparse Retrieval

```python
# Qdrant sparse search
results = qdrant.search(
    collection="rice_code",
    query_vector=NamedSparseVector(
        name="sparse",
        vector=sparse_vector
    ),
    limit=100,
    with_payload=True
)
```

Scoring: Dot product of sparse vectors (similar to BM25).

### Dense Retrieval

```python
# Qdrant dense search
results = qdrant.search(
    collection="rice_code",
    query_vector=NamedVector(
        name="dense",
        vector=dense_vector
    ),
    limit=100,
    with_payload=True
)
```

Scoring: Cosine similarity.

---

## Step 3: RRF Fusion

Reciprocal Rank Fusion combines results from multiple retrievers.

### Formula

```
RRF_score(d) = Σ (weight_i / (k + rank_i(d)))
```

Where:
- `d` = document
- `weight_i` = weight for retriever i (default: 0.5 each)
- `k` = smoothing constant (default: 60)
- `rank_i(d)` = rank of document d in retriever i's results

### Example

```
Document X:
- Sparse rank: 3
- Dense rank: 7

RRF_score = (0.5 / (60 + 3)) + (0.5 / (60 + 7))
          = (0.5 / 63) + (0.5 / 67)
          = 0.00794 + 0.00746
          = 0.01540
```

### Why RRF?

| Property | Benefit |
|----------|---------|
| Score-agnostic | Works with different score scales |
| Simple | No tuning needed |
| Effective | Proven in information retrieval |
| Robust | Handles missing documents gracefully |

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `sparse_weight` | 0.5 | Weight for sparse retriever |
| `dense_weight` | 0.5 | Weight for dense retriever |
| `rrf_k` | 60 | Smoothing constant |

---

## Step 4: Reranking

Optional cross-encoder reranking for improved precision.

### Event-Driven Reranking

Reranking uses the event bus for ML operations:

```
Search Service                     Event Bus                      ML Service
      │                               │                               │
      │  ml.rerank.request            │                               │
      │  {query, documents, top_k}    │                               │
      │──────────────────────────────>│──────────────────────────────>│
      │                               │                               │
      │  ml.rerank.response           │   (cross-encoder scoring)     │
      │  {results: [{index, score}]}  │                               │
      │<──────────────────────────────│<──────────────────────────────│
```

### How It Works

```
For each candidate document:
    score = CrossEncoder(query, document_content)
    
Sort by score descending
Return top K
```

### Cross-Encoder vs Bi-Encoder

| Aspect | Bi-Encoder (Retrieval) | Cross-Encoder (Reranking) |
|--------|------------------------|---------------------------|
| Speed | Fast (pre-computed) | Slow (computed per pair) |
| Quality | Good | Better |
| Use case | Initial retrieval | Final ranking |

### Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `enable_reranking` | true | Enable/disable reranking |
| `rerank_top_k` | 30 | Candidates to rerank |

---

## Step 5: Filtering

Filters are applied during retrieval, not post-retrieval.

### Path Prefix Filter

```python
# Only search in src/auth/
filter = Filter(
    must=[
        FieldCondition(
            key="path",
            match=MatchText(text="src/auth/")
        )
    ]
)
```

### Language Filter

```python
# Only Go and TypeScript
filter = Filter(
    must=[
        FieldCondition(
            key="language",
            match=MatchAny(any=["go", "typescript"])
        )
    ]
)
```

### Combined Filters

```python
filter = Filter(
    must=[
        FieldCondition(key="path", match=MatchText(text="src/")),
        FieldCondition(key="language", match=MatchAny(any=["go"]))
    ]
)
```

---

## Performance Characteristics

### Latency Breakdown

| Stage | Typical Latency | Notes |
|-------|-----------------|-------|
| Query encoding | 20-50ms | Parallel sparse + dense |
| Sparse retrieval | 10-20ms | Qdrant sparse search |
| Dense retrieval | 20-40ms | Qdrant dense search |
| RRF fusion | 1-5ms | In-memory merge |
| Reranking | 40-80ms | Cross-encoder inference |
| **Total (with rerank)** | 90-200ms | |
| **Total (no rerank)** | 50-120ms | |

### Scaling

| Documents | Expected Latency |
|-----------|------------------|
| 10K chunks | 50-100ms |
| 100K chunks | 60-120ms |
| 1M chunks | 80-150ms |

---

## Quality Tuning

### Sparse-Heavy (Keyword-Focused)

```json
{
    "sparse_weight": 0.7,
    "dense_weight": 0.3
}
```

Best for: Exact matches, known function names, specific terms.

### Dense-Heavy (Semantic-Focused)

```json
{
    "sparse_weight": 0.3,
    "dense_weight": 0.7
}
```

Best for: Conceptual queries, "how to", exploratory search.

### Balanced (Default)

```json
{
    "sparse_weight": 0.5,
    "dense_weight": 0.5
}
```

Best for: General use, mixed queries.

---

## Edge Cases

### Empty Query

- Return error: "query cannot be empty"

### No Results

- Return empty results array
- Total = 0

### Document Not in Both Retrievers

- RRF handles gracefully
- Missing retriever contributes 0 to score

### Very Long Query

- Truncate to 512 tokens for SPLADE
- Truncate to 8192 tokens for dense
- Log warning if truncated

---

## Query Understanding

Query understanding uses CodeBERT to convert natural language queries into optimized search terms.

### How It Works

```
User Query: "find authentication handler in golang"
                        │
                        ▼
              ┌─────────────────┐
              │    CodeBERT     │
              │   (125M params) │
              └────────┬────────┘
                       │
                       ▼
              ┌─────────────────────────────────────────┐
              │ ParsedQuery:                            │
              │   Original: "find authentication..."    │
              │   Intent: navigational                  │
              │   Keywords: [authentication, handler]   │
              │   CodeTerms: [golang, go, handler]      │
              │   Expanded: [auth, login, http.Handler] │
              │   Confidence: 0.92                      │
              │   UsedModel: true                       │
              └─────────────────────────────────────────┘
```

### Intent Types

| Intent | Description | Example Query |
|--------|-------------|---------------|
| `navigational` | Find specific code location | "where is the auth handler" |
| `factual` | Answer about code | "what does ParseQuery do" |
| `exploratory` | Discover related code | "how does authentication work" |
| `analytical` | Understand patterns | "why does login call validate" |

### Keyword Expansion

CodeBERT expands queries with code-aware synonyms:

| Original Term | Expanded Terms |
|---------------|----------------|
| `auth` | authentication, authenticate, login, session |
| `handler` | handler, Handler, http.Handler, HandlerFunc |
| `error` | error, err, Error, exception, panic |
| `config` | config, configuration, settings, options |

### Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `QueryEnabled` | `true` | Use CodeBERT model |
| `QueryModel` | `microsoft/codebert-base` | Model to use |
| `QueryGPU` | `true` | Run on GPU |

### Fallback

When model is disabled or fails:
- Uses heuristic keyword extraction (`keyword_extractor.go`)
- Pattern-based intent detection
- Static synonym expansion
- Still provides quality results, just less semantic understanding

### Response Metadata

Query understanding results are included in search response:

```json
{
  "parsed_query": {
    "original": "find authentication handler",
    "intent": "navigational",
    "keywords": ["authentication", "handler"],
    "code_terms": ["handler"],
    "expanded": ["auth", "authenticate", "login"],
    "confidence": 0.92,
    "used_model": true
  }
}
```
