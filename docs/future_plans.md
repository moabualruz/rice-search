# Future Plans & Missing Features

This document tracks features mentioned in earlier documentation or planned for the future, but not yet implemented in the current codebase.

## CLI
- **Rust-based CLI**: The current active CLI is Python-based (`backend/src/cli`). A standalone Rust CLI was planned (scaffolded in `client/`) but is not the primary tool.

## Search Features
- **Deep Rerank**: The "deep-rerank" retrieval strategy (using LLM to rerank top results) is mentioned in high-level docs but not yet fully implemented in `SearchEngine` logic.

## Infrastructure & DevOps
- **Telemetry**: OpenTelemetry integration (`telemetry.enabled`) is present in settings but not fully instrumented across all services.
- **Testing Structure**: Refactor the current flat `backend/tests/` directory into structured `unit/`, `integration/`, and `e2e/` directories as originally planned.

## Security
- **Advanced Authentication**: Implementation of OAuth 2.0, OIDC (Keycloak), and JWT token management.
- **Rate Limiting**: Per-user or per-IP rate limiting (`slowapi`) is planned but not implemented.
- **Audit Logging**: Comprehensive logging of sensitive operations (file access, deletions) is pending.

## Documentation
- **API Examples**: Generate interactive API documentation (Swagger/OpenAPI) with real request examples.
- **Glossary**: Create a strictly defined glossary of terms (e.g., "Chunk", "Node", "Collection") to avoid ambiguity.
