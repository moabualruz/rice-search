// Package onnx provides ONNX Runtime integration for ML inference.
package onnx

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// Runtime manages the ONNX Runtime environment.
type Runtime struct {
	mu          sync.Mutex
	initialized bool
	device      Device
	sessions    map[string]*Session
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

	if err := r.initialize(cfg); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *Runtime) initialize(cfg RuntimeConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	// Set library path if provided
	if cfg.LibraryPath != "" {
		ort.SetSharedLibraryPath(cfg.LibraryPath)
	}

	// Initialize the runtime
	if err := ort.InitializeEnvironment(); err != nil {
		return errors.Wrap(errors.CodeMLError, "failed to initialize ONNX runtime", err)
	}

	r.initialized = true
	return nil
}

// LoadSession loads an ONNX model and returns a session.
func (r *Runtime) LoadSession(name, modelPath string, opts ...SessionOption) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already loaded
	if session, ok := r.sessions[name]; ok {
		return session, nil
	}

	// Verify model file exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, errors.NotFoundError(fmt.Sprintf("model file: %s", modelPath))
	}

	// Create session options
	sessionOpts := defaultSessionOptions()
	for _, opt := range opts {
		opt(&sessionOpts)
	}

	// Create the session
	session, err := newSession(modelPath, r.device, sessionOpts)
	if err != nil {
		return nil, err
	}

	r.sessions[name] = session
	return session, nil
}

// GetSession returns a loaded session by name.
func (r *Runtime) GetSession(name string) (*Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[name]
	if !ok {
		return nil, errors.NotFoundError(fmt.Sprintf("session: %s", name))
	}

	return session, nil
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

	if r.initialized {
		if err := ort.DestroyEnvironment(); err != nil {
			lastErr = err
		}
		r.initialized = false
	}

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

// FindModels finds ONNX models in a directory.
func FindModels(dir string) ([]string, error) {
	var models []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".onnx" {
			models = append(models, path)
		}

		return nil
	})

	return models, err
}
