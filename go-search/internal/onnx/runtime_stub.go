// Stub implementation for platforms without ONNX Runtime support.
// This allows the code to compile everywhere.

package onnx

import (
	"log"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// stubRuntime is a stub implementation that returns errors.
type stubRuntime struct{}

func newRuntimeImpl(cfg RuntimeConfig) (runtimeResult, error) {
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
	return false
}
