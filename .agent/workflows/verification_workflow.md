---
description: Comprehensive System Verification Protocol
---

# System Verification Protocol

**trigger**: automated-on-completion
**description**: MUST be executed before marking any complex feature or refactor as "Complete".

## 1. Runtime Environment Check

- [ ] **Service Status**: Run `docker ps` or process list. Verify ALL required services (Backend, DB, Redis, Frontend) are `Up`.
- [ ] **Health Check**: `curl -f http://localhost:8000/api/v1/health` (or equivalent) MUST return 200 OK.
- [ ] **Logs check**: Run `docker-compose logs --tail=50 backend` to check for startup errors.
- [ ] **Clean Import Check**: Run `python debug_imports.py` (or equivalent) to ensure no blocking top-level imports.

## 2. Configuration Verification

- [ ] **Environment Parity**: Verify running container env vars match `docker-compose.yml` and expected defaults.
- [ ] **Persistence Check**: If data pathways were touched, verify volume mounts exist on host: `ls -l ./data/...`

## 3. Integration Verification (No Mocks)

- [ ] **Real API Call**: Execute a critical path API call using `curl` or a robust script _against the running instance_.
  - Example: `curl -X POST http://localhost:8000/api/v1/search -d '{"query": "test"}'`
- [ ] **State Validation**: If an Admin setting was changed, query the database/Redis via CLI to confirm the change persisted.

## 4. UI/Frontend Check (If applicable)

- [ ] **Asset Check**: Verify frontend assets are served (`curl -I http://localhost:3000`).
- [ ] **Code-to-Runtime Link**: Confirm that frontend inputs (e.g., "Add Model") trigger expected backend logs.

**FAILURE PROTOCOL**:
If ANY step fails, the task is **NOT COMPLETE**.

- Stop.
- Fix the environment.
- Re-run verification.
