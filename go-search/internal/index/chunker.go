package index

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ricesearch/rice-search/internal/ast"
)

// ChunkerConfig holds configuration for the chunker.
type ChunkerConfig struct {
	// TargetSize is the target chunk size in tokens (approximate).
	TargetSize int

	// Overlap is the number of tokens to overlap between chunks.
	Overlap int

	// MinSize is the minimum chunk size.
	MinSize int

	// MaxSize is the maximum chunk size.
	MaxSize int
}

// DefaultChunkerConfig returns sensible defaults.
func DefaultChunkerConfig() ChunkerConfig {
	return ChunkerConfig{
		TargetSize: 512,
		Overlap:    64,
		MinSize:    32,
		MaxSize:    2048,
	}
}

// Chunker splits documents into searchable chunks.
type Chunker struct {
	config ChunkerConfig
	parser ast.Parser
}

// NewChunker creates a new chunker with the given configuration.
func NewChunker(cfg ChunkerConfig) *Chunker {
	if cfg.TargetSize == 0 {
		cfg = DefaultChunkerConfig()
	}
	return &Chunker{
		config: cfg,
		parser: ast.NewParser(),
	}
}

// ChunkDocument splits a document into chunks.
func (c *Chunker) ChunkDocument(store string, doc *Document) []*Chunk {
	// Try AST chunking first
	if c.parser.SupportsLanguage(doc.Language) {
		chunks, err := c.parser.Chunk(context.Background(), []byte(doc.Content), doc.Language, c.config.TargetSize)
		if err == nil && len(chunks) > 0 {
			// Convert AST chunks to index chunks
			var result []*Chunk
			for _, ac := range chunks {
				result = append(result, &Chunk{
					ID:           ComputeChunkID(store, doc.Path, ac.StartLine, ac.EndLine),
					DocumentID:   doc.Hash,
					Store:        store,
					Path:         doc.Path,
					Language:     doc.Language,
					Content:      ac.Content,
					Symbols:      ac.Symbols,
					StartLine:    ac.StartLine,
					EndLine:      ac.EndLine,
					StartChar:    0,               // TODO: map properly if easy
					EndChar:      len(ac.Content), // Approximate
					TokenCount:   c.estimateTokens(ac.Content),
					Hash:         ComputeChunkHash(store, doc.Path, ac.Content, ac.StartLine, ac.EndLine),
					IndexedAt:    time.Now(),
					ConnectionID: doc.ConnectionID,
				})
			}
			return result
		}
	}

	lines := strings.Split(doc.Content, "\n")

	// For small files, return as single chunk
	if c.estimateTokens(doc.Content) <= c.config.TargetSize {
		return c.createSingleChunk(store, doc, lines)
	}

	// Use language-specific chunking
	switch doc.Language {
	case "go", "rust", "java", "csharp", "kotlin", "scala", "swift":
		return c.chunkByBraces(store, doc, lines)
	case "python", "yaml":
		return c.chunkByIndentation(store, doc, lines)
	case "markdown":
		return c.chunkByHeadings(store, doc, lines)
	default:
		return c.chunkByLines(store, doc, lines)
	}
}

// createSingleChunk creates a single chunk from the entire document.
func (c *Chunker) createSingleChunk(store string, doc *Document, lines []string) []*Chunk {
	chunk := &Chunk{
		ID:           ComputeChunkID(store, doc.Path, 1, len(lines)),
		DocumentID:   doc.Hash,
		Store:        store,
		Path:         doc.Path,
		Language:     doc.Language,
		Content:      doc.Content,
		Symbols:      ExtractSymbols(doc.Content, doc.Language),
		StartLine:    1,
		EndLine:      len(lines),
		StartChar:    0,
		EndChar:      len(doc.Content),
		TokenCount:   c.estimateTokens(doc.Content),
		Hash:         ComputeChunkHash(store, doc.Path, doc.Content, 1, len(lines)),
		IndexedAt:    time.Now(),
		ConnectionID: doc.ConnectionID,
	}

	return []*Chunk{chunk}
}

// chunkByBraces chunks by finding function/class boundaries using braces.
func (c *Chunker) chunkByBraces(store string, doc *Document, lines []string) []*Chunk {
	var chunks []*Chunk
	var currentChunk strings.Builder
	var currentSymbols []string
	var previousOverlap string
	startLine := 1
	braceDepth := 0
	charOffset := 0

	for i, line := range lines {
		lineNum := i + 1
		currentChunk.WriteString(line)
		if i < len(lines)-1 {
			currentChunk.WriteString("\n")
		}

		// Track brace depth
		braceDepth += strings.Count(line, "{") - strings.Count(line, "}")

		// Check if we're at a natural boundary and have enough content
		tokens := c.estimateTokens(currentChunk.String())
		atBoundary := braceDepth == 0 && strings.TrimSpace(line) == "}"
		isLastLine := lineNum == len(lines)
		exceedsMax := tokens >= c.config.MaxSize

		if (atBoundary && tokens >= c.config.MinSize) || exceedsMax || isLastLine {
			content := currentChunk.String()
			if strings.TrimSpace(content) != "" {
				// Add overlap from previous chunk
				finalContent := previousOverlap + content

				// Extract symbols from this chunk
				chunkSymbols := ExtractSymbols(finalContent, doc.Language)
				if len(chunkSymbols) > MaxSymbolsPerChunk {
					chunkSymbols = chunkSymbols[:MaxSymbolsPerChunk]
				}
				currentSymbols = append(currentSymbols, chunkSymbols...)

				endChar := charOffset + len(content)
				chunk := &Chunk{
					ID:           ComputeChunkID(store, doc.Path, startLine, lineNum),
					DocumentID:   doc.Hash,
					Store:        store,
					Path:         doc.Path,
					Language:     doc.Language,
					Content:      finalContent,
					Symbols:      currentSymbols,
					StartLine:    startLine,
					EndLine:      lineNum,
					StartChar:    charOffset,
					EndChar:      endChar,
					TokenCount:   c.estimateTokens(finalContent),
					Hash:         ComputeChunkHash(store, doc.Path, finalContent, startLine, lineNum),
					IndexedAt:    time.Now(),
					ConnectionID: doc.ConnectionID,
				}
				chunks = append(chunks, chunk)

				// Extract overlap for next chunk
				previousOverlap = c.extractOverlap(content)

				// Reset for next chunk
				charOffset = endChar
				startLine = lineNum + 1
				currentChunk.Reset()
				currentSymbols = nil
			}
		}
	}

	if len(chunks) == 0 {
		return c.createSingleChunk(store, doc, lines)
	}

	return chunks
}

// chunkByIndentation chunks by finding blocks at indentation level 0.
func (c *Chunker) chunkByIndentation(store string, doc *Document, lines []string) []*Chunk {
	var chunks []*Chunk
	var currentChunk strings.Builder
	var previousOverlap string
	startLine := 1
	charOffset := 0

	for i, line := range lines {
		lineNum := i + 1
		currentChunk.WriteString(line)
		if i < len(lines)-1 {
			currentChunk.WriteString("\n")
		}

		// Check if next line starts at indentation 0 (new top-level block)
		atBoundary := false
		if i+1 < len(lines) {
			nextLine := lines[i+1]
			trimmed := strings.TrimSpace(nextLine)
			if len(trimmed) > 0 && (nextLine[0] != ' ' && nextLine[0] != '\t') {
				atBoundary = true
			}
		}

		tokens := c.estimateTokens(currentChunk.String())
		isLastLine := lineNum == len(lines)
		exceedsMax := tokens >= c.config.MaxSize

		if (atBoundary && tokens >= c.config.MinSize) || exceedsMax || isLastLine {
			content := currentChunk.String()
			if strings.TrimSpace(content) != "" {
				// Add overlap from previous chunk
				finalContent := previousOverlap + content

				symbols := ExtractSymbols(finalContent, doc.Language)
				if len(symbols) > MaxSymbolsPerChunk {
					symbols = symbols[:MaxSymbolsPerChunk]
				}

				endChar := charOffset + len(content)
				chunk := &Chunk{
					ID:           ComputeChunkID(store, doc.Path, startLine, lineNum),
					DocumentID:   doc.Hash,
					Store:        store,
					Path:         doc.Path,
					Language:     doc.Language,
					Content:      finalContent,
					Symbols:      symbols,
					StartLine:    startLine,
					EndLine:      lineNum,
					StartChar:    charOffset,
					EndChar:      endChar,
					TokenCount:   c.estimateTokens(finalContent),
					Hash:         ComputeChunkHash(store, doc.Path, finalContent, startLine, lineNum),
					IndexedAt:    time.Now(),
					ConnectionID: doc.ConnectionID,
				}
				chunks = append(chunks, chunk)

				// Extract overlap for next chunk
				previousOverlap = c.extractOverlap(content)

				charOffset = endChar
				startLine = lineNum + 1
				currentChunk.Reset()
			}
		}
	}

	if len(chunks) == 0 {
		return c.createSingleChunk(store, doc, lines)
	}

	return chunks
}

// chunkByHeadings chunks markdown by headings.
func (c *Chunker) chunkByHeadings(store string, doc *Document, lines []string) []*Chunk {
	var chunks []*Chunk
	var currentChunk strings.Builder
	var previousOverlap string
	startLine := 1
	charOffset := 0

	for i, line := range lines {
		lineNum := i + 1

		// Check if this is a heading line (starts with ##)
		isHeading := strings.HasPrefix(strings.TrimSpace(line), "##")
		tokens := c.estimateTokens(currentChunk.String())

		if isHeading && tokens >= c.config.MinSize {
			content := currentChunk.String()
			if strings.TrimSpace(content) != "" {
				// Add overlap from previous chunk
				finalContent := previousOverlap + content

				endChar := charOffset + len(content)
				chunk := &Chunk{
					ID:           ComputeChunkID(store, doc.Path, startLine, lineNum-1),
					DocumentID:   doc.Hash,
					Store:        store,
					Path:         doc.Path,
					Language:     doc.Language,
					Content:      finalContent,
					StartLine:    startLine,
					EndLine:      lineNum - 1,
					StartChar:    charOffset,
					EndChar:      endChar,
					TokenCount:   c.estimateTokens(finalContent),
					Hash:         ComputeChunkHash(store, doc.Path, finalContent, startLine, lineNum-1),
					IndexedAt:    time.Now(),
					ConnectionID: doc.ConnectionID,
				}
				chunks = append(chunks, chunk)

				// Extract overlap for next chunk
				previousOverlap = c.extractOverlap(content)

				charOffset = endChar
				startLine = lineNum
				currentChunk.Reset()
			}
		}

		currentChunk.WriteString(line)
		if i < len(lines)-1 {
			currentChunk.WriteString("\n")
		}
	}

	// Final chunk
	content := currentChunk.String()
	if strings.TrimSpace(content) != "" {
		// Add overlap from previous chunk
		finalContent := previousOverlap + content

		endChar := charOffset + len(content)
		chunk := &Chunk{
			ID:           ComputeChunkID(store, doc.Path, startLine, len(lines)),
			DocumentID:   doc.Hash,
			Store:        store,
			Path:         doc.Path,
			Language:     doc.Language,
			Content:      finalContent,
			StartLine:    startLine,
			EndLine:      len(lines),
			StartChar:    charOffset,
			EndChar:      endChar,
			TokenCount:   c.estimateTokens(finalContent),
			Hash:         ComputeChunkHash(store, doc.Path, finalContent, startLine, len(lines)),
			IndexedAt:    time.Now(),
			ConnectionID: doc.ConnectionID,
		}
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		return c.createSingleChunk(store, doc, lines)
	}

	return chunks
}

// chunkByLines is the fallback chunker that splits by line count.
func (c *Chunker) chunkByLines(store string, doc *Document, lines []string) []*Chunk {
	var chunks []*Chunk
	var currentChunk strings.Builder
	var previousOverlap string
	startLine := 1
	charOffset := 0

	for i, line := range lines {
		lineNum := i + 1
		currentChunk.WriteString(line)
		if i < len(lines)-1 {
			currentChunk.WriteString("\n")
		}

		tokens := c.estimateTokens(currentChunk.String())
		isLastLine := lineNum == len(lines)

		if tokens >= c.config.TargetSize || isLastLine {
			content := currentChunk.String()
			if strings.TrimSpace(content) != "" {
				// Add overlap from previous chunk
				finalContent := previousOverlap + content

				symbols := ExtractSymbols(finalContent, doc.Language)
				if len(symbols) > MaxSymbolsPerChunk {
					symbols = symbols[:MaxSymbolsPerChunk]
				}

				endChar := charOffset + len(content)
				chunk := &Chunk{
					ID:           ComputeChunkID(store, doc.Path, startLine, lineNum),
					DocumentID:   doc.Hash,
					Store:        store,
					Path:         doc.Path,
					Language:     doc.Language,
					Content:      finalContent,
					Symbols:      symbols,
					StartLine:    startLine,
					EndLine:      lineNum,
					StartChar:    charOffset,
					EndChar:      endChar,
					TokenCount:   c.estimateTokens(finalContent),
					Hash:         ComputeChunkHash(store, doc.Path, finalContent, startLine, lineNum),
					IndexedAt:    time.Now(),
					ConnectionID: doc.ConnectionID,
				}
				chunks = append(chunks, chunk)

				// Extract overlap for next chunk
				previousOverlap = c.extractOverlap(content)

				charOffset = endChar
				startLine = lineNum + 1
				currentChunk.Reset()
			}
		}
	}

	if len(chunks) == 0 {
		return c.createSingleChunk(store, doc, lines)
	}

	return chunks
}

// estimateTokens estimates the token count for text.
// Uses a simple heuristic: ~4 characters per token for code.
func (c *Chunker) estimateTokens(text string) int {
	runeCount := utf8.RuneCountInString(text)
	return (runeCount + 3) / 4 // Round up
}

// extractOverlap extracts the last N tokens from content for overlap with next chunk.
// Returns a string containing approximately Overlap tokens from the end of content.
func (c *Chunker) extractOverlap(content string) string {
	if c.config.Overlap == 0 || content == "" {
		return ""
	}

	// Calculate approximate character count for desired overlap
	// Using the same heuristic: 4 characters per token
	overlapChars := c.config.Overlap * 4

	// Don't extract more than half the content
	if overlapChars > len(content)/2 {
		overlapChars = len(content) / 2
	}

	// Find a good boundary (newline or space) near the overlap point
	startPos := len(content) - overlapChars
	if startPos <= 0 {
		return content
	}

	// Try to find a newline boundary
	newlinePos := strings.LastIndex(content[:startPos+overlapChars/2], "\n")
	if newlinePos > startPos-overlapChars/4 && newlinePos < len(content) {
		return content[newlinePos+1:]
	}

	// Fallback: find a space boundary
	spacePos := strings.LastIndex(content[:startPos+overlapChars/2], " ")
	if spacePos > startPos-overlapChars/4 && spacePos < len(content) {
		return content[spacePos+1:]
	}

	// If no good boundary found, use approximate position
	return content[startPos:]
}
