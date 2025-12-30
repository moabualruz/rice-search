# Implementation Plan: Answer Mode (RAG)

**Priority:** 🟢 P3 (Low)  
**Effort:** Low (0.5 days)  
**Dependencies:** None

---

## Overview

Add `-a, --answer` flag to CLI search that formats results in a RAG-friendly way with citation markers that AI assistants can reference.

## Goals

1. **Answer flag** - `-a, --answer` enables RAG output format
2. **Citation format** - `<cite i="N"/>` markers for AI to reference
3. **Context window friendly** - Concise format optimized for LLM context

## CLI Interface

```bash
# Normal search (existing)
rice-search search "authentication handler"

# Answer mode
rice-search search "authentication handler" -a
rice-search search "how does auth work" --answer
```

## Output Format

**Normal mode:**
```
./src/auth/handler.go:10-25 (score: 0.85)
./src/auth/middleware.go:5-20 (score: 0.78)
...
```

**Answer mode:**
```
Based on the indexed files, here are the relevant sections:

[1] src/auth/handler.go:10-25
```go
func AuthHandler(w http.ResponseWriter, r *http.Request) {
    token := r.Header.Get("Authorization")
    // ...
}
```

[2] src/auth/middleware.go:5-20
```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ...
    })
}
```

---
Sources: <cite i="1"/> <cite i="2"/>
```

## Implementation

### Step 1: Add Flag

**File:** `cmd/rice-search/search.go`
```go
var (
    searchAnswer bool
)

func init() {
    searchCmd.Flags().BoolVarP(&searchAnswer, "answer", "a", false, "Output in RAG-friendly format with citations")
}
```

### Step 2: Add Formatter

**File:** `cmd/rice-search/format_answer.go`
```go
package main

import (
    "fmt"
    "strings"
    
    "github.com/ricesearch/go-search/internal/search"
)

func formatAnswerResponse(results *search.Response) string {
    var sb strings.Builder
    
    sb.WriteString("Based on the indexed files, here are the relevant sections:\n\n")
    
    for i, r := range results.Results {
        // Citation header
        sb.WriteString(fmt.Sprintf("[%d] %s:%d-%d\n", i+1, r.Path, r.StartLine, r.EndLine))
        
        // Code block
        sb.WriteString(fmt.Sprintf("```%s\n", r.Language))
        sb.WriteString(r.Content)
        if !strings.HasSuffix(r.Content, "\n") {
            sb.WriteString("\n")
        }
        sb.WriteString("```\n\n")
    }
    
    // Citation markers for AI
    sb.WriteString("---\nSources: ")
    for i := range results.Results {
        if i > 0 {
            sb.WriteString(" ")
        }
        sb.WriteString(fmt.Sprintf("<cite i=\"%d\"/>", i+1))
    }
    sb.WriteString("\n")
    
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
    if searchAnswer {
        fmt.Print(formatAnswerResponse(results))
    } else if searchJSON {
        // ... existing JSON format ...
    } else {
        // ... existing text format ...
    }
    
    return nil
}
```

## Example Usage with AI

User query to AI: "How does authentication work?"

AI uses rice-search:
```bash
rice-search search "authentication implementation" -a -k 5
```

AI receives formatted response and can say:
> The authentication is handled in the AuthHandler function <cite i="1"/> which extracts the token from the Authorization header. The AuthMiddleware <cite i="2"/> wraps handlers to enforce authentication...

This works equally well for non-code content:
```bash
rice-search search "deployment process" -a -k 5 -s docs
```

## Success Metrics

- [ ] `-a, --answer` flag works
- [ ] Output includes numbered citations
- [ ] Code blocks have language syntax hints
- [ ] Citation markers at bottom
- [ ] Works with AI assistants

## References

- Old implementation: `ricegrep/src/commands/search.ts` (formatAskResponse)
