# Verification Audit

**Date:** December 31, 2025
**Purpose:** Identify gaps between implemented features/requirements and E2E test coverage.

## Summary

| Feature              | Implemented | E2E Coverage                        | Gap                                |
| -------------------- | ----------- | ----------------------------------- | ---------------------------------- |
| **Search API**       | ✅ Yes      | ✅ `api/search.spec.ts`             | Good.                              |
| **Stores API**       | ✅ Yes      | ✅ `api/stores.spec.ts`             | Good.                              |
| **Models API**       | ✅ Yes      | ✅ `api/models.spec.ts`             | Good.                              |
| **Admin API**        | ✅ Yes      | ✅ `api/admin.spec.ts`              | Good.                              |
| **Connections API**  | ✅ Yes      | ✅ `api/connections.spec.ts`        | Good.                              |
| **CLI Search**       | ✅ Yes      | ✅ `cli/client.spec.ts`             | Good.                              |
| **CLI Index**        | ✅ Yes      | ✅ `cli/client.spec.ts`             | Good.                              |
| **CLI Watch**        | ✅ Yes      | ⚠️ `cli/watch.spec.ts` (Failing)    | **CRITICAL: gRPC Limit Error**     |
| **MCP Server**       | ✅ Yes      | ❌ None                             | **Missing MCP E2E test**           |
| **Query Expansion**  | ✅ Yes      | ❌ None (Explicit)                  | **Missing expansion cases**        |
| **Query Log Export** | ✅ Yes      | ❌ None                             | **Missing Observability API test** |
| **IR Evaluation**    | ✅ Yes      | ❌ None                             | **Missing Evaluation API test**    |
| **Answer Mode**      | ✅ Yes      | ❌ None                             | **Missing `-a` flag test**         |
| **Verbose Stats**    | ✅ Yes      | ❌ None                             | **Missing `-v` flag test**         |
| **Real-time SSE**    | ✅ Yes      | ✅ `web/admin_ui.spec.ts` (Implied) | Verified manually.                 |

## Action Plan (Phase 5)

1.  **Create `tests/api/observability.spec.ts`**
    - Test `GET /v1/observability/export` (JSONL/CSV).
    - Test `POST /v1/evaluation/evaluate`.
    - Test `POST /v1/evaluation/judgments`.
2.  **Update `tests/cli/client.spec.ts`**
    - Test `search "query" -a` (Check for citations).
    - Test `search "query" -v` (Check for stats).
3.  **Update `tests/api/search.spec.ts`**
    - Test abbreviation expansion (`cfg` -> `config`).
    - Test case splitting (`getUser` -> `get user`).
4.  **Investigate MCP Testing**
    - Can we test Unix Socket from Playwright? (Node `net.createConnection`).
