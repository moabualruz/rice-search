// Stub implementation for platforms without ONNX Runtime support.
// This allows the code to compile everywhere.

package onnx

import (
	"log"
	"os"
	"strings"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// stubRuntime is a stub implementation that returns errors.
type stubRuntime struct{}

// mockRuntime is a mock implementation for testing.
type mockRuntime struct{}

func newRuntimeImpl(cfg RuntimeConfig) (runtimeResult, error) {
	// Check for Mock mode
	if os.Getenv("RICE_SEARCH_MOCK_ML") == "true" {
		log.Printf("[INFO] ML Mock Mode Enabled (RICE_SEARCH_MOCK_ML=true)")
		return runtimeResult{
			impl:         &mockRuntime{},
			actualDevice: DeviceCPU,
		}, nil
	}

	// Log warning if GPU was requested but we're falling back to stub
	if cfg.Device == DeviceCUDA || cfg.Device == DeviceTensorRT {
		log.Printf("[WARN] GPU device '%s' requested but ONNX Runtime is not available on this platform. "+
			"Falling back to stub implementation. ML inference will fail until ONNX Runtime is installed.", cfg.Device)
	} else if cfg.Device == DeviceCPU {
		log.Printf("[WARN] ONNX Runtime is not available on this platform. " +
			"ML inference will fail until ONNX Runtime is installed.")
	}

	// Return stub with actual device set to "stub" to indicate fallback
	return runtimeResult{
		impl:         &stubRuntime{},
		actualDevice: DeviceStub,
	}, nil
}

func (s *stubRuntime) createSession(name, modelPath string, device Device, opts SessionOptions) (*Session, error) {
	return nil, errors.New(errors.CodeMLError, "ONNX Runtime not available on this platform - models must be downloaded and ONNX Runtime installed")
}

func (s *stubRuntime) close() error {
	return nil
}

func isRuntimeAvailable() bool {
	// If Mock mode is enabled, we claim runtime IS available
	if os.Getenv("RICE_SEARCH_MOCK_ML") == "true" {
		return true
	}
	return false
}

// Mock Implementation

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

func (m *mockRuntime) close() error {
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

func (s *mockSession) close() error {
	return nil
}
