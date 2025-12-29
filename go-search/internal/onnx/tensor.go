package onnx

import (
	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// Tensor represents a multi-dimensional array for ONNX operations.
type Tensor struct {
	shape    []int64
	dataType TensorType
	data     any // []float32, []int64, etc.
}

// NewTensorFloat32 creates a new float32 tensor.
func NewTensorFloat32(data []float32, shape []int64) *Tensor {
	return &Tensor{
		shape:    shape,
		dataType: TensorTypeFloat32,
		data:     data,
	}
}

// NewTensorInt64 creates a new int64 tensor.
func NewTensorInt64(data []int64, shape []int64) *Tensor {
	return &Tensor{
		shape:    shape,
		dataType: TensorTypeInt64,
		data:     data,
	}
}

// NewTensorInt32 creates a new int32 tensor.
func NewTensorInt32(data []int32, shape []int64) *Tensor {
	return &Tensor{
		shape:    shape,
		dataType: TensorTypeInt32,
		data:     data,
	}
}

// Shape returns the tensor shape.
func (t *Tensor) Shape() []int64 {
	return t.shape
}

// DataType returns the tensor data type.
func (t *Tensor) DataType() TensorType {
	return t.dataType
}

// Float32Data returns the data as float32 slice.
func (t *Tensor) Float32Data() []float32 {
	if data, ok := t.data.([]float32); ok {
		return data
	}
	return nil
}

// Int64Data returns the data as int64 slice.
func (t *Tensor) Int64Data() []int64 {
	if data, ok := t.data.([]int64); ok {
		return data
	}
	return nil
}

// Int32Data returns the data as int32 slice.
func (t *Tensor) Int32Data() []int32 {
	if data, ok := t.data.([]int32); ok {
		return data
	}
	return nil
}

// NumElements returns the total number of elements.
func (t *Tensor) NumElements() int64 {
	if len(t.shape) == 0 {
		return 0
	}

	n := int64(1)
	for _, dim := range t.shape {
		n *= dim
	}
	return n
}

// Reshape returns a new tensor with a different shape.
func (t *Tensor) Reshape(newShape []int64) (*Tensor, error) {
	// Verify element count matches
	oldN := t.NumElements()
	newN := int64(1)
	for _, dim := range newShape {
		newN *= dim
	}

	if oldN != newN {
		return nil, errors.ValidationError("reshape element count mismatch")
	}

	return &Tensor{
		shape:    newShape,
		dataType: t.dataType,
		data:     t.data,
	}, nil
}

// Clone creates a deep copy of the tensor.
func (t *Tensor) Clone() *Tensor {
	var dataCopy any

	switch data := t.data.(type) {
	case []float32:
		c := make([]float32, len(data))
		copy(c, data)
		dataCopy = c
	case []int64:
		c := make([]int64, len(data))
		copy(c, data)
		dataCopy = c
	case []int32:
		c := make([]int32, len(data))
		copy(c, data)
		dataCopy = c
	}

	shapeCopy := make([]int64, len(t.shape))
	copy(shapeCopy, t.shape)

	return &Tensor{
		shape:    shapeCopy,
		dataType: t.dataType,
		data:     dataCopy,
	}
}

// Zeros creates a zero-filled tensor.
func Zeros(shape []int64, dtype TensorType) *Tensor {
	n := int64(1)
	for _, dim := range shape {
		n *= dim
	}

	switch dtype {
	case TensorTypeFloat32:
		return NewTensorFloat32(make([]float32, n), shape)
	case TensorTypeInt64:
		return NewTensorInt64(make([]int64, n), shape)
	case TensorTypeInt32:
		return NewTensorInt32(make([]int32, n), shape)
	default:
		return &Tensor{shape: shape, dataType: dtype}
	}
}

// Ones creates a one-filled float32 tensor.
func Ones(shape []int64) *Tensor {
	n := int64(1)
	for _, dim := range shape {
		n *= dim
	}

	data := make([]float32, n)
	for i := range data {
		data[i] = 1.0
	}

	return NewTensorFloat32(data, shape)
}
