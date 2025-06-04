package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"cache-kv-purger/internal/auth"
)

// OptimizedClient represents an optimized Cloudflare API client with connection pooling
type OptimizedClient struct {
	BaseURL    string
	HTTPClient *http.Client
	Creds      *auth.CredentialInfo
	
	// Performance optimizations
	transport *http.Transport
	mu        sync.RWMutex
}

// OptimizedClientOption is a function that configures an OptimizedClient
type OptimizedClientOption func(*OptimizedClient)

// WithOptimizedBaseURL sets the base URL for the API client
func WithOptimizedBaseURL(baseURL string) OptimizedClientOption {
	return func(c *OptimizedClient) {
		c.BaseURL = baseURL
	}
}

// WithOptimizedTimeout sets the HTTP client timeout
func WithOptimizedTimeout(timeout time.Duration) OptimizedClientOption {
	return func(c *OptimizedClient) {
		c.HTTPClient.Timeout = timeout
	}
}

// WithOptimizedCredentials sets the authentication credentials
func WithOptimizedCredentials(creds *auth.CredentialInfo) OptimizedClientOption {
	return func(c *OptimizedClient) {
		c.Creds = creds
	}
}

// NewOptimizedClient creates a new optimized Cloudflare API client
func NewOptimizedClient(options ...OptimizedClientOption) (*OptimizedClient, error) {
	// Create optimized transport with connection pooling
	transport := &http.Transport{
		// Connection pool settings
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
		
		// Performance settings
		DisableCompression: false,
		DisableKeepAlives:  false,
		ForceAttemptHTTP2:  true,
		
		// Timeouts
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		
		// Buffer settings
		WriteBufferSize: 65536, // 64KB
		ReadBufferSize:  65536, // 64KB
	}
	
	// Create client with optimized settings
	client := &OptimizedClient{
		BaseURL:   "https://api.cloudflare.com/client/v4",
		transport: transport,
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second, // Reduced from 300s for better responsiveness
		},
	}
	
	// Apply options
	for _, option := range options {
		option(client)
	}
	
	// If no credentials are provided, try to get them from environment
	if client.Creds == nil {
		creds, err := auth.GetCredentials()
		if err != nil {
			return nil, err
		}
		client.Creds = creds
	}
	
	return client, nil
}

// Request makes a request to the Cloudflare API with retries and better error handling
func (c *OptimizedClient) Request(method, path string, query url.Values, body interface{}) ([]byte, error) {
	return c.RequestWithContext(context.Background(), method, path, query, body)
}

// RequestWithContext makes a request with context support for cancellation
func (c *OptimizedClient) RequestWithContext(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, error) {
	// Build URL
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	
	// Add query parameters if provided
	if query != nil {
		u.RawQuery = query.Encode()
	}
	
	// Create request body if provided
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}
	
	// Implement retry logic with exponential backoff
	maxRetries := 3
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		
		// Create request with context
		req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
		if err != nil {
			return nil, err
		}
		
		// Set headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Connection", "keep-alive")
		
		// Set authentication
		if c.Creds != nil {
			switch c.Creds.Type {
			case auth.AuthTypeAPIKey:
				req.Header.Set("X-Auth-Key", c.Creds.Key)
				req.Header.Set("X-Auth-Email", c.Creds.Email)
			case auth.AuthTypeAPIToken:
				req.Header.Set("Authorization", "Bearer "+c.Creds.Key)
			}
		}
		
		// Make request
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			// Retry on network errors
			if isRetryableError(err) && attempt < maxRetries {
				continue
			}
			return nil, err
		}
		defer resp.Body.Close()
		
		// Read response body
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}
		
		// Check for rate limiting
		if resp.StatusCode == 429 && attempt < maxRetries {
			// Get retry-after header
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
					select {
					case <-time.After(seconds):
					case <-ctx.Done():
						return nil, ctx.Err()
					}
					continue
				}
			}
			// Default retry after 5 seconds for rate limit
			time.Sleep(5 * time.Second)
			continue
		}
		
		// Check for server errors that might be temporary
		if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
			lastErr = fmt.Errorf("server error (HTTP %d)", resp.StatusCode)
			continue
		}
		
		// Check for client errors
		if resp.StatusCode >= 400 {
			errorMsg := string(respBody)
			
			// Check if this might be a token scope issue
			if resp.StatusCode == 403 && c.Creds.Type == auth.AuthTypeAPIToken {
				if scopeHint := auth.CheckTokenScope(errorMsg); scopeHint != "" {
					return nil, fmt.Errorf("%s (HTTP %d): %s", errorMsg, resp.StatusCode, scopeHint)
				}
			}
			
			return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, errorMsg)
		}
		
		// Success
		return respBody, nil
	}
	
	return nil, fmt.Errorf("request failed after %d retries: %v", maxRetries, lastErr)
}

// GetRawResponse returns the raw HTTP response for streaming
func (c *OptimizedClient) GetRawResponse(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	// Build URL
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	
	// Add query parameters if provided
	if query != nil {
		u.RawQuery = query.Encode()
	}
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Connection", "keep-alive")
	
	// Set authentication
	if c.Creds != nil {
		switch c.Creds.Type {
		case auth.AuthTypeAPIKey:
			req.Header.Set("X-Auth-Key", c.Creds.Key)
			req.Header.Set("X-Auth-Email", c.Creds.Email)
		case auth.AuthTypeAPIToken:
			req.Header.Set("Authorization", "Bearer "+c.Creds.Key)
		}
	}
	
	// Make request
	return c.HTTPClient.Do(req)
}

// Close closes idle connections
func (c *OptimizedClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
}

// isRetryableError checks if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for network errors
	if netErr, ok := err.(net.Error); ok {
		return netErr.Temporary() || netErr.Timeout()
	}
	
	// Check for context errors
	if err == context.DeadlineExceeded {
		return true
	}
	
	return false
}

// ConvertToOptimizedClient converts a regular Client to an OptimizedClient
func ConvertToOptimizedClient(c *Client) *OptimizedClient {
	// Create optimized transport
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		ForceAttemptHTTP2:   true,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		WriteBufferSize:       65536,
		ReadBufferSize:        65536,
	}
	
	return &OptimizedClient{
		BaseURL:   c.BaseURL,
		Creds:     c.Creds,
		transport: transport,
		HTTPClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}