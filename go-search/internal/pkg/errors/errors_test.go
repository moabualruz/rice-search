package errors

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *AppError
		want string
	}{
		{
			name: "without wrapped error",
			err:  New(CodeValidation, "invalid input"),
			want: "VALIDATION_ERROR: invalid input",
		},
		{
			name: "with wrapped error",
			err:  Wrap(CodeInternal, "something failed", errors.New("underlying")),
			want: "INTERNAL_ERROR: something failed: underlying",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	underlying := errors.New("underlying error")
	err := Wrap(CodeInternal, "wrapped", underlying)

	if unwrapped := err.Unwrap(); unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		code   string
		status int
	}{
		{CodeValidation, http.StatusBadRequest},
		{CodeInvalidRequest, http.StatusBadRequest},
		{CodeNotFound, http.StatusNotFound},
		{CodeAlreadyExists, http.StatusConflict},
		{CodeUnauthorized, http.StatusUnauthorized},
		{CodeForbidden, http.StatusForbidden},
		{CodeRateLimited, http.StatusTooManyRequests},
		{CodeUnavailable, http.StatusServiceUnavailable},
		{CodeTimeout, http.StatusGatewayTimeout},
		{CodeInternal, http.StatusInternalServerError},
		{CodeMLError, http.StatusInternalServerError},
		{CodeQdrantError, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := New(tt.code, "test")
			if status := err.HTTPStatus(); status != tt.status {
				t.Errorf("HTTPStatus() = %d, want %d", status, tt.status)
			}
		})
	}
}

func TestAppError_WithDetails(t *testing.T) {
	err := New(CodeValidation, "invalid").
		WithDetails(map[string]string{"field": "name"})

	if err.Details["field"] != "name" {
		t.Errorf("Details[field] = %s, want name", err.Details["field"])
	}
}

func TestAppError_WithDetail(t *testing.T) {
	err := New(CodeValidation, "invalid").
		WithDetail("field", "name").
		WithDetail("reason", "required")

	if err.Details["field"] != "name" {
		t.Errorf("Details[field] = %s, want name", err.Details["field"])
	}

	if err.Details["reason"] != "required" {
		t.Errorf("Details[reason] = %s, want required", err.Details["reason"])
	}
}

func TestConvenienceConstructors(t *testing.T) {
	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError("bad input")
		if err.Code != CodeValidation {
			t.Errorf("Code = %s, want %s", err.Code, CodeValidation)
		}
	})

	t.Run("NotFoundError", func(t *testing.T) {
		err := NotFoundError("user")
		if err.Code != CodeNotFound {
			t.Errorf("Code = %s, want %s", err.Code, CodeNotFound)
		}
		if err.Message != "user not found" {
			t.Errorf("Message = %s, want 'user not found'", err.Message)
		}
	})

	t.Run("AlreadyExistsError", func(t *testing.T) {
		err := AlreadyExistsError("store")
		if err.Code != CodeAlreadyExists {
			t.Errorf("Code = %s, want %s", err.Code, CodeAlreadyExists)
		}
	})

	t.Run("InternalError", func(t *testing.T) {
		underlying := errors.New("db error")
		err := InternalError("failed", underlying)
		if err.Code != CodeInternal {
			t.Errorf("Code = %s, want %s", err.Code, CodeInternal)
		}
		if err.Unwrap() != underlying {
			t.Error("Underlying error not preserved")
		}
	})

	t.Run("MLError", func(t *testing.T) {
		err := MLError("inference failed", errors.New("oom"))
		if err.Code != CodeMLError {
			t.Errorf("Code = %s, want %s", err.Code, CodeMLError)
		}
	})

	t.Run("QdrantError", func(t *testing.T) {
		err := QdrantError("connection failed", errors.New("timeout"))
		if err.Code != CodeQdrantError {
			t.Errorf("Code = %s, want %s", err.Code, CodeQdrantError)
		}
	})
}

func TestIsNotFound(t *testing.T) {
	notFound := NotFoundError("test")
	other := ValidationError("test")

	if !IsNotFound(notFound) {
		t.Error("IsNotFound(NotFoundError) = false, want true")
	}

	if IsNotFound(other) {
		t.Error("IsNotFound(ValidationError) = true, want false")
	}

	if IsNotFound(errors.New("standard error")) {
		t.Error("IsNotFound(standard error) = true, want false")
	}
}

func TestIsValidation(t *testing.T) {
	validation := ValidationError("test")
	other := NotFoundError("test")

	if !IsValidation(validation) {
		t.Error("IsValidation(ValidationError) = false, want true")
	}

	if IsValidation(other) {
		t.Error("IsValidation(NotFoundError) = true, want false")
	}
}
