package ml

import (
	"context"
	"math"
	"path/filepath"

	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/onnx"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// Embedder generates dense embeddings from text.
type Embedder struct {
	session   *onnx.Session
	tokenizer *onnx.Tokenizer
	cfg       config.MLConfig
	log       *logger.Logger
}

// NewEmbedder creates a new embedder.
func NewEmbedder(runtime *onnx.Runtime, cfg config.MLConfig, log *logger.Logger) (*Embedder, error) {
	modelDir := filepath.Join(cfg.ModelsDir, cfg.EmbedModel)
	modelPath := filepath.Join(modelDir, "model.onnx")

	// Determine device for embedder based on config
	device := onnx.DeviceCPU
	if cfg.EmbedGPU && (cfg.Device == "cuda" || cfg.Device == "tensorrt") {
		if cfg.Device == "tensorrt" {
			device = onnx.DeviceTensorRT
		} else {
			device = onnx.DeviceCUDA
		}
		log.Info("Loading embedder on GPU", "device", device)
	} else {
		log.Info("Loading embedder on CPU")
	}

	// Load session with device-specific setting
	session, err := runtime.LoadSessionWithDevice("embedder", modelPath, device)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to load embedder model", err)
	}

	// Load tokenizer
	tokCfg := onnx.DefaultTokenizerConfig()
	tokCfg.MaxLength = cfg.MaxSeqLength

	tokenizer, err := onnx.NewTokenizer(modelDir, tokCfg)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to load embedder tokenizer", err)
	}

	return &Embedder{
		session:   session,
		tokenizer: tokenizer,
		cfg:       cfg,
		log:       log,
	}, nil
}

// Embed generates embeddings for texts.
func (e *Embedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Process in batches
	batchSize := e.cfg.EmbedBatchSize
	if batchSize <= 0 {
		batchSize = 32
	}

	allEmbeddings := make([][]float32, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, err
		}

		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	return allEmbeddings, nil
}

func (e *Embedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Tokenize
	encoding, err := e.tokenizer.EncodePadded(texts, true)
	if err != nil {
		return nil, err
	}

	// Run inference
	outputs, err := e.session.RunFloat32(
		encoding.InputIDs,
		encoding.AttentionMask,
		encoding.Shape(),
	)
	if err != nil {
		return nil, err
	}

	// Extract embeddings (mean pooling over sequence)
	embeddings := e.meanPooling(outputs, encoding)

	// L2 normalize
	for i := range embeddings {
		embeddings[i] = l2Normalize(embeddings[i])
	}

	return embeddings, nil
}

// meanPooling performs mean pooling over the sequence dimension.
func (e *Embedder) meanPooling(output []float32, encoding *onnx.BatchEncoding) [][]float32 {
	batchSize := encoding.BatchSize
	seqLen := encoding.SeqLength
	hiddenSize := e.cfg.EmbedDim

	embeddings := make([][]float32, batchSize)

	for b := 0; b < batchSize; b++ {
		embedding := make([]float32, hiddenSize)
		count := float32(0)

		for s := 0; s < seqLen; s++ {
			// Check attention mask
			maskIdx := b*seqLen + s
			if maskIdx < len(encoding.AttentionMask) && encoding.AttentionMask[maskIdx] == 0 {
				continue
			}

			count++
			for h := 0; h < hiddenSize; h++ {
				idx := b*seqLen*hiddenSize + s*hiddenSize + h
				if idx < len(output) {
					embedding[h] += output[idx]
				}
			}
		}

		// Average
		if count > 0 {
			for h := 0; h < hiddenSize; h++ {
				embedding[h] /= count
			}
		}

		embeddings[b] = embedding
	}

	return embeddings
}

// Close releases resources.
func (e *Embedder) Close() error {
	if e.tokenizer != nil {
		e.tokenizer.Close()
	}
	return nil
}

// l2Normalize normalizes a vector to unit length.
func l2Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}

	norm := float32(math.Sqrt(sum))
	if norm == 0 {
		return v
	}

	result := make([]float32, len(v))
	for i, x := range v {
		result[i] = x / norm
	}

	return result
}
