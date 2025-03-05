package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"cache-kv-purger/internal/auth"
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
	// Create client with default values
	client := &Client{
		BaseURL: "https://api.cloudflare.com/client/v4",
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
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

// Request makes a request to the Cloudflare API
func (c *Client) Request(method, path string, query url.Values, body interface{}) ([]byte, error) {
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

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

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