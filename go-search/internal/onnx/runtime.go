// Package onnx provides ONNX Runtime integration for ML inference.
package onnx

import (
	"runtime"
	"sync"
)

// Runtime manages the ONNX Runtime environment.
type Runtime struct {
	mu          sync.Mutex
	initialized bool
	device      Device
	sessions    map[string]*Session
	impl        runtimeImpl
}

// Device represents the execution device.
type Device string

const (
	DeviceCPU      Device = "cpu"
	DeviceCUDA     Device = "cuda"
	DeviceTensorRT Device = "tensorrt"
)

// RuntimeConfig holds runtime configuration.
type RuntimeConfig struct {
	Device         Device
	CUDADeviceID   int
	IntraOpThreads int
	InterOpThreads int
	MemoryLimit    int64 // bytes, 0 = unlimited
	LibraryPath    string
}

// DefaultRuntimeConfig returns sensible defaults.
func DefaultRuntimeConfig() RuntimeConfig {
	threads := runtime.NumCPU()
	if threads > 8 {
		threads = 8
	}

	return RuntimeConfig{
		Device:         DeviceCPU,
		CUDADeviceID:   0,
		IntraOpThreads: threads,
		InterOpThreads: 1,
		MemoryLimit:    0,
	}
}

// NewRuntime creates a new ONNX Runtime.
func NewRuntime(cfg RuntimeConfig) (*Runtime, error) {
	r := &Runtime{
		device:   cfg.Device,
		sessions: make(map[string]*Session),
	}

	// Initialize platform-specific implementation
	impl, err := newRuntimeImpl(cfg)
	if err != nil {
		return nil, err
	}
	r.impl = impl
	r.initialized = true

	return r, nil
}

// LoadSession loads an ONNX model and returns a session.
func (r *Runtime) LoadSession(name, modelPath string, opts ...SessionOption) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already loaded
	if session, ok := r.sessions[name]; ok {
		return session, nil
	}

	// Create session options
	sessionOpts := defaultSessionOptions()
	for _, opt := range opts {
		opt(&sessionOpts)
	}

	// Create the session using implementation
	session, err := r.impl.createSession(name, modelPath, r.device, sessionOpts)
	if err != nil {
		return nil, err
	}

	r.sessions[name] = session
	return session, nil
}

// GetSession returns a loaded session by name.
func (r *Runtime) GetSession(name string) (*Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[name]
	return session, ok
}

// UnloadSession unloads a session.
func (r *Runtime) UnloadSession(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[name]
	if !ok {
		return nil
	}

	if err := session.Close(); err != nil {
		return err
	}

	delete(r.sessions, name)
	return nil
}

// Close closes the runtime and all sessions.
func (r *Runtime) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for name, session := range r.sessions {
		if err := session.Close(); err != nil {
			lastErr = err
		}
		delete(r.sessions, name)
	}

	if r.impl != nil {
		if err := r.impl.close(); err != nil {
			lastErr = err
		}
	}

	r.initialized = false
	return lastErr
}

// Device returns the configured device.
func (r *Runtime) Device() Device {
	return r.device
}

// IsGPU returns true if using GPU acceleration.
func (r *Runtime) IsGPU() bool {
	return r.device == DeviceCUDA || r.device == DeviceTensorRT
}

// IsAvailable returns true if ONNX Runtime is available on this platform.
func IsAvailable() bool {
	return isRuntimeAvailable()
}

// runtimeImpl is the platform-specific runtime implementation.
type runtimeImpl interface {
	createSession(name, modelPath string, device Device, opts SessionOptions) (*Session, error)
	close() error
}
