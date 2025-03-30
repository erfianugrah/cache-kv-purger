package kv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
)

// GetValue gets a value from a KV namespace
func GetValue(client *api.Client, accountID, namespaceID, key string) (string, error) {
	return GetValueWithOptions(client, accountID, namespaceID, key, nil)
}

// GetValueWithOptions gets a value from a KV namespace with additional options
func GetValueWithOptions(client *api.Client, accountID, namespaceID, key string, options *GetOptions) (string, error) {
	if accountID == "" {
		return "", fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return "", fmt.Errorf("namespace ID is required")
	}
	if key == "" {
		return "", fmt.Errorf("key is required")
	}

	// URL encode the key
	encodedKey := url.PathEscape(key)
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, encodedKey)

	var queryParams url.Values
	if options != nil && options.IncludeMetadata {
		queryParams = url.Values{}
		queryParams.Set("metadata", "true")
	}

	respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
	if err != nil {
		return "", err
	}

	return string(respBody), nil
}

// GetKeyWithMetadata gets a key-value pair including its metadata
func GetKeyWithMetadata(client *api.Client, accountID, namespaceID, key string) (*KeyValuePair, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}

	// First get the value of the key
	value, err := GetValue(client, accountID, namespaceID, key)
	if err != nil {
		return nil, err
	}

	// Get metadata using the correct endpoint
	encodedKey := url.PathEscape(key)
	metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s", accountID, namespaceID, encodedKey)

	// Request metadata specifically
	metadataRespBody, err := client.Request(http.MethodGet, metadataPath, nil, nil)

	// Metadata is optional, so if there's an error (like 404), we just continue without metadata
	var metadata *KeyValueMetadata

	if err == nil {
		// Try to parse the metadata response
		var metadataResponse struct {
			Success bool                   `json:"success"`
			Errors  []api.Error            `json:"errors,omitempty"`
			Result  map[string]interface{} `json:"result,omitempty"`
		}

		if err := json.Unmarshal(metadataRespBody, &metadataResponse); err == nil && metadataResponse.Success {
			if len(metadataResponse.Result) > 0 {
				metadataObj := KeyValueMetadata(metadataResponse.Result)
				metadata = &metadataObj
			}
		}
	}

	// Return the key-value pair with any metadata we found
	return &KeyValuePair{
		Key:      key,
		Value:    value,
		Metadata: metadata,
	}, nil
}

// KeyExists checks if a key exists in a KV namespace
func KeyExists(client *api.Client, accountID, namespaceID, key string) (bool, error) {
	if accountID == "" {
		return false, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return false, fmt.Errorf("namespace ID is required")
	}
	if key == "" {
		return false, fmt.Errorf("key is required")
	}

	// URL encode the key
	encodedKey := url.PathEscape(key)
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, encodedKey)

	// We'll use a HEAD request to check if the key exists without retrieving the value
	// This is handled manually since it's a special case
	req, err := http.NewRequest(http.MethodHead, client.BaseURL+path, nil)
	if err != nil {
		return false, err
	}

	// Add authentication headers
	if client.Creds != nil {
		switch client.Creds.Type {
		case auth.AuthTypeAPIKey:
			req.Header.Set("X-Auth-Key", client.Creds.Key)
			req.Header.Set("X-Auth-Email", client.Creds.Email)
		case auth.AuthTypeAPIToken:
			req.Header.Set("Authorization", "Bearer "+client.Creds.Key)
		}
	}

	// Make request
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode == http.StatusOK {
		return true, nil
	} else if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else {
		// Read response body for error details
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("API error checking key existence (HTTP %d): %s", resp.StatusCode, string(body))
	}
}