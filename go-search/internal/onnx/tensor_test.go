package onnx

import (
	"testing"
)

func TestNewTensorFloat32(t *testing.T) {
	data := []float32{1.0, 2.0, 3.0, 4.0}
	shape := []int64{2, 2}

	tensor := NewTensorFloat32(data, shape)

	if tensor.DataType() != TensorTypeFloat32 {
		t.Errorf("DataType = %v, want TensorTypeFloat32", tensor.DataType())
	}

	if len(tensor.Shape()) != 2 {
		t.Errorf("Shape length = %d, want 2", len(tensor.Shape()))
	}

	got := tensor.Float32Data()
	if len(got) != len(data) {
		t.Errorf("Float32Data length = %d, want %d", len(got), len(data))
	}
}

func TestNewTensorInt64(t *testing.T) {
	data := []int64{1, 2, 3, 4}
	shape := []int64{4}

	tensor := NewTensorInt64(data, shape)

	if tensor.DataType() != TensorTypeInt64 {
		t.Errorf("DataType = %v, want TensorTypeInt64", tensor.DataType())
	}

	got := tensor.Int64Data()
	if len(got) != len(data) {
		t.Errorf("Int64Data length = %d, want %d", len(got), len(data))
	}
}

func TestTensor_NumElements(t *testing.T) {
	tests := []struct {
		shape []int64
		want  int64
	}{
		{[]int64{2, 3}, 6},
		{[]int64{4}, 4},
		{[]int64{2, 3, 4}, 24},
		{[]int64{}, 0},
	}

	for _, tt := range tests {
		tensor := &Tensor{shape: tt.shape}
		got := tensor.NumElements()
		if got != tt.want {
			t.Errorf("NumElements(%v) = %d, want %d", tt.shape, got, tt.want)
		}
	}
}

func TestTensor_Reshape(t *testing.T) {
	data := []float32{1, 2, 3, 4, 5, 6}
	tensor := NewTensorFloat32(data, []int64{2, 3})

	// Valid reshape
	reshaped, err := tensor.Reshape([]int64{3, 2})
	if err != nil {
		t.Fatalf("Reshape error: %v", err)
	}

	if reshaped.Shape()[0] != 3 || reshaped.Shape()[1] != 2 {
		t.Errorf("reshaped shape = %v, want [3, 2]", reshaped.Shape())
	}

	// Invalid reshape (different element count)
	_, err = tensor.Reshape([]int64{2, 2})
	if err == nil {
		t.Error("expected error for invalid reshape")
	}
}

func TestTensor_Clone(t *testing.T) {
	data := []float32{1, 2, 3}
	tensor := NewTensorFloat32(data, []int64{3})

	clone := tensor.Clone()

	// Modify original
	data[0] = 999

	// Clone should be unchanged
	if clone.Float32Data()[0] != 1 {
		t.Error("clone was modified when original changed")
	}
}

func TestZeros(t *testing.T) {
	tensor := Zeros([]int64{2, 3}, TensorTypeFloat32)

	if tensor.NumElements() != 6 {
		t.Errorf("NumElements = %d, want 6", tensor.NumElements())
	}

	data := tensor.Float32Data()
	for i, v := range data {
		if v != 0 {
			t.Errorf("Zeros[%d] = %f, want 0", i, v)
		}
	}
}

func TestOnes(t *testing.T) {
	tensor := Ones([]int64{2, 2})

	data := tensor.Float32Data()
	for i, v := range data {
		if v != 1.0 {
			t.Errorf("Ones[%d] = %f, want 1", i, v)
		}
	}
}

func TestTensor_WrongType(t *testing.T) {
	tensor := NewTensorFloat32([]float32{1, 2, 3}, []int64{3})

	// Getting wrong type should return nil
	if tensor.Int64Data() != nil {
		t.Error("Int64Data on float32 tensor should return nil")
	}

	if tensor.Int32Data() != nil {
		t.Error("Int32Data on float32 tensor should return nil")
	}
}
