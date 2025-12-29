package hash

import (
	"strings"
	"testing"
)

func TestSHA256(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{
			[]byte("hello"),
			"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		},
		{
			[]byte(""),
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			got := SHA256(tt.input)
			if got != tt.want {
				t.Errorf("SHA256(%q) = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

func TestSHA256String(t *testing.T) {
	got := SHA256String("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if got != want {
		t.Errorf("SHA256String(hello) = %s, want %s", got, want)
	}
}

func TestSHA256Short(t *testing.T) {
	hash := SHA256([]byte("hello"))

	tests := []struct {
		n    int
		want string
	}{
		{8, hash[:8]},
		{16, hash[:16]},
		{32, hash[:32]},
		{64, hash},  // full hash
		{100, hash}, // exceeds length, returns full
	}

	for _, tt := range tests {
		got := SHA256Short([]byte("hello"), tt.n)
		if got != tt.want {
			t.Errorf("SHA256Short(hello, %d) = %s, want %s", tt.n, got, tt.want)
		}
	}
}

func TestChunkID(t *testing.T) {
	// Same inputs should produce same output
	id1 := ChunkID("src/main.go", 10, 50)
	id2 := ChunkID("src/main.go", 10, 50)

	if id1 != id2 {
		t.Errorf("ChunkID not deterministic: %s != %s", id1, id2)
	}

	// Different inputs should produce different output
	id3 := ChunkID("src/main.go", 10, 51)
	if id1 == id3 {
		t.Errorf("ChunkID collision: %s == %s", id1, id3)
	}

	// Should be 16 characters
	if len(id1) != 16 {
		t.Errorf("ChunkID length = %d, want 16", len(id1))
	}

	// Should be hex
	for _, c := range id1 {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("ChunkID contains non-hex character: %c", c)
		}
	}
}

func TestDocumentID(t *testing.T) {
	// Same inputs should produce same output
	id1 := DocumentID("src/main.go", "abc123")
	id2 := DocumentID("src/main.go", "abc123")

	if id1 != id2 {
		t.Errorf("DocumentID not deterministic: %s != %s", id1, id2)
	}

	// Different inputs should produce different output
	id3 := DocumentID("src/main.go", "abc124")
	if id1 == id3 {
		t.Errorf("DocumentID collision: %s == %s", id1, id3)
	}

	// Should be 16 characters
	if len(id1) != 16 {
		t.Errorf("DocumentID length = %d, want 16", len(id1))
	}
}

func BenchmarkSHA256(b *testing.B) {
	data := []byte("benchmark test data for hashing performance measurement")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SHA256(data)
	}
}

func BenchmarkChunkID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ChunkID("src/components/Button.tsx", 100, 200)
	}
}
