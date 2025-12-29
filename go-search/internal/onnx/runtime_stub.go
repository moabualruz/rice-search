// Stub implementation for platforms without ONNX Runtime support.
// This allows the code to compile everywhere.

package onnx

import (
	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// stubRuntime is a stub implementation that returns errors.
type stubRuntime struct{}

func newRuntimeImpl(cfg RuntimeConfig) (runtimeImpl, error) {
	// Return stub - actual ONNX operations will fail gracefully
	return &stubRuntime{}, nil
}

func (s *stubRuntime) createSession(name, modelPath string, device Device, opts SessionOptions) (*Session, error) {
	return nil, errors.New(errors.CodeMLError, "ONNX Runtime not available on this platform - models must be downloaded and ONNX Runtime installed")
}

func (s *stubRuntime) close() error {
	return nil
}

func isRuntimeAvailable() bool {
	return false
}
