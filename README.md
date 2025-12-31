<div align="center">

<img src=".branding/logo.svg" alt="Rice Search" width="120">

# **🔍Rice Search Platform🔎**

[![License: CC BY-NC-SA 4.0](https://img.shields.io/badge/License-CC%20BY--NC--SA%204.0-lightgrey.svg)](https://creativecommons.org/licenses/by-nc-sa/4.0/)

**Intelligent hybrid search with adaptive retrieval**

</div>

## Overview

Rice Search is a fully local, self-hosted hybrid search platform combining keyword search with semantic embeddings. Unlike static hybrid search, Rice Search uses **retrieval intelligence** to adapt its search strategy based on query characteristics.

## Key Features

### Intelligent Retrieval

- **Intent Classification** - Detects query type (navigational, factual, exploratory, analytical)
- **Adaptive Strategy** - Routes queries to optimal retrieval path:
  - `sparse-only` - Fast BM25 for exact lookups
  - `balanced` - Standard hybrid for general queries
  - `dense-heavy` - Semantic-focused for concept searches
  - `deep-rerank` - Multi-pass reranking for complex queries
- **Multi-Pass Reranking** - Two-stage neural reranking with early exit for efficiency
- **Query Expansion** - Automatic synonym expansion for better recall

### Post-Processing Pipeline

- **Semantic Deduplication** - Removes near-duplicate chunks (configurable threshold)
- **MMR Diversity** - Maximal Marginal Relevance ensures varied results
- **File Aggregation** - Groups chunks by file with representative selection

### Infrastructure

- **Fully Local** - No external API calls, all data stays on your machine
- **GPU Optional** - CPU by default, GPU acceleration available
- **MCP Support** - Model Context Protocol for AI assistant integration
- **ricegrep CLI** - Fast command-line search with watch mode

## Architecture

## Quick Start

### Prerequisites

### 1. Setup & Start

### 2. Index Your Code

### 3. Search

**Web UI**:

**API**:

**Client CLI**:

## Search API

Response includes intelligence metadata

## Client CLI

## MCP Integration

Rice Search supports the Model Context Protocol for AI assistant integration.

**Available Tools:**

**Configuration:**

## GPU vs CPU Mode

## Configuration

### Service Ports

## Data Persistence

## Development

## License

CC BY-NC-SA 4.0

## Credits
