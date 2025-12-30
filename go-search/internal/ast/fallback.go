//go:build !cgo

package ast

import (
	"context"
	"log/slog"
)

// fallbackParser is a stub implementation that does nothing (or uses regex).
// For Phase 4, we will just return a basic "not supported" or simple pass-through.
// In a real implementation, this would contain the regex logic from `chunker.go`.
type fallbackParser struct{}

func NewParser() Parser {
	slog.Warn("Tree-Sitter not available (CGO disabled), using fallback parser")
	return &fallbackParser{}
}

func (p *fallbackParser) Parse(ctx context.Context, content []byte, language string) (*Node, error) {
	// Minimal stub
	return &Node{
		Type:      "file",
		StartByte: 0,
		EndByte:   len(content),
	}, nil
}

func (p *fallbackParser) Chunk(ctx context.Context, content []byte, language string, maxLines int) ([]Chunk, error) {
	// Return single chunk as fallback
	return []Chunk{{
		Content:   string(content),
		StartLine: 1,
		EndLine:   1, // Dummy line numbers, chunker.go will likely overwrite or handle this if we integration deeply
		Language:  language,
	}}, nil
}

func (p *fallbackParser) ExtractSymbols(ctx context.Context, content []byte, language string) ([]string, error) {
	return []string{}, nil
}

func (p *fallbackParser) SupportsLanguage(language string) bool {
	return false
}
