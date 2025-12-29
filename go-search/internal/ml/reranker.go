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

// Reranker scores query-document pairs using a cross-encoder.
type Reranker struct {
	session   *onnx.Session
	tokenizer *onnx.Tokenizer
	cfg       config.MLConfig
	log       *logger.Logger
}

// NewReranker creates a new reranker.
func NewReranker(runtime *onnx.Runtime, cfg config.MLConfig, log *logger.Logger) (*Reranker, error) {
	modelDir := filepath.Join(cfg.ModelsDir, cfg.RerankModel)
	modelPath := filepath.Join(modelDir, "model.onnx")

	// Determine device for reranker based on config
	device := onnx.DeviceCPU
	if cfg.RerankGPU && (cfg.Device == "cuda" || cfg.Device == "tensorrt") {
		if cfg.Device == "tensorrt" {
			device = onnx.DeviceTensorRT
		} else {
			device = onnx.DeviceCUDA
		}
		log.Info("Loading reranker on GPU", "device", device)
	} else {
		log.Info("Loading reranker on CPU")
	}

	// Load session with device-specific setting
	session, err := runtime.LoadSessionWithDevice("reranker", modelPath, device)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to load reranker model", err)
	}

	// Load tokenizer
	tokCfg := onnx.DefaultTokenizerConfig()
	tokCfg.MaxLength = cfg.MaxSeqLength

	tokenizer, err := onnx.NewTokenizer(modelDir, tokCfg)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to load reranker tokenizer", err)
	}

	return &Reranker{
		session:   session,
		tokenizer: tokenizer,
		cfg:       cfg,
		log:       log,
	}, nil
}

// Rerank scores and reranks documents for a query.
func (r *Reranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RankedResult, error) {
	if len(documents) == 0 {
		return nil, nil
	}

	if topK <= 0 || topK > len(documents) {
		topK = len(documents)
	}

	// Score all documents
	scores, err := r.score(ctx, query, documents)
	if err != nil {
		return nil, err
	}

	// Create results with original indices
	results := make([]RankedResult, len(documents))
	for i, score := range scores {
		results[i] = RankedResult{
			Index: i,
			Score: score,
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top-k
	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// score computes relevance scores for query-document pairs.
func (r *Reranker) score(ctx context.Context, query string, documents []string) ([]float32, error) {
	batchSize := r.cfg.RerankBatchSize
	if batchSize <= 0 {
		batchSize = 16
	}

	scores := make([]float32, len(documents))

	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}

		batch := documents[i:end]
		batchScores, err := r.scoreBatch(ctx, query, batch)
		if err != nil {
			return nil, err
		}

		copy(scores[i:end], batchScores)
	}

	return scores, nil
}

func (r *Reranker) scoreBatch(ctx context.Context, query string, documents []string) ([]float32, error) {
	// For cross-encoder, we need to tokenize query-document pairs
	// Format: [CLS] query [SEP] document [SEP]
	pairs := make([]string, len(documents))
	for i, doc := range documents {
		// Simple concatenation - tokenizer handles special tokens
		pairs[i] = query + " [SEP] " + doc
	}

	// Tokenize pairs
	encoding, err := r.tokenizer.EncodePadded(pairs, true)
	if err != nil {
		return nil, err
	}

	// Run inference
	outputs, err := r.session.RunFloat32(
		encoding.InputIDs,
		encoding.AttentionMask,
		encoding.Shape(),
	)
	if err != nil {
		return nil, err
	}

	// Extract scores (typically the logit for relevance class)
	scores := r.extractScores(outputs, len(documents))

	return scores, nil
}

// extractScores extracts relevance scores from model output.
func (r *Reranker) extractScores(output []float32, batchSize int) []float32 {
	scores := make([]float32, batchSize)

	// For binary classification, output is typically [batch_size, 2] logits
	// We take the positive class logit or apply sigmoid for single output
	if len(output) >= batchSize*2 {
		// Two-class output: take positive class
		for i := 0; i < batchSize; i++ {
			scores[i] = output[i*2+1] // Positive class
		}
	} else if len(output) >= batchSize {
		// Single output per sample
		for i := 0; i < batchSize; i++ {
			scores[i] = sigmoid(output[i])
		}
	}

	return scores
}

// Close releases resources.
func (r *Reranker) Close() error {
	if r.tokenizer != nil {
		r.tokenizer.Close()
	}
	return nil
}

// sigmoid applies the sigmoid function.
func sigmoid(x float32) float32 {
	return float32(1.0 / (1.0 + exp(-float64(x))))
}

// exp is a simple exp approximation for float64.
func exp(x float64) float64 {
	// Use standard library through conversion
	if x > 88 {
		return 1e38
	}
	if x < -88 {
		return 0
	}

	// Taylor series approximation for small x, lookup for larger
	if x >= -1 && x <= 1 {
		// Taylor series: e^x = 1 + x + x^2/2! + x^3/3! + ...
		result := 1.0
		term := 1.0
		for i := 1; i <= 10; i++ {
			term *= x / float64(i)
			result += term
		}
		return result
	}

	// For larger values, use repeated squaring
	if x < 0 {
		return 1.0 / exp(-x)
	}

	// e^x = e^(x/2) * e^(x/2)
	half := exp(x / 2)
	return half * half
}
