# Verification TODO - Code & Doc Fixes

**Created**: 2025-12-29
**Status**: ✅ ALL COMPLETED

## Code Fixes (High Priority)

### 1. ✅ Register ML Event Handlers
- **File**: `cmd/rice-search-server/main.go`
- **Fix**: Added `ml.NewEventHandler().Register(ctx)` after bus creation

### 2. ✅ HTTP Metrics Middleware
- **File**: `internal/metrics/http.go` (new)
- **Metrics**:
  - `rice_http_requests_total{method, path, status}`
  - `rice_http_request_duration_seconds{method, path}`
  - `rice_http_requests_in_flight`
  - `rice_http_request_size_bytes{method, path}`

### 3. ✅ Event Bus Metrics
- **File**: `internal/bus/instrumented.go` (new)
- **Metrics**:
  - `rice_bus_events_published_total{topic}`
  - `rice_bus_event_latency_seconds{topic}`
  - `rice_bus_errors_total{topic}`

### 4. ✅ ML Cache Metrics
- **File**: `internal/ml/cache.go`
- **Metrics**:
  - `rice_ml_cache_hits_total{type}`
  - `rice_ml_cache_misses_total{type}`
  - `rice_ml_cache_size{type}`

### 5. ✅ Search Stage Duration Metrics
- **File**: `internal/search/service.go`
- **Metric**: `rice_search_stage_duration_ms{store, stage}`
- **Stages**: sparse, dense, fusion, rerank

### 6. ✅ ML HTTP Endpoint Wrappers
- **File**: `cmd/rice-search-server/main.go`
- **Endpoints**:
  - `POST /v1/ml/embed`
  - `POST /v1/ml/sparse`
  - `POST /v1/ml/rerank`

### 7. ✅ Response Wrapper Middleware
- **File**: `internal/server/response.go` (new)
- **Format**: `{data: {...}, meta: {request_id, latency_ms, timestamp}}`

### 8. ✅ Convenience Search Endpoint
- **File**: `cmd/rice-search-server/main.go`
- **Endpoint**: `POST /v1/search` (uses default store)

### 9. ✅ Version Endpoint Fields
- **Files**: `cmd/rice-search-server/main.go`, `internal/search/health.go`
- **Fields**: `git_commit`, `build_time`, `go_version`

## Documentation Fixes (Medium Priority)

### 10. ✅ Update 09-config.md
- Fixed all env var names to use `RICE_` prefix
- Documented connection tracking, model registry, event logging

### 11. ✅ Update 04-search.md
- Changed intent types to actual: `find/explain/list/fix/compare`

### 12. ✅ Update 07-api.md
- Documented event-driven ML architecture
- Added missing endpoints (reindex, stats, files)

### 13. ✅ Update 03-data-models.md
- Updated to match actual flattened structures
- Added ConnectionID fields

### 14. ✅ Update 06-ml.md
- Marked GPU load modes as "Planned/Future"

### 15. ✅ Update 13-observability.md
- Added HTTP metrics documentation
- Added event bus metrics documentation
- Added ML cache metrics documentation
- Added search stage metrics documentation

## Verification

### 16. ✅ Build & Vet
```bash
cd go-search && go build ./... && go vet ./...
# SUCCESS: Build and vet passed!
```

---

## Progress Tracking

| Task | Assignee | Status |
|------|----------|--------|
| 1. ML Handler Registration | Main | ✅ |
| 2. HTTP Metrics | Agent | ✅ |
| 3. Event Bus Metrics | Agent | ✅ |
| 4. ML Cache Metrics | Agent | ✅ |
| 5. Search Stage Metrics | Agent | ✅ |
| 6. ML HTTP Endpoints | Main | ✅ |
| 7. Response Wrapper | Main | ✅ |
| 8. Convenience Endpoint | Main | ✅ |
| 9. Version Fields | Main | ✅ |
| 10-15. Doc Updates | Agent | ✅ |
| 16. Build Verification | Main | ✅ |

---

## Summary

**Completed**: 2025-12-29
**Total Tasks**: 16/16 ✅
**Build Status**: PASSING
**Vet Status**: PASSING
