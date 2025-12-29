package onnx

// Tokenizer wraps a HuggingFace tokenizer.
type Tokenizer struct {
	maxLength int
	padID     int32
	vocabSize int
	impl      tokenizerImpl
}

// TokenizerConfig holds tokenizer configuration.
type TokenizerConfig struct {
	MaxLength int
	PadToken  string
}

// DefaultTokenizerConfig returns default configuration.
func DefaultTokenizerConfig() TokenizerConfig {
	return TokenizerConfig{
		MaxLength: 512,
		PadToken:  "[PAD]",
	}
}

// NewTokenizer creates a tokenizer from a tokenizer.json file.
func NewTokenizer(modelDir string, cfg TokenizerConfig) (*Tokenizer, error) {
	impl, vocabSize, err := newTokenizerImpl(modelDir, cfg)
	if err != nil {
		return nil, err
	}

	return &Tokenizer{
		maxLength: cfg.MaxLength,
		padID:     0,
		vocabSize: vocabSize,
		impl:      impl,
	}, nil
}

// Encode tokenizes a single text.
func (t *Tokenizer) Encode(text string, addSpecialTokens bool) (*Encoding, error) {
	return t.impl.encode(text, addSpecialTokens)
}

// EncodeBatch tokenizes multiple texts.
func (t *Tokenizer) EncodeBatch(texts []string, addSpecialTokens bool) ([]*Encoding, error) {
	encodings := make([]*Encoding, len(texts))

	for i, text := range texts {
		enc, err := t.Encode(text, addSpecialTokens)
		if err != nil {
			return nil, err
		}
		encodings[i] = enc
	}

	return encodings, nil
}

// EncodePadded tokenizes and pads to max length.
func (t *Tokenizer) EncodePadded(texts []string, addSpecialTokens bool) (*BatchEncoding, error) {
	encodings, err := t.EncodeBatch(texts, addSpecialTokens)
	if err != nil {
		return nil, err
	}

	return t.PadBatch(encodings), nil
}

// PadBatch pads a batch of encodings to the same length.
func (t *Tokenizer) PadBatch(encodings []*Encoding) *BatchEncoding {
	if len(encodings) == 0 {
		return &BatchEncoding{}
	}

	// Find max length in batch (capped by t.maxLength)
	maxLen := 0
	for _, enc := range encodings {
		if len(enc.IDs) > maxLen {
			maxLen = len(enc.IDs)
		}
	}

	if maxLen > t.maxLength {
		maxLen = t.maxLength
	}

	// Pad each encoding
	batchSize := len(encodings)
	inputIDs := make([]int64, batchSize*maxLen)
	attentionMask := make([]int64, batchSize*maxLen)

	for i, enc := range encodings {
		offset := i * maxLen

		// Copy actual tokens (truncate if needed)
		copyLen := len(enc.IDs)
		if copyLen > maxLen {
			copyLen = maxLen
		}

		for j := 0; j < copyLen; j++ {
			inputIDs[offset+j] = int64(enc.IDs[j])
			attentionMask[offset+j] = int64(enc.AttentionMask[j])
		}

		// Padding is already 0 (from make)
	}

	return &BatchEncoding{
		InputIDs:      inputIDs,
		AttentionMask: attentionMask,
		BatchSize:     batchSize,
		SeqLength:     maxLen,
	}
}

// Decode converts token IDs back to text.
func (t *Tokenizer) Decode(ids []uint32, skipSpecialTokens bool) string {
	return t.impl.decode(ids, skipSpecialTokens)
}

// VocabSize returns the vocabulary size.
func (t *Tokenizer) VocabSize() int {
	return t.vocabSize
}

// Close releases tokenizer resources.
func (t *Tokenizer) Close() error {
	if t.impl != nil {
		return t.impl.close()
	}
	return nil
}

// Encoding holds the result of tokenization.
type Encoding struct {
	IDs           []uint32
	AttentionMask []uint32
	Tokens        []string
}

// BatchEncoding holds padded batch encodings ready for model input.
type BatchEncoding struct {
	InputIDs      []int64
	AttentionMask []int64
	BatchSize     int
	SeqLength     int
}

// Shape returns the tensor shape [batch_size, seq_length].
func (b *BatchEncoding) Shape() []int64 {
	return []int64{int64(b.BatchSize), int64(b.SeqLength)}
}

// tokenizerImpl is the platform-specific tokenizer implementation.
type tokenizerImpl interface {
	encode(text string, addSpecialTokens bool) (*Encoding, error)
	decode(ids []uint32, skipSpecialTokens bool) string
	close() error
}
