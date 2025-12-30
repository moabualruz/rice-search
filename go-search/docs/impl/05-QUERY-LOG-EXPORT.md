# Implementation Plan: Query Log Export

**Priority:** 🟡 P2 (Medium)  
**Effort:** Low (0.5 days)  
**Dependencies:** None

---

## Overview

Add query log export functionality to allow exporting search queries as JSONL or CSV for analysis and replay.

## Goals

1. **JSONL export** - One query per line, machine-readable
2. **CSV export** - Spreadsheet-friendly format
3. **Date range filtering** - Export queries from specific period
4. **Store filtering** - Export queries for specific store

## API Endpoints

```
GET /v1/observability/export?format=jsonl&store=default&from=2025-01-01&to=2025-01-31
GET /v1/observability/export?format=csv&store=default&days=7
```

## Implementation

### Step 1: Add Export Handler

**File:** `internal/search/handlers_observability.go`
```go
package search

import (
    "encoding/csv"
    "encoding/json"
    "fmt"
    "net/http"
    "strconv"
    "time"
)

func (h *Handlers) handleObservabilityExport(w http.ResponseWriter, r *http.Request) {
    format := r.URL.Query().Get("format")
    if format == "" {
        format = "jsonl"
    }
    
    store := r.URL.Query().Get("store")
    
    // Parse date range
    var from, to time.Time
    if fromStr := r.URL.Query().Get("from"); fromStr != "" {
        from, _ = time.Parse("2006-01-02", fromStr)
    }
    if toStr := r.URL.Query().Get("to"); toStr != "" {
        to, _ = time.Parse("2006-01-02", toStr)
    }
    if daysStr := r.URL.Query().Get("days"); daysStr != "" {
        days, _ := strconv.Atoi(daysStr)
        to = time.Now()
        from = to.AddDate(0, 0, -days)
    }
    
    // Default: last 7 days
    if from.IsZero() {
        to = time.Now()
        from = to.AddDate(0, 0, -7)
    }
    
    // Get queries
    queries, err := h.observability.GetQueriesInRange(r.Context(), store, from, to)
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }
    
    // Set headers based on format
    filename := fmt.Sprintf("queries_%s_%s.%s", 
        from.Format("20060102"), 
        to.Format("20060102"),
        format)
    
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
    
    switch format {
    case "jsonl":
        h.exportJSONL(w, queries)
    case "csv":
        h.exportCSV(w, queries)
    default:
        writeError(w, http.StatusBadRequest, "Invalid format. Use 'jsonl' or 'csv'")
    }
}

func (h *Handlers) exportJSONL(w http.ResponseWriter, queries []QueryLogEntry) {
    w.Header().Set("Content-Type", "application/x-ndjson")
    
    encoder := json.NewEncoder(w)
    for _, q := range queries {
        encoder.Encode(q)
    }
}

func (h *Handlers) exportCSV(w http.ResponseWriter, queries []QueryLogEntry) {
    w.Header().Set("Content-Type", "text/csv")
    
    writer := csv.NewWriter(w)
    defer writer.Flush()
    
    // Header
    writer.Write([]string{
        "timestamp", "store", "query", "intent", "strategy", 
        "difficulty", "confidence", "results", "latency_ms",
        "rerank_enabled", "rerank_latency_ms",
    })
    
    // Data
    for _, q := range queries {
        writer.Write([]string{
            q.Timestamp.Format(time.RFC3339),
            q.Store,
            q.Query,
            q.Intent,
            q.Strategy,
            q.Difficulty,
            fmt.Sprintf("%.2f", q.Confidence),
            strconv.Itoa(q.ResultCount),
            strconv.Itoa(int(q.LatencyMs)),
            strconv.FormatBool(q.RerankEnabled),
            strconv.Itoa(int(q.RerankLatencyMs)),
        })
    }
}
```

### Step 2: Add Query Range Method

**File:** `internal/observability/service.go`
```go
package observability

import (
    "context"
    "time"
)

// GetQueriesInRange returns queries within a date range
func (s *Service) GetQueriesInRange(ctx context.Context, store string, from, to time.Time) ([]QueryLogEntry, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var results []QueryLogEntry
    
    for _, entry := range s.queryLog {
        // Filter by store
        if store != "" && entry.Store != store {
            continue
        }
        
        // Filter by date range
        if entry.Timestamp.Before(from) || entry.Timestamp.After(to) {
            continue
        }
        
        results = append(results, entry)
    }
    
    return results, nil
}
```

### Step 3: Register Route

**File:** `cmd/rice-search-server/main.go`
```go
// Add to route registration
mux.HandleFunc("GET /v1/observability/export", handlers.handleObservabilityExport)
```

## Usage Examples

```bash
# Export last 7 days as JSONL
curl "http://localhost:8080/v1/observability/export?format=jsonl&days=7" > queries.jsonl

# Export specific date range as CSV
curl "http://localhost:8080/v1/observability/export?format=csv&from=2025-01-01&to=2025-01-31" > queries.csv

# Export specific store
curl "http://localhost:8080/v1/observability/export?format=jsonl&store=myproject&days=30" > myproject_queries.jsonl
```

## JSONL Format

```json
{"timestamp":"2025-01-15T10:30:00Z","store":"default","query":"authentication handler","intent":"navigational","strategy":"balanced","difficulty":"easy","confidence":0.85,"results":15,"latency_ms":45,"rerank_enabled":true,"rerank_latency_ms":12}
{"timestamp":"2025-01-15T10:31:00Z","store":"default","query":"how does error handling work","intent":"exploratory","strategy":"dense-heavy","difficulty":"medium","confidence":0.72,"results":20,"latency_ms":78,"rerank_enabled":true,"rerank_latency_ms":25}
```

## CSV Format

```csv
timestamp,store,query,intent,strategy,difficulty,confidence,results,latency_ms,rerank_enabled,rerank_latency_ms
2025-01-15T10:30:00Z,default,authentication handler,navigational,balanced,easy,0.85,15,45,true,12
2025-01-15T10:31:00Z,default,how does error handling work,exploratory,dense-heavy,medium,0.72,20,78,true,25
```

## Success Metrics

- [ ] `/v1/observability/export` endpoint works
- [ ] JSONL format produces valid NDJSON
- [ ] CSV format opens correctly in Excel/Sheets
- [ ] Date range filtering works
- [ ] Store filtering works
- [ ] Content-Disposition header triggers download

## References

- Old implementation: `api/src/observability/query-log.service.ts`
