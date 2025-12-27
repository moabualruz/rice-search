# Rice Search - Agent Guidelines

## Build & Test Commands

```bash
# API (NestJS + Bun)
cd api
bun install && bun run build && bun run start:dev  # Dev server
bun run lint && bun run typecheck                  # Quality checks
bun test                                           # Run all tests

# ricegrep CLI (TypeScript + Bun)  
cd ricegrep
bun install && bun run build                       # Build CLI
bun run format && bun run typecheck                # Quality checks
bun test                                           # Run all tests
bun test --filter "Search"                         # Run specific test pattern

# Web UI (Next.js + Bun)
cd web-ui
bun install && bun run build                       # Build for production
bun run dev                                        # Dev server (http://localhost:3000)
bun run lint                                       # ESLint checks

# Full Platform
docker-compose up -d                               # Start all services
bash scripts/smoke_test.sh                         # End-to-end test
```

## Code Style & Standards

**Formatting**: Biome (ricegrep) + ESLint/Prettier (API) + Next.js ESLint (web-ui)  
**Types**: Strict TypeScript. Never use `any`, `@ts-ignore`, or suppress errors  
**Imports**: Node built-ins with `node:` prefix (`node:fs`, `node:path`)  
**Strings**: Double quotes (API/CLI), single quotes in JSX (web-ui), 2-space indentation  
**Architecture**: Uses bun throughout. Keep services decoupled (api/, ricegrep/, web-ui/)  
**Config**: YAML config files (`.ricegreprc.yaml`), env vars with `RICEGREP_` prefix  
**Error Handling**: Custom error classes, proper validation with zod/class-validator  
**File Organization**: `lib/` for utilities, `commands/` for CLI, `src/` for main code  
**SVG Files**: Never read SVG files directly (causes critical errors in opencode). Use PNG alternatives or file paths only  

Run `bun run typecheck` before any significant changes. Never commit without clean diagnostics.