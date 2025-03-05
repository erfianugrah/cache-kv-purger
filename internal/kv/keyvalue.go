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

// FilterKeysByMetadata filters keys based on their metadata values
// metadataField: the name of the metadata field to check
// metadataValue: the value to match against (if empty, any non-empty value matches)
func FilterKeysByMetadata(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, progressCallback func(fetched, processed, total int)) ([]KeyValuePair, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if metadataField == "" {
		return nil, fmt.Errorf("metadata field name is required")
	}
	
	// List all keys first
	fetchProgressCallback := func(fetched, total int) {
		if progressCallback != nil {
			progressCallback(fetched, 0, total)
		}
	}
	
	allKeys, err := ListAllKeys(client, accountID, namespaceID, fetchProgressCallback)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	
	if len(allKeys) == 0 {
		return []KeyValuePair{}, nil // Return empty slice, not nil
	}
	
	// For each key, we need to fetch the metadata
	var matchingKeys []KeyValuePair
	for i, key := range allKeys {
		// Get key with metadata
		keyWithMeta, err := GetKeyWithMetadata(client, accountID, namespaceID, key.Key)
		if err != nil {
			// Log error but continue
			fmt.Printf("Warning: failed to get metadata for key '%s': %v\n", key.Key, err)
			continue
		}
		
		// Update progress
		if progressCallback != nil {
			progressCallback(len(allKeys), i+1, len(allKeys))
		}
		
		// Check if the key has metadata
		if keyWithMeta.Metadata == nil {
			continue
		}
		
		// Check if the metadata has the field we're looking for
		metadataMap := *keyWithMeta.Metadata
		fieldValue, exists := metadataMap[metadataField]
		if !exists {
			continue
		}
		
		// If metadataValue is empty, we match any non-empty value
		// Otherwise, we need to match the specific value
		if metadataValue == "" || fmt.Sprintf("%v", fieldValue) == metadataValue {
			matchingKeys = append(matchingKeys, *keyWithMeta)
		}
	}
	
	return matchingKeys, nil
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

// processMetadataChunk processes a chunk of keys and returns those with matching metadata
func processMetadataChunk(client *api.Client, accountID, namespaceID string, 
	chunkKeys []KeyValuePair, metadataField, metadataValue string, concurrency int,
	progressCallback func(processed, matched int)) ([]KeyValuePair, error) {
	
	// Create worker pool for parallel processing
	workChan := make(chan KeyValuePair, len(chunkKeys))
	resultChan := make(chan struct {
		keyPair   *KeyValuePair
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
			
			for key := range workChan {
				// Add a small delay between workers to prevent API rate limiting
				time.Sleep(time.Duration(workerNum*5) * time.Millisecond)
				
				// Only get the metadata to make the process faster
				encodedKey := url.PathEscape(key.Key)
				metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s", accountID, namespaceID, encodedKey)
				
				// Thread-safe request for metadata
				clientMutex.Lock()
				metadataRespBody, err := client.Request(http.MethodGet, metadataPath, nil, nil)
				clientMutex.Unlock()
				
				// Default to not matched
				matches := false
				var kvPair *KeyValuePair
				
				if err == nil {
					// Parse the metadata response
					var metadataResponse struct {
						Success bool                   `json:"success"`
						Errors  []api.Error            `json:"errors,omitempty"`
						Result  map[string]interface{} `json:"result,omitempty"`
					}
					
					if err := json.Unmarshal(metadataRespBody, &metadataResponse); err == nil && metadataResponse.Success {
						if metadataResponse.Result != nil && len(metadataResponse.Result) > 0 {
							// Create metadata object
							metadataObj := KeyValueMetadata(metadataResponse.Result)
							
							// Check if metadata has the field we're looking for
							fieldValue, exists := metadataResponse.Result[metadataField]
							if exists {
								// Check if value matches
								if metadataValue == "" || fmt.Sprintf("%v", fieldValue) == metadataValue {
									matches = true
									
									// Only if we have a match, get the value (this saves API calls)
									if matches {
										value, err := GetValue(client, accountID, namespaceID, key.Key)
										if err == nil {
											kvPair = &KeyValuePair{
												Key:      key.Key,
												Value:    value,
												Metadata: &metadataObj,
											}
										}
									}
								}
							}
						}
					}
				}
				
				// If we have a match but couldn't get the value, still report the key
				if matches && kvPair == nil {
					kvPair = &KeyValuePair{
						Key: key.Key,
					}
				}
				
				// Report result
				resultChan <- struct {
					keyPair   *KeyValuePair
					matches   bool
					processed bool
				}{
					keyPair:   kvPair,
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
	matchedKeys := []KeyValuePair{}
	processed := 0
	matched := 0
	
	for processed < len(chunkKeys) {
		result := <-resultChan
		processed++
		
		// Add matched key to results
		if result.matches && result.keyPair != nil {
			matchedKeys = append(matchedKeys, *result.keyPair)
			matched++
		}
		
		// Update progress
		if processed % 10 == 0 || processed == len(chunkKeys) {
			progressCallback(processed, matched)
		}
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	return matchedKeys, nil
}

// DeleteKeysByMetadata deletes keys based on their metadata values
func DeleteKeysByMetadata(client *api.Client, accountID, namespaceID, metadataField, metadataValue string, 
	batchSize int, dryRun bool, progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)) (int, error) {
	
	if accountID == "" {
		return 0, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return 0, fmt.Errorf("namespace ID is required")
	}
	if metadataField == "" {
		return 0, fmt.Errorf("metadata field is required")
	}
	
	// Default batch size
	if batchSize <= 0 {
		batchSize = 100
	}
	
	// Default concurrency for the streaming filter
	concurrency := 10
	
	// Use the streaming filter to find matching keys
	matchedKeys, err := StreamingFilterKeysByMetadata(client, accountID, namespaceID, 
		metadataField, metadataValue, batchSize, concurrency, 
		func(keysFetched, keysProcessed, keysMatched, total int) {
			if progressCallback != nil {
				progressCallback(keysFetched, keysProcessed, keysMatched, 0, total)
			}
		})
	
	if err != nil {
		return 0, fmt.Errorf("failed to filter keys by metadata: %w", err)
	}
	
	if len(matchedKeys) == 0 {
		return 0, nil // No keys to delete
	}
	
	// If dry run, just return the count
	if dryRun {
		return len(matchedKeys), nil
	}
	
	// Extract keys for deletion
	keysToDelete := make([]string, len(matchedKeys))
	for i, key := range matchedKeys {
		keysToDelete[i] = key.Key
	}
	
	// Delete in batches
	totalDeleted := 0
	for i := 0; i < len(keysToDelete); i += batchSize {
		end := i + batchSize
		if end > len(keysToDelete) {
			end = len(keysToDelete)
		}
		
		batch := keysToDelete[i:end]
		
		// Delete this batch
		err := DeleteMultipleValues(client, accountID, namespaceID, batch)
		if err != nil {
			// Continue with other batches even if one fails
			fmt.Printf("Warning: failed to delete batch %d-%d: %v\n", i, end-1, err)
		} else {
			totalDeleted += len(batch)
		}
		
		// Update progress
		if progressCallback != nil {
			progressCallback(len(keysToDelete), len(keysToDelete), len(matchedKeys), totalDeleted, len(keysToDelete))
		}
	}
	
	return totalDeleted, nil
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
	
	// Process in chunks to reduce memory usage
	for i := 0; i < totalKeys; i += chunkSize {
		end := i + chunkSize
		if end > totalKeys {
			end = totalKeys
		}
		
		chunkKeys := keys[i:end]
		
		// Process this chunk
		matchedKeys, err := processKeyChunk(client, accountID, namespaceID, chunkKeys, 
			tagField, tagValue, concurrency, func(processed int) {
				totalProcessed = i + processed
				progressCallback(totalKeys, totalProcessed, totalDeleted, totalKeys)
			})
		
		if err != nil {
			return totalDeleted, fmt.Errorf("error processing chunk %d-%d: %w", i, end-1, err)
		}
		
		// Update total matched
		totalMatched += len(matchedKeys)
		
		// Skip deletion if dry run
		if dryRun {
			continue
		}
		
		// Delete matched keys (if any)
		if len(matchedKeys) > 0 {
			err = DeleteMultipleValues(client, accountID, namespaceID, matchedKeys)
			if err != nil {
				return totalDeleted, fmt.Errorf("error deleting matched keys in chunk %d-%d: %w", i, end-1, err)
			}
			
			// Update counts
			totalDeleted += len(matchedKeys)
			progressCallback(totalKeys, totalProcessed, totalDeleted, totalKeys)
		}
	}
	
	if dryRun {
		return totalMatched, nil // Return how many would have been deleted
	}
	
	return totalDeleted, nil
}

// processKeyChunk fetches values for a chunk of keys and returns keys that match the tag
func processKeyChunk(client *api.Client, accountID, namespaceID string, 
	chunkKeys []KeyValuePair, tagField, tagValue string, concurrency int,
	progressCallback func(processed int)) ([]string, error) {
	
	// Create worker pool for parallel processing
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
			
			for key := range workChan {
				// Add a small delay between workers to prevent API rate limiting
				time.Sleep(time.Duration(workerNum*5) * time.Millisecond)
				
				// Get the value
				clientMutex.Lock()
				value, err := GetValue(client, accountID, namespaceID, key.Key)
				clientMutex.Unlock()
				
				if err != nil {
					// Report as processed but not matched
					resultChan <- struct{
						key       string
						matches   bool
						processed bool
					}{
						key:       key.Key,
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
						key:       key.Key,
						matches:   false, 
						processed: true,
					}
					continue
				}
				
				// Check if the tag field exists and matches
				foundTagValue, ok := valueMap[tagField]
				if !ok {
					// Tag field not found
					resultChan <- struct{
						key       string
						matches   bool
						processed bool
					}{
						key:       key.Key,
						matches:   false,
						processed: true,
					}
					continue
				}
				
				// Convert tag value to string
				foundTagStr, ok := foundTagValue.(string)
				if !ok {
					// Tag value not a string
					resultChan <- struct{
						key       string
						matches   bool
						processed bool
					}{
						key:       key.Key,
						matches:   false,
						processed: true,
					}
					continue
				}
				
				// Check if tag value matches (if tagValue is empty, match any tag)
				matches := tagValue == "" || foundTagStr == tagValue
				
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
		
		// Update progress
		if processed % 10 == 0 || processed == len(chunkKeys) {
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