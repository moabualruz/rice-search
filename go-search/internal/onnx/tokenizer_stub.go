// Stub tokenizer implementation for platforms without HuggingFace tokenizers support.

package onnx

import (
	"strings"
	"unicode"
)

// stubTokenizer provides a simple word-based tokenizer for development/testing.
type stubTokenizer struct {
	vocab     map[string]uint32
	invVocab  map[uint32]string
	maxLength int
}

func newTokenizerImpl(modelDir string, cfg TokenizerConfig) (tokenizerImpl, int, error) {
	// Create a simple stub tokenizer
	// In production, this would load the actual tokenizer
	vocab := makeBasicVocab()

	invVocab := make(map[uint32]string)
	for word, id := range vocab {
		invVocab[id] = word
	}

	return &stubTokenizer{
		vocab:     vocab,
		invVocab:  invVocab,
		maxLength: cfg.MaxLength,
	}, len(vocab), nil
}

func makeBasicVocab() map[string]uint32 {
	// Basic vocabulary for stub tokenizer
	vocab := map[string]uint32{
		"[PAD]":  0,
		"[UNK]":  1,
		"[CLS]":  2,
		"[SEP]":  3,
		"[MASK]": 4,
	}

	// Add common words and characters
	id := uint32(5)
	commonWords := []string{
		"the", "a", "an", "is", "are", "was", "were", "be", "been",
		"have", "has", "had", "do", "does", "did", "will", "would",
		"could", "should", "may", "might", "must", "shall",
		"and", "or", "but", "if", "then", "else", "when", "where",
		"what", "which", "who", "how", "why", "this", "that", "these",
		"function", "class", "def", "return", "import", "from", "as",
		"for", "in", "of", "to", "with", "on", "at", "by", "not", "no",
	}

	for _, word := range commonWords {
		vocab[word] = id
		id++
	}

	// Add letters and digits
	for c := 'a'; c <= 'z'; c++ {
		vocab[string(c)] = id
		id++
	}
	for c := '0'; c <= '9'; c++ {
		vocab[string(c)] = id
		id++
	}

	return vocab
}

func (t *stubTokenizer) encode(text string, addSpecialTokens bool) (*Encoding, error) {
	// Simple word tokenization
	words := tokenizeSimple(text)

	ids := make([]uint32, 0, len(words)+2)
	mask := make([]uint32, 0, len(words)+2)
	tokens := make([]string, 0, len(words)+2)

	if addSpecialTokens {
		ids = append(ids, t.vocab["[CLS]"])
		mask = append(mask, 1)
		tokens = append(tokens, "[CLS]")
	}

	for _, word := range words {
		word = strings.ToLower(word)
		if id, ok := t.vocab[word]; ok {
			ids = append(ids, id)
		} else {
			ids = append(ids, t.vocab["[UNK]"])
		}
		mask = append(mask, 1)
		tokens = append(tokens, word)
	}

	if addSpecialTokens {
		ids = append(ids, t.vocab["[SEP]"])
		mask = append(mask, 1)
		tokens = append(tokens, "[SEP]")
	}

	// Truncate if needed
	if len(ids) > t.maxLength {
		ids = ids[:t.maxLength]
		mask = mask[:t.maxLength]
		tokens = tokens[:t.maxLength]
	}

	return &Encoding{
		IDs:           ids,
		AttentionMask: mask,
		Tokens:        tokens,
	}, nil
}

func (t *stubTokenizer) decode(ids []uint32, skipSpecialTokens bool) string {
	var words []string
	for _, id := range ids {
		if word, ok := t.invVocab[id]; ok {
			if skipSpecialTokens && strings.HasPrefix(word, "[") && strings.HasSuffix(word, "]") {
				continue
			}
			words = append(words, word)
		} else {
			// Unknown token - return the ID as a placeholder
			words = append(words, "[UNK]")
		}
	}
	if len(words) == 0 {
		return "[empty]"
	}
	return strings.Join(words, " ")
}

func (t *stubTokenizer) close() error {
	return nil
}

// tokenizeSimple splits text into words.
func tokenizeSimple(text string) []string {
	var words []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}
