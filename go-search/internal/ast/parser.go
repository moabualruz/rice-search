package ast

import "context"

// Node represents a parsed AST node
type Node struct {
	Type      string // function_declaration, class_declaration, etc.
	Name      string // Symbol name if applicable
	StartLine int
	EndLine   int
	StartByte int
	EndByte   int
	Children  []*Node
}

// Chunk represents a code chunk with AST-aware boundaries
type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
	Symbols   []string
	NodeType  string // Primary node type (function, class, etc.)
	Language  string
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
