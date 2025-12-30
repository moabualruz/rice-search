# Implementation Plan: Tree-sitter AST Parsing

**Priority:** 🔴 P1 (Critical)  
**Effort:** High (3-5 days)  
**Dependencies:** None

---

## Overview

Replace regex-based symbol extraction and chunking with proper AST parsing using Tree-sitter. The current regex approach becomes the fallback for unknown languages or WASM failures.

> **Note:** Tree-sitter is optimized for programming languages. Non-code files (documentation, configs, logs) continue to work with the regex/generic fallback, which provides good chunking for any text.

## Goals

1. **Accurate chunk boundaries** - Split code at function/class/method boundaries
2. **Precise symbol extraction** - Extract symbols directly from AST nodes
3. **35+ language support** - Match the old API's language coverage
4. **Graceful fallback** - Use regex when Tree-sitter fails or language unknown (also handles non-code files like markdown, logs, configs)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     index.Service                            │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     ast.Parser                               │
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │  Tree-sitter    │    │     Fallback (Regex)            │ │
│  │  (35+ langs)    │    │     (12 langs + generic)        │ │
│  │                 │    │                                 │ │
│  │  WASM parsers   │───▶│  Current symbols.go             │ │
│  │  loaded lazily  │fail│  Current chunker.go             │ │
│  └─────────────────┘    └─────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Package Structure

```
internal/
├── ast/
│   ├── parser.go          # Main parser interface
│   ├── treesitter.go      # Tree-sitter implementation
│   ├── fallback.go        # Regex fallback (move from index/)
│   ├── languages.go       # Language configs & node types
│   ├── wasm/              # WASM parser files (embedded)
│   │   ├── tree-sitter-go.wasm
│   │   ├── tree-sitter-typescript.wasm
│   │   └── ... (35+ files, ~50MB total)
│   └── wasm_embed.go      # go:embed for WASM files
├── index/
│   ├── chunker.go         # Uses ast.Parser now
│   └── symbols.go         # Delegates to ast.Parser
```

## Implementation Steps

### Step 1: Add Tree-sitter Go Bindings

**File:** `go.mod`
```go
require (
    github.com/smacker/go-tree-sitter v0.0.0-20231219031718-2c55c15b1a71
)
```

**Note:** `go-tree-sitter` uses CGo. For pure Go, consider `github.com/tree-sitter/go-tree-sitter` (newer, official).

### Step 2: Define AST Parser Interface

**File:** `internal/ast/parser.go`
```go
package ast

import "context"

// Node represents a parsed AST node
type Node struct {
    Type       string   // function_declaration, class_declaration, etc.
    Name       string   // Symbol name if applicable
    StartLine  int
    EndLine    int
    StartByte  int
    EndByte    int
    Children   []*Node
}

// Chunk represents a code chunk with AST-aware boundaries
type Chunk struct {
    Content    string
    StartLine  int
    EndLine    int
    Symbols    []string
    NodeType   string   // Primary node type (function, class, etc.)
    Language   string
}

// Parser extracts AST and chunks from source code
type Parser interface {
    // Parse returns the AST root node
    Parse(ctx context.Context, content []byte, language string) (*Node, error)
    
    // Chunk splits code into AST-aware chunks
    Chunk(ctx context.Context, content []byte, language string, maxLines int) ([]Chunk, error)
    
    // ExtractSymbols returns all symbol names
    ExtractSymbols(ctx context.Context, content []byte, language string) ([]string, error)
    
    // SupportsLanguage returns true if Tree-sitter parser available
    SupportsLanguage(language string) bool
}

// NewParser creates a parser with Tree-sitter primary and regex fallback
func NewParser() Parser {
    return &combinedParser{
        treesitter: newTreeSitterParser(),
        fallback:   newFallbackParser(),
    }
}
```

### Step 3: Implement Tree-sitter Parser

**File:** `internal/ast/treesitter.go`
```go
package ast

import (
    "context"
    "embed"
    "fmt"
    "sync"
    
    sitter "github.com/smacker/go-tree-sitter"
)

//go:embed wasm/*.wasm
var wasmFiles embed.FS

type treeSitterParser struct {
    mu       sync.RWMutex
    parsers  map[string]*sitter.Parser
    langs    map[string]*sitter.Language
}

func newTreeSitterParser() *treeSitterParser {
    return &treeSitterParser{
        parsers: make(map[string]*sitter.Parser),
        langs:   make(map[string]*sitter.Language),
    }
}

// getParser lazily loads parser for language
func (p *treeSitterParser) getParser(language string) (*sitter.Parser, error) {
    p.mu.RLock()
    if parser, ok := p.parsers[language]; ok {
        p.mu.RUnlock()
        return parser, nil
    }
    p.mu.RUnlock()
    
    p.mu.Lock()
    defer p.mu.Unlock()
    
    // Double-check after acquiring write lock
    if parser, ok := p.parsers[language]; ok {
        return parser, nil
    }
    
    // Load WASM and create parser
    lang, err := p.loadLanguage(language)
    if err != nil {
        return nil, err
    }
    
    parser := sitter.NewParser()
    parser.SetLanguage(lang)
    p.parsers[language] = parser
    p.langs[language] = lang
    
    return parser, nil
}

func (p *treeSitterParser) loadLanguage(language string) (*sitter.Language, error) {
    wasmName := languageToWASM[language]
    if wasmName == "" {
        return nil, fmt.Errorf("unsupported language: %s", language)
    }
    
    wasmBytes, err := wasmFiles.ReadFile("wasm/" + wasmName + ".wasm")
    if err != nil {
        return nil, fmt.Errorf("failed to load WASM for %s: %w", language, err)
    }
    
    // Load WASM into Tree-sitter
    lang, err := sitter.LoadLanguage(wasmBytes)
    if err != nil {
        return nil, fmt.Errorf("failed to parse WASM for %s: %w", language, err)
    }
    
    return lang, nil
}

func (p *treeSitterParser) Parse(ctx context.Context, content []byte, language string) (*Node, error) {
    parser, err := p.getParser(language)
    if err != nil {
        return nil, err
    }
    
    tree := parser.Parse(nil, content)
    defer tree.Close()
    
    return convertNode(tree.RootNode(), content), nil
}
```

### Step 4: Define Language Configurations

**File:** `internal/ast/languages.go`
```go
package ast

// languageToWASM maps our language names to WASM file names
var languageToWASM = map[string]string{
    // Web / Frontend
    "javascript": "tree-sitter-javascript",
    "typescript": "tree-sitter-typescript",
    "tsx":        "tree-sitter-tsx",
    "jsx":        "tree-sitter-javascript",
    "html":       "tree-sitter-html",
    "css":        "tree-sitter-css",
    "vue":        "tree-sitter-vue",
    
    // Systems / Backend
    "go":         "tree-sitter-go",
    "rust":       "tree-sitter-rust",
    "python":     "tree-sitter-python",
    "java":       "tree-sitter-java",
    "kotlin":     "tree-sitter-kotlin",
    "scala":      "tree-sitter-scala",
    "c":          "tree-sitter-c",
    "cpp":        "tree-sitter-cpp",
    "csharp":     "tree-sitter-c_sharp",
    "swift":      "tree-sitter-swift",
    "dart":       "tree-sitter-dart",
    
    // Scripting
    "ruby":       "tree-sitter-ruby",
    "php":        "tree-sitter-php",
    "lua":        "tree-sitter-lua",
    "elixir":     "tree-sitter-elixir",
    "perl":       "tree-sitter-perl",
    
    // Shell
    "bash":       "tree-sitter-bash",
    "shell":      "tree-sitter-bash",
    
    // Data
    "json":       "tree-sitter-json",
    "yaml":       "tree-sitter-yaml",
    "toml":       "tree-sitter-toml",
    
    // Other
    "zig":        "tree-sitter-zig",
    "solidity":   "tree-sitter-solidity",
    "ocaml":      "tree-sitter-ocaml",
    "elm":        "tree-sitter-elm",
    "haskell":    "tree-sitter-haskell",
}

// chunkBoundaryNodes defines which AST node types create chunk boundaries
var chunkBoundaryNodes = map[string][]string{
    "go": {
        "function_declaration",
        "method_declaration", 
        "type_declaration",
        "type_spec",
    },
    "typescript": {
        "function_declaration",
        "class_declaration",
        "method_definition",
        "interface_declaration",
        "type_alias_declaration",
        "enum_declaration",
    },
    "python": {
        "function_definition",
        "class_definition",
        "decorated_definition",
        "async_function_definition",
    },
    "rust": {
        "function_item",
        "impl_item",
        "struct_item",
        "enum_item",
        "trait_item",
        "mod_item",
    },
    // ... (copy all from old treesitter-chunker.service.ts)
}

// symbolNodes defines which AST node types contain symbol names
var symbolNodes = map[string][]string{
    "go": {
        "function_declaration",
        "method_declaration",
        "type_spec",
        "const_spec",
        "var_spec",
    },
    "typescript": {
        "function_declaration",
        "class_declaration",
        "interface_declaration",
        "type_alias_declaration",
        "variable_declarator",
    },
    // ...
}
```

### Step 5: Implement Combined Parser with Fallback

**File:** `internal/ast/combined.go`
```go
package ast

import (
    "context"
    "log/slog"
)

type combinedParser struct {
    treesitter *treeSitterParser
    fallback   *fallbackParser
}

func (p *combinedParser) Chunk(ctx context.Context, content []byte, language string, maxLines int) ([]Chunk, error) {
    // Try Tree-sitter first
    if p.treesitter.SupportsLanguage(language) {
        chunks, err := p.treesitter.Chunk(ctx, content, language, maxLines)
        if err == nil {
            return chunks, nil
        }
        slog.Warn("Tree-sitter chunking failed, using fallback",
            "language", language,
            "error", err,
        )
    }
    
    // Fallback to regex
    return p.fallback.Chunk(ctx, content, language, maxLines)
}

func (p *combinedParser) ExtractSymbols(ctx context.Context, content []byte, language string) ([]string, error) {
    // Try Tree-sitter first
    if p.treesitter.SupportsLanguage(language) {
        symbols, err := p.treesitter.ExtractSymbols(ctx, content, language)
        if err == nil {
            return symbols, nil
        }
        slog.Warn("Tree-sitter symbol extraction failed, using fallback",
            "language", language,
            "error", err,
        )
    }
    
    // Fallback to regex
    return p.fallback.ExtractSymbols(ctx, content, language)
}

func (p *combinedParser) SupportsLanguage(language string) bool {
    return p.treesitter.SupportsLanguage(language) || p.fallback.SupportsLanguage(language)
}
```

### Step 6: Move Regex Code to Fallback

**File:** `internal/ast/fallback.go`
```go
package ast

// Move current symbols.go and chunker.go logic here
// This becomes the fallback when Tree-sitter is unavailable

type fallbackParser struct{}

func newFallbackParser() *fallbackParser {
    return &fallbackParser{}
}

func (p *fallbackParser) SupportsLanguage(language string) bool {
    _, ok := symbolPatterns[language]
    return ok
}

// ... (copy from current index/symbols.go)
```

### Step 7: Update Index Service

**File:** `internal/index/service.go`
```go
package index

import (
    "github.com/ricesearch/go-search/internal/ast"
)

type Service struct {
    parser ast.Parser  // Add AST parser
    // ...
}

func NewService(/* ... */) *Service {
    return &Service{
        parser: ast.NewParser(),
        // ...
    }
}

func (s *Service) processFile(ctx context.Context, file File) ([]Chunk, error) {
    // Use AST parser for chunking
    astChunks, err := s.parser.Chunk(ctx, []byte(file.Content), file.Language, s.cfg.MaxChunkLines)
    if err != nil {
        return nil, err
    }
    
    // Convert to index chunks
    chunks := make([]Chunk, len(astChunks))
    for i, ac := range astChunks {
        chunks[i] = Chunk{
            Content:   ac.Content,
            StartLine: ac.StartLine,
            EndLine:   ac.EndLine,
            Symbols:   ac.Symbols,
            Language:  ac.Language,
            // ...
        }
    }
    
    return chunks, nil
}
```

### Step 8: Download WASM Files

**File:** `scripts/download-wasm.sh`
```bash
#!/bin/bash
# Download Tree-sitter WASM parsers

CDN_URL="https://cdn.jsdelivr.net/npm/tree-sitter-wasms@0.1.13/out"
OUT_DIR="internal/ast/wasm"

LANGUAGES=(
    "tree-sitter-javascript"
    "tree-sitter-typescript"
    "tree-sitter-tsx"
    "tree-sitter-python"
    "tree-sitter-go"
    "tree-sitter-rust"
    "tree-sitter-java"
    "tree-sitter-c"
    "tree-sitter-cpp"
    "tree-sitter-c_sharp"
    "tree-sitter-ruby"
    "tree-sitter-php"
    "tree-sitter-bash"
    "tree-sitter-json"
    "tree-sitter-yaml"
    # ... add all 35+
)

mkdir -p "$OUT_DIR"

for lang in "${LANGUAGES[@]}"; do
    echo "Downloading $lang.wasm..."
    curl -sL "$CDN_URL/$lang.wasm" -o "$OUT_DIR/$lang.wasm"
done

echo "Downloaded ${#LANGUAGES[@]} WASM parsers"
```

## Testing Strategy

1. **Unit tests** for each language's chunk boundaries
2. **Comparison tests** - Compare Tree-sitter output vs regex fallback
3. **Benchmark tests** - Ensure acceptable performance
4. **Integration tests** - Full indexing pipeline with AST

```go
func TestTreeSitterChunking_Go(t *testing.T) {
    parser := ast.NewParser()
    content := []byte(`
package main

func Hello() {
    fmt.Println("hello")
}

func World() {
    fmt.Println("world")
}
`)
    chunks, err := parser.Chunk(context.Background(), content, "go", 50)
    require.NoError(t, err)
    require.Len(t, chunks, 2)
    assert.Equal(t, "function_declaration", chunks[0].NodeType)
    assert.Contains(t, chunks[0].Symbols, "Hello")
}
```

## Performance Considerations

1. **Lazy loading** - Only load WASM for languages actually used
2. **Parser pooling** - Reuse parsed trees where possible
3. **Memory limits** - Large files may need chunked parsing
4. **Embed size** - ~50MB for all WASM files (acceptable for single binary)

## Rollout Plan

1. **Phase 1:** Implement core parser with 10 key languages (Go, TS, Python, Rust, Java, etc.)
2. **Phase 2:** Add remaining 25 languages
3. **Phase 3:** Optimize performance, add caching
4. **Phase 4:** Remove regex patterns for languages with Tree-sitter support

## Success Metrics

- [ ] 35+ languages with Tree-sitter support
- [ ] Chunk boundaries match function/class definitions
- [ ] Symbol extraction accuracy >95% (vs manual review)
- [ ] Performance: <100ms for 10KB file
- [ ] Graceful fallback for unsupported languages
- [ ] All existing tests pass

## References

- Old implementation: `api/src/services/treesitter-chunker.service.ts`
- Tree-sitter Go bindings: https://github.com/smacker/go-tree-sitter
- WASM parsers: https://www.npmjs.com/package/tree-sitter-wasms
