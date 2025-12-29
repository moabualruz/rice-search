package onnx

import (
	"testing"
)

func TestStubTokenizer_Encode(t *testing.T) {
	tok, err := NewTokenizer(".", DefaultTokenizerConfig())
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	enc, err := tok.Encode("hello world", true)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Should have CLS, hello, world, SEP
	if len(enc.IDs) < 4 {
		t.Errorf("expected at least 4 tokens, got %d", len(enc.IDs))
	}

	// First token should be CLS
	if enc.IDs[0] != 2 { // [CLS] = 2
		t.Errorf("first token should be [CLS] (2), got %d", enc.IDs[0])
	}

	// Last token should be SEP
	if enc.IDs[len(enc.IDs)-1] != 3 { // [SEP] = 3
		t.Errorf("last token should be [SEP] (3), got %d", enc.IDs[len(enc.IDs)-1])
	}

	// Attention mask should be all 1s
	for i, mask := range enc.AttentionMask {
		if mask != 1 {
			t.Errorf("attention mask[%d] = %d, want 1", i, mask)
		}
	}
}

func TestStubTokenizer_EncodeNoSpecialTokens(t *testing.T) {
	tok, err := NewTokenizer(".", DefaultTokenizerConfig())
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	enc, err := tok.Encode("hello world", false)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	// Should only have hello, world (no CLS/SEP)
	if len(enc.IDs) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(enc.IDs))
	}
}

func TestStubTokenizer_EncodeBatch(t *testing.T) {
	tok, err := NewTokenizer(".", DefaultTokenizerConfig())
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	texts := []string{"hello", "world", "test"}
	encodings, err := tok.EncodeBatch(texts, true)
	if err != nil {
		t.Fatalf("EncodeBatch error: %v", err)
	}

	if len(encodings) != 3 {
		t.Errorf("expected 3 encodings, got %d", len(encodings))
	}
}

func TestStubTokenizer_EncodePadded(t *testing.T) {
	tok, err := NewTokenizer(".", DefaultTokenizerConfig())
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	texts := []string{"hello", "hello world test"}
	batch, err := tok.EncodePadded(texts, true)
	if err != nil {
		t.Fatalf("EncodePadded error: %v", err)
	}

	if batch.BatchSize != 2 {
		t.Errorf("batch size = %d, want 2", batch.BatchSize)
	}

	// All sequences should be padded to the same length
	expectedLen := batch.SeqLength
	if len(batch.InputIDs) != batch.BatchSize*expectedLen {
		t.Errorf("input IDs length = %d, want %d", len(batch.InputIDs), batch.BatchSize*expectedLen)
	}

	// Check shape
	shape := batch.Shape()
	if shape[0] != int64(batch.BatchSize) || shape[1] != int64(batch.SeqLength) {
		t.Errorf("shape = %v, want [%d, %d]", shape, batch.BatchSize, batch.SeqLength)
	}
}

func TestStubTokenizer_Decode(t *testing.T) {
	tok, err := NewTokenizer(".", DefaultTokenizerConfig())
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	enc, _ := tok.Encode("hello world", true)
	decoded := tok.Decode(enc.IDs, true)

	// Should contain hello and world
	if decoded == "" {
		t.Error("decoded string should not be empty")
	}
}

func TestStubTokenizer_Truncation(t *testing.T) {
	cfg := TokenizerConfig{
		MaxLength: 5,
	}

	tok, err := NewTokenizer(".", cfg)
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	// Long text that would exceed max length
	enc, err := tok.Encode("a b c d e f g h i j", true)
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	if len(enc.IDs) > 5 {
		t.Errorf("tokens not truncated: got %d, max %d", len(enc.IDs), 5)
	}
}

func TestStubTokenizer_VocabSize(t *testing.T) {
	tok, err := NewTokenizer(".", DefaultTokenizerConfig())
	if err != nil {
		t.Fatalf("NewTokenizer error: %v", err)
	}
	defer tok.Close()

	vocabSize := tok.VocabSize()
	if vocabSize < 5 { // At least special tokens
		t.Errorf("vocab size too small: %d", vocabSize)
	}
}
