package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"cache-kv-purger/internal/api"
)

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
		index int
		item  BulkWriteItem
		err   error
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
					// kv/export.go (continued)
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
			if progressCallback != nil && processed%10 == 0 { // Update progress every 10 items
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
	errorCount := 0
	errorCountMutex := &sync.Mutex{}
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
					// Use mutex instead of atomic
					errorCountMutex.Lock()
					errorCount++
					errorCountMutex.Unlock()

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

				if len(metadataResponse.Result) > 0 {
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
	errorCountMutex.Lock()
	tooManyErrors := errorCount > len(keys)/2
	errorCountMutex.Unlock()

	if tooManyErrors {
		return metadataMap, firstError
	}

	return metadataMap, nil
}
