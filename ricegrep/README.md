<div align="center">
  <a href="https://github.com/rice-search/ricegrep">
    <img src="../.branding/logo.svg" alt="ricegrep" width="96" height="96" />
  </a>
  <h1>ricegrep</h1>
  <p><em>Port of ricegrep with local hybrid search provider (Rice Search). CLI-native semantic code search with BM25 + embeddings.</em></p>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License: Apache 2.0" /></a>

  <br>

  <p align="center">
    <video src="https://github.com/user-attachments/assets/7cb6d2ab-f96b-4092-9088-abbca85b0d52" controls="controls" style="max-width: 730px;">
      Your browser does not support the video tag.
    </video>
  </p>
</div>

## What is ricegrep?

**ricegrep** is a CLI tool that uses **Rice Search** - a fully local hybrid search platform combining BM25 (keyword) + semantic embeddings for code search.

### Attribution & Inspiration

This project is inspired by and builds upon the excellent work of:
- **[Mixedbread AI's mgrep](https://github.com/mixedbread-ai/mgrep)** - The original semantic code search CLI
- **[Mixedbread AI platform](https://mixedbread.ai)** - Pioneering hybrid search technology

**ricegrep** serves as a **fully local replacement** that brings the power of semantic code search to your own infrastructure, eliminating cloud dependencies while maintaining the intuitive search experience of the original tools.

### Why ricegrep?
- 🏠 **Fully Local**: All search runs on your own hardware - no cloud dependencies
- 🔍 **Hybrid Search**: Combines BM25 keyword matching + semantic embeddings for best results
- 🚀 **Fast**: Natural-language search that feels as immediate as `grep`
- 🎯 **Code-First**: Built specifically for searching codebases with symbol/path awareness
- 🔄 **Smart Indexing**: Incremental updates via `ricegrep watch` - only re-indexes changed files
- 🤖 **Agent-Ready**: Designed for AI coding assistants with clean, structured output

```bash
# index your repo once
ricegrep watch

# then ask your repo things in natural language
ricegrep "where do we set up auth?"
```

## Quick Start

### Prerequisites

First, start the Rice Search platform (see [main README](../README.md)):
```bash
cd rice-search
docker-compose up -d
```

### 1. **Install ricegrep**

   **Using npm (recommended):**
   ```bash
   cd ricegrep
   npm install -g .
   ```

   **Using bun (faster alternative):**
   ```bash
   cd ricegrep
   bun install -g .
   ```

   **Using pnpm:**
   ```bash
   cd ricegrep
   pnpm install -g .
   ```

   **From source (development):**
   ```bash
   cd ricegrep
   npm install        # or bun install
   npm run build      # or bun run build  
   npm link           # or bun link
   ```

### 2. **Configure (optional)**
   ricegrep connects to `http://localhost:8080` by default. To change:
   ```bash
   export RICEGREP_BASE_URL=http://your-rice-search-url:8080
   export RICEGREP_STORE=default  # or your custom store name
   ```

### 3. **Index a project**
   ```bash
   cd path/to/your/repo
   ricegrep watch
   ```
   `watch` performs an initial sync, respects `.gitignore`, then keeps the local Rice Search index updated as files change.

### 4. **Search anything**
   ```bash
   ricegrep "where do we set up auth?" src/lib
   ricegrep -m 25 "store schema"
   ```
   Searches default to the current working directory unless you pass a path.

## Using it with Coding Agents

`ricegrep` supports assisted installation commands for many agents:
- `ricegrep install-claude-code` for Claude Code
- `ricegrep install-opencode` for OpenCode
- `ricegrep install-codex` for Codex
- `ricegrep install-droid` for Factory Droid

These commands configure the agent to use Rice Search via `ricegrep`. After that you only have to start the agent in your project folder.

### More Agents Coming Soon

More agents (Cursor, Windsurf, etc.) are on the way—this section will grow as soon as each integration lands.

## How ricegrep improves code search

Hybrid search (BM25 + semantic embeddings) finds relevant code much faster than keyword-only tools.

**Why hybrid search works better:**
- BM25 finds exact symbol/function matches
- Semantic embeddings understand intent ("where do we handle auth?" finds authentication logic)
- Fusion ranking combines both for optimal relevance
- Your AI agent spends tokens on reasoning instead of scanning irrelevant code

`ricegrep` finds the relevant snippets in a few queries, reducing wasted search iterations.

## About Rice Search & ricegrep

**Rice Search** is a fully local code search platform that combines:
- **Tantivy** (BM25 sparse search) for keyword/symbol matching
- **Milvus** (vector database) for semantic embeddings
- **Hybrid fusion ranking** (RRF + code-specific heuristics)

**ricegrep** is a port of Rice Search's `ricegrep` CLI tool, modified to use Rice Search instead of cloud services. All the benefits of semantic code search, running entirely on your own infrastructure.

### Why Local Search Matters

- **Privacy**: Your code never leaves your machine
- **No API costs**: No per-query fees or usage limits
- **Offline capable**: Works without internet connection
- **Control**: Customize embeddings, ranking, and indexing behavior

Under the hood, Rice Search uses state-of-the-art retrieval models (BGE embeddings) and code-aware parsing (Tree-sitter) to provide a natural language companion to `grep`. We believe both tools belong in your toolkit: use `grep` for exact matches, `ricegrep` for semantic understanding and intent.


## When to use what

We designed `ricegrep` to complement `grep`, not replace it. The best code search combines `ricegrep` with `grep`.

| Use `grep` (or `ripgrep`) for... | Use `ricegrep` for... |
| --- | --- |
| **Exact Matches** | **Intent Search** |
| Symbol tracing, Refactoring, Regex | Code exploration, Feature discovery, Onboarding |

## Commands at a Glance

| Command | Purpose |
| --- | --- |
| `ricegrep` / `ricegrep search <pattern> [path]` | Natural-language search with many `grep`-style flags (`-i`, `-r`, `-m`...). |
| `ricegrep watch` | Index current repo and keep the Rice Search store in sync via file watchers. |
| `ricegrep login` & `ricegrep logout` | Manage device-based authentication with Rice Search. |
| `ricegrep install-claude-code` | Authenticate, add the Rice Search ricegrep plugin to Claude Code. |
| `ricegrep install-opencode` | Authenticate and add the Rice Search ricegrep to OpenCode. |
| `ricegrep install-codex` | Authenticate and add the Rice Search ricegrep to Codex. |
| `ricegrep install-droid` | Authenticate and add the Rice Search ricegrep hooks/skills to Factory Droid. |

### ricegrep search

`ricegrep search` is the default command. It can be used to search the current
directory for a pattern.

| Option | Description |
| --- | --- |
| `-m <max_count>` | The maximum number of results to return |
| `-c`, `--content` | Show content of the results |
| `-a`, `--answer` | Generate an answer to the question based on the results |
| `-w`, `--web` | Include web search results alongside local files |
| `-s`, `--sync` | Sync the local files to the store before searching |
| `-d`, `--dry-run` | Dry run the search process (no actual file syncing) |
| `--no-rerank` | Disable reranking of search results |
| `--max-file-size <bytes>` | Maximum file size in bytes to upload (overrides config) |
| `--max-file-count <count>` | Maximum number of files to upload (overrides config) |

All search options can also be configured via environment variables (see
[Environment Variables](#environment-variables) section below).

**Examples:**
```bash
ricegrep "What code parsers are available?"  # search in the current directory
ricegrep "How are chunks defined?" src/models  # search in the src/models directory
ricegrep -m 10 "What is the maximum number of concurrent workers in the code parser?"  # limit the number of results to 10
ricegrep -a "What code parsers are available?"  # generate an answer to the question based on the results
ricegrep --web --answer "How do I integrate a JavaScript runtime into Deno?"  # search the web and get a summarized answer
```

### ricegrep watch

`ricegrep watch` is used to index the current repository and keep the Rice Search
store in sync via file watchers.

It respects the current `.gitignore`, as well as a `.ricegrepignore` file in the
root of the repository. The `.ricegrepignore` file follows the same syntax as the
[`.gitignore`](https://git-scm.com/docs/gitignore) file.

| Option | Description |
| --- | --- |
| `-d`, `--dry-run` | Dry run the watch process (no actual file syncing) |
| `--max-file-size <bytes>` | Maximum file size in bytes to upload (overrides config) |
| `--max-file-count <count>` | Maximum number of files to upload (overrides config) |

**Examples:**
```bash
ricegrep watch  # index the current repository and keep the Rice Search store in sync via file watchers
ricegrep watch --max-file-size 1048576  # limit uploads to files under 1MB
ricegrep watch --max-file-count 5000  # limit uploads to directories with 5000 files or fewer
```

## Rice Search under the hood

- Every file is pushed into a Rice Search Store using the same SDK your apps get.
- Searches request top-k matches with Rice Search reranking enabled by default
  for tighter relevance (can be disabled with `--no-rerank` or
  `RICEGREP_RERANK=0`).
- Results include relative paths plus contextual hints (line ranges for text, page numbers for PDFs, etc.) for a skim-friendly experience.
- Because stores are cloud-backed, agents and teammates can query the same corpus without re-uploading.

## Configuration

ricegrep can be configured via config files, environment variables, or CLI flags.

### Config File

Create a `.ricegreprc.yaml` (or `.ricegreprc.yml`) in your project root for local configuration, or `~/.config/ricegrep/config.yaml` (or `config.yml`) for global configuration.

```yaml
# Maximum file size in bytes to upload (default: 10MB)
maxFileSize: 5242880

# Maximum number of files to upload (default: 10000)
maxFileCount: 5000
```

**Configuration precedence** (highest to lowest):
1. CLI flags (`--max-file-size`, `--max-file-count`)
2. Environment variables (`RICEGREP_MAX_FILE_SIZE`, `RICEGREP_MAX_FILE_COUNT`)
3. Local config file (`.ricegreprc.yaml` in project directory)
4. Global config file (`~/.config/ricegrep/config.yaml`)
5. Default values

### Configuration Tips

- `--store <name>` lets you isolate workspaces (per repo, per team, per experiment). Stores are created on demand if they do not exist yet.
- Ignore rules come straight from git, so temp files, build outputs, and vendored deps stay out of your embeddings.
- `watch` reports progress (`processed / uploaded`) as it scans; leave it running in a terminal tab to keep your store fresh.
- `search` accepts most `grep`-style switches, and politely ignores anything it cannot support, so existing muscle memory still works.

## Environment Variables

All search options can be configured via environment variables, which is
especially useful for CI/CD pipelines or when you want to set defaults for all
searches.

### Authentication & Store

- `RICEGREP_API_KEY`: Set this to authenticate without browser login (ideal for CI/CD)
- `RICEGREP_STORE`: Override the default store name (default: `ricegrep`)

### Search Options

- `RICEGREP_MAX_COUNT`: Maximum number of results to return (default: `10`)
- `RICEGREP_CONTENT`: Show content of the results (set to `1` or `true` to enable)
- `RICEGREP_ANSWER`: Generate an answer based on the results (set to `1` or `true` to enable)
- `RICEGREP_WEB`: Include web search results (set to `1` or `true` to enable)
- `RICEGREP_SYNC`: Sync files before searching (set to `1` or `true` to enable)
- `RICEGREP_DRY_RUN`: Enable dry run mode (set to `1` or `true` to enable)
- `RICEGREP_RERANK`: Enable reranking of search results (set to `0` or `false` to disable, default: enabled)

### Sync Options

- `RICEGREP_MAX_FILE_SIZE`: Maximum file size in bytes to upload (default: `10485760` / 10MB)
- `RICEGREP_MAX_FILE_COUNT`: Maximum number of files to upload (default: `10000`)

**Examples:**
```bash
# Set default max results to 25
export RICEGREP_MAX_COUNT=25
ricegrep "search query"

# Always show content in results
export RICEGREP_CONTENT=1
ricegrep "search query"

# Disable reranking globally
export RICEGREP_RERANK=0
ricegrep "search query"

# Use multiple options together
export RICEGREP_MAX_COUNT=20
export RICEGREP_CONTENT=1
export RICEGREP_ANSWER=1
ricegrep "search query"
```

Note: Command-line options always override environment variables.

## Development

**Using pnpm (recommended):**
```bash
pnpm install
pnpm build        # or pnpm dev for a quick compile + run
pnpm format       # biome formatting + linting
pnpm test         # run tests
```

**Using bun (faster alternative):**
```bash
bun install
bun run build     # or bun run dev for a quick compile + run
bun run format    # biome formatting + linting  
bun run test      # run tests
```

**Using npm:**
```bash
npm install
npm run build     # or npm run dev for a quick compile + run
npm run format    # biome formatting + linting
npm test          # run tests
```

### Development Notes

- The executable lives at `dist/index.js` (built from TypeScript via `tsc`).
- Husky is wired via `pnpx husky init` (run `npx husky init` or `bunx husky init` once after cloning).
- `pnpm typecheck` / `bun run typecheck` / `npm run typecheck` is your best friend before publishing.
- To connect to a local Rice Search API set `export NODE_ENV=development`.

### Testing

The tests are written using [bats](https://bats-core.readthedocs.io/en/stable/).

## Troubleshooting

- **Watcher feels noisy**: set `RICEGREP_STORE` or pass `--store` to separate experiments, or pause the watcher and restart after large refactors.
- **Need a fresh store**: create a new store using the Rice Search API, then run `ricegrep watch`. Stores are created automatically when needed.
- **Connection issues**: Ensure the Rice Search backend is running at `localhost:8080` (or your configured `RICEGREP_BASE_URL`).

## Support

For usage questions, feedback, or other support, please open an issue in the Rice Search repository.

## Credits

- **[Mixedbread AI](https://mixedbread.ai)** - Original inspiration and hybrid search technology
- **[mgrep](https://github.com/mixedbread-ai/mgrep)** - The original semantic code search CLI that inspired this project
- Rice Search contributors - For creating the local infrastructure that powers ricegrep

## License

Apache-2.0. See the [LICENSE](https://opensource.org/licenses/Apache-2.0) file for details.
