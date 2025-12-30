# Implementation Plan: Verbose CLI Stats

**Priority:** 🟢 P3 (Low)  
**Effort:** Low (0.5 days)  
**Dependencies:** None

---

## Overview

Add `-v, --verbose` flag to CLI search that displays detailed intelligence and timing statistics similar to the Web UI.

## Goals

1. **Verbose flag** - `-v, --verbose` shows detailed stats
2. **Intelligence stats** - Intent, strategy, difficulty, confidence
3. **Timing breakdown** - Sparse, dense, rerank, postrank latencies
4. **Pipeline stats** - Dedup removed, diversity score, etc.

## CLI Interface

```bash
# Normal search
rice-search search "auth handler"

# Verbose search
rice-search search "auth handler" -v
rice-search search "auth handler" --verbose
```

## Output Format

**Normal mode:**
```
./src/auth/handler.go:10-25 (0.85)
./src/auth/middleware.go:5-20 (0.78)
./src/auth/jwt.go:1-15 (0.72)

3 results in 45ms
```

**Verbose mode:**
```
./src/auth/handler.go:10-25 (0.85)
  Language: go | Symbols: AuthHandler, validateToken
  Scores: sparse=12.5 dense=0.82 | Ranks: sparse=#1 dense=#3

./src/auth/middleware.go:5-20 (0.78)
  Language: go | Symbols: AuthMiddleware
  Scores: sparse=10.2 dense=0.79 | Ranks: sparse=#2 dense=#2

./src/auth/jwt.go:1-15 (0.72)
  Language: go | Symbols: ParseJWT, ValidateToken
  Scores: sparse=8.1 dense=0.75 | Ranks: sparse=#4 dense=#1

─────────────────────────────────────────────────────────
Intelligence:
  Intent: navigational | Difficulty: easy | Strategy: balanced
  Confidence: 85%

Retrieval:
  Sparse: 15ms (weight: 0.50) | Dense: 22ms (weight: 0.50)
  Candidates: 100 → Fusion: 50

Reranking:
  Enabled: true | Pass 1: 12ms (30 candidates)
  Pass 2: skipped (early exit: high_confidence)

PostRank:
  Dedup: 5 removed (threshold: 0.85) | 3ms
  Diversity: enabled (λ=0.70, avg=72%) | 2ms
  Aggregation: 15 unique files

─────────────────────────────────────────────────────────
3 results in 45ms (sparse=15ms, dense=22ms, rerank=12ms, postrank=5ms)
```

## Implementation

### Step 1: Add Flag

**File:** `cmd/rice-search/search.go`
```go
var (
    searchVerbose bool
)

func init() {
    searchCmd.Flags().BoolVarP(&searchVerbose, "verbose", "v", false, "Show detailed timing and intelligence stats")
}
```

### Step 2: Add Verbose Formatter

**File:** `cmd/rice-search/format_verbose.go`
```go
package main

import (
    "fmt"
    "strings"
    
    "github.com/ricesearch/go-search/internal/search"
)

func formatVerboseResponse(results *search.Response) string {
    var sb strings.Builder
    
    // Results with details
    for _, r := range results.Results {
        sb.WriteString(fmt.Sprintf("%s:%d-%d (%.2f)\n", r.Path, r.StartLine, r.EndLine, r.FinalScore))
        sb.WriteString(fmt.Sprintf("  Language: %s", r.Language))
        if len(r.Symbols) > 0 {
            sb.WriteString(fmt.Sprintf(" | Symbols: %s", strings.Join(r.Symbols, ", ")))
        }
        sb.WriteString("\n")
        sb.WriteString(fmt.Sprintf("  Scores: sparse=%.1f dense=%.2f | Ranks: sparse=#%d dense=#%d\n",
            r.SparseScore, r.DenseScore, r.SparseRank, r.DenseRank))
        sb.WriteString("\n")
    }
    
    // Separator
    sb.WriteString(strings.Repeat("─", 60) + "\n")
    
    // Intelligence
    if results.Intelligence != nil {
        i := results.Intelligence
        sb.WriteString("Intelligence:\n")
        sb.WriteString(fmt.Sprintf("  Intent: %s | Difficulty: %s | Strategy: %s\n",
            i.Intent, i.Difficulty, i.Strategy))
        sb.WriteString(fmt.Sprintf("  Confidence: %.0f%%\n", i.Confidence*100))
        sb.WriteString("\n")
    }
    
    // Retrieval timing
    sb.WriteString("Retrieval:\n")
    sb.WriteString(fmt.Sprintf("  Sparse: %dms (weight: %.2f) | Dense: %dms (weight: %.2f)\n",
        results.Timing.SparseMs, results.Options.SparseWeight,
        results.Timing.DenseMs, results.Options.DenseWeight))
    sb.WriteString(fmt.Sprintf("  Candidates: %d → Fusion: %d\n",
        results.Timing.Candidates, results.Timing.FusionCount))
    sb.WriteString("\n")
    
    // Reranking
    if results.Reranking != nil {
        r := results.Reranking
        sb.WriteString("Reranking:\n")
        sb.WriteString(fmt.Sprintf("  Enabled: %t", r.Enabled))
        if r.Enabled {
            sb.WriteString(fmt.Sprintf(" | Pass 1: %dms (%d candidates)\n", r.Pass1LatencyMs, r.Candidates))
            if r.Pass2Applied {
                sb.WriteString(fmt.Sprintf("  Pass 2: %dms\n", r.Pass2LatencyMs))
            } else if r.EarlyExit {
                sb.WriteString(fmt.Sprintf("  Pass 2: skipped (early exit: %s)\n", r.EarlyExitReason))
            }
        } else {
            sb.WriteString("\n")
        }
        sb.WriteString("\n")
    }
    
    // PostRank
    if results.PostRank != nil {
        p := results.PostRank
        sb.WriteString("PostRank:\n")
        if p.Dedup != nil {
            sb.WriteString(fmt.Sprintf("  Dedup: %d removed (threshold: %.2f) | %dms\n",
                p.Dedup.Removed, p.Dedup.Threshold, p.Dedup.LatencyMs))
        }
        if p.Diversity != nil {
            sb.WriteString(fmt.Sprintf("  Diversity: enabled (λ=%.2f, avg=%.0f%%) | %dms\n",
                p.Diversity.Lambda, p.Diversity.AvgDiversity*100, p.Diversity.LatencyMs))
        }
        if p.Aggregation != nil {
            sb.WriteString(fmt.Sprintf("  Aggregation: %d unique files\n", p.Aggregation.UniqueFiles))
        }
        sb.WriteString("\n")
    }
    
    // Separator and summary
    sb.WriteString(strings.Repeat("─", 60) + "\n")
    sb.WriteString(fmt.Sprintf("%d results in %dms (sparse=%dms, dense=%dms, rerank=%dms, postrank=%dms)\n",
        len(results.Results),
        results.Timing.TotalMs,
        results.Timing.SparseMs,
        results.Timing.DenseMs,
        results.Timing.RerankMs,
        results.Timing.PostRankMs))
    
    return sb.String()
}
```

### Step 3: Update Search Command

**File:** `cmd/rice-search/search.go`
```go
func runSearch(cmd *cobra.Command, args []string) error {
    query := args[0]
    
    // ... existing search logic ...
    
    results, err := client.Search(ctx, req)
    if err != nil {
        return err
    }
    
    // Format output
    if searchVerbose {
        fmt.Print(formatVerboseResponse(results))
    } else if searchAnswer {
        fmt.Print(formatAnswerResponse(results))
    } else if searchJSON {
        // ... existing JSON format ...
    } else {
        // ... existing text format ...
    }
    
    return nil
}
```

### Step 4: Ensure Response Has All Fields

The API response must include timing and stats fields:

```go
type Response struct {
    Results      []Result           `json:"results"`
    Intelligence *IntelligenceInfo  `json:"intelligence,omitempty"`
    Reranking    *RerankingInfo     `json:"reranking,omitempty"`
    PostRank     *PostRankInfo      `json:"postrank,omitempty"`
    Timing       *TimingInfo        `json:"timing,omitempty"`
    Options      *OptionsInfo       `json:"options,omitempty"`
}

type TimingInfo struct {
    TotalMs     int64 `json:"total_ms"`
    SparseMs    int64 `json:"sparse_ms"`
    DenseMs     int64 `json:"dense_ms"`
    RerankMs    int64 `json:"rerank_ms"`
    PostRankMs  int64 `json:"postrank_ms"`
    Candidates  int   `json:"candidates"`
    FusionCount int   `json:"fusion_count"`
}
```

## Success Metrics

- [ ] `-v, --verbose` flag works
- [ ] Shows intelligence (intent, strategy, difficulty)
- [ ] Shows timing breakdown (sparse, dense, rerank, postrank)
- [ ] Shows per-result details (scores, ranks, symbols)
- [ ] Shows dedup/diversity stats
- [ ] Clean formatting with separators

## References

- Old implementation: `ricegrep/src/commands/search.ts` (formatIntelligenceStats)
- Web UI: `internal/web/templates/search.templ` (stats section)
