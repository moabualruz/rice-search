// Package errors provides custom error types and error handling utilities.
package errors

import (
	"fmt"
	"net/http"
)

// Error codes.
const (
	// Client errors (4xx).
	CodeValidation     = "VALIDATION_ERROR"
	CodeNotFound       = "NOT_FOUND"
	CodeAlreadyExists  = "ALREADY_EXISTS"
	CodeUnauthorized   = "UNAUTHORIZED"
	CodeForbidden      = "FORBIDDEN"
	CodeRateLimited    = "RATE_LIMITED"
	CodeInvalidRequest = "INVALID_REQUEST"

	// Server errors (5xx).
	CodeInternal      = "INTERNAL_ERROR"
	CodeUnavailable   = "SERVICE_UNAVAILABLE"
	CodeTimeout       = "TIMEOUT"
	CodeMLError       = "ML_ERROR"
	CodeQdrantError   = "QDRANT_ERROR"
	CodeIndexingError = "INDEXING_ERROR"
)

// AppError represents an application error with code and details.
type AppError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
	Err     error             `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error.
func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus returns the HTTP status code for this error.
func (e *AppError) HTTPStatus() int {
	switch e.Code {
	case CodeValidation, CodeInvalidRequest:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists:
		return http.StatusConflict
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// New creates a new AppError.
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with an AppError.
func Wrap(code, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// WithDetails adds details to the error.
func (e *AppError) WithDetails(details map[string]string) *AppError {
	e.Details = details
	return e
}

// WithDetail adds a single detail to the error.
func (e *AppError) WithDetail(key, value string) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]string)
	}
	e.Details[key] = value
	return e
}

// Convenience constructors.

// ValidationError creates a validation error.
func ValidationError(message string) *AppError {
	return New(CodeValidation, message)
}

// NotFoundError creates a not found error.
func NotFoundError(resource string) *AppError {
	return New(CodeNotFound, fmt.Sprintf("%s not found", resource))
}

// AlreadyExistsError creates an already exists error.
func AlreadyExistsError(resource string) *AppError {
	return New(CodeAlreadyExists, fmt.Sprintf("%s already exists", resource))
}

// InternalError creates an internal error.
func InternalError(message string, err error) *AppError {
	return Wrap(CodeInternal, message, err)
}

// MLError creates an ML service error.
func MLError(message string, err error) *AppError {
	return Wrap(CodeMLError, message, err)
}

// QdrantError creates a Qdrant error.
func QdrantError(message string, err error) *AppError {
	return Wrap(CodeQdrantError, message, err)
}

// IsNotFound checks if error is a not found error.
func IsNotFound(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == CodeNotFound
	}
	return false
}

// IsValidation checks if error is a validation error.
func IsValidation(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == CodeValidation
	}
	return false
}
