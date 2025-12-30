package index

import (
	"testing"
)

func TestChunkDocument_AST_Go(t *testing.T) {
	// Simple Go code with two functions
	code := `package main

import "fmt"

func Hello() {
	fmt.Println("Hello")
}

func World() {
	fmt.Println("World")
}
`

	doc := &Document{
		Path:     "main.go",
		Content:  code,
		Language: "go",
		Hash:     "hash1",
	}

	// Use default config, but ensure parser is initialized
	// Note: AST parsing requires CGO. If running without CGO, this test might assume fallback unless mocked.
	// But valid execution of "go test" usually enables CGO on dev machines.
	chunker := NewChunker(DefaultChunkerConfig())

	chunks := chunker.ChunkDocument("store1", doc)

	// We expect 2 chunks if AST works (one for Hello, one for World)
	// If AST fails, it might return 1 chunk (small size) or line-based chunks.
	// Our chunker logic for AST specifically returns chunks for discovered nodes.

	// Check results
	expectedChunks := 2
	if len(chunks) != expectedChunks {
		t.Logf("Expected %d chunks, got %d", expectedChunks, len(chunks))
		for i, c := range chunks {
			t.Logf("Chunk %d: %q", i, c.Content)
		}
		// Failure here means either AST parser failed (fallback used?) or chunking logic is different
		// If fallback used: estimateTokens for `code` is ~20 tokens. DefaultTarget is 512.
		// So fallback `createSingleChunk` would return 1 chunk.
		t.Fail()
	}

	if len(chunks) >= 1 {
		if chunks[0].Content != "func Hello() {\n\tfmt.Println(\"Hello\")\n}" {
			t.Errorf("Unexpected content for chunk 0: %q", chunks[0].Content)
		}
	}
}
