//go:build cgo

package ast

import (
	"context"
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
	// "github.com/smacker/go-tree-sitter/rust" // Rust grammar might not be in standard smacker repo or named differently
)

type treeSitterParser struct {
	parsers map[string]*sitter.Parser
	mu      sync.Mutex
}

func NewParser() Parser {
	return &treeSitterParser{
		parsers: make(map[string]*sitter.Parser),
	}
}

func (p *treeSitterParser) getParser(language string) *sitter.Parser {
	p.mu.Lock()
	defer p.mu.Unlock()

	if parser, ok := p.parsers[language]; ok {
		return parser
	}

	var lang *sitter.Language
	switch language {
	case LangGo:
		lang = golang.GetLanguage()
	case LangPython:
		lang = python.GetLanguage()
	case LangTypeScript:
		lang = typescript.GetLanguage()
	case LangJava:
		lang = java.GetLanguage()
	case LangRust:
		lang = rust.GetLanguage()
	default:
		return nil
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	p.parsers[language] = parser
	return parser
}

func (p *treeSitterParser) Parse(ctx context.Context, content []byte, language string) (*Node, error) {
	parser := p.getParser(language)
	if parser == nil {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	tree, _ := parser.ParseCtx(ctx, nil, content)
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return nil, fmt.Errorf("failed to parse content")
	}

	return convertNode(root), nil
}

func convertNode(n *sitter.Node) *Node {
	node := &Node{
		Type:      n.Type(),
		StartByte: int(n.StartByte()),
		EndByte:   int(n.EndByte()),
		StartLine: int(n.StartPoint().Row),
		EndLine:   int(n.EndPoint().Row),
	}

	count := int(n.ChildCount())
	for i := 0; i < count; i++ {
		child := n.Child(i)
		if child != nil {
			node.Children = append(node.Children, convertNode(child))
		}
	}
	return node
}

func (p *treeSitterParser) Chunk(ctx context.Context, content []byte, language string, maxLines int) ([]Chunk, error) {
	node, err := p.Parse(ctx, content, language)
	if err != nil {
		return nil, err
	}

	var chunks []Chunk
	var traverse func(*Node)

	// Simple traversal that treats top-level functions/classes as chunks
	traverse = func(n *Node) {
		// Identify chunkable nodes based on language (simplified for now)
		isChunkable := false
		switch n.Type {
		// Go
		case "function_declaration", "method_declaration", "type_declaration":
			isChunkable = true
		// Python
		case "function_definition", "class_definition":
			isChunkable = true
		// TypeScript/Java
		case "class_declaration", "interface_declaration", "method_definition":
			isChunkable = true
		}

		if isChunkable {
			// Extract content
			chunkContent := string(content[n.StartByte:n.EndByte])
			chunks = append(chunks, Chunk{
				Content:   chunkContent,
				StartLine: n.StartLine + 1, // 0-indexed to 1-indexed
				EndLine:   n.EndLine + 1,
				NodeType:  n.Type,
				Language:  language,
				// TODO: extracting symbols would go here
			})
			// Don't traverse deeper into a chunk (for now)
			return
		}

		// Continue traversal
		for _, child := range n.Children {
			traverse(child)
		}
	}

	traverse(node)

	// If no chunks found (e.g. script file without functions), return whole file?
	// Or let the caller fallback? returning empty list lets caller fallback.
	if len(chunks) == 0 {
		return nil, nil
	}

	return chunks, nil
}

func (p *treeSitterParser) ExtractSymbols(ctx context.Context, content []byte, language string) ([]string, error) {
	// TODO: Implement query-based symbol extraction
	return []string{}, nil
}

func (p *treeSitterParser) SupportsLanguage(language string) bool {
	parser := p.getParser(language)
	return parser != nil
}
