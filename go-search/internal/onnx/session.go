package onnx

import (
	"sync"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// Session wraps an ONNX Runtime session.
type Session struct {
	mu         sync.Mutex
	name       string
	path       string
	inputInfo  []TensorInfo
	outputInfo []TensorInfo
	closed     bool
	impl       sessionImpl
}

// TensorInfo describes an input or output tensor.
type TensorInfo struct {
	Name  string
	Shape []int64
	Type  TensorType
}

// TensorType represents ONNX tensor types.
type TensorType int

const (
	TensorTypeFloat32 TensorType = iota
	TensorTypeFloat16
	TensorTypeInt64
	TensorTypeInt32
)

// SessionOptions holds session configuration.
type SessionOptions struct {
	GraphOptLevel     int
	EnableMemPattern  bool
	EnableCPUMemArena bool
}

// SessionOption is a function that modifies session options.
type SessionOption func(*SessionOptions)

func defaultSessionOptions() SessionOptions {
	return SessionOptions{
		GraphOptLevel:     99, // All optimizations
		EnableMemPattern:  true,
		EnableCPUMemArena: true,
	}
}

// WithGraphOptLevel sets the graph optimization level.
func WithGraphOptLevel(level int) SessionOption {
	return func(o *SessionOptions) {
		o.GraphOptLevel = level
	}
}

// Run executes the session with the given inputs.
func (s *Session) Run(inputs map[string]*Tensor) (map[string]*Tensor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New(errors.CodeMLError, "session is closed")
	}

	if s.impl == nil {
		return nil, errors.New(errors.CodeMLError, "session not initialized")
	}

	return s.impl.run(inputs)
}

// RunFloat32 is a convenience method for float32 inference.
func (s *Session) RunFloat32(inputIDs, attentionMask []int64, shape []int64) ([]float32, error) {
	inputs := map[string]*Tensor{
		"input_ids":      NewTensorInt64(inputIDs, shape),
		"attention_mask": NewTensorInt64(attentionMask, shape),
	}

	outputs, err := s.Run(inputs)
	if err != nil {
		return nil, err
	}

	// Look for common output names
	for _, name := range []string{"last_hidden_state", "logits", "embeddings", "output"} {
		if output, ok := outputs[name]; ok {
			return output.Float32Data(), nil
		}
	}

	// Return first output
	for _, output := range outputs {
		return output.Float32Data(), nil
	}

	return nil, errors.New(errors.CodeMLError, "no output tensor found")
}

// InputInfo returns input tensor information.
func (s *Session) InputInfo() []TensorInfo {
	return s.inputInfo
}

// OutputInfo returns output tensor information.
func (s *Session) OutputInfo() []TensorInfo {
	return s.outputInfo
}

// Path returns the model path.
func (s *Session) Path() string {
	return s.path
}

// Name returns the session name.
func (s *Session) Name() string {
	return s.name
}

// Close closes the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	if s.impl != nil {
		if err := s.impl.close(); err != nil {
			return err
		}
	}

	s.closed = true
	return nil
}

// sessionImpl is the platform-specific session implementation.
type sessionImpl interface {
	run(inputs map[string]*Tensor) (map[string]*Tensor, error)
	close() error
}
