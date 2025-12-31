//go:build !cgo

// Stub implementation for platforms without ONNX Runtime support.
// This allows the code to compile everywhere.

package onnx

import (
	"log"
	"os"
)

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

func isRuntimeAvailable() bool {
	// If Mock mode is enabled, we claim runtime IS available
	if os.Getenv("RICE_SEARCH_MOCK_ML") == "true" {
		return true
	}
	return false
}
