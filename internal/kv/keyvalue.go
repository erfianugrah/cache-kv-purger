package kv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
)

// KeyValuePair represents a key-value pair in a KV namespace
type KeyValuePair struct {
	Key        string            `json:"name"`
	Value      string            `json:"-"` // Value doesn't come from the API in list operations
	Expiration int64             `json:"expiration,omitempty"`
	Metadata   *KeyValueMetadata `json:"metadata,omitempty"`
}

// KeyValueMetadata represents metadata for a key in a KV namespace
type KeyValueMetadata map[string]interface{}

// KeyName represents a single key name for bulk operations
type KeyName struct {
	Name string `json:"name"`
}

// KeyValuesResponse represents a response containing multiple key-value pairs
type KeyValuesResponse struct {
	api.PaginatedResponse
	Result []KeyValuePair `json:"result"`
}

// WriteOptions represents options for writing a value
type WriteOptions struct {
	Expiration    int64            `json:"expiration,omitempty"` // Unix timestamp
	ExpirationTTL int64            `json:"expiration_ttl,omitempty"` // TTL in seconds
	Metadata      KeyValueMetadata `json:"metadata,omitempty"`
}

// GetOptions represents options for reading a value
type GetOptions struct {
	IncludeMetadata bool // Whether to include metadata in the response
}

// ListKeysOptions represents options for listing keys
type ListKeysOptions struct {
	Limit  int    `json:"limit,omitempty"`  // Maximum number of keys to return (max 1000)
	Cursor string `json:"cursor,omitempty"` // Cursor for pagination
	Prefix string `json:"prefix,omitempty"` // Filter keys by prefix
}

// ListKeysResult represents the result of a list keys operation, with pagination info
type ListKeysResult struct {
	Keys       []KeyValuePair `json:"keys"`
	Cursor     string         `json:"cursor"`
	HasMore    bool           `json:"has_more"`
	TotalCount int            `json:"total_count"`
}

// KeyValueResponse represents a response for a single key-value operation
type KeyValueResponse struct {
	api.APIResponse
	Result *KeyValuePair `json:"result,omitempty"`
}

// ListKeys lists all keys in a KV namespace
func ListKeys(client *api.Client, accountID, namespaceID string) ([]KeyValuePair, error) {
	result, err := ListKeysWithOptions(client, accountID, namespaceID, nil)
	if err != nil {
		return nil, err
	}
	return result.Keys, nil
}

// ListKeysWithOptions lists keys in a KV namespace with advanced options and pagination
func ListKeysWithOptions(client *api.Client, accountID, namespaceID string, options *ListKeysOptions) (*ListKeysResult, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/keys", accountID, namespaceID)
	
	// Add query parameters if options are provided
	var queryParams url.Values
	if options != nil {
		queryParams = url.Values{}
		
		if options.Limit > 0 {
			queryParams.Set("limit", fmt.Sprintf("%d", options.Limit))
		}
		
		if options.Cursor != "" {
			queryParams.Set("cursor", options.Cursor)
		}
		
		if options.Prefix != "" {
			queryParams.Set("prefix", options.Prefix)
		}
	}
	
	respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
	if err != nil {
		return nil, err
	}

	var keysResp KeyValuesResponse
	if err := json.Unmarshal(respBody, &keysResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !keysResp.Success {
		errorStr := "API reported failure"
		if len(keysResp.Errors) > 0 {
			errorStr = keysResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to list keys: %s", errorStr)
	}

	// Prepare result
	result := &ListKeysResult{
		Keys:    keysResp.Result,
		Cursor:  keysResp.ResultInfo.Cursor,
		HasMore: keysResp.ResultInfo.Cursor != "",
	}

	return result, nil
}

// ListAllKeys lists all keys in a KV namespace, handling pagination automatically
func ListAllKeys(client *api.Client, accountID, namespaceID string, progressCallback func(fetched, total int)) ([]KeyValuePair, error) {
	var allKeys []KeyValuePair
	options := &ListKeysOptions{
		Limit: 1000, // Maximum limit per request
	}
	
	totalFetched := 0
	
	for {
		result, err := ListKeysWithOptions(client, accountID, namespaceID, options)
		if err != nil {
			return nil, err
		}
		
		allKeys = append(allKeys, result.Keys...)
		totalFetched += len(result.Keys)
		
		if progressCallback != nil {
			progressCallback(totalFetched, -1) // -1 means total unknown
		}
		
		if !result.HasMore {
			break
		}
		
		// Update cursor for next request
		options.Cursor = result.Cursor
	}
	
	return allKeys, nil
}

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

	// URL encode the key
	encodedKey := url.PathEscape(key)
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, encodedKey)
	
	queryParams := url.Values{}
	queryParams.Set("metadata", "true")
	
	respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
	if err != nil {
		return nil, err
	}

	// Try to parse as JSON to get metadata
	var kvResponse KeyValueResponse
	if err := json.Unmarshal(respBody, &kvResponse); err != nil {
		// If not JSON, it's just the raw value
		return &KeyValuePair{
			Key:   key,
			Value: string(respBody),
		}, nil
	}

	if !kvResponse.Success {
		errorStr := "API reported failure"
		if len(kvResponse.Errors) > 0 {
			errorStr = kvResponse.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to get key with metadata: %s", errorStr)
	}

	// If we got here, we have a response with metadata
	if kvResponse.Result != nil {
		kvResponse.Result.Key = key // Ensure key is set
		return kvResponse.Result, nil
	}

	// Fallback if result wasn't in expected format
	return &KeyValuePair{
		Key:   key,
		Value: string(respBody),
	}, nil
}

// WriteValue writes a value to a KV namespace
func WriteValue(client *api.Client, accountID, namespaceID, key, value string, options *WriteOptions) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return fmt.Errorf("namespace ID is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	// URL encode the key
	encodedKey := url.PathEscape(key)
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, encodedKey)
	
	var query url.Values
	if options != nil {
		query = url.Values{}
		
		if options.Expiration > 0 {
			query.Set("expiration", fmt.Sprintf("%d", options.Expiration))
		}
		
		if options.ExpirationTTL > 0 {
			query.Set("expiration_ttl", fmt.Sprintf("%d", options.ExpirationTTL))
		}
		
		if options.Metadata != nil {
			metadataJSON, err := json.Marshal(options.Metadata)
			if err != nil {
				return fmt.Errorf("failed to encode metadata: %w", err)
			}
			query.Set("metadata", string(metadataJSON))
		}
	}
	
	// Use custom Request implementation to handle text value
	req, err := http.NewRequest(http.MethodPut, client.BaseURL+path, bytes.NewBuffer([]byte(value)))
	if err != nil {
		return err
	}
	
	// Set query parameters
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	
	// Set content type for the value
	req.Header.Set("Content-Type", "text/plain")
	
	// Set authentication
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
		return err
	}
	defer resp.Body.Close()
	
	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	
	// Check for errors
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	
	var apiResp api.APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}
	
	if !apiResp.Success {
		errorStr := "API reported failure"
		if len(apiResp.Errors) > 0 {
			errorStr = apiResp.Errors[0].Message
		}
		return fmt.Errorf("failed to write value: %s", errorStr)
	}
	
	return nil
}

// DeleteValue deletes a value from a KV namespace
func DeleteValue(client *api.Client, accountID, namespaceID, key string) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return fmt.Errorf("namespace ID is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	// URL encode the key
	encodedKey := url.PathEscape(key)
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, encodedKey)
	
	respBody, err := client.Request(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}

	var resp api.APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !resp.Success {
		errorStr := "API reported failure"
		if len(resp.Errors) > 0 {
			errorStr = resp.Errors[0].Message
		}
		return fmt.Errorf("failed to delete value: %s", errorStr)
	}

	return nil
}

// BulkWriteItem represents an item for bulk writes
type BulkWriteItem struct {
	Key          string                 `json:"key"`
	Value        string                 `json:"value"`
	Expiration   int64                  `json:"expiration,omitempty"`
	ExpirationTTL int64                 `json:"expiration_ttl,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// BulkWriteResult represents the result of a bulk write operation
type BulkWriteResult struct {
	api.APIResponse
	Result struct {
		SuccessCount int `json:"success_count"`
		ErrorCount   int `json:"error_count"`
		Errors       []struct {
			Key   string `json:"key"`
			Error string `json:"error"`
		} `json:"errors,omitempty"`
	} `json:"result"`
}

// WriteMultipleValues writes multiple values to a KV namespace
func WriteMultipleValues(client *api.Client, accountID, namespaceID string, items []BulkWriteItem) error {
	result, err := WriteMultipleValuesWithResult(client, accountID, namespaceID, items)
	if err != nil {
		return err
	}
	
	// If there were any errors in individual items, report them
	if result.Result.ErrorCount > 0 {
		errorMsgs := make([]string, len(result.Result.Errors))
		for i, err := range result.Result.Errors {
			errorMsgs[i] = fmt.Sprintf("key '%s': %s", err.Key, err.Error)
		}
		return fmt.Errorf("bulk write partially failed (%d/%d succeeded): %s",
			result.Result.SuccessCount,
			result.Result.SuccessCount+result.Result.ErrorCount,
			strings.Join(errorMsgs, "; "))
	}
	
	return nil
}

// WriteMultipleValuesWithResult writes multiple values to a KV namespace and returns detailed results
func WriteMultipleValuesWithResult(client *api.Client, accountID, namespaceID string, items []BulkWriteItem) (*BulkWriteResult, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}
	if len(items) > 10000 {
		return nil, fmt.Errorf("maximum of 10000 items can be written in a single bulk operation")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk", accountID, namespaceID)
	
	respBody, err := client.Request(http.MethodPut, path, nil, items)
	if err != nil {
		return nil, err
	}

	var resp BulkWriteResult
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !resp.Success {
		errorStr := "API reported failure"
		if len(resp.Errors) > 0 {
			errorStr = resp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to write multiple values: %s", errorStr)
	}

	return &resp, nil
}

// WriteMultipleValuesInBatches writes multiple values to a KV namespace in batches
// This is useful when you have a large number of items to write
func WriteMultipleValuesInBatches(client *api.Client, accountID, namespaceID string, items []BulkWriteItem, batchSize int, progressCallback func(completed, total int)) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	
	if batchSize <= 0 {
		batchSize = 1000 // Default batch size
	} else if batchSize > 10000 {
		batchSize = 10000 // Max batch size supported by API
	}
	
	totalSuccess := 0
	totalItems := len(items)
	
	// Process in batches
	for i := 0; i < totalItems; i += batchSize {
		end := i + batchSize
		if end > totalItems {
			end = totalItems
		}
		
		batch := items[i:end]
		
		// Write this batch
		result, err := WriteMultipleValuesWithResult(client, accountID, namespaceID, batch)
		if err != nil {
			// Return partial success count and the error
			return totalSuccess, fmt.Errorf("batch %d failed: %w", i/batchSize+1, err)
		}
		
		// Update success count
		totalSuccess += result.Result.SuccessCount
		
		// Call progress callback if provided
		if progressCallback != nil {
			progressCallback(i+len(batch), totalItems)
		}
	}
	
	return totalSuccess, nil
}

// DeleteMultipleValues deletes multiple values from a KV namespace
func DeleteMultipleValues(client *api.Client, accountID, namespaceID string, keys []string) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return fmt.Errorf("namespace ID is required")
	}
	if len(keys) == 0 {
		return fmt.Errorf("at least one key is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk", accountID, namespaceID)
	
	// API expects an array of objects with 'name' property
	keyObjects := make([]KeyName, len(keys))
	for i, key := range keys {
		keyObjects[i] = KeyName{Name: key}
	}
	requestBody := keyObjects
	
	respBody, err := client.Request(http.MethodDelete, path, nil, requestBody)
	if err != nil {
		return err
	}

	var resp api.APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !resp.Success {
		errorStr := "API reported failure"
		if len(resp.Errors) > 0 {
			errorStr = resp.Errors[0].Message
		}
		return fmt.Errorf("failed to delete multiple values: %s", errorStr)
	}

	return nil
}

// DeleteMultipleValuesInBatches deletes multiple values from a KV namespace in batches
func DeleteMultipleValuesInBatches(client *api.Client, accountID, namespaceID string, keys []string, batchSize int, progressCallback func(completed, total int)) error {
	if len(keys) == 0 {
		return nil
	}
	
	if batchSize <= 0 {
		batchSize = 1000 // Default batch size
	}
	
	totalItems := len(keys)
	
	// Process in batches
	for i := 0; i < totalItems; i += batchSize {
		end := i + batchSize
		if end > totalItems {
			end = totalItems
		}
		
		batch := keys[i:end]
		
		// Delete this batch
		err := DeleteMultipleValues(client, accountID, namespaceID, batch)
		if err != nil {
			return fmt.Errorf("batch %d failed: %w", i/batchSize+1, err)
		}
		
		// Call progress callback if provided
		if progressCallback != nil {
			progressCallback(i+len(batch), totalItems)
		}
	}
	
	return nil
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
	
	// Create a HEAD request to check if the key exists
	req, err := http.NewRequest(http.MethodHead, client.BaseURL+path, nil)
	if err != nil {
		return false, err
	}
	
	// Set authentication
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

// ExportKeysAndValuesToJSON exports all keys and values from a KV namespace to a JSON file
func ExportKeysAndValuesToJSON(client *api.Client, accountID, namespaceID string, includeMetadata bool, progressCallback func(fetched, total int)) ([]BulkWriteItem, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	
	// First, list all keys
	keys, err := ListAllKeys(client, accountID, namespaceID, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(keys) == 0 {
		return []BulkWriteItem{}, nil // Return empty slice, not nil
	}
	
	// Create result array
	results := make([]BulkWriteItem, len(keys))
	
	// For each key, fetch its value
	for i, key := range keys {
		// Update progress callback for value fetching
		if progressCallback != nil {
			progressCallback(i, len(keys))
		}
		
		var value string
		var metadata map[string]interface{}
		
		if includeMetadata {
			// Get value with metadata
			kvPair, err := GetKeyWithMetadata(client, accountID, namespaceID, key.Key)
			if err != nil {
				return nil, fmt.Errorf("failed to get value for key '%s': %w", key.Key, err)
			}
			
			value = kvPair.Value
			if kvPair.Metadata != nil {
				metadata = *kvPair.Metadata
			}
		} else {
			// Get value without metadata
			val, err := GetValue(client, accountID, namespaceID, key.Key)
			if err != nil {
				return nil, fmt.Errorf("failed to get value for key '%s': %w", key.Key, err)
			}
			value = val
		}
		
		// Add to results
		results[i] = BulkWriteItem{
			Key:        key.Key,
			Value:      value,
			Expiration: key.Expiration,
			Metadata:   metadata,
		}
	}
	
	return results, nil
}

// FilterKeys filters keys in a KV namespace based on a custom filter function
func FilterKeys(client *api.Client, accountID, namespaceID string, filterFunc func(key KeyValuePair) bool, progressCallback func(fetched, total int)) ([]KeyValuePair, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	
	// List all keys
	allKeys, err := ListAllKeys(client, accountID, namespaceID, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(allKeys) == 0 {
		return []KeyValuePair{}, nil // Return empty slice, not nil
	}
	
	// Apply the filter
	var filteredKeys []KeyValuePair
	for _, key := range allKeys {
		if filterFunc(key) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	
	return filteredKeys, nil
}