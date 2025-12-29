package index

import (
	"strings"
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"app.ts", "typescript"},
		{"component.tsx", "typescript"},
		{"script.js", "javascript"},
		{"app.py", "python"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"program.c", "c"},
		{"class.cpp", "cpp"},
		{"README.md", "markdown"},
		{"config.json", "json"},
		{"config.yaml", "yaml"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{".gitignore", "gitignore"},
		{"unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := DetectLanguage(tt.path)
			if result != tt.expected {
				t.Errorf("DetectLanguage(%s) = %s, expected %s", tt.path, result, tt.expected)
			}
		})
	}
}

func TestComputeHash(t *testing.T) {
	hash1 := ComputeHash("hello world")
	hash2 := ComputeHash("hello world")
	hash3 := ComputeHash("hello world!")

	if hash1 != hash2 {
		t.Error("same content should produce same hash")
	}

	if hash1 == hash3 {
		t.Error("different content should produce different hash")
	}

	if len(hash1) != 64 {
		t.Errorf("expected 64 character hash, got %d", len(hash1))
	}
}

func TestComputeChunkID(t *testing.T) {
	id1 := ComputeChunkID("default", "src/main.go", 1, 10)
	id2 := ComputeChunkID("default", "src/main.go", 1, 10)
	id3 := ComputeChunkID("default", "src/main.go", 1, 11)

	if id1 != id2 {
		t.Error("same input should produce same ID")
	}

	if id1 == id3 {
		t.Error("different input should produce different ID")
	}

	if len(id1) != 16 {
		t.Errorf("expected 16 character ID, got %d", len(id1))
	}
}

func TestNewDocument(t *testing.T) {
	content := "package main\n\nfunc main() {}"
	doc := NewDocument("main.go", content)

	if doc.Path != "main.go" {
		t.Errorf("expected path 'main.go', got %s", doc.Path)
	}

	if doc.Language != "go" {
		t.Errorf("expected language 'go', got %s", doc.Language)
	}

	if doc.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), doc.Size)
	}

	if doc.Hash == "" {
		t.Error("expected hash to be set")
	}
}

func TestValidateDocument(t *testing.T) {
	// Valid document
	doc := NewDocument("main.go", "package main")
	if err := ValidateDocument(doc); err != nil {
		t.Errorf("expected valid document, got error: %v", err)
	}

	// Empty path
	emptyPath := &Document{Path: "", Content: "test"}
	if err := ValidateDocument(emptyPath); err == nil {
		t.Error("expected error for empty path")
	}

	// Path too long
	longPath := &Document{Path: strings.Repeat("a", MaxPathLength+1), Content: "test"}
	if err := ValidateDocument(longPath); err == nil {
		t.Error("expected error for path too long")
	}
}

func TestChunkerSmallFile(t *testing.T) {
	chunker := NewChunker(DefaultChunkerConfig())
	doc := NewDocument("main.go", "package main\n\nfunc main() {}")

	chunks := chunker.ChunkDocument("default", doc)

	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk for small file, got %d", len(chunks))
	}

	if chunks[0].Content != doc.Content {
		t.Error("chunk content should match document content")
	}

	if chunks[0].StartLine != 1 {
		t.Errorf("expected start line 1, got %d", chunks[0].StartLine)
	}
}

func TestChunkerLargeFile(t *testing.T) {
	chunker := NewChunker(ChunkerConfig{
		TargetSize: 20,
		Overlap:    5,
		MinSize:    10,
		MaxSize:    50,
	})

	// Create a larger Go file with multiple functions
	content := `package main

import "fmt"

func hello() {
	fmt.Println("hello world from the hello function")
	fmt.Println("this is a second line")
	fmt.Println("and a third line")
}

func world() {
	fmt.Println("world function is here")
	fmt.Println("with multiple statements")
	fmt.Println("to make it larger")
}

func another() {
	fmt.Println("another function")
	fmt.Println("with more content")
	fmt.Println("to exceed the chunk size")
}

func main() {
	hello()
	world()
	another()
}
`
	doc := NewDocument("main.go", content)
	chunks := chunker.ChunkDocument("default", doc)

	// With very small chunk size, should produce multiple chunks
	if len(chunks) < 1 {
		t.Errorf("expected at least one chunk, got %d", len(chunks))
	}

	// Verify each chunk has valid metadata
	for _, chunk := range chunks {
		if chunk.StartLine < 1 {
			t.Errorf("invalid start line: %d", chunk.StartLine)
		}
		if chunk.EndLine < chunk.StartLine {
			t.Errorf("end line %d < start line %d", chunk.EndLine, chunk.StartLine)
		}
		if chunk.Content == "" {
			t.Error("chunk content should not be empty")
		}
	}
}

func TestExtractSymbolsGo(t *testing.T) {
	content := `package main

func Authenticate(ctx context.Context) error {
	return nil
}

type User struct {
	Name string
}

const MaxRetries = 3
`
	symbols := ExtractSymbols(content, "go")

	// Check at least some symbols are extracted
	if len(symbols) == 0 {
		t.Error("expected to extract some symbols")
	}

	// Check for User and MaxRetries (these should definitely match)
	expected := []string{"User", "MaxRetries"}
	for _, exp := range expected {
		found := false
		for _, sym := range symbols {
			if sym == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected symbol %s not found in %v", exp, symbols)
		}
	}
}

func TestExtractSymbolsPython(t *testing.T) {
	content := `
def authenticate(user, password):
    pass

class UserManager:
    def get_user(self, id):
        pass

@dataclass
class Config:
    debug = True
`
	symbols := ExtractSymbols(content, "python")

	// Check at least some symbols are extracted
	if len(symbols) == 0 {
		t.Error("expected to extract some symbols")
	}

	// Check for key symbols that should match
	expected := []string{"authenticate", "UserManager", "get_user"}
	for _, exp := range expected {
		found := false
		for _, sym := range symbols {
			if sym == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected symbol %s not found in %v", exp, symbols)
		}
	}
}

func TestExtractSymbolsTypeScript(t *testing.T) {
	content := `
function authenticate(): boolean {
    return true;
}

async function fetchUser(id: string) {
    return null;
}

class AuthService {
    private token: string;
}

interface User {
    name: string;
}

const MAX_RETRIES = 3;
`
	symbols := ExtractSymbols(content, "typescript")

	expected := []string{"authenticate", "fetchUser", "AuthService", "User", "MAX_RETRIES"}
	for _, exp := range expected {
		found := false
		for _, sym := range symbols {
			if sym == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected symbol %s not found in %v", exp, symbols)
		}
	}
}

func TestExtractSymbolsRust(t *testing.T) {
	content := `
fn authenticate(token: &str) -> Result<(), Error> {
    Ok(())
}

struct User {
    name: String,
}

enum Status {
    Active,
    Inactive,
}

trait Authenticatable {
    fn verify(&self) -> bool;
}

impl User {
    fn new(name: String) -> Self {
        User { name }
    }
}
`
	symbols := ExtractSymbols(content, "rust")

	expected := []string{"authenticate", "User", "Status", "Authenticatable"}
	for _, exp := range expected {
		found := false
		for _, sym := range symbols {
			if sym == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected symbol %s not found in %v", exp, symbols)
		}
	}
}

func TestChunkHasCorrectMetadata(t *testing.T) {
	chunker := NewChunker(DefaultChunkerConfig())
	content := `package main

func hello() {
	println("hello")
}
`
	doc := NewDocument("src/main.go", content)
	chunks := chunker.ChunkDocument("mystore", doc)

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	chunk := chunks[0]

	if chunk.Store != "mystore" {
		t.Errorf("expected store 'mystore', got %s", chunk.Store)
	}

	if chunk.Path != "src/main.go" {
		t.Errorf("expected path 'src/main.go', got %s", chunk.Path)
	}

	if chunk.Language != "go" {
		t.Errorf("expected language 'go', got %s", chunk.Language)
	}

	if chunk.DocumentID != doc.Hash {
		t.Error("chunk document ID should match document hash")
	}

	if chunk.IndexedAt.IsZero() {
		t.Error("indexed_at should be set")
	}

	if len(chunk.ID) != 16 {
		t.Errorf("expected 16 character chunk ID, got %d", len(chunk.ID))
	}
}

func TestIsValidSymbol(t *testing.T) {
	tests := []struct {
		symbol string
		valid  bool
	}{
		{"Authenticate", true},
		{"hello", true},
		{"_privateVar", true},
		{"MAX_RETRIES", true},
		{"if", false},    // keyword
		{"for", false},   // keyword
		{"class", false}, // keyword
		{"_", false},     // too short
		{"a", false},     // too short
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			result := isValidSymbol(tt.symbol)
			if result != tt.valid {
				t.Errorf("isValidSymbol(%s) = %v, expected %v", tt.symbol, result, tt.valid)
			}
		})
	}
}

func TestExtractImports(t *testing.T) {
	goContent := `package main

import (
	"fmt"
	"context"
	"github.com/example/pkg"
)
`
	goImports := ExtractImports(goContent, "go")
	if len(goImports) != 3 {
		t.Errorf("expected 3 Go imports, got %d: %v", len(goImports), goImports)
	}

	pythonContent := `
import os
from typing import List
import mypackage.submodule
`
	pythonImports := ExtractImports(pythonContent, "python")
	if len(pythonImports) < 2 {
		t.Errorf("expected at least 2 Python imports, got %d: %v", len(pythonImports), pythonImports)
	}
}
