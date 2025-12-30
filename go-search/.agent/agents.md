# Agent Steering & Rules

This document serves as the primary source of truth for agent behavior, project-specific rules, and architectural standards.

## Project Philosophy
- **Pure Go**: No sidecars (except Qdrant/Redis binaries if needed), no Python, no Node.js for the backend.
- **GPU-First**: ML operations should prioritize GPU (CUDA/TensorRT) with transparent CPU fallbacks.
- **Event-Driven**: Services communicate via the `internal/bus` package. Avoid direct coupling where possible.
- **Zero-JS Web**: Use `templ` + `HTMX` for the frontend. No React/Vue/Svelte.

## Code Standards
- **Directory Structure**: strict adherence to `internal/` layout.
- **Error Handling**: Wrap errors with context.
- **Configuration**: Use `kelseyhightower/envconfig` for environment variables.

## Workflows
- **Spec-Driven Development (SDD)**: (To be implemented) - Future changes should follow a specification phase.

## Task Management
- Always maintain `task.md` for current progress.
- Update `system_overview.md` when architecture changes.
