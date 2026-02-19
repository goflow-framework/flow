package flow

import (
	"fmt"
	"net/http"
)

// HTTPError represents a rich HTTP-aware error that handlers can return.
type HTTPError struct {
	StatusCode int    // HTTP status code to write
	Code       string // optional machine-readable error code
	Message    string // user-facing message
	Err        error  // underlying wrapped error
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap makes errors.Is/As work with HTTPError.
func (e *HTTPError) Unwrap() error { return e.Err }

// NewHTTPError constructs an HTTPError.
func NewHTTPError(status int, code, message string, err error) *HTTPError {
	return &HTTPError{StatusCode: status, Code: code, Message: message, Err: err}
}

// DefaultErrorHandler writes an HTTP response for the provided error.
// If verbose is true, the underlying error details are included in responses
// (useful for non-production environments).
func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error, verbose bool) {
	if err == nil {
		return
	}
	// If the error is an HTTPError, use its metadata.
	if he, ok := err.(*HTTPError); ok {
		msg := he.Message
		if verbose && he.Err != nil {
			msg = fmt.Sprintf("%s: %v", he.Message, he.Err)
		}
		http.Error(w, msg, he.StatusCode)
		return
	}
	// Unknown error: avoid leaking internals unless verbose.
	if verbose {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
