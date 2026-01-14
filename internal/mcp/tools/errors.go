package tools

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/usestring/powhttp-mcp/pkg/client"
)

// Error codes for MCP tool responses.
const (
	ErrCodeNotFound     = "NOT_FOUND"
	ErrCodePowHTTPError = "POWHTTP_ERROR"
	ErrCodeInvalidInput = "INVALID_INPUT"
	ErrCodeTimeout      = "TIMEOUT"
)

// CodedError is an error with an associated error code.
type CodedError struct {
	Code    string
	Message string
	Cause   error
}

func (e *CodedError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *CodedError) Unwrap() error {
	return e.Cause
}

// WrapPowHTTPError converts a client.APIError or other error to a coded error.
func WrapPowHTTPError(err error) error {
	if err == nil {
		return nil
	}

	var coded *CodedError

	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == 404 {
			coded = &CodedError{
				Code:    ErrCodeNotFound,
				Message: apiErr.Message,
				Cause:   err,
			}
		} else {
			coded = &CodedError{
				Code:    ErrCodePowHTTPError,
				Message: apiErr.Message,
				Cause:   err,
			}
		}
	} else {
		// Check for timeout errors
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			coded = &CodedError{
				Code:    ErrCodeTimeout,
				Message: "request timed out",
				Cause:   err,
			}
		} else if strings.Contains(err.Error(), "context deadline exceeded") {
			coded = &CodedError{
				Code:    ErrCodeTimeout,
				Message: "request timed out",
				Cause:   err,
			}
		} else {
			coded = &CodedError{
				Code:    ErrCodePowHTTPError,
				Message: err.Error(),
				Cause:   err,
			}
		}
	}

	slog.Warn("powhttp API error",
		slog.String("code", coded.Code),
		slog.String("message", coded.Message),
	)

	return coded
}

// ErrNotFound creates a not found error.
func ErrNotFound(resource, id string) error {
	return &CodedError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found: %s", resource, id),
	}
}

// ErrInvalidInput creates an invalid input error.
func ErrInvalidInput(message string) error {
	return &CodedError{
		Code:    ErrCodeInvalidInput,
		Message: message,
	}
}
