// Package errors provides custom error types and error handling utilities.
package errors

import (
	"encoding/json"
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

// IndexingError creates an indexing error.
func IndexingError(message string, err error) *AppError {
	return Wrap(CodeIndexingError, message, err)
}

// InvalidRequestError creates an invalid request error.
func InvalidRequestError(message string) *AppError {
	return New(CodeInvalidRequest, message)
}

// UnauthorizedError creates an unauthorized error.
func UnauthorizedError() *AppError {
	return New(CodeUnauthorized, "unauthorized")
}

// ForbiddenError creates a forbidden error.
func ForbiddenError(message string) *AppError {
	if message == "" {
		message = "access denied"
	}
	return New(CodeForbidden, message)
}

// RateLimitedError creates a rate limited error with retry information.
func RateLimitedError(retryAfterSeconds int) *AppError {
	err := New(CodeRateLimited, "rate limit exceeded")
	if retryAfterSeconds > 0 {
		err = err.WithDetail("retry_after", fmt.Sprintf("%d", retryAfterSeconds))
	}
	return err
}

// TimeoutError creates a timeout error for a specific operation.
func TimeoutError(operation string) *AppError {
	message := "operation timed out"
	if operation != "" {
		message = fmt.Sprintf("%s timed out", operation)
	}
	return New(CodeTimeout, message)
}

// ServiceUnavailableError creates a service unavailable error.
func ServiceUnavailableError(service string) *AppError {
	message := "service unavailable"
	if service != "" {
		message = fmt.Sprintf("%s is unavailable", service)
	}
	return New(CodeUnavailable, message)
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

// ErrorResponse is the standard JSON error response structure.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Code    string            `json:"code"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// WriteJSON writes a JSON error response to the ResponseWriter.
// This is the low-level function used by WriteError.
func WriteJSON(w http.ResponseWriter, status int, resp ErrorResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Ignore encoding errors - headers already sent
	_ = json.NewEncoder(w).Encode(resp)
}

// WriteError writes an error response with proper sanitization.
// If err is an *AppError, it uses the code and status from the error.
// For other errors, it sanitizes the message to prevent leaking internal details.
func WriteError(w http.ResponseWriter, err error) {
	// Check if it's an AppError
	if appErr, ok := err.(*AppError); ok {
		WriteJSON(w, appErr.HTTPStatus(), ErrorResponse{
			Error:   appErr.Message,
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		})
		return
	}

	// For non-AppError errors, sanitize the message
	// Don't leak internal error details to clients
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{
		Error:   "internal server error",
		Code:    CodeInternal,
		Message: "An unexpected error occurred",
	})
}

// WriteErrorWithStatus writes an error with a specific HTTP status code.
// The error message is sanitized based on the status code:
// - 4xx errors: message is shown to client
// - 5xx errors: message is sanitized (internal details hidden)
func WriteErrorWithStatus(w http.ResponseWriter, status int, err error) {
	// Check if it's an AppError - use its code
	if appErr, ok := err.(*AppError); ok {
		WriteJSON(w, status, ErrorResponse{
			Error:   appErr.Message,
			Code:    appErr.Code,
			Message: appErr.Message,
			Details: appErr.Details,
		})
		return
	}

	// For 4xx errors, we can show the message (client error)
	if status >= 400 && status < 500 {
		code := codeForStatus(status)
		WriteJSON(w, status, ErrorResponse{
			Error:   err.Error(),
			Code:    code,
			Message: err.Error(),
		})
		return
	}

	// For 5xx errors, sanitize the message
	WriteJSON(w, status, ErrorResponse{
		Error:   "internal server error",
		Code:    CodeInternal,
		Message: "An unexpected error occurred",
	})
}

// codeForStatus returns an error code for common HTTP status codes.
func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeInvalidRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeAlreadyExists
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusServiceUnavailable:
		return CodeUnavailable
	case http.StatusGatewayTimeout:
		return CodeTimeout
	default:
		return CodeInternal
	}
}
