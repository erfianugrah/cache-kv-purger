package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"cache-kv-purger/internal/common"
)

// RequestWithRetry makes a request with automatic retry on failure
func (c *Client) RequestWithRetry(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, error) {
	// Create a retry-specific function
	var result []byte
	retryFunc := func() error {
		resp, err := c.RequestWithContext(ctx, method, path, query, body)
		if err != nil {
			return err
		}
		result = resp
		return nil
	}

	// Use custom retry policy for API requests
	policy := &APIRetryPolicy{
		config: &common.RetryConfig{
			MaxAttempts:  5,
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
			Jitter:       0.2,
		},
	}

	err := common.Retry(ctx, retryFunc, policy)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// APIRetryPolicy implements a retry policy specific to API requests
type APIRetryPolicy struct {
	config       *common.RetryConfig
	lastResponse *http.Response
}

// ShouldRetry determines if an API error is retryable
func (p *APIRetryPolicy) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}

	if attempt >= p.config.MaxAttempts {
		return false
	}

	errStr := err.Error()

	// Always retry on rate limit errors
	if contains(errStr, "429") || contains(errStr, "rate limit") {
		return true
	}

	// Retry on server errors
	if contains(errStr, "500") || contains(errStr, "502") ||
		contains(errStr, "503") || contains(errStr, "504") {
		return true
	}

	// Retry on network errors
	if contains(errStr, "timeout") || contains(errStr, "connection refused") ||
		contains(errStr, "EOF") || contains(errStr, "broken pipe") {
		return true
	}

	// Don't retry on client errors (4xx except 429)
	if contains(errStr, "400") || contains(errStr, "401") ||
		contains(errStr, "403") || contains(errStr, "404") {
		return false
	}

	return false
}

// NextDelay calculates the next retry delay
func (p *APIRetryPolicy) NextDelay(attempt int) time.Duration {
	// Check if we have a Retry-After header from a 429 response
	if p.lastResponse != nil && p.lastResponse.StatusCode == 429 {
		if retryAfter := p.lastResponse.Header.Get("Retry-After"); retryAfter != "" {
			// Try to parse as seconds
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				return time.Duration(seconds) * time.Second
			}
			// Try to parse as HTTP date
			if t, err := http.ParseTime(retryAfter); err == nil {
				return time.Until(t)
			}
		}
	}

	// Use standard exponential backoff
	standardPolicy := common.NewStandardRetryPolicy(p.config)
	return standardPolicy.NextDelay(attempt)
}

// RequestBatchWithRetry makes multiple requests with retry and manages concurrency
func (c *Client) RequestBatchWithRetry(ctx context.Context, requests []BatchRequest, concurrency int) ([]BatchResponse, error) {
	if concurrency <= 0 {
		concurrency = 10
	}

	responses := make([]BatchResponse, len(requests))
	semaphore := make(chan struct{}, concurrency)

	// Process requests concurrently
	errChan := make(chan error, len(requests))
	for i, req := range requests {
		semaphore <- struct{}{} // Acquire semaphore

		go func(index int, request BatchRequest) {
			defer func() { <-semaphore }() // Release semaphore

			// Make request with retry
			resp, err := c.RequestWithRetry(ctx, request.Method, request.Path, request.Query, request.Body)

			responses[index] = BatchResponse{
				Index:    index,
				Response: resp,
				Error:    err,
			}

			if err != nil {
				errChan <- err
			}
		}(i, req)
	}

	// Wait for all requests to complete
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- struct{}{}
	}

	// Check for errors
	close(errChan)
	var firstError error
	errorCount := 0
	for err := range errChan {
		if firstError == nil {
			firstError = err
		}
		errorCount++
	}

	if errorCount > 0 {
		return responses, fmt.Errorf("%d/%d requests failed, first error: %w", errorCount, len(requests), firstError)
	}

	return responses, nil
}

// BatchRequest represents a batch request
type BatchRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   interface{}
}

// BatchResponse represents a batch response
type BatchResponse struct {
	Index    int
	Response []byte
	Error    error
}

// RetryableKVOperation wraps common KV operations with retry logic
func RetryableKVOperation(ctx context.Context, operation string, fn func() error) error {
	// Use operation-specific circuit breaker
	return common.RetryOperation(ctx, "kv_"+operation, fn)
}

// Helper function (duplicate from main client, but needed here)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(stringContains(toLowerCase(s), toLowerCase(substr))))
}

func toLowerCase(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
