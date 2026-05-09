package apibind

import (
	"fmt"
	"net/http"
)

// APIError represents an HTTP error response.
// Use errors.Is() to check the status code.
//
// Example:
//
//	_, err := apibind.Call(client, MyAPI, req)
//	if errors.Is(err, apibind.ErrBadRequest) {
//	    // handle 400 Bad Request
//	}
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// Is reports whether the target error matches this APIError.
// A target with StatusCode 0 matches any APIError.
// This enables errors.Is(err, apibind.ErrBadRequest) to work correctly.
func (e *APIError) Is(target error) bool {
	t, ok := target.(*APIError)
	if !ok {
		return false
	}
	return t.StatusCode == 0 || t.StatusCode == e.StatusCode
}

// Sentinel errors for common HTTP status codes.
// Use with errors.Is() to check the error type.
var (
	ErrBadRequest  = &APIError{StatusCode: http.StatusBadRequest}
	ErrNotFound    = &APIError{StatusCode: http.StatusNotFound}
	ErrServerError = &APIError{StatusCode: http.StatusInternalServerError}
)
