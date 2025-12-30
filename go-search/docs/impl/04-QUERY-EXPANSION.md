# Implementation Plan: Extended Query Expansion

**Priority:** 🟡 P2 (Medium)  
**Effort:** Low (1 day)  
**Dependencies:** None

---

## Overview

Extend the existing query understanding system with 134 abbreviations, case splitting, and query type detection. This integrates with the current `query.Service` which uses model-based understanding (when enabled) or heuristic fallback.

> **Note:** While the abbreviations are optimized for code terminology (auth→authentication, cfg→config), the expansion system improves recall for all document types. Natural language queries benefit from synonym expansion regardless of content type.

## Current Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     query.Service                            │
│                                                              │
│   Parse(query) ─────────────────────────────────────────┐   │
│         │                                               │   │
│         ▼                                               │   │
│   ┌─────────────────────┐     ┌─────────────────────┐  │   │
│   │ ModelBasedUnderstand│ OR  │  KeywordExtractor   │  │   │
│   │ (LM + heuristics)   │     │  (pure heuristics)  │  │   │
│   └──────────┬──────────┘     └──────────┬──────────┘  │   │
│              │                            │             │   │
│              └────────────┬───────────────┘             │   │
│                           ▼                             │   │
│                  expandWithSynonyms()  ◄── EXTEND HERE  │   │
│                  (uses CodeTerms map)                   │   │
│                           │                             │   │
│                           ▼                             │   │
│                    ParsedQuery                          │   │
│                    - Keywords                           │   │
│                    - Expanded ◄── enhanced expansion    │   │
│                    - SearchQuery                        │   │
└─────────────────────────────────────────────────────────────┘
```

## Goals

1. **134 abbreviations** - auth→authentication, btn→button, cfg→config, etc. (optimized for code but useful for technical docs)
2. **Case splitting** - camelCase/snake_case/kebab-case splitting
3. **Query type detection** - code/natural/mixed (influences expansion strategy)
4. **Weighted terms** - Original terms > synonyms > abbreviations (for sparse boosting)

## Implementation Steps

### Step 1: Add Abbreviations to code_terms.go

**File:** `internal/query/code_terms.go`
```go
// Add after existing CodeTerms map

// CodeAbbreviations maps common abbreviations to their full forms.
// These are uni-directional: abbreviation → expansions.
var CodeAbbreviations = map[string][]string{
    // Authentication & Security
    "auth":   {"authentication", "authorization"},
    "authn":  {"authentication"},
    "authz":  {"authorization"},
    "cred":   {"credential", "credentials"},
    "creds":  {"credentials"},
    "pwd":    {"password"},
    "passwd": {"password"},
    "perm":   {"permission", "permissions"},
    "perms":  {"permissions"},
    "sec":    {"security", "secure"},
    
    // Configuration
    "cfg":    {"config", "configuration"},
    "conf":   {"config", "configuration"},
    "config": {"configuration"},
    "env":    {"environment"},
    "opt":    {"option", "options"},
    "opts":   {"options"},
    "param":  {"parameter"},
    "params": {"parameters"},
    "prop":   {"property", "properties"},
    "props":  {"properties"},
    
    // UI & Components
    "btn":    {"button"},
    "nav":    {"navigation", "navbar"},
    "hdr":    {"header"},
    "ftr":    {"footer"},
    "dlg":    {"dialog"},
    "comp":   {"component"},
    "comps":  {"components"},
    "el":     {"element"},
    "elem":   {"element"},
    "tpl":    {"template"},
    "tmpl":   {"template"},
    "img":    {"image"},
    
    // Data & Database
    "db":     {"database"},
    "repo":   {"repository"},
    "repos":  {"repositories"},
    "tbl":    {"table"},
    "col":    {"column"},
    "cols":   {"columns"},
    "idx":    {"index"},
    "rec":    {"record"},
    "recs":   {"records"},
    "doc":    {"document"},
    "docs":   {"documents", "documentation"},
    
    // Functions & Methods
    "fn":     {"function"},
    "func":   {"function"},
    "funcs":  {"functions"},
    "meth":   {"method"},
    "proc":   {"procedure", "process"},
    "cb":     {"callback"},
    "hdlr":   {"handler"},
    "impl":   {"implementation", "implement"},
    "init":   {"initialize", "initialization"},
    "ctor":   {"constructor"},
    
    // Types & Structures
    "str":    {"string"},
    "int":    {"integer"},
    "num":    {"number"},
    "bool":   {"boolean"},
    "arr":    {"array"},
    "obj":    {"object"},
    "dict":   {"dictionary"},
    "ptr":    {"pointer"},
    "ref":    {"reference"},
    "iface":  {"interface"},
    
    // Operations
    "req":    {"request"},
    "reqs":   {"requests"},
    "res":    {"response", "result"},
    "resp":   {"response"},
    "ret":    {"return"},
    "args":   {"arguments"},
    "arg":    {"argument"},
    "msg":    {"message"},
    "msgs":   {"messages"},
    "err":    {"error"},
    "errs":   {"errors"},
    "exc":    {"exception"},
    "warn":   {"warning"},
    
    // Files & Paths
    "dir":    {"directory"},
    "dirs":   {"directories"},
    "src":    {"source"},
    "dest":   {"destination"},
    "dst":    {"destination"},
    "tmp":    {"temporary", "temp"},
    "temp":   {"temporary"},
    
    // Network & API
    "api":    {"endpoint"},
    "url":    {"link"},
    "ws":     {"websocket"},
    "rpc":    {"remote procedure call"},
    "srv":    {"server", "service"},
    "svc":    {"service"},
    "svcs":   {"services"},
    "cli":    {"client", "command line"},
    
    // Async & Concurrency
    "async":  {"asynchronous"},
    "sync":   {"synchronous", "synchronize"},
    "chan":   {"channel"},
    "ctx":    {"context"},
    "mut":    {"mutex", "mutable"},
    "wg":     {"waitgroup"},
    
    // Misc
    "util":   {"utility"},
    "utils":  {"utilities"},
    "lib":    {"library"},
    "libs":   {"libraries"},
    "pkg":    {"package"},
    "pkgs":   {"packages"},
    "mod":    {"module", "modifier"},
    "ext":    {"extension", "external"},
    "info":   {"information"},
    "max":    {"maximum"},
    "min":    {"minimum"},
    "avg":    {"average"},
    "cnt":    {"count"},
    "len":    {"length"},
    "sz":     {"size"},
    "pos":    {"position"},
    "loc":    {"location"},
    "prev":   {"previous"},
    "cur":    {"current"},
    "curr":   {"current"},
    "nxt":    {"next"},
}

// GetAbbreviationExpansions returns expansions for an abbreviation.
func GetAbbreviationExpansions(term string) []string {
    lower := strings.ToLower(term)
    if expansions, ok := CodeAbbreviations[lower]; ok {
        return expansions
    }
    return nil
}
```

### Step 2: Add Case Splitting

**File:** `internal/query/case_split.go`
```go
package query

import (
    "regexp"
    "strings"
    "unicode"
)

var (
    camelCaseRe = regexp.MustCompile(`([a-z])([A-Z])`)
    snakeCaseRe = regexp.MustCompile(`_+`)
    kebabCaseRe = regexp.MustCompile(`-+`)
)

// SplitCases splits a term by camelCase, snake_case, and kebab-case.
// Returns unique parts including the original term.
func SplitCases(term string) []string {
    if len(term) < 3 {
        return []string{strings.ToLower(term)}
    }
    
    parts := make(map[string]struct{})
    
    // Add original (lowercased)
    parts[strings.ToLower(term)] = struct{}{}
    
    // camelCase: getUserName -> get User Name -> get, user, name
    if hasCamelCase(term) {
        camelSplit := camelCaseRe.ReplaceAllString(term, "${1} ${2}")
        for _, p := range strings.Fields(camelSplit) {
            if len(p) > 1 {
                parts[strings.ToLower(p)] = struct{}{}
            }
        }
    }
    
    // snake_case: get_user_name -> get, user, name
    if strings.Contains(term, "_") {
        for _, p := range snakeCaseRe.Split(term, -1) {
            if len(p) > 1 {
                parts[strings.ToLower(p)] = struct{}{}
            }
        }
    }
    
    // kebab-case: get-user-name -> get, user, name
    if strings.Contains(term, "-") {
        for _, p := range kebabCaseRe.Split(term, -1) {
            if len(p) > 1 {
                parts[strings.ToLower(p)] = struct{}{}
            }
        }
    }
    
    result := make([]string, 0, len(parts))
    for p := range parts {
        result = append(result, p)
    }
    
    return result
}

func hasCamelCase(s string) bool {
    for i := 1; i < len(s); i++ {
        if unicode.IsLower(rune(s[i-1])) && unicode.IsUpper(rune(s[i])) {
            return true
        }
    }
    return false
}

// QueryType represents the detected query style.
type QueryType string

const (
    QueryTypeCode    QueryType = "code"
    QueryTypeNatural QueryType = "natural"
    QueryTypeMixed   QueryType = "mixed"
)

// DetectQueryType determines if query is code-like, natural language, or mixed.
func DetectQueryType(query string) QueryType {
    codeSignals := 0
    naturalSignals := 0
    
    // Code signals
    if strings.Contains(query, "_") || strings.Contains(query, ".") {
        codeSignals++
    }
    if hasCamelCase(query) {
        codeSignals++
    }
    if hasCodeKeyword(query) {
        codeSignals++
    }
    
    // Natural language signals
    words := strings.Fields(query)
    if len(words) > 3 {
        naturalSignals++
    }
    if hasQuestionWord(query) {
        naturalSignals++
    }
    if hasArticle(query) {
        naturalSignals++
    }
    
    if codeSignals > naturalSignals {
        return QueryTypeCode
    }
    if naturalSignals > codeSignals {
        return QueryTypeNatural
    }
    return QueryTypeMixed
}

func hasCodeKeyword(s string) bool {
    keywords := []string{"function", "class", "method", "variable", "error", "handler", "struct", "interface"}
    lower := strings.ToLower(s)
    for _, kw := range keywords {
        if strings.Contains(lower, kw) {
            return true
        }
    }
    return false
}

func hasQuestionWord(s string) bool {
    words := []string{"how", "what", "where", "why", "when", "which", "who"}
    lower := strings.ToLower(s)
    for _, w := range words {
        if strings.HasPrefix(lower, w+" ") {
            return true
        }
    }
    return false
}

func hasArticle(s string) bool {
    lower := strings.ToLower(s)
    return strings.Contains(lower, " the ") || 
           strings.Contains(lower, " a ") || 
           strings.Contains(lower, " an ")
}
```

### Step 3: Update expandWithSynonyms in keyword_extractor.go

**File:** `internal/query/keyword_extractor.go`

Replace `expandWithSynonyms` function:

```go
// expandWithSynonyms expands keywords with synonyms, abbreviations, and case splits.
func expandWithSynonyms(keywords, codeTerms []string) []string {
    expanded := make([]string, 0)
    seen := make(map[string]bool)
    
    // Add original keywords first (highest priority)
    for _, kw := range keywords {
        if !seen[kw] {
            expanded = append(expanded, kw)
            seen[kw] = true
        }
    }
    
    // Add case-split parts for compound terms
    for _, kw := range keywords {
        for _, part := range SplitCases(kw) {
            if !seen[part] && part != kw {
                expanded = append(expanded, part)
                seen[part] = true
            }
        }
    }
    
    // Add synonyms for code terms
    for _, term := range codeTerms {
        synonyms := GetSynonyms(term)
        for _, syn := range synonyms {
            if !seen[syn] {
                expanded = append(expanded, syn)
                seen[syn] = true
            }
        }
    }
    
    // Add abbreviation expansions
    for _, kw := range keywords {
        expansions := GetAbbreviationExpansions(kw)
        for _, exp := range expansions {
            if !seen[exp] {
                expanded = append(expanded, exp)
                seen[exp] = true
            }
        }
    }
    
    return expanded
}
```

### Step 4: Add QueryType to ParsedQuery

**File:** `internal/query/types.go`

Add to `ParsedQuery` struct:

```go
type ParsedQuery struct {
    // ... existing fields ...
    
    // QueryType indicates if query is code-like, natural, or mixed.
    QueryType QueryType `json:"query_type"`
}
```

### Step 5: Update KeywordExtractor.Parse

**File:** `internal/query/keyword_extractor.go`

Add query type detection:

```go
func (e *KeywordExtractor) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
    // ... existing code ...
    
    // Detect query type
    queryType := DetectQueryType(query)
    
    result := &ParsedQuery{
        // ... existing fields ...
        QueryType:    queryType,
    }
    
    // ... rest of function ...
}
```

### Step 6: Update ModelBasedUnderstanding.Parse

**File:** `internal/query/model_understanding.go`

Add query type to model-based path:

```go
func (m *ModelBasedUnderstanding) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
    // ... existing embedding + intent code ...
    
    // Detect query type
    queryType := DetectQueryType(query)
    
    result := &ParsedQuery{
        // ... existing fields ...
        QueryType:    queryType,
    }
    
    // ... rest of function ...
}
```

## Integration Summary

| Component | Change |
|-----------|--------|
| `code_terms.go` | Add `CodeAbbreviations` map (134 entries) + `GetAbbreviationExpansions()` |
| `case_split.go` | New file: `SplitCases()`, `DetectQueryType()` |
| `types.go` | Add `QueryType` field to `ParsedQuery` |
| `keyword_extractor.go` | Update `expandWithSynonyms()` to use all expansion sources |
| `model_understanding.go` | Add `QueryType` detection |

## Expansion Priority Order

When building the `Expanded` field:
1. **Original keywords** (weight: 1.0) - exact user terms
2. **Case-split parts** (weight: 0.8) - getUserName → get, user, name
3. **Synonyms** (weight: 0.7) - function → method, procedure
4. **Abbreviation expansions** (weight: 0.5) - auth → authentication

This priority can be used by sparse search to boost original terms.

## Testing

```go
func TestExpandWithSynonyms_Abbreviations(t *testing.T) {
    keywords := []string{"auth", "middleware"}
    codeTerms := []string{}
    
    expanded := expandWithSynonyms(keywords, codeTerms)
    
    assert.Contains(t, expanded, "auth")           // original
    assert.Contains(t, expanded, "authentication") // abbreviation expansion
    assert.Contains(t, expanded, "authorization")  // abbreviation expansion
    assert.Contains(t, expanded, "middleware")     // original
}

func TestExpandWithSynonyms_CaseSplit(t *testing.T) {
    keywords := []string{"getUserName"}
    codeTerms := []string{}
    
    expanded := expandWithSynonyms(keywords, codeTerms)
    
    assert.Contains(t, expanded, "getusername") // original lowercased
    assert.Contains(t, expanded, "get")         // case split
    assert.Contains(t, expanded, "user")        // case split
    assert.Contains(t, expanded, "name")        // case split
}

func TestDetectQueryType(t *testing.T) {
    cases := []struct {
        query    string
        expected QueryType
    }{
        {"getUserName", QueryTypeCode},
        {"get_user_name", QueryTypeCode},
        {"how does authentication work", QueryTypeNatural},
        {"auth handler", QueryTypeMixed},
    }
    
    for _, tc := range cases {
        result := DetectQueryType(tc.query)
        assert.Equal(t, tc.expected, result, "query: %s", tc.query)
    }
}
```

## Success Metrics

- [ ] 134 abbreviations added to `CodeAbbreviations`
- [ ] Case splitting extracts parts from camelCase/snake_case/kebab-case
- [ ] Query type detection (code/natural/mixed) works
- [ ] All expansions appear in `ParsedQuery.Expanded`
- [ ] Both model-based and fallback paths include new expansions
- [ ] Existing tests pass
- [ ] Benchmark: <1ms for typical query

## References

- Current implementation: `internal/query/keyword_extractor.go`, `internal/query/code_terms.go`
- Old NestJS implementation: `api/src/sparse/query-expansion.service.ts`
