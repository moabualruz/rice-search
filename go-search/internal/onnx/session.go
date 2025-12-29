package onnx

import (
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// Session wraps an ONNX Runtime session.
type Session struct {
	mu         sync.Mutex
	path       string
	session    *ort.DynamicAdvancedSession
	inputInfo  []TensorInfo
	outputInfo []TensorInfo
	closed     bool
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

func newSession(modelPath string, device Device, opts SessionOptions) (*Session, error) {
	// Create session options based on device
	var session *ort.DynamicAdvancedSession
	var err error

	inputNames := []string{"input_ids", "attention_mask"}
	outputNames := []string{"last_hidden_state"}

	// Note: The actual input/output names depend on the model
	// These will be detected from the model in a full implementation

	switch device {
	case DeviceCUDA:
		session, err = ort.NewDynamicAdvancedSessionWithCUDA(
			modelPath,
			inputNames,
			outputNames,
			0, // CUDA device ID
		)
	case DeviceTensorRT:
		session, err = ort.NewDynamicAdvancedSessionWithCUDA(
			modelPath,
			inputNames,
			outputNames,
			0,
		)
	default:
		session, err = ort.NewDynamicAdvancedSession(
			modelPath,
			inputNames,
			outputNames,
			nil,
		)
	}

	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to create ONNX session", err)
	}

	return &Session{
		path:    modelPath,
		session: session,
		inputInfo: []TensorInfo{
			{Name: "input_ids", Type: TensorTypeInt64},
			{Name: "attention_mask", Type: TensorTypeInt64},
		},
		outputInfo: []TensorInfo{
			{Name: "last_hidden_state", Type: TensorTypeFloat32},
		},
	}, nil
}

// Run executes the session with the given inputs.
func (s *Session) Run(inputs map[string]*Tensor) (map[string]*Tensor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, errors.New(errors.CodeMLError, "session is closed")
	}

	// Convert inputs to ONNX tensors
	ortInputs := make([]ort.ArbitraryTensor, 0, len(inputs))
	for _, info := range s.inputInfo {
		tensor, ok := inputs[info.Name]
		if !ok {
			return nil, errors.ValidationError("missing input: " + info.Name)
		}

		ortTensor, err := tensor.toORT()
		if err != nil {
			return nil, err
		}
		ortInputs = append(ortInputs, ortTensor)
	}

	// Run inference
	ortOutputs, err := s.session.Run(ortInputs)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "inference failed", err)
	}

	// Convert outputs
	outputs := make(map[string]*Tensor)
	for i, info := range s.outputInfo {
		if i >= len(ortOutputs) {
			break
		}
		outputs[info.Name] = tensorFromORT(ortOutputs[i])
	}

	return outputs, nil
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

	output, ok := outputs["last_hidden_state"]
	if !ok {
		return nil, errors.New(errors.CodeMLError, "missing output: last_hidden_state")
	}

	return output.Float32Data(), nil
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

// Close closes the session.
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	if s.session != nil {
		if err := s.session.Destroy(); err != nil {
			return errors.Wrap(errors.CodeMLError, "failed to destroy session", err)
		}
	}

	s.closed = true
	return nil
}
