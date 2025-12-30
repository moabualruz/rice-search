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

	t.Run("IndexingError", func(t *testing.T) {
		err := IndexingError("chunking failed", errors.New("too large"))
		if err.Code != CodeIndexingError {
			t.Errorf("Code = %s, want %s", err.Code, CodeIndexingError)
		}
	})

	t.Run("InvalidRequestError", func(t *testing.T) {
		err := InvalidRequestError("missing required field")
		if err.Code != CodeInvalidRequest {
			t.Errorf("Code = %s, want %s", err.Code, CodeInvalidRequest)
		}
	})

	t.Run("UnauthorizedError", func(t *testing.T) {
		err := UnauthorizedError()
		if err.Code != CodeUnauthorized {
			t.Errorf("Code = %s, want %s", err.Code, CodeUnauthorized)
		}
		if err.Message != "unauthorized" {
			t.Errorf("Message = %s, want 'unauthorized'", err.Message)
		}
	})

	t.Run("ForbiddenError", func(t *testing.T) {
		err := ForbiddenError("no access to store")
		if err.Code != CodeForbidden {
			t.Errorf("Code = %s, want %s", err.Code, CodeForbidden)
		}
		// Test default message
		errDefault := ForbiddenError("")
		if errDefault.Message != "access denied" {
			t.Errorf("Default message = %s, want 'access denied'", errDefault.Message)
		}
	})

	t.Run("RateLimitedError", func(t *testing.T) {
		err := RateLimitedError(60)
		if err.Code != CodeRateLimited {
			t.Errorf("Code = %s, want %s", err.Code, CodeRateLimited)
		}
		if err.Details["retry_after"] != "60" {
			t.Errorf("Details[retry_after] = %s, want '60'", err.Details["retry_after"])
		}
		// Test without retry
		errNoRetry := RateLimitedError(0)
		if errNoRetry.Details != nil && errNoRetry.Details["retry_after"] != "" {
			t.Errorf("Should not have retry_after with 0")
		}
	})

	t.Run("TimeoutError", func(t *testing.T) {
		err := TimeoutError("search")
		if err.Code != CodeTimeout {
			t.Errorf("Code = %s, want %s", err.Code, CodeTimeout)
		}
		if err.Message != "search timed out" {
			t.Errorf("Message = %s, want 'search timed out'", err.Message)
		}
		// Test default message
		errDefault := TimeoutError("")
		if errDefault.Message != "operation timed out" {
			t.Errorf("Default message = %s, want 'operation timed out'", errDefault.Message)
		}
	})

	t.Run("ServiceUnavailableError", func(t *testing.T) {
		err := ServiceUnavailableError("Qdrant")
		if err.Code != CodeUnavailable {
			t.Errorf("Code = %s, want %s", err.Code, CodeUnavailable)
		}
		if err.Message != "Qdrant is unavailable" {
			t.Errorf("Message = %s, want 'Qdrant is unavailable'", err.Message)
		}
		// Test default message
		errDefault := ServiceUnavailableError("")
		if errDefault.Message != "service unavailable" {
			t.Errorf("Default message = %s, want 'service unavailable'", errDefault.Message)
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
