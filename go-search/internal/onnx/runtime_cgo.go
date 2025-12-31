//go:build cgo

package onnx

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"unsafe"

	ort "github.com/yalue/onnxruntime_go"
)

// cgoRuntime implements the Runtime interface using ONNX Runtime C bindings.
type cgoRuntime struct {
	initialized bool
}

// Ensure interface compliance
var _ runtimeImpl = (*cgoRuntime)(nil)

func newRuntimeImpl(cfg RuntimeConfig) (runtimeResult, error) {
	// Check for Mock mode
	if os.Getenv("RICE_SEARCH_MOCK_ML") == "true" {
		log.Printf("[INFO] ML Mock Mode Enabled (RICE_SEARCH_MOCK_ML=true)")
		return runtimeResult{
			impl:         &mockRuntime{},
			actualDevice: DeviceCPU,
		}, nil
	}

	// 1. Locate Shared Library
	libPath := cfg.LibraryPath
	if libPath == "" {
		libPath = findLibraryPath()
	}

	if libPath == "" {
		log.Printf("[WARN] ONNX Runtime shared library not found. Falling back to stub.")
		return runtimeResult{
			impl:         &stubRuntime{},
			actualDevice: DeviceStub,
		}, nil
	}

	log.Printf("[INFO] Initializing ONNX Runtime with library: %s", libPath)

	// 2. Initialize Environment
	ort.SetSharedLibraryPath(libPath)
	err := ort.InitializeEnvironment()
	if err != nil {
		return runtimeResult{}, fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	actualDevice := cfg.Device

	return runtimeResult{
		impl:         &cgoRuntime{initialized: true},
		actualDevice: actualDevice,
	}, nil
}

func (c *cgoRuntime) createSession(name, modelPath string, device Device, opts SessionOptions) (*Session, error) {
	// Configure Session Options
	options, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	// Append Execution Providers
	switch device {
	case DeviceCUDA:
		cudaOptions, err := ort.NewCUDAProviderOptions()
		if err == nil {
			cudaOptions.Update(map[string]string{"device_id": "0"})
			options.AppendExecutionProviderCUDA(cudaOptions)
			cudaOptions.Destroy() // Destroy options after appending
		} else {
			log.Printf("[WARN] Failed to create CUDA options for %s: %v", name, err)
		}
	case DeviceTensorRT:
		trtOptions, err := ort.NewTensorRTProviderOptions()
		if err == nil {
			options.AppendExecutionProviderTensorRT(trtOptions)
			trtOptions.Destroy()
		} else {
			log.Printf("[WARN] Failed to create TensorRT options for %s: %v", name, err)
		}
		// Fallback to CUDA
		cudaOptions, err := ort.NewCUDAProviderOptions()
		if err == nil {
			cudaOptions.Update(map[string]string{"device_id": "0"})
			options.AppendExecutionProviderCUDA(cudaOptions)
			cudaOptions.Destroy()
		}
	}

	// Always append CPU as fallback (Implicit in ORT)
	// options.AppendExecutionProviderCPU()

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model file not found: %s", modelPath)
	}

	// Probe Model Info
	inputInfo, outputInfo, err := ort.GetInputOutputInfo(modelPath)
	if err != nil {
		log.Printf("[WARN] Failed to probe model info for %s: %v. Using defaults.", name, err)
	}

	var inputNames []string
	var outputNames []string

	if len(inputInfo) > 0 {
		for _, info := range inputInfo {
			inputNames = append(inputNames, info.Name)
		}
		for _, info := range outputInfo {
			outputNames = append(outputNames, info.Name)
		}
	} else {
		// Fallback Defaults
		inputNames = []string{"input_ids", "attention_mask"}
		if strings.Contains(name, "codbert") || strings.Contains(name, "bert") || strings.Contains(name, "jina") {
			outputNames = []string{"last_hidden_state", "pooler_output", "token_embeddings", "sentence_embedding"}
		} else if strings.Contains(name, "t5") {
			outputNames = []string{"encoder_last_hidden_state", "last_hidden_state"}
		} else if strings.Contains(name, "rerank") {
			outputNames = []string{"logits"}
		} else {
			outputNames = []string{"last_hidden_state", "logits"}
		}
	}

	// Create Dynamic Advanced Session
	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create ORT session: %w", err)
	}

	return &Session{
		name: name,
		path: modelPath,
		impl: &cgoSession{
			session:     session,
			inputNames:  inputNames,
			outputNames: outputNames,
		},
	}, nil
}

func (c *cgoRuntime) Close() error {
	return ort.DestroyEnvironment()
}

func isRuntimeAvailable() bool {
	return true
}

// cgoSession wraps the ORT session
type cgoSession struct {
	session     *ort.DynamicAdvancedSession
	inputNames  []string
	outputNames []string
}

func toBytes[T any](data []T) []byte {
	if len(data) == 0 {
		return nil
	}
	var zero T
	size := unsafe.Sizeof(zero)
	length := len(data) * int(size)

	header := (*reflect.SliceHeader)(unsafe.Pointer(&data))

	// Create byte slice
	var res []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&res))
	hdr.Data = header.Data
	hdr.Len = length
	hdr.Cap = length

	return res
}

func (s *cgoSession) run(inputs map[string]*Tensor) (map[string]*Tensor, error) {
	// Prepare Inputs
	inputValues := make([]ort.Value, len(s.inputNames))

	var toDestroy []ort.Value
	defer func() {
		for _, v := range toDestroy {
			v.Destroy()
		}
	}()

	for i, name := range s.inputNames {
		inp, ok := inputs[name]
		if !ok {
			return nil, fmt.Errorf("missing input: %s", name)
		}

		// Convert Shape: internal []int64 -> ort.Shape ([]int64)
		// Since they are compatible, just cast
		shape := ort.Shape(inp.Shape())

		var ortVal ort.Value
		var err error

		switch inp.DataType() {
		case TensorTypeFloat32:
			data := inp.Float32Data()
			if data == nil {
				return nil, fmt.Errorf("input %s has invalid float32 data", name)
			}
			bytes := toBytes(data)
			dtype := ort.GetTensorElementDataType[float32]()
			t, e := ort.NewCustomDataTensor(shape, bytes, ort.TensorElementDataType(dtype))
			if e != nil {
				return nil, e
			}
			ortVal = t

		case TensorTypeInt64:
			data := inp.Int64Data()
			if data == nil {
				return nil, fmt.Errorf("input %s has invalid int64 data", name)
			}
			bytes := toBytes(data)
			dtype := ort.GetTensorElementDataType[int64]()
			t, e := ort.NewCustomDataTensor(shape, bytes, ort.TensorElementDataType(dtype))
			if e != nil {
				return nil, e
			}
			ortVal = t

		case TensorTypeInt32:
			data := inp.Int32Data()
			if data == nil {
				return nil, fmt.Errorf("input %s has invalid int32 data", name)
			}
			bytes := toBytes(data)
			dtype := ort.GetTensorElementDataType[int32]()
			t, e := ort.NewCustomDataTensor(shape, bytes, ort.TensorElementDataType(dtype))
			if e != nil {
				return nil, e
			}
			ortVal = t

		default:
			return nil, fmt.Errorf("unsupported tensor type %v for input %s", inp.DataType(), name)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create ORT tensor for %s: %w", name, err)
		}

		inputValues[i] = ortVal
		toDestroy = append(toDestroy, ortVal)
	}

	// Prepare Outputs
	outputValues := make([]ort.Value, len(s.outputNames))

	// Run
	err := s.session.Run(inputValues, outputValues)
	if err != nil {
		return nil, fmt.Errorf("run failed: %w", err)
	}

	// Ensure outputs are destroyed
	for _, v := range outputValues {
		if v != nil {
			defer v.Destroy()
		}
	}

	// Convert Outputs
	result := make(map[string]*Tensor)

	for i, ortOut := range outputValues {
		if ortOut == nil {
			continue
		}
		name := s.outputNames[i]

		var outTensor *Tensor

		// Type switch and extraction
		switch t := ortOut.(type) {
		case *ort.Tensor[float32]:
			data := t.GetData()
			c := make([]float32, len(data))
			copy(c, data)
			shape := []int64(t.GetShape())
			outTensor = NewTensorFloat32(c, shape)

		case *ort.Tensor[int64]:
			data := t.GetData()
			c := make([]int64, len(data))
			copy(c, data)
			shape := []int64(t.GetShape())
			outTensor = NewTensorInt64(c, shape)

		case *ort.Tensor[int32]:
			data := t.GetData()
			c := make([]int32, len(data))
			copy(c, data)
			shape := []int64(t.GetShape())
			outTensor = NewTensorInt32(c, shape)

		case *ort.Tensor[float64]:
			data := t.GetData()
			shape := []int64(t.GetShape())
			// Convert to float32
			data32 := make([]float32, len(data))
			for k, v := range data {
				data32[k] = float32(v)
			}
			outTensor = NewTensorFloat32(data32, shape)

		default:
			log.Printf("[WARN] Unsupported output tensor type %T for %s", ortOut, name)
			continue
		}

		result[name] = outTensor
	}

	return result, nil
}

func findLibraryPath() string {
	if env := os.Getenv("ONNX_RUNTIME_LIB"); env != "" {
		return env
	}
	dllName := "onnxruntime.dll"
	if runtime.GOOS == "linux" {
		dllName = "libonnxruntime.so"
	} else if runtime.GOOS == "darwin" {
		dllName = "libonnxruntime.dylib"
	}
	if _, err := os.Stat(dllName); err == nil {
		return dllName
	}
	return ""
}

func (s *cgoSession) Close() error {
	return s.session.Destroy()
}
