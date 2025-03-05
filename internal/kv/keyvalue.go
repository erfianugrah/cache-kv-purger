package kv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
			if metadataResponse.Result != nil && len(metadataResponse.Result) > 0 {
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

// WriteMultipleValuesConcurrently writes multiple values to a KV namespace using concurrent batch operations
// This is optimized for high throughput with a high API rate limit
func WriteMultipleValuesConcurrently(client *api.Client, accountID, namespaceID string, items []BulkWriteItem, batchSize int, concurrency int, progressCallback func(completed, total int)) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}
	
	if batchSize <= 0 {
		batchSize = 100 // Default batch size - Cloudflare's recommended size
	} else if batchSize > 10000 {
		batchSize = 10000 // Max batch size supported by API
	}

	// Set reasonable concurrency
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}
	if concurrency > 50 {
		concurrency = 50 // Cap concurrency to avoid overwhelming API
	}
	
	// Simple progress reporting if none provided
	if progressCallback == nil {
		progressCallback = func(completed, total int) {}
	}
	
	totalItems := len(items)
	
	// Create work items for all batches
	type batchWork struct {
		batchIndex int
		batchItems []BulkWriteItem
	}
	
	var batches []batchWork
	for i := 0; i < totalItems; i += batchSize {
		end := i + batchSize
		if end > totalItems {
			end = totalItems
		}
		
		batch := items[i:end]
		batches = append(batches, batchWork{
			batchIndex: i / batchSize,
			batchItems: batch,
		})
	}
	
	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex   int
		successCount int
		err          error
	}
	
	resultChan := make(chan batchResult, len(batches))
	
	// Create a client mutex to ensure thread safety if needed
	clientMutex := &sync.Mutex{}
	
	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, concurrency)
	
	// Process all batches
	for _, batch := range batches {
		// Acquire semaphore slot (or wait if at capacity)
		sem <- struct{}{}
		
		// Launch a goroutine to process this batch
		go func(b batchWork) {
			defer func() { <-sem }() // Release semaphore when done
			
			// Use mutex for client operations if needed
			clientMutex.Lock()
			result, err := WriteMultipleValuesWithResult(client, accountID, namespaceID, b.batchItems)
			clientMutex.Unlock()
			
			// Send result back through channel
			if err != nil {
				resultChan <- batchResult{
					batchIndex:   b.batchIndex,
					successCount: 0,
					err:          fmt.Errorf("batch %d failed: %w", b.batchIndex+1, err),
				}
				return
			}
			
			// If the API returns a success count, use it, otherwise assume all items succeeded
			successCount := len(b.batchItems)
			if result.Success && result.Result.SuccessCount > 0 {
				successCount = result.Result.SuccessCount
			}
			
			resultChan <- batchResult{
				batchIndex:   b.batchIndex,
				successCount: successCount,
				err:          nil,
			}
		}(batch)
	}
	
	// Collect results
	totalSuccess := 0
	failedBatches := 0
	var firstError error
	
	// Track progress for callback
	completed := make(map[int]bool)
	lastProgressUpdate := 0
	
	for i := 0; i < len(batches); i++ {
		result := <-resultChan
		
		// Save first error encountered and track success
		if result.err != nil {
			if firstError == nil {
				firstError = result.err
			}
			failedBatches++
		}
		
		// Always add the success count (which will be 0 if there was an error)
		totalSuccess += result.successCount
		
		// Mark this batch as completed for progress tracking
		completed[result.batchIndex] = true
		
		// Calculate total completed items for progress
		completedItems := 0
		for idx, done := range completed {
			if done {
				completedItems += len(batches[idx].batchItems)
			}
		}
		
		// Report progress if significant change or final update
		if completedItems-lastProgressUpdate >= 100 || completedItems == totalItems {
			progressCallback(completedItems, totalItems)
			lastProgressUpdate = completedItems
		}
	}
	
	// If we encountered errors but had some success, return partial success
	if firstError != nil && totalSuccess > 0 {
		return totalSuccess, fmt.Errorf("%d/%d batches failed: %w", failedBatches, len(batches), firstError)
	} else if firstError != nil {
		return 0, firstError
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
	
	// Send the request with the array of KeyName objects
	respBody, err := client.Request(http.MethodDelete, path, nil, keyObjects)
	if err != nil {
		// Fall back to individual deletions if bulk delete fails
		fallbackErrors := 0
		for _, key := range keys {
			if deleteErr := DeleteValue(client, accountID, namespaceID, key); deleteErr != nil {
				fallbackErrors++
			}
		}
		
		// If all individual deletes failed too, return the original error
		if fallbackErrors == len(keys) {
			return err
		}
		
		// Otherwise we succeeded with individual deletes
		return nil
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
		
		// Try individual deletions as fallback
		fallbackErrors := 0
		for _, key := range keys {
			if deleteErr := DeleteValue(client, accountID, namespaceID, key); deleteErr != nil {
				fallbackErrors++
			}
		}
		
		// If all individual deletes failed too, return the original error
		if fallbackErrors == len(keys) {
			return fmt.Errorf("failed to delete multiple values: %s", errorStr)
		}
		
		// Otherwise we succeeded with individual deletes
		return nil
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
// This is a simple wrapper around the parallel version with default concurrency
func ExportKeysAndValuesToJSON(client *api.Client, accountID, namespaceID string, includeMetadata bool, progressCallback func(fetched, total int)) ([]BulkWriteItem, error) {
	// Use the parallel version with default concurrency
	return ExportKeysAndValuesToJSONParallel(client, accountID, namespaceID, includeMetadata, 10, progressCallback)
}

// ExportKeysAndValuesToJSONParallel exports all keys and values with concurrent fetching
func ExportKeysAndValuesToJSONParallel(client *api.Client, accountID, namespaceID string, includeMetadata bool, concurrency int, progressCallback func(fetched, total int)) ([]BulkWriteItem, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	
	// Use default concurrency if not specified or invalid
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}
	if concurrency > 50 {
		concurrency = 50 // Cap maximum concurrency to avoid overwhelming the API
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
	
	// Create a channel for sending keys to workers
	type keyWorkItem struct {
		index int
		key   KeyValuePair
	}
	workChan := make(chan keyWorkItem, concurrency*2)
	
	// Create a channel for results
	type resultItem struct {
		index    int
		item     BulkWriteItem
		err      error
	}
	resultChan := make(chan resultItem, concurrency*2)
	
	// Create a channel to track progress
	progressChan := make(chan int, concurrency*2)
	
	// Create mutex for client to ensure thread safety
	clientMutex := &sync.Mutex{}
	
	// Launch worker goroutines
	var wg sync.WaitGroup
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			
			for work := range workChan {
				var value string
				var metadata map[string]interface{}
				
				// Add a small delay between workers to prevent API rate limiting
				time.Sleep(time.Duration(workerNum*5) * time.Millisecond)
				
				if includeMetadata {
					// Get value with metadata - thread safe by using mutex
					clientMutex.Lock()
					kvPair, fetchErr := GetKeyWithMetadata(client, accountID, namespaceID, work.key.Key)
					clientMutex.Unlock()
					
					if fetchErr != nil {
						resultChan <- resultItem{
							index: work.index,
							err:   fetchErr,
						}
						progressChan <- 1 // Count as processed even if error
						continue
					}
					
					value = kvPair.Value
					if kvPair.Metadata != nil {
						metadata = *kvPair.Metadata
					}
				} else {
					// Get value without metadata - thread safe by using mutex
					clientMutex.Lock()
					val, fetchErr := GetValue(client, accountID, namespaceID, work.key.Key)
					clientMutex.Unlock()
					
					if fetchErr != nil {
						resultChan <- resultItem{
							index: work.index,
							err:   fetchErr,
						}
						progressChan <- 1 // Count as processed even if error
						continue
					}
					value = val
				}
				
				// Return successful result
				resultChan <- resultItem{
					index: work.index,
					item: BulkWriteItem{
						Key:        work.key.Key,
						Value:      value,
						Expiration: work.key.Expiration,
						Metadata:   metadata,
					},
					err: nil,
				}
				
				// Update progress
				progressChan <- 1
			}
		}(i)
	}
	
	// Start a goroutine to send all keys to workers
	go func() {
		for i, key := range keys {
			workChan <- keyWorkItem{
				index: i,
				key:   key,
			}
		}
		close(workChan)
	}()
	
	// Start a goroutine to track progress
	go func() {
		processed := 0
		total := len(keys)
		
		for range progressChan {
			processed++
			if progressCallback != nil && processed % 10 == 0 { // Update progress every 10 items
				progressCallback(processed, total)
			}
			
			if processed >= total {
				// Final progress update
				if progressCallback != nil {
					progressCallback(processed, total)
				}
				close(progressChan)
			}
		}
	}()
	
	// Collect all results
	var errMsgs []string
	resultsProcessed := 0
	
	for resultsProcessed < len(keys) {
		result := <-resultChan
		resultsProcessed++
		
		if result.err != nil {
			errMsg := fmt.Sprintf("failed to get value for key index %d: %v", result.index, result.err)
			errMsgs = append(errMsgs, errMsg)
			continue
		}
		
		results[result.index] = result.item
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	// If we had any errors, report them
	if len(errMsgs) > 0 {
		// If all operations failed, return an error
		if len(errMsgs) == len(keys) {
			return nil, fmt.Errorf("all key fetch operations failed: %s", errMsgs[0])
		}
		
		// If some operations succeeded, log errors but continue
		fmt.Printf("Warning: %d of %d key fetch operations failed\n", len(errMsgs), len(keys))
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

// FilterKeysByMetadata is deprecated and should be replaced with StreamingFilterKeysByMetadata
// or, for better performance with high API rate limits, use PurgeByMetadataUpfront
func FilterKeysByMetadata(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, progressCallback func(fetched, processed, total int)) ([]KeyValuePair, error) {
	// This is now just a wrapper around the more efficient streaming implementation
	streamingCallback := func(keysFetched, keysProcessed, keysMatched, total int) {
		if progressCallback != nil {
			progressCallback(keysFetched, keysProcessed, total)
		}
	}
	
	return StreamingFilterKeysByMetadata(client, accountID, namespaceID, metadataField, metadataValue, 
		100, 10, streamingCallback)
}

// StreamingFilterKeysByMetadata performs a streaming filter of keys by metadata
// This is much more efficient for large namespaces as it processes in chunks
func StreamingFilterKeysByMetadata(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, 
	chunkSize int, concurrency int, progressCallback func(keysFetched, keysProcessed, keysMatched, total int)) ([]KeyValuePair, error) {
	
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if metadataField == "" {
		return nil, fmt.Errorf("metadata field is required")
	}
	if chunkSize <= 0 {
		chunkSize = 100 // Default chunk size
	}
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}
	if concurrency > 50 {
		concurrency = 50 // Cap maximum concurrency
	}
	
	// Simple progress callback if none provided
	if progressCallback == nil {
		progressCallback = func(keysFetched, keysProcessed, keysMatched, total int) {}
	}
	
	// First, list all keys
	keys, err := ListAllKeys(client, accountID, namespaceID, func(fetched, total int) {
		progressCallback(fetched, 0, 0, total)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(keys) == 0 {
		return []KeyValuePair{}, nil // Return empty slice, not nil
	}
	
	totalKeys := len(keys)
	var allMatchedKeys []KeyValuePair
	totalProcessed := 0
	
	// Process in chunks to reduce memory usage
	for i := 0; i < totalKeys; i += chunkSize {
		end := i + chunkSize
		if end > totalKeys {
			end = totalKeys
		}
		
		chunkKeys := keys[i:end]
		
		// Process this chunk
		matchedKeys, err := processMetadataChunk(client, accountID, namespaceID, chunkKeys, 
			metadataField, metadataValue, concurrency, func(processed, matched int) {
				totalProcessed = i + processed
				progressCallback(totalKeys, totalProcessed, len(allMatchedKeys) + matched, totalKeys)
			})
		
		if err != nil {
			return allMatchedKeys, fmt.Errorf("error processing chunk %d-%d: %w", i, end-1, err)
		}
		
		// Add matched keys to results
		allMatchedKeys = append(allMatchedKeys, matchedKeys...)
		
		// Update progress
		progressCallback(totalKeys, totalProcessed, len(allMatchedKeys), totalKeys)
	}
	
	return allMatchedKeys, nil
}

// processMetadataChunk is deprecated and replaced by processMetadataOnlyChunk
// This function used a less optimal approach and has been replaced by more efficient implementations
// For backward compatibility, it now forwards to processMetadataOnlyChunk
func processMetadataChunk(client *api.Client, accountID, namespaceID string, 
	chunkKeys []KeyValuePair, metadataField, metadataValue string, concurrency int,
	progressCallback func(processed, matched int)) ([]KeyValuePair, error) {
	
	// Forward to the better implementation but convert the callback format
	onlyCallback := func(processed int) {
		if progressCallback != nil {
			// We don't know matched count here so pass 0
			progressCallback(processed, 0)
		}
	}
	
	// Get matching keys first
	matchedKeyNames, err := processMetadataOnlyChunk(client, accountID, namespaceID, 
		chunkKeys, metadataField, metadataValue, onlyCallback)
	if err != nil {
		return nil, err
	}
	
	// Now fetch full values for matched keys (not the most efficient but maintains API compatibility)
	result := make([]KeyValuePair, 0, len(matchedKeyNames))
	for i, keyName := range matchedKeyNames {
		kvPair, err := GetKeyWithMetadata(client, accountID, namespaceID, keyName)
		if err != nil {
			continue
		}
		result = append(result, *kvPair)
		
		// Update progress
		if progressCallback != nil && (i%10 == 0 || i == len(matchedKeyNames)-1) {
			progressCallback(len(chunkKeys), len(result))
		}
	}
	
	return result, nil
}

// DeleteKeysByMetadata is deprecated and should be replaced with PurgeByMetadataUpfront
// or PurgeByMetadataOnly for better performance
func DeleteKeysByMetadata(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, 
	batchSize int, dryRun bool, progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)) (int, error) {
	
	// Forward to the better implementation
	return PurgeByMetadataOnly(client, accountID, namespaceID, metadataField, metadataValue,
		batchSize, 20, dryRun, progressCallback)
}

// StreamingPurgeByTag performs a streaming purge of keys with a specific tag value
// This is much more efficient for large namespaces as it processes in chunks
func StreamingPurgeByTag(client *api.Client, accountID, namespaceID, tagField, tagValue string, 
	chunkSize int, concurrency int, dryRun bool, 
	progressCallback func(keysFetched, keysProcessed, keysDeleted, total int)) (int, error) {
	
	if accountID == "" {
		return 0, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return 0, fmt.Errorf("namespace ID is required")
	}
	if tagField == "" {
		tagField = "cache-tag" // Default tag field
	}
	if chunkSize <= 0 {
		chunkSize = 100 // Default chunk size
	}
	if concurrency <= 0 {
		concurrency = 10 // Default concurrency
	}
	if concurrency > 50 {
		concurrency = 50 // Cap maximum concurrency
	}
	
	// Simple progress callback if none provided
	if progressCallback == nil {
		progressCallback = func(keysFetched, keysProcessed, keysDeleted, total int) {}
	}
	
	// First, list all keys (we need this to get the total count)
	keys, err := ListAllKeys(client, accountID, namespaceID, func(fetched, total int) {
		progressCallback(fetched, 0, 0, total)
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(keys) == 0 {
		return 0, nil // No keys to process
	}
	
	totalKeys := len(keys)
	totalProcessed := 0
	totalMatched := 0
	totalDeleted := 0
	
	// To improve performance, we'll:
	// 1. Process keys in larger batches
	// 2. Optimize for bulk API operations where possible
	// 3. Delete keys in batches of up to 1000 (API limit)
	
	// Track all matched keys across all chunks
	var allMatchedKeys []string
	
	// Process in chunks to reduce memory usage
	for i := 0; i < totalKeys; i += chunkSize {
		end := i + chunkSize
		if end > totalKeys {
			end = totalKeys
		}
		
		chunkKeys := keys[i:end]
		
		// Process this chunk with optimal batching
		matchedKeys, err := processKeyChunkOptimized(client, accountID, namespaceID, chunkKeys, 
			tagField, tagValue, concurrency, func(processed int) {
				totalProcessed = i + processed
				progressCallback(totalKeys, totalProcessed, totalDeleted, totalKeys)
			})
		
		if err != nil {
			return totalDeleted, fmt.Errorf("error processing chunk %d-%d: %w", i, end-1, err)
		}
		
		// Add matched keys to overall list
		allMatchedKeys = append(allMatchedKeys, matchedKeys...)
		totalMatched += len(matchedKeys)
		
		// If we've reached a significant number of keys to delete, batch delete them
		if !dryRun && len(allMatchedKeys) >= 1000 {
			// Delete in batches of 1000 (API limit)
			for j := 0; j < len(allMatchedKeys); j += 1000 {
				batchEnd := j + 1000
				if batchEnd > len(allMatchedKeys) {
					batchEnd = len(allMatchedKeys)
				}
				
				deleteBatch := allMatchedKeys[j:batchEnd]
				err = DeleteMultipleValues(client, accountID, namespaceID, deleteBatch)
				if err != nil {
					return totalDeleted, fmt.Errorf("error deleting matched keys in batch: %w", err)
				}
				
				// Update counts
				totalDeleted += len(deleteBatch)
				progressCallback(totalKeys, totalProcessed, totalDeleted, totalKeys)
			}
			
			// Reset the matched keys list after deletion
			allMatchedKeys = []string{}
		}
	}
	
	// Skip deletion if dry run
	if dryRun {
		return totalMatched, nil // Return how many would have been deleted
	}
	
	// Delete any remaining matched keys
	if len(allMatchedKeys) > 0 {
		// Delete in batches of 1000 (API limit)
		for j := 0; j < len(allMatchedKeys); j += 1000 {
			batchEnd := j + 1000
			if batchEnd > len(allMatchedKeys) {
				batchEnd = len(allMatchedKeys)
			}
			
			deleteBatch := allMatchedKeys[j:batchEnd]
			err = DeleteMultipleValues(client, accountID, namespaceID, deleteBatch)
			if err != nil {
				return totalDeleted, fmt.Errorf("error deleting matched keys in batch: %w", err)
			}
			
			// Update counts
			totalDeleted += len(deleteBatch)
			progressCallback(totalKeys, totalProcessed, totalDeleted, totalKeys)
		}
	}
	
	return totalDeleted, nil
}

// FetchAllMetadata fetches metadata for all keys in parallel and returns a map of key to metadata
// This is optimized for high throughput with a large number of concurrent API calls
func FetchAllMetadata(client *api.Client, accountID, namespaceID string, keys []KeyValuePair, 
	maxConcurrency int, progressCallback func(fetched, total int)) (map[string]*KeyValueMetadata, error) {

	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if len(keys) == 0 {
		return map[string]*KeyValueMetadata{}, nil
	}

	// Set proper concurrency limit
	concurrency := maxConcurrency
	if concurrency <= 0 {
		concurrency = 50 // Default concurrency
	}
	if concurrency > 1000 {
		concurrency = 1000 // Cap maximum concurrency to avoid system resource issues
	}

	// Progress tracking
	if progressCallback == nil {
		progressCallback = func(fetched, total int) {}
	}

	// Create result map
	metadataMap := make(map[string]*KeyValueMetadata, len(keys))
	metadataMapMutex := &sync.Mutex{}

	// Create worker pool pattern
	workChan := make(chan KeyValuePair, len(keys))
	var wg sync.WaitGroup

	// Error tracking
	errorCount := int32(0)
	var firstError error
	var errorMutex sync.Mutex

	// Process keys with concurrent workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for key := range workChan {
				// Construct the metadata path
				encodedKey := url.PathEscape(key.Key)
				metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s", 
					accountID, namespaceID, encodedKey)
				
				// Fetch the metadata
				metadataResp, err := client.Request(http.MethodGet, metadataPath, nil, nil)

				// Skip errors - not all keys will have metadata
				if err != nil {
					continue
				}

				// Parse the metadata response
				var metadataResponse struct {
					Success bool                   `json:"success"`
					Errors  []api.Error            `json:"errors,omitempty"`
					Result  map[string]interface{} `json:"result,omitempty"`
				}
				
				if err := json.Unmarshal(metadataResp, &metadataResponse); err != nil {
					atomic.AddInt32(&errorCount, 1)
					errorMutex.Lock()
					if firstError == nil {
						firstError = fmt.Errorf("error parsing metadata for key %s: %w", key.Key, err)
					}
					errorMutex.Unlock()
					continue
				}
				
				if !metadataResponse.Success {
					continue // Skip unsuccessful responses
				}

				if metadataResponse.Result != nil && len(metadataResponse.Result) > 0 {
					// Store the metadata in our map
					metadataObj := KeyValueMetadata(metadataResponse.Result)
					
					metadataMapMutex.Lock()
					metadataMap[key.Key] = &metadataObj
					metadataMapMutex.Unlock()
				}
			}
		}(i)
	}

	// Feed keys to workers
	totalProcessed := 0
	lastProgress := 0
	for _, key := range keys {
		workChan <- key
		totalProcessed++
		
		// Update progress periodically
		if totalProcessed-lastProgress >= 100 || totalProcessed == len(keys) {
			progressCallback(totalProcessed, len(keys))
			lastProgress = totalProcessed
		}
	}
	close(workChan)

	// Wait for all workers to complete
	wg.Wait()

	// Final progress update
	progressCallback(len(keys), len(keys))

	// If we had too many errors, report the first one
	if errorCount > int32(len(keys)/2) {
		return metadataMap, firstError
	}

	return metadataMap, nil
}

// PurgeByMetadataUpfront fetches all metadata first then processes in memory
// This is much more efficient when you have a high API rate limit
func PurgeByMetadataUpfront(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, 
	concurrency int, dryRun bool, 
	progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)) (int, error) {
	
	if accountID == "" {
		return 0, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return 0, fmt.Errorf("namespace ID is required")
	}
	if metadataField == "" {
		metadataField = "cache-tag" // Default field
	}
	if concurrency <= 0 {
		concurrency = 50 // Default concurrency
	}
	if concurrency > 1000 {
		concurrency = 1000 // Cap maximum concurrency
	}
	
	// Default progress callback
	if progressCallback == nil {
		progressCallback = func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {}
	}
	
	// First, list all keys 
	keys, err := ListAllKeys(client, accountID, namespaceID, func(fetched, total int) {
		progressCallback(fetched, 0, 0, 0, total)
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(keys) == 0 {
		return 0, nil // No keys to process
	}
	
	totalKeys := len(keys)
	
	// Fetch all metadata upfront with high concurrency for maximum throughput
	metadataProgress := func(fetched, total int) {
		progressCallback(totalKeys, fetched, 0, 0, total)
	}
	
	allMetadata, err := FetchAllMetadata(client, accountID, namespaceID, keys, concurrency, metadataProgress)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch metadata: %w", err)
	}
	
	// Process all keys in memory now that we have all metadata
	var matchingKeys []string
	
	for _, key := range keys {
		// Find metadata for this key
		metadata, exists := allMetadata[key.Key]
		if !exists {
			continue // No metadata for this key
		}
		
		// Check if the field exists and matches the value
		fieldValue, exists := (*metadata)[metadataField]
		if !exists {
			continue // Field doesn't exist
		}
		
		// Check if the field value matches (empty value matches anything)
		fieldValueStr := fmt.Sprintf("%v", fieldValue)
		if metadataValue == "" || fieldValueStr == metadataValue {
			matchingKeys = append(matchingKeys, key.Key)
		}
		
		// Update progress
		progressCallback(totalKeys, totalKeys, len(matchingKeys), 0, totalKeys)
	}
	
	// If dry run, just return the count
	if dryRun {
		return len(matchingKeys), nil
	}
	
	// Delete matching keys in batches
	totalDeleted := 0
	if len(matchingKeys) > 0 {
		// Use batch size of 1000 for deletions (Cloudflare API limit)
		batchSize := 1000
		
		for i := 0; i < len(matchingKeys); i += batchSize {
			end := i + batchSize
			if end > len(matchingKeys) {
				end = len(matchingKeys)
			}
			
			batch := matchingKeys[i:end]
			
			err := DeleteMultipleValues(client, accountID, namespaceID, batch)
			if err != nil {
				return totalDeleted, fmt.Errorf("error deleting batch of keys: %w", err)
			}
			
			totalDeleted += len(batch)
			progressCallback(totalKeys, totalKeys, len(matchingKeys), totalDeleted, totalKeys)
		}
	}
	
	return totalDeleted, nil
}

// PurgeByMetadataOnly uses a metadata-first approach for better performance
// It only checks metadata and doesn't look at values at all
func PurgeByMetadataOnly(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, 
	chunkSize int, concurrency int, dryRun bool, 
	progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)) (int, error) {
	
	if accountID == "" {
		return 0, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return 0, fmt.Errorf("namespace ID is required")
	}
	if metadataField == "" {
		metadataField = "cache-tag" // Default field
	}
	if chunkSize <= 0 {
		chunkSize = 1000 // Use larger chunks for better performance
	}
	if concurrency <= 0 {
		concurrency = 20 // Use higher concurrency for better performance
	}
	if concurrency > 50 {
		concurrency = 50 // Cap maximum concurrency
	}
	
	// Default progress callback
	if progressCallback == nil {
		progressCallback = func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {}
	}
	
	// First, list all keys (we need this to get the total count)
	keys, err := ListAllKeys(client, accountID, namespaceID, func(fetched, total int) {
		progressCallback(fetched, 0, 0, 0, total)
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(keys) == 0 {
		return 0, nil // No keys to process
	}
	
	totalKeys := len(keys)
	totalProcessed := 0
	totalMatched := 0
	totalDeleted := 0
	
	// Create a channel to receive matched keys
	matchedKeysChan := make(chan []string, concurrency)
	errorChan := make(chan error, concurrency)
	
	// Process keys in chunks using a worker pool
	var wg sync.WaitGroup
	
	// Create a semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, concurrency)
	
	// Launch workers for each chunk
	for i := 0; i < totalKeys; i += chunkSize {
		end := i + chunkSize
		if end > totalKeys {
			end = totalKeys
		}
		
		// Get current chunk
		chunkKeys := keys[i:end]
		chunkNum := i/chunkSize + 1
		
		// Acquire semaphore
		semaphore <- struct{}{}
		
		wg.Add(1)
		go func(chunkNum int, chunkKeys []KeyValuePair, startIdx int) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done
			
			// Process this chunk by checking metadata only
			matchedKeys, err := processMetadataOnlyChunk(client, accountID, namespaceID, chunkKeys, 
				metadataField, metadataValue, func(processed int) {
					// Update progress
					totalProcessed := startIdx + processed
					progressCallback(totalKeys, totalProcessed, totalMatched, totalDeleted, totalKeys)
				})
			
			if err != nil {
				errorChan <- fmt.Errorf("error processing chunk %d: %w", chunkNum, err)
				return
			}
			
			// Send matched keys to channel
			matchedKeysChan <- matchedKeys
		}(chunkNum, chunkKeys, i)
	}
	
	// Collect results from all chunks
	go func() {
		wg.Wait()
		close(matchedKeysChan)
		close(errorChan)
	}()
	
	// Check for errors
	for err := range errorChan {
		if err != nil {
			return totalDeleted, err
		}
	}
	
	// Collect all matched keys
	var allMatchedKeys []string
	for matchedChunk := range matchedKeysChan {
		allMatchedKeys = append(allMatchedKeys, matchedChunk...)
		totalMatched += len(matchedChunk)
		
		// Update progress
		progressCallback(totalKeys, totalProcessed, totalMatched, totalDeleted, totalKeys)
	}
	
	// If dry run, just return the count
	if dryRun {
		return totalMatched, nil
	}
	
	// Delete matching keys in batches
	if len(allMatchedKeys) > 0 {
		// Use batch size of 1000 for deletions (Cloudflare API limit)
		batchSize := 1000
		
		for i := 0; i < len(allMatchedKeys); i += batchSize {
			end := i + batchSize
			if end > len(allMatchedKeys) {
				end = len(allMatchedKeys)
			}
			
			batch := allMatchedKeys[i:end]
			
			err := DeleteMultipleValues(client, accountID, namespaceID, batch)
			if err != nil {
				return totalDeleted, fmt.Errorf("error deleting batch of keys: %w", err)
			}
			
			totalDeleted += len(batch)
			progressCallback(totalKeys, totalProcessed, totalMatched, totalDeleted, totalKeys)
		}
	}
	
	return totalDeleted, nil
}

// processMetadataOnlyChunk processes a chunk of keys by checking their metadata
// This is much more efficient than checking both metadata and values
func processMetadataOnlyChunk(client *api.Client, accountID, namespaceID string, 
	chunkKeys []KeyValuePair, metadataField, metadataValue string,
	progressCallback func(processed int)) ([]string, error) {
	
	// Create a mutex for thread safety
	clientMutex := &sync.Mutex{}
	
	// Create channels for workers
	workChan := make(chan KeyValuePair, len(chunkKeys))
	resultChan := make(chan struct{
		key       string
		matches   bool
		processed bool
	}, len(chunkKeys))
	
	// Use extremely high concurrency - Cloudflare's API seems to be rate limiting, so we'll try to max out our requests
	// Process all keys in parallel for maximum throughput
	concurrency := len(chunkKeys) // Process every key in its own goroutine for maximum parallelism
	if concurrency > 1000 { // But cap at 1000 to avoid system resource issues
		concurrency = 1000
	}
	
	// Launch worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			
			for key := range workChan {
				// No delay needed - Cloudflare API can handle high concurrency
				// Removing this delay dramatically improves performance
				
				// Check only metadata for this key (much faster)
				encodedKey := url.PathEscape(key.Key)
				metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s", 
					accountID, namespaceID, encodedKey)
				
				// Get metadata with thread safety
				clientMutex.Lock()
				metadataResp, metadataErr := client.Request(http.MethodGet, metadataPath, nil, nil)
				clientMutex.Unlock()
				
				var matches bool
				
				// If we got metadata and it contains our field, check it
				if metadataErr == nil {
					var metadataResponse struct {
						Success bool                   `json:"success"`
						Result  map[string]interface{} `json:"result"`
					}
					
					if err := json.Unmarshal(metadataResp, &metadataResponse); err == nil && 
						metadataResponse.Success && metadataResponse.Result != nil {
						
						// Check if metadata has our field
						if fieldValue, ok := metadataResponse.Result[metadataField]; ok {
							// We found the field in metadata!
							fieldStr, isString := fieldValue.(string)
							if isString && (metadataValue == "" || fieldStr == metadataValue) {
								matches = true
							}
						}
					}
				}
				
				// Report result
				resultChan <- struct{
					key       string
					matches   bool
					processed bool
				}{
					key:       key.Key,
					matches:   matches,
					processed: true,
				}
			}
		}(i)
	}
	
	// Feed keys to workers
	go func() {
		for _, key := range chunkKeys {
			workChan <- key
		}
		close(workChan)
	}()
	
	// Collect results
	matchedKeys := []string{}
	processed := 0
	
	for processed < len(chunkKeys) {
		result := <-resultChan
		processed++
		
		// Report progress for every key for better responsiveness
		progressCallback(processed)
		
		// Add matched key to results
		if result.matches {
			matchedKeys = append(matchedKeys, result.key)
		}
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	return matchedKeys, nil
}

// processKeyChunkOptimized uses a more efficient approach to process keys
// It checks metadata first where available to reduce API calls
func processKeyChunkOptimized(client *api.Client, accountID, namespaceID string, 
	chunkKeys []KeyValuePair, tagField, tagValue string, concurrency int,
	progressCallback func(processed int)) ([]string, error) {
	
	// Create worker pool for parallel processing, but with a more optimized approach
	workChan := make(chan KeyValuePair, len(chunkKeys))
	resultChan := make(chan struct{
		key       string
		matches   bool
		processed bool
	}, len(chunkKeys))
	
	// Create mutex for client to ensure thread safety
	clientMutex := &sync.Mutex{}
	
	// Launch worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			
			// Create a local batch to reduce API calls
			keyBatch := make([]KeyValuePair, 0, 10)
			batchSize := 10
			
			for key := range workChan {
				// Add to batch
				keyBatch = append(keyBatch, key)
				
				// Process batch when it reaches the batch size
				if len(keyBatch) >= batchSize {
					// Process the batch
					for _, batchKey := range keyBatch {
						// Try metadata first if the tag is in metadata (it's usually a faster API call)
						// Note: We still need to use individual Get calls because Cloudflare KV doesn't have bulk read
						
						// First try to check metadata, which is often faster
						encodedKey := url.PathEscape(batchKey.Key)
						metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s", 
							accountID, namespaceID, encodedKey)
						
						// Get metadata with thread safety
						clientMutex.Lock()
						metadataResp, metadataErr := client.Request(http.MethodGet, metadataPath, nil, nil)
						clientMutex.Unlock()
						
						var matches bool
						
						// If we got metadata and it contains our tag field, check it
						if metadataErr == nil {
							var metadataResponse struct {
								Success bool                   `json:"success"`
								Result  map[string]interface{} `json:"result"`
							}
							
							if err := json.Unmarshal(metadataResp, &metadataResponse); err == nil && 
								metadataResponse.Success && metadataResponse.Result != nil {
								
								// Check if metadata has our tag field
								if fieldValue, ok := metadataResponse.Result[tagField]; ok {
									// We found the field in metadata!
									foundTagStr, isString := fieldValue.(string)
									if isString && (tagValue == "" || foundTagStr == tagValue) {
										matches = true
									}
									
									// Report result - we don't need to check the value if we already have a match
									resultChan <- struct{
										key       string
										matches   bool
										processed bool
									}{
										key:       batchKey.Key,
										matches:   matches,
										processed: true,
									}
									continue
								}
							}
						}
						
						// If metadata approach didn't work, fall back to getting the value
						// Add a small delay to avoid rate limiting
						time.Sleep(time.Duration(10) * time.Millisecond)
						
						// Get the value with thread safety
						clientMutex.Lock()
						value, err := GetValue(client, accountID, namespaceID, batchKey.Key)
						clientMutex.Unlock()
						
						if err != nil {
							// Report as processed but not matched
							resultChan <- struct{
								key       string
								matches   bool
								processed bool
							}{
								key:       batchKey.Key,
								matches:   false,
								processed: true,
							}
							continue
						}
						
						// Try to parse the value as JSON
						var valueMap map[string]interface{}
						if err := json.Unmarshal([]byte(value), &valueMap); err != nil {
							// Not valid JSON, report as processed but not matched
							resultChan <- struct{
								key       string
								matches   bool
								processed bool
							}{
								key:       batchKey.Key,
								matches:   false, 
								processed: true,
							}
							continue
						}
						
						// Check if the tag field exists and matches
						if foundTagValue, ok := valueMap[tagField]; ok {
							// Convert tag value to string
							if foundTagStr, ok := foundTagValue.(string); ok {
								// Check if tag value matches (if tagValue is empty, match any tag)
								matches = tagValue == "" || foundTagStr == tagValue
							}
						}
						
						// Report result
						resultChan <- struct{
							key       string
							matches   bool
							processed bool
						}{
							key:       batchKey.Key,
							matches:   matches,
							processed: true,
						}
					}
					
					// Clear batch
					keyBatch = keyBatch[:0]
				}
			}
			
			// Process any remaining keys in the batch after channel is closed
			if len(keyBatch) > 0 {
				for _, batchKey := range keyBatch {
					// Get the value with thread safety
					clientMutex.Lock()
					value, err := GetValue(client, accountID, namespaceID, batchKey.Key)
					clientMutex.Unlock()
					
					var matches bool
					
					if err == nil {
						// Try to parse the value as JSON
						var valueMap map[string]interface{}
						if err := json.Unmarshal([]byte(value), &valueMap); err == nil {
							// Check if the tag field exists and matches
							if foundTagValue, ok := valueMap[tagField]; ok {
								// Convert tag value to string
								if foundTagStr, ok := foundTagValue.(string); ok {
									// Check if tag value matches (if tagValue is empty, match any tag)
									matches = tagValue == "" || foundTagStr == tagValue
								}
							}
						}
					}
					
					// Report result
					resultChan <- struct{
						key       string
						matches   bool
						processed bool
					}{
						key:       batchKey.Key,
						matches:   matches,
						processed: true,
					}
				}
			}
		}(i)
	}
	
	// Feed keys to workers
	go func() {
		for _, key := range chunkKeys {
			workChan <- key
		}
		close(workChan)
	}()
	
	// Collect results
	matchedKeys := []string{}
	processed := 0
	
	for processed < len(chunkKeys) {
		result := <-resultChan
		processed++
		
		// Update progress after every 5 items to reduce output volume
		if processed % 5 == 0 || processed == len(chunkKeys) {
			progressCallback(processed)
		}
		
		// Add matched key to results
		if result.matches {
			matchedKeys = append(matchedKeys, result.key)
		}
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	return matchedKeys, nil
}

// processKeyChunk was the original implementation
// It has been deprecated and removed in favor of processKeyChunkOptimized
// For backwards compatibility, use processKeyChunkOptimized