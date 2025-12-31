package onnx

import (
	"strings"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// stubRuntime is a stub implementation that returns errors.
// It is available in both CGO and non-CGO builds to serve as fallback.
type stubRuntime struct{}

var _ runtimeImpl = (*stubRuntime)(nil)
var _ runtimeImpl = (*mockRuntime)(nil)
var _ sessionImpl = (*mockSession)(nil)

func (s *stubRuntime) createSession(name, modelPath string, device Device, opts SessionOptions) (*Session, error) {
	return nil, errors.New(errors.CodeMLError, "ONNX Runtime not available on this platform - models must be downloaded and ONNX Runtime installed")
}

func (s *stubRuntime) Close() error {
	return nil
}

// mockRuntime is a mock implementation for testing.
// It is available in both CGO and non-CGO builds to support RICE_SEARCH_MOCK_ML.
type mockRuntime struct{}

func (m *mockRuntime) createSession(name, modelPath string, device Device, opts SessionOptions) (*Session, error) {
	// Determine expected output shape based on model name
	var outputDim int64 = 384 // Default embedding size
	if strings.Contains(name, "sparse") || strings.Contains(name, "splade") {
		outputDim = 30522 // BERT vocab size
	} else if strings.Contains(name, "rerank") {
		outputDim = 1 // Logits
	}

	// Create a mock session
	impl := &mockSession{
		name:      name,
		outputDim: outputDim,
	}

	return &Session{
		name: name,
		path: modelPath,
		impl: impl,
	}, nil
}

func (m *mockRuntime) Close() error {
	return nil
}

type mockSession struct {
	name      string
	outputDim int64
}

func (s *mockSession) run(inputs map[string]*Tensor) (map[string]*Tensor, error) {
	// Find batch size from input_ids
	var batchSize int64 = 1

	if t, ok := inputs["input_ids"]; ok {
		shape := t.Shape()
		if len(shape) > 0 {
			batchSize = shape[0]
		}
	}

	outputs := make(map[string]*Tensor)

	if strings.Contains(s.name, "sparse") || strings.Contains(s.name, "splade") {
		// For sparse, we don't want all 1s (would be 30k terms).
		// Return valid logits for a few determinstic indices.
		// SPLADE takes positive values.
		size := batchSize * s.outputDim
		data := make([]float32, size)

		// Set a few indices to 1.0 for every batch item
		// ensuring they match each other.
		// BERT vocab size ~30k. Let's pick indices 100, 101, 102.
		for b := int64(0); b < batchSize; b++ {
			offset := b * s.outputDim
			if s.outputDim > 105 {
				data[offset+100] = 1.0
				data[offset+101] = 1.0
				data[offset+102] = 1.0
			}
		}

		outputTensor := NewTensorFloat32(data, []int64{batchSize, s.outputDim})
		outputs["output"] = outputTensor
		outputs["logits"] = outputTensor

	} else {
		// Dense and Reranker
		// Return all 1s to ensure max similarity/score
		size := batchSize * s.outputDim
		data := make([]float32, size)
		for i := range data {
			data[i] = 1.0
		}

		outputTensor := NewTensorFloat32(data, []int64{batchSize, s.outputDim})
		outputs["last_hidden_state"] = outputTensor
		outputs["logits"] = outputTensor
		outputs["embeddings"] = outputTensor
		outputs["output"] = outputTensor
	}

	return outputs, nil
}

func (s *mockSession) Close() error {
	return nil
}
