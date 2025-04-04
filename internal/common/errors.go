package common

import (
	"fmt"
)

// BatchError represents an error that occurred during batch processing
type BatchError struct {
	BatchIndex int
	Message    string
	Cause      error
}

// Error implements the error interface
func (e *BatchError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("batch %d: %s: %v", e.BatchIndex+1, e.Message, e.Cause)
	}
	return fmt.Sprintf("batch %d: %s", e.BatchIndex+1, e.Message)
}

// Unwrap returns the underlying error
func (e *BatchError) Unwrap() error {
	return e.Cause
}

// NewBatchError creates a new batch error
func NewBatchError(batchIndex int, message string, cause error) *BatchError {
	return &BatchError{
		BatchIndex: batchIndex,
		Message:    message,
		Cause:      cause,
	}
}

// SummarizeBatchErrors provides a summary of batch errors
func SummarizeBatchErrors(errors []error) string {
	if len(errors) == 0 {
		return "No errors"
	}

	if len(errors) == 1 {
		return fmt.Sprintf("1 error: %v", errors[0])
	}

	return fmt.Sprintf("%d errors, first error: %v", len(errors), errors[0])
}

// FormatAPIError formats an API error with an operation description
func FormatAPIError(err error, operation string) error {
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// ClientCreationError is a helper for the common API client creation error
func ClientCreationError(err error) error {
	return fmt.Errorf("failed to create API client: %w", err)
}
