# Master Requirements Document: Rice Search System (V3)

**Status:** Approved Specification
**Scope:** End-to-End Functional, Architectural, Operational, and Administrative Requirements
**Positioning:** Open-Source, Local-First, GPU-Accelerated Code Search & Intelligence Platform
**Philosophy:** "The Intelligence Layer for Code" - General-Purpose, Code-Optimized, RAG-Native.

---

## 1. Project Vision

Rice Search V3 is a **general-purpose neural search and intelligence platform**, optimized for source code but capable of ingesting any structured or unstructured data (code, documentation, configs, logs).

The system exists to:

1.  **Understand**: Move beyond keyword matching to semantic and structural understanding beyond keywords, using embeddings, ASTs, and metadata.
2.  **Answer**: Synthesize accurate, cited answers across large codebases and documents using RAG.
3.  **Serve**: Act as the backend intelligence brain for:
    - Human developers (CLI / Web UI)
    - AI agents (MCP-compatible tools, IDEs, assistants)

---

## 2. Architectural Foundations

### 2.1. Architectural Laws (Non-Negotiable)

- **Domain-Driven Design (DDD):** The system is partitioned into strict bounded contexts. Domains do not share internal state.
- **Event-Driven Architecture (EDA):** All state mutations emit events. Producers are unaware of consumers. Events are the only cross-domain communication mechanism.
- **Configuration-Driven System:** All behavior must be configurable. No runtime behavior may require code changes.
- **Runtime Reconfigurability:** All configuration must be reloadable at runtime via the Admin UI.
- **Test-Driven Development (TDD):** Every requirement must be testable. Failure cases define correct behavior.

---

## 3. Deployment Topology (Dual-Mode)

The system must support both modes from a single codebase, controlled entirely by configuration.

### 3.1. Mode A — Modular Monolith (Default)

- **Process:** Single process.
- **Communication:** In-memory event bus.
- **Ingestion:** Local filesystem ingestion.
- **Database:** Local Qdrant instance.
- **GPU:** Enabled by default (if hardware allows).
- **Target:** Local development, laptops, single servers, personal "second brain" usage.

### 3.2. Mode B — Distributed Microservices

- **Process:** Independent services per domain (containers).
- **Communication:** Network event bus.
- **Compute:** Dedicated GPU inference nodes.
- **Database:** Qdrant cluster with sharding.
- **Target:** Enterprise, SaaS, large monorepos, multi-tenant environments.

---

## 4. Mandatory Technology Constraints

### 4.1. Vector Database

- **Qdrant is Mandatory.** It acts as the single source of truth for:
  - Vector embeddings.
  - Chunk metadata.
  - Index state.
- **Features:**
  - GPU acceleration enabled by default.
  - Payloads store structured metadata (not treated as a relational DB).
  - If relational needs arise later, an additional DB may be introduced without breaking abstractions.

### 4.2. Event Bus

- **Abstract Interface:** Must support pluggable backends:
  - In-memory (Monolith default).
  - Redis (Simple distributed).
  - NATS / Kafka (High scale).
- **Hot-Swappable:** Backend configurable at runtime.

---

## 5. Runtime Configuration & GPU Behavior

### 5.1. GPU Execution

- **Default:** GPU mode is ALWAYS ON by default.
- **Scope:** Applies to Embedding models, Reranking models, and Vector search.

### 5.2. Runtime GPU Control

- **Control:** Users may enable/disable GPU usage at runtime via Admin UI.
- **Safety:** System must warn of interruptions and gracefully pause/resume workloads.

### 5.3. Configuration Management

- **Capabilities:**
  - Schema-validated settings.
  - Reloadable at runtime.
  - Editable via Admin UI and API.
  - Supported Configs: Models, GPU usage, Event bus, Search weights, Indexing behavior, Tenancy rules, Security policies.
  - **Rollback:** Must be supported.

---

## 6. Functional Domains (Bounded Contexts)

### 6.1. Ingestion Domain

**Responsibility:** Convert raw inputs into structured semantic units.

- **REQ-ING-01 (AST-Aware Parsing):** Code is parsed structurally using Tree-sitter. Chunks map to Functions, Classes, Methods, Logical blocks.
- **REQ-ING-02 (Watch Mode):** Real-time filesystem monitoring with debounce + deduplication (<1s propagation target).
- **REQ-ING-03 (Polyglot Support):** Go, Python, JS/TS, Rust, Java, C++, with adapter-based extension.
- **REQ-ING-04 (Content Addressability):** Global deduplication via content hashing.
- **REQ-ING-05 (Ignore Semantics):** Respect `.gitignore`, `.riceignore` with hierarchical precedence.

### 6.2. Indexing & Storage Domain

**Responsibility:** Manage hybrid semantic representation.

- **REQ-IDX-01 (Hybrid Index):** Dense embeddings (semantic) + Sparse vectors (lexical / SPLADE).
- **REQ-IDX-02 (Metadata Enrichment):** Language, Symbols, File path, Chunk type, Ownership.
- **REQ-IDX-03 (GPU-Aware Indexing):** Indexing lifecycle adapts to GPU availability.
- **REQ-IDX-04 (Sharding):** Transparent sharding support via Qdrant collections.

### 6.3. Search Domain

**Responsibility:** Intent → Retrieve → Rank.

- **REQ-SRCH-01 (Query Intent Analysis):** Infer Lookup vs Explanation, Scope (path, language, store).
- **REQ-SRCH-02 (Hybrid Retrieval):** Fusion of Dense + Sparse scores (Configurable strategy, RRF default).
- **REQ-SRCH-03 (Neural Reranking):** Cross-encoder reranking of top-K results.
- **REQ-SRCH-04 (Determinism):** Same query + same state = same results.

### 6.4. Intelligence Domain (RAG)

**Responsibility:** Synthesize answers and serve agents.

- **REQ-INT-01 (Generative Answers):** Natural language synthesis with **mandatory line-level citations**.
- **REQ-INT-02 (Iterative Deep Dive):** Multi-step retrieval with configurable depth.
- **REQ-INT-03 (Agent Protocol / MCP):**
  - **Transports:** StdIO (CLI Proxy), TCP / SSE.
  - **Tools:** `search`, `read_file`, `list_files`.
- **REQ-INT-04 (Structured Outputs):** Markdown, JSON, Machine-readable citations.

### 6.5. Identity, Tenancy & Collaboration Domain

**Responsibility:** Managing Scope, Access, and User Context.

- **Identity:** Integration with OpenID Connect / OAuth2 (Keycloak, Ory). RBAC + MFA support.
- **Multi-Tenant Model:**
  - **Organizations:** SaaS-style tenants.
  - **Users:** Belong to organizations.
  - **Connections:** Represent physical machines/devices, scoped to a user.
  - **Stores:** Logical indexes (prod, staging, dev, personal).
- **Roles:** System Admin, Org Admin, Team Member, Read-Only User.
- **Collaboration Rules:**
  - Users index personal work to personal stores.
  - Team admins control access to shared (prod/staging) stores.
  - Connections track indexed content independently.

---

## 7. Model Registry & Execution (Mandatory)

### 7.1. Default Models

The system **MUST** ship with and enable these specific models by default:

- **Embedding:** `jinaai/jina-code-embeddings-1.5b` (1536 dim, code-optimized).
- **Reranker:** `jinaai/jina-reranker-v2-base-multilingual` (Code-aware cross-encoder).
- **Query Understanding:** `microsoft/codebert-base` (Intent classification).

### 7.2. Model Lifecycle Features

- **Hot-Swapping:** Change active models at runtime.
- **Offline Mode:** Models run locally by default (ONNX).
- **GPU Toggle:** Per-model configuration.
- **Declarative Mappers:** YAML-based output tensor mapping.

---

## 8. User Interface Requirements

**Philosophy:** UI is control + observability, not logic.

### 8.1. Core Pages

- **Search Console:** The primary search interface.
- **Answer View:** RAG results with side-by-side context.
- **File Explorer:** Navigable view of indexed content.
- **Store Gallery:** Overview of available indexes.
- **Store Detail:** Deep dive into a specific index's stats and content.

### 8.2. Admin Interface ("Mission Control")

- **Capabilities:**
  - **Model Management:** Download, Activate, Delete, GPU toggle.
  - **Event Bus Config:** Runtime backend switching.
  - **Runtime Config Editor:** Edit all settings with rollback.
  - **User/Team Management:** Manage identities and connection scopes.
- **Observability:**
  - **Metrics:** Prometheus integration.
  - **Traces:** Search latency breakdowns.
  - **Resources:** Real-time GPU/RAM usage.
  - **Audit:** All admin actions are logged.

---

## 9. Non-Functional Requirements

### 9.1. Performance

- **Latency:** Search P95 < 200ms (excluding generation).
- **Throughput:** Ingestion > 50MB/s per node.

### 9.2. Scalability

- **Volume:** 10GB+ Monorepos.
- **Sharding:** Vector DB clustering support.

### 9.3. Observability

- **Metrics:** Prometheus endpoint.
- **Tracing:** Distributed correlation IDs across domains.
