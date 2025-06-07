package common

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	Jitter          float64
	RetryableErrors []string // Error messages that should trigger retry
}

// DefaultRetryConfig returns sensible retry defaults
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableErrors: []string{
			"429",                // Rate limit
			"500",                // Server error
			"502",                // Bad gateway
			"503",                // Service unavailable
			"504",                // Gateway timeout
			"timeout",            // Request timeout
			"connection refused", // Connection issues
			"EOF",                // Connection closed
			"context deadline",   // Timeout
		},
	}
}

// RetryPolicy defines when to retry
type RetryPolicy interface {
	ShouldRetry(err error, attempt int) bool
	NextDelay(attempt int) time.Duration
}

// StandardRetryPolicy implements a standard retry policy
type StandardRetryPolicy struct {
	config *RetryConfig
}

// NewStandardRetryPolicy creates a new standard retry policy
func NewStandardRetryPolicy(config *RetryConfig) *StandardRetryPolicy {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &StandardRetryPolicy{config: config}
}

// ShouldRetry determines if an error is retryable
func (p *StandardRetryPolicy) ShouldRetry(err error, attempt int) bool {
	if err == nil {
		return false
	}

	if attempt >= p.config.MaxAttempts {
		return false
	}

	// Check if error is retryable
	errStr := strings.ToLower(err.Error())
	for _, retryable := range p.config.RetryableErrors {
		if strings.Contains(errStr, strings.ToLower(retryable)) {
			return true
		}
	}

	// Check for context errors
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false // Don't retry context cancellations
	}

	return false
}

// NextDelay calculates the next retry delay with exponential backoff and jitter
func (p *StandardRetryPolicy) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return p.config.InitialDelay
	}

	// Calculate exponential backoff
	delay := float64(p.config.InitialDelay) * math.Pow(p.config.Multiplier, float64(attempt-1))

	// Apply max delay cap
	if delay > float64(p.config.MaxDelay) {
		delay = float64(p.config.MaxDelay)
	}

	// Apply jitter
	if p.config.Jitter > 0 {
		jitter := delay * p.config.Jitter
		// Add random jitter between -jitter and +jitter
		delay = delay + (rand.Float64()*2-1)*jitter
	}

	return time.Duration(delay)
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// Retry executes a function with retry logic
func Retry(ctx context.Context, fn RetryableFunc, policy RetryPolicy) error {
	if policy == nil {
		policy = NewStandardRetryPolicy(nil)
	}

	for attempt := 1; ; attempt++ {
		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		// Check if we should retry
		if !policy.ShouldRetry(err, attempt) {
			return fmt.Errorf("after %d attempts: %w", attempt, err)
		}

		// Calculate delay
		delay := policy.NextDelay(attempt)

		// Wait with context
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled after %d attempts: %w", attempt, ctx.Err())
		}
	}
}

// RetryWithResult executes a function that returns a result with retry logic
func RetryWithResult[T any](ctx context.Context, fn func() (T, error), policy RetryPolicy) (T, error) {
	if policy == nil {
		policy = NewStandardRetryPolicy(nil)
	}

	var zero T

	for attempt := 1; ; attempt++ {
		// Execute the function
		result, err := fn()
		if err == nil {
			return result, nil
		}

		// Check if we should retry
		if !policy.ShouldRetry(err, attempt) {
			return zero, fmt.Errorf("after %d attempts: %w", attempt, err)
		}

		// Calculate delay
		delay := policy.NextDelay(attempt)

		// Wait with context
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled after %d attempts: %w", attempt, ctx.Err())
		}
	}
}

// CircuitBreaker implements a circuit breaker pattern
type CircuitBreaker struct {
	name            string
	maxFailures     int
	resetTimeout    time.Duration
	halfOpenTimeout time.Duration

	failures     int
	lastFailTime time.Time
	state        CircuitState
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:            name,
		maxFailures:     maxFailures,
		resetTimeout:    resetTimeout,
		halfOpenTimeout: resetTimeout / 2,
		state:           CircuitClosed,
	}
}

// Execute runs a function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	// Check circuit state
	switch cb.getState() {
	case CircuitOpen:
		return fmt.Errorf("circuit breaker '%s' is open", cb.name)
	case CircuitHalfOpen:
		// Allow one request through
		err := fn()
		if err == nil {
			cb.onSuccess()
		} else {
			cb.onFailure()
		}
		return err
	default: // CircuitClosed
		err := fn()
		if err != nil {
			cb.onFailure()
		} else {
			cb.onSuccess()
		}
		return err
	}
}

// getState returns the current circuit state
func (cb *CircuitBreaker) getState() CircuitState {
	if cb.state == CircuitOpen {
		// Check if we should transition to half-open
		if time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
		}
	}
	return cb.state
}

// onSuccess handles successful execution
func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0
	cb.state = CircuitClosed
}

// onFailure handles failed execution
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// RetryManager combines retry logic with circuit breakers
type RetryManager struct {
	retryPolicy     RetryPolicy
	circuitBreakers map[string]*CircuitBreaker
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config *RetryConfig) *RetryManager {
	return &RetryManager{
		retryPolicy:     NewStandardRetryPolicy(config),
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// ExecuteWithRetry executes a function with retry and circuit breaker
func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, name string, fn func() error) error {
	// Get or create circuit breaker
	cb, exists := rm.circuitBreakers[name]
	if !exists {
		cb = NewCircuitBreaker(name, 5, 30*time.Second)
		rm.circuitBreakers[name] = cb
	}

	// Execute with retry through circuit breaker
	return Retry(ctx, func() error {
		return cb.Execute(fn)
	}, rm.retryPolicy)
}

// GlobalRetryManager is a singleton retry manager
var globalRetryManager = NewRetryManager(DefaultRetryConfig())

// SetGlobalRetryConfig updates the global retry configuration
func SetGlobalRetryConfig(config *RetryConfig) {
	globalRetryManager.retryPolicy = NewStandardRetryPolicy(config)
}

// RetryOperation retries an operation using the global retry manager
func RetryOperation(ctx context.Context, name string, fn func() error) error {
	return globalRetryManager.ExecuteWithRetry(ctx, name, fn)
}
