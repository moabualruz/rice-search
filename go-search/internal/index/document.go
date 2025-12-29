// Package index provides document processing and indexing for Rice Search.
package index

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Document represents a source file to be indexed.
type Document struct {
	Path     string   `json:"path"`
	Content  string   `json:"content"`
	Language string   `json:"language"`
	Symbols  []string `json:"symbols"`
	Hash     string   `json:"hash"`
	Size     int64    `json:"size"`
}

// Chunk represents a searchable unit extracted from a document.
type Chunk struct {
	ID         string    `json:"id"`
	DocumentID string    `json:"document_id"`
	Store      string    `json:"store"`
	Path       string    `json:"path"`
	Language   string    `json:"language"`
	Content    string    `json:"content"`
	Symbols    []string  `json:"symbols"`
	StartLine  int       `json:"start_line"`
	EndLine    int       `json:"end_line"`
	StartChar  int       `json:"start_char"`
	EndChar    int       `json:"end_char"`
	TokenCount int       `json:"token_count"`
	Hash       string    `json:"hash"`
	IndexedAt  time.Time `json:"indexed_at"`
}

// NewDocument creates a new document from path and content.
func NewDocument(path, content string) *Document {
	doc := &Document{
		Path:    path,
		Content: content,
		Size:    int64(len(content)),
	}

	doc.Language = DetectLanguage(path)
	doc.Hash = ComputeHash(content)

	return doc
}

// ComputeHash computes SHA256 hash of content.
func ComputeHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

// ComputeChunkID generates a deterministic chunk ID.
func ComputeChunkID(store, path string, startLine, endLine int) string {
	input := fmt.Sprintf("%s:%s:%d:%d", store, path, startLine, endLine)
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)[:16]
}

// ComputeChunkHash computes hash for chunk deduplication.
func ComputeChunkHash(store, path, content string, startLine, endLine int) string {
	input := fmt.Sprintf("%s:%s:%d:%d:%s", store, path, startLine, endLine, content)
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}

// Language mapping from extension
var languageExtensions = map[string]string{
	".go":      "go",
	".ts":      "typescript",
	".tsx":     "typescript",
	".js":      "javascript",
	".jsx":     "javascript",
	".py":      "python",
	".rs":      "rust",
	".java":    "java",
	".c":       "c",
	".h":       "c",
	".cpp":     "cpp",
	".cc":      "cpp",
	".hpp":     "cpp",
	".rb":      "ruby",
	".php":     "php",
	".swift":   "swift",
	".kt":      "kotlin",
	".scala":   "scala",
	".cs":      "csharp",
	".md":      "markdown",
	".json":    "json",
	".yaml":    "yaml",
	".yml":     "yaml",
	".toml":    "toml",
	".sql":     "sql",
	".sh":      "bash",
	".bash":    "bash",
	".zsh":     "bash",
	".html":    "html",
	".htm":     "html",
	".css":     "css",
	".scss":    "scss",
	".less":    "less",
	".vue":     "vue",
	".svelte":  "svelte",
	".lua":     "lua",
	".r":       "r",
	".R":       "r",
	".pl":      "perl",
	".pm":      "perl",
	".ex":      "elixir",
	".exs":     "elixir",
	".erl":     "erlang",
	".hs":      "haskell",
	".clj":     "clojure",
	".ml":      "ocaml",
	".mli":     "ocaml",
	".nim":     "nim",
	".zig":     "zig",
	".proto":   "protobuf",
	".graphql": "graphql",
	".gql":     "graphql",
}

// DetectLanguage detects programming language from file extension.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if lang, ok := languageExtensions[ext]; ok {
		return lang
	}

	// Check for special files
	base := strings.ToLower(filepath.Base(path))
	switch {
	case base == "dockerfile", strings.HasPrefix(base, "dockerfile."):
		return "dockerfile"
	case base == "makefile", base == "gnumakefile":
		return "makefile"
	case base == ".gitignore", base == ".dockerignore":
		return "gitignore"
	case strings.HasSuffix(base, "rc") && !strings.Contains(base, "."):
		return "shell"
	}

	return "unknown"
}

// IsTextFile checks if a file should be indexed based on language.
func IsTextFile(language string) bool {
	return language != "unknown"
}

// Content limits
const (
	MaxDocumentSize    = 10 * 1024 * 1024 // 10MB
	MaxChunkTokens     = 8192
	MaxQueryTokens     = 4096
	MaxPathLength      = 1024
	MaxSymbolsPerChunk = 100
)

// ValidateDocument validates a document for indexing.
func ValidateDocument(doc *Document) error {
	if doc.Path == "" {
		return fmt.Errorf("document path cannot be empty")
	}

	if len(doc.Path) > MaxPathLength {
		return fmt.Errorf("path exceeds maximum length of %d", MaxPathLength)
	}

	if doc.Size > MaxDocumentSize {
		return fmt.Errorf("document size %d exceeds maximum of %d bytes", doc.Size, MaxDocumentSize)
	}

	return nil
}
