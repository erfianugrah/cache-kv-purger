package common

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu          sync.Mutex
	tokens      float64
	maxTokens   float64
	refillRate  float64 // tokens per second
	lastRefill  time.Time
	waitTimeout time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(ratePerSecond int, burst int, waitTimeout time.Duration) *RateLimiter {
	if ratePerSecond <= 0 {
		ratePerSecond = 100
	}
	if burst <= 0 {
		burst = ratePerSecond * 2
	}
	if waitTimeout <= 0 {
		waitTimeout = 30 * time.Second
	}

	return &RateLimiter{
		tokens:      float64(burst),
		maxTokens:   float64(burst),
		refillRate:  float64(ratePerSecond),
		lastRefill:  time.Now(),
		waitTimeout: waitTimeout,
	}
}

// Wait blocks until a token is available or context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.WaitN(ctx, 1)
}

// WaitN blocks until n tokens are available or context is cancelled
func (rl *RateLimiter) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	// Create a deadline if not already present
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, rl.waitTimeout)
		defer cancel()
		deadline, _ = ctx.Deadline()
	}

	// Try to acquire tokens with exponential backoff
	backoff := 10 * time.Millisecond
	maxBackoff := 1 * time.Second

	for {
		// Check if we can acquire tokens
		if rl.tryAcquire(n) {
			return nil
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if we have time to wait
		if time.Until(deadline) < backoff {
			return fmt.Errorf("rate limit: timeout waiting for %d tokens", n)
		}

		// Wait with backoff
		time.Sleep(backoff)

		// Increase backoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// TryAcquire attempts to acquire a token without blocking
func (rl *RateLimiter) TryAcquire() bool {
	return rl.tryAcquire(1)
}

// TryAcquireN attempts to acquire n tokens without blocking
func (rl *RateLimiter) TryAcquireN(n int) bool {
	return rl.tryAcquire(n)
}

// tryAcquire attempts to acquire n tokens
func (rl *RateLimiter) tryAcquire(n int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	rl.refill()

	// Check if we have enough tokens
	needed := float64(n)
	if rl.tokens >= needed {
		rl.tokens -= needed
		return true
	}

	return false
}

// refill adds tokens based on time elapsed
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// Add tokens based on refill rate
	rl.tokens += elapsed * rl.refillRate

	// Cap at max tokens
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	rl.lastRefill = now
}

// Available returns the number of currently available tokens
func (rl *RateLimiter) Available() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()
	return int(rl.tokens)
}

// SetRate updates the rate limiter's rate
func (rl *RateLimiter) SetRate(ratePerSecond int, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if ratePerSecond > 0 {
		rl.refillRate = float64(ratePerSecond)
	}
	if burst > 0 {
		rl.maxTokens = float64(burst)
		// Don't reduce current tokens, but cap at new max
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
	}
}

// MultiRateLimiter manages multiple rate limiters for different endpoints
type MultiRateLimiter struct {
	mu           sync.RWMutex
	limiters     map[string]*RateLimiter
	defaultRate  int
	defaultBurst int
	waitTimeout  time.Duration
}

// NewMultiRateLimiter creates a new multi-endpoint rate limiter
func NewMultiRateLimiter(defaultRate, defaultBurst int, waitTimeout time.Duration) *MultiRateLimiter {
	return &MultiRateLimiter{
		limiters:     make(map[string]*RateLimiter),
		defaultRate:  defaultRate,
		defaultBurst: defaultBurst,
		waitTimeout:  waitTimeout,
	}
}

// Wait waits for a token for the specified endpoint
func (mrl *MultiRateLimiter) Wait(ctx context.Context, endpoint string) error {
	limiter := mrl.getLimiter(endpoint)
	return limiter.Wait(ctx)
}

// WaitN waits for n tokens for the specified endpoint
func (mrl *MultiRateLimiter) WaitN(ctx context.Context, endpoint string, n int) error {
	limiter := mrl.getLimiter(endpoint)
	return limiter.WaitN(ctx, n)
}

// getLimiter gets or creates a rate limiter for an endpoint
func (mrl *MultiRateLimiter) getLimiter(endpoint string) *RateLimiter {
	mrl.mu.RLock()
	limiter, exists := mrl.limiters[endpoint]
	mrl.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create new limiter
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	// Double-check after acquiring write lock
	limiter, exists = mrl.limiters[endpoint]
	if exists {
		return limiter
	}

	limiter = NewRateLimiter(mrl.defaultRate, mrl.defaultBurst, mrl.waitTimeout)
	mrl.limiters[endpoint] = limiter
	return limiter
}

// SetEndpointRate sets a specific rate for an endpoint
func (mrl *MultiRateLimiter) SetEndpointRate(endpoint string, ratePerSecond, burst int) {
	mrl.mu.Lock()
	defer mrl.mu.Unlock()

	limiter, exists := mrl.limiters[endpoint]
	if !exists {
		limiter = NewRateLimiter(ratePerSecond, burst, mrl.waitTimeout)
		mrl.limiters[endpoint] = limiter
	} else {
		limiter.SetRate(ratePerSecond, burst)
	}
}

// GlobalRateLimiter is a singleton rate limiter for the entire application
var globalRateLimiter = NewMultiRateLimiter(100, 200, 30*time.Second)

// ConfigureGlobalRateLimit sets the default rate limit
func ConfigureGlobalRateLimit(ratePerSecond, burst int) {
	globalRateLimiter.defaultRate = ratePerSecond
	globalRateLimiter.defaultBurst = burst
}

// ConfigureEndpointRateLimit sets rate limit for a specific endpoint
func ConfigureEndpointRateLimit(endpoint string, ratePerSecond, burst int) {
	globalRateLimiter.SetEndpointRate(endpoint, ratePerSecond, burst)
}

// WaitForRateLimit waits for rate limit token for an endpoint
func WaitForRateLimit(ctx context.Context, endpoint string) error {
	return globalRateLimiter.Wait(ctx, endpoint)
}

// Common endpoint constants
const (
	EndpointKVList     = "kv_list"
	EndpointKVGet      = "kv_get"
	EndpointKVPut      = "kv_put"
	EndpointKVDelete   = "kv_delete"
	EndpointKVBulk     = "kv_bulk"
	EndpointKVMetadata = "kv_metadata"
)

// InitializeDefaultRateLimits sets up default rate limits for common endpoints
func InitializeDefaultRateLimits() {
	// These are conservative defaults to avoid rate limiting
	ConfigureEndpointRateLimit(EndpointKVList, 50, 100)      // List operations
	ConfigureEndpointRateLimit(EndpointKVGet, 100, 200)      // Get operations
	ConfigureEndpointRateLimit(EndpointKVPut, 50, 100)       // Write operations
	ConfigureEndpointRateLimit(EndpointKVDelete, 50, 100)    // Delete operations
	ConfigureEndpointRateLimit(EndpointKVBulk, 20, 40)       // Bulk operations
	ConfigureEndpointRateLimit(EndpointKVMetadata, 100, 200) // Metadata operations
}
