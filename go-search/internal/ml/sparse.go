package ml

import (
	"context"
	"path/filepath"
	"sort"

	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/onnx"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// SparseEncoder generates sparse vectors using SPLADE.
type SparseEncoder struct {
	session   *onnx.Session
	tokenizer *onnx.Tokenizer
	cfg       config.MLConfig
	log       *logger.Logger
	topK      int // Top-k terms to keep per document
}

// NewSparseEncoder creates a new sparse encoder.
func NewSparseEncoder(runtime *onnx.Runtime, cfg config.MLConfig, log *logger.Logger) (*SparseEncoder, error) {
	modelDir := filepath.Join(cfg.ModelsDir, cfg.SparseModel)
	modelPath := filepath.Join(modelDir, "model.onnx")

	// Determine device for sparse encoder based on config
	device := onnx.DeviceCPU
	if cfg.SparseGPU && (cfg.Device == "cuda" || cfg.Device == "tensorrt") {
		if cfg.Device == "tensorrt" {
			device = onnx.DeviceTensorRT
		} else {
			device = onnx.DeviceCUDA
		}
		log.Info("Loading sparse encoder on GPU", "device", device)
	} else {
		log.Info("Loading sparse encoder on CPU")
	}

	// Load session with device-specific setting
	session, err := runtime.LoadSessionWithDevice("sparse", modelPath, device)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to load sparse model", err)
	}

	// Load tokenizer
	tokCfg := onnx.DefaultTokenizerConfig()
	tokCfg.MaxLength = cfg.MaxSeqLength

	tokenizer, err := onnx.NewTokenizer(modelDir, tokCfg)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to load sparse tokenizer", err)
	}

	return &SparseEncoder{
		session:   session,
		tokenizer: tokenizer,
		cfg:       cfg,
		log:       log,
		topK:      256, // Default: keep top 256 terms
	}, nil
}

// Encode generates sparse vectors for texts.
func (s *SparseEncoder) Encode(ctx context.Context, texts []string) ([]SparseVector, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Process in batches
	batchSize := s.cfg.SparseBatchSize
	if batchSize <= 0 {
		batchSize = 32
	}

	allVectors := make([]SparseVector, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		vectors, err := s.encodeBatch(ctx, batch)
		if err != nil {
			return nil, err
		}

		allVectors = append(allVectors, vectors...)
	}

	return allVectors, nil
}

func (s *SparseEncoder) encodeBatch(ctx context.Context, texts []string) ([]SparseVector, error) {
	// Tokenize
	encoding, err := s.tokenizer.EncodePadded(texts, true)
	if err != nil {
		return nil, err
	}

	// Run inference - SPLADE outputs logits over vocabulary
	outputs, err := s.session.RunFloat32(
		encoding.InputIDs,
		encoding.AttentionMask,
		encoding.Shape(),
	)
	if err != nil {
		return nil, err
	}

	// Convert to sparse vectors
	vectors := s.toSparseVectors(outputs, encoding)

	return vectors, nil
}

// toSparseVectors converts SPLADE output to sparse vectors.
func (s *SparseEncoder) toSparseVectors(output []float32, encoding *onnx.BatchEncoding) []SparseVector {
	batchSize := encoding.BatchSize
	vocabSize := s.tokenizer.VocabSize()

	vectors := make([]SparseVector, batchSize)

	for b := 0; b < batchSize; b++ {
		// Collect non-zero weights
		type termWeight struct {
			idx    uint32
			weight float32
		}

		weights := make([]termWeight, 0)

		for v := 0; v < vocabSize; v++ {
			idx := b*vocabSize + v
			if idx >= len(output) {
				break
			}

			weight := output[idx]
			if weight > 0 {
				weights = append(weights, termWeight{
					idx:    uint32(v),
					weight: weight,
				})
			}
		}

		// Sort by weight descending
		sort.Slice(weights, func(i, j int) bool {
			return weights[i].weight > weights[j].weight
		})

		// Keep top-k
		if len(weights) > s.topK {
			weights = weights[:s.topK]
		}

		// Convert to SparseVector
		indices := make([]uint32, len(weights))
		values := make([]float32, len(weights))

		for i, w := range weights {
			indices[i] = w.idx
			values[i] = w.weight
		}

		vectors[b] = SparseVector{
			Indices: indices,
			Values:  values,
		}
	}

	return vectors
}

// Close releases resources.
func (s *SparseEncoder) Close() error {
	if s.tokenizer != nil {
		s.tokenizer.Close()
	}
	return nil
}
