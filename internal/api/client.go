package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cache-kv-purger/internal/auth"
	"cache-kv-purger/internal/common"
)

// Client represents a Cloudflare API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Creds      *auth.CredentialInfo
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the API client
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

// WithTimeout sets the HTTP client timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.HTTPClient.Timeout = timeout
	}
}

// WithCredentials sets the authentication credentials
func WithCredentials(creds *auth.CredentialInfo) ClientOption {
	return func(c *Client) {
		c.Creds = creds
	}
}

// NewClient creates a new Cloudflare API client
func NewClient(options ...ClientOption) (*Client, error) {
	// Create optimized transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        500,              // Increased pool size
		MaxIdleConnsPerHost: 100,              // More connections per host
		MaxConnsPerHost:     100,              // Limit concurrent connections
		IdleConnTimeout:     90 * time.Second, // Keep connections alive longer
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true,              // API responses are already compressed
		ForceAttemptHTTP2:   true,              // Enable HTTP/2 for multiplexing
	}

	// Create client with default values
	client := &Client{
		BaseURL: "https://api.cloudflare.com/client/v4",
		HTTPClient: &http.Client{
			Timeout:   300 * time.Second, // Increased from 30s to 300s to handle large operations
			Transport: transport,
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

// GetTransportStats returns connection pool statistics for monitoring
func (c *Client) GetTransportStats() (idleConns int, totalConns int) {
	if transport, ok := c.HTTPClient.Transport.(*http.Transport); ok {
		// Note: These are approximations as Go doesn't expose all transport internals
		return transport.MaxIdleConns, transport.MaxConnsPerHost
	}
	return 0, 0
}

// Request makes a request to the Cloudflare API
func (c *Client) Request(method, path string, query url.Values, body interface{}) ([]byte, error) {
	// Determine endpoint for rate limiting
	endpoint := determineEndpoint(method, path)
	
	// Wait for rate limit
	ctx := context.Background()
	if err := common.WaitForRateLimit(ctx, endpoint); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}
	
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

	// Create request
	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body using pooled buffer
	buf := common.MemoryPools.GetByteBuffer()
	defer common.MemoryPools.PutByteBuffer(buf)
	
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	respBody := buf.Bytes()

	// Check for errors
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

	return respBody, nil
}

// RequestWithContext makes a request with context support
func (c *Client) RequestWithContext(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, error) {
	// Determine endpoint for rate limiting
	endpoint := determineEndpoint(method, path)
	
	// Wait for rate limit
	if err := common.WaitForRateLimit(ctx, endpoint); err != nil {
		return nil, fmt.Errorf("rate limit: %w", err)
	}
	
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

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reqBody)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

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
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body using pooled buffer
	buf := common.MemoryPools.GetByteBuffer()
	defer common.MemoryPools.PutByteBuffer(buf)
	
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	respBody := buf.Bytes()

	// Check for errors
	if resp.StatusCode >= 400 {
		errorMsg := string(respBody)

		// Check if this might be a token scope issue
		if resp.StatusCode == 403 && c.Creds.Type == auth.AuthTypeAPIToken {
			if scopeHint := auth.CheckTokenScope(errorMsg); scopeHint != "" {
				return nil, fmt.Errorf("%s (HTTP %d): %s", errorMsg, resp.StatusCode, scopeHint)
			}
		}

		// Check for rate limiting
		if resp.StatusCode == 429 {
			// Try to parse retry-after header
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				return nil, fmt.Errorf("rate limited (HTTP 429), retry after: %s", retryAfter)
			}
			return nil, fmt.Errorf("rate limited (HTTP 429): %s", errorMsg)
		}

		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, errorMsg)
	}

	return respBody, nil
}

// determineEndpoint determines the rate limit endpoint from the request
func determineEndpoint(method, path string) string {
	// Normalize path
	path = strings.ToLower(path)
	
	// KV operations
	if strings.Contains(path, "/storage/kv/namespaces") {
		if strings.Contains(path, "/bulk") {
			return common.EndpointKVBulk
		}
		if strings.Contains(path, "/metadata") {
			return common.EndpointKVMetadata
		}
		if strings.Contains(path, "/keys") {
			return common.EndpointKVList
		}
		if strings.Contains(path, "/values") {
			switch method {
			case http.MethodGet, http.MethodHead:
				return common.EndpointKVGet
			case http.MethodPut:
				return common.EndpointKVPut
			case http.MethodDelete:
				return common.EndpointKVDelete
			}
		}
		// Default KV operation
		return common.EndpointKVGet
	}
	
	// Default endpoint
	return "default"
}
