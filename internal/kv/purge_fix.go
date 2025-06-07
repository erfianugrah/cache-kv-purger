package kv

import (
	"cache-kv-purger/internal/api"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// StreamingPurgeByTagFixed performs a streaming purge of keys with a specific tag value
// This is much more efficient for large namespaces as it processes in chunks
// This version fixes race conditions using atomic operations and proper synchronization
func StreamingPurgeByTagFixed(client *api.Client, accountID, namespaceID, tagField, tagValue string,
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
		concurrency = 20 // Default concurrency
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
	var totalProcessed int32 // Use atomic counter for thread safety
	var totalDeleted int32   // Use atomic counter for thread safety

	// To improve performance, we'll:
	// 1. Process keys in larger batches
	// 2. Optimize for bulk API operations where possible
	// 3. Delete keys in batches of up to 1000 (API limit)

	// Mutex for synchronizing access to the matched keys slice
	var matchedKeysMutex sync.Mutex
	var allMatchedKeys []string

	// Process in chunks to reduce memory usage
	for i := 0; i < totalKeys; i += chunkSize {
		end := i + chunkSize
		if end > totalKeys {
			end = totalKeys
		}

		chunkKeys := keys[i:end]

		// Process this chunk with optimal batching
		matchedKeys, err := processKeyChunkOptimizedFixed(client, accountID, namespaceID, chunkKeys,
			tagField, tagValue, concurrency, func(processed int) {
				// Update the processed counter atomically
				newProcessed := atomic.AddInt32(&totalProcessed, int32(processed))
				progressCallback(totalKeys, int(newProcessed), int(atomic.LoadInt32(&totalDeleted)), totalKeys)
			})

		if err != nil {
			return int(atomic.LoadInt32(&totalDeleted)), fmt.Errorf("error processing chunk %d-%d: %w", i, end-1, err)
		}

		// Add matched keys to overall list with proper synchronization
		if len(matchedKeys) > 0 {
			matchedKeysMutex.Lock()
			allMatchedKeys = append(allMatchedKeys, matchedKeys...)
			matchedKeysMutex.Unlock()
		}

		// If we've reached a significant number of keys to delete, batch delete them
		if !dryRun && len(allMatchedKeys) >= 1000 {
			matchedKeysMutex.Lock()
			keysToDelete := allMatchedKeys
			// Create a new slice rather than emptying the existing one to avoid race conditions
			allMatchedKeys = make([]string, 0, 1000)
			matchedKeysMutex.Unlock()

			// Delete in batches of 1000 (API limit)
			for j := 0; j < len(keysToDelete); j += 1000 {
				batchEnd := j + 1000
				if batchEnd > len(keysToDelete) {
					batchEnd = len(keysToDelete)
				}

				deleteBatch := keysToDelete[j:batchEnd]
				err = DeleteMultipleValues(client, accountID, namespaceID, deleteBatch)
				if err != nil {
					return int(atomic.LoadInt32(&totalDeleted)), fmt.Errorf("error deleting matched keys in batch: %w", err)
				}

				// Update counts atomically
				atomic.AddInt32(&totalDeleted, int32(len(deleteBatch)))
				progressCallback(totalKeys, int(atomic.LoadInt32(&totalProcessed)),
					int(atomic.LoadInt32(&totalDeleted)), totalKeys)
			}
		}
	}

	// Skip deletion if dry run
	if dryRun {
		// Return the total count of matched keys
		matchedKeysMutex.Lock()
		matchedCount := len(allMatchedKeys)
		matchedKeysMutex.Unlock()
		return matchedCount, nil
	}

	// Delete any remaining matched keys with proper synchronization
	matchedKeysMutex.Lock()
	keysToDelete := allMatchedKeys
	matchedKeysMutex.Unlock()

	if len(keysToDelete) > 0 {
		// Delete in batches of 1000 (API limit)
		for j := 0; j < len(keysToDelete); j += 1000 {
			batchEnd := j + 1000
			if batchEnd > len(keysToDelete) {
				batchEnd = len(keysToDelete)
			}

			deleteBatch := keysToDelete[j:batchEnd]
			err = DeleteMultipleValues(client, accountID, namespaceID, deleteBatch)
			if err != nil {
				return int(atomic.LoadInt32(&totalDeleted)), fmt.Errorf("error deleting matched keys in batch: %w", err)
			}

			// Update counts atomically
			atomic.AddInt32(&totalDeleted, int32(len(deleteBatch)))
			progressCallback(totalKeys, int(atomic.LoadInt32(&totalProcessed)),
				int(atomic.LoadInt32(&totalDeleted)), totalKeys)
		}
	}

	return int(atomic.LoadInt32(&totalDeleted)), nil
}

// PurgeByMetadataOnlyFixed uses a metadata-first approach for better performance with fixed concurrency
// It only checks metadata and doesn't look at values at all
// This version fixes race conditions using atomic operations and proper synchronization
func PurgeByMetadataOnlyFixed(client *api.Client, accountID, namespaceID, metadataField, metadataValue string,
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
	var totalProcessed int32 // Use atomic for thread safety
	var totalMatched int32   // Use atomic for thread safety
	var totalDeleted int32   // Use atomic for thread safety

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
		// startIdx := i // Variable not used

		// Acquire semaphore
		semaphore <- struct{}{}

		wg.Add(1)
		go func(chunkNum int, chunkKeys []KeyValuePair) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done

			// Process this chunk by checking metadata only
			matchedKeys, err := processMetadataOnlyChunkFixed(client, accountID, namespaceID, chunkKeys,
				metadataField, metadataValue, func(processed int) {
					// Update progress atomically
					newProcessed := atomic.AddInt32(&totalProcessed, int32(processed))
					// Use load functions to get the current values safely
					progressCallback(totalKeys, int(newProcessed),
						int(atomic.LoadInt32(&totalMatched)),
						int(atomic.LoadInt32(&totalDeleted)), totalKeys)
				})

			if err != nil {
				errorChan <- fmt.Errorf("error processing chunk %d: %w", chunkNum, err)
				return
			}

			// Send matched keys to channel
			if len(matchedKeys) > 0 {
				matchedKeysChan <- matchedKeys
				// Update matched count atomically
				atomic.AddInt32(&totalMatched, int32(len(matchedKeys)))
			}
		}(chunkNum, chunkKeys)
	}

	// Use a separate goroutine to wait for all workers to finish
	go func() {
		wg.Wait()
		close(matchedKeysChan)
		close(errorChan)
	}()

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return int(atomic.LoadInt32(&totalDeleted)), err
		}
	}

	// Collect all matched keys with proper synchronization
	var allMatchedKeys []string
	for matchedChunk := range matchedKeysChan {
		allMatchedKeys = append(allMatchedKeys, matchedChunk...)
		// Progress callback already updated in the worker goroutines
	}

	// If dry run, just return the count
	if dryRun {
		return len(allMatchedKeys), nil
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
				return int(atomic.LoadInt32(&totalDeleted)), fmt.Errorf("error deleting batch of keys: %w", err)
			}

			// Update deleted count atomically
			atomic.AddInt32(&totalDeleted, int32(len(batch)))
			progressCallback(totalKeys, int(atomic.LoadInt32(&totalProcessed)),
				int(atomic.LoadInt32(&totalMatched)),
				int(atomic.LoadInt32(&totalDeleted)), totalKeys)
		}
	}

	return int(atomic.LoadInt32(&totalDeleted)), nil
}

// processMetadataOnlyChunkFixed processes a chunk of keys by checking their metadata
// This version fixes race conditions using proper synchronization
func processMetadataOnlyChunkFixed(client *api.Client, accountID, namespaceID string,
	chunkKeys []KeyValuePair, metadataField, metadataValue string,
	progressCallback func(processed int)) ([]string, error) {

	// Create a mutex for thread safety
	var mu sync.Mutex
	matchedKeys := []string{}
	processed := 0

	// Process each key in the chunk checking for metadata
	for _, key := range chunkKeys {
		processedIncrement := 1 // Default increment

		// First check if key already has metadata from the list response
		if key.Metadata != nil {
			// Check if metadata contains our field
			if fieldValue, ok := (*key.Metadata)[metadataField]; ok {
				// We found the field in metadata!
				fieldStr, isString := fieldValue.(string)
				if isString && (metadataValue == "" || fieldStr == metadataValue) {
					mu.Lock()
					matchedKeys = append(matchedKeys, key.Key)
					mu.Unlock()
				}
				// Update progress after each key
				progressCallback(processedIncrement)
				processed += processedIncrement
				continue // Already checked metadata, no need for API call
			}
		}

		// If we get here, metadata was not in the list response or didn't have our field
		// Fall back to making an API call for this key's metadata
		encodedKey := url.PathEscape(key.Key)
		metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s",
			accountID, namespaceID, encodedKey)

		// Get metadata
		metadataResp, metadataErr := client.Request(http.MethodGet, metadataPath, nil, nil)

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
						mu.Lock()
						matchedKeys = append(matchedKeys, key.Key)
						mu.Unlock()
					}
				}
			}
		}

		// Update progress after each key with API call
		progressCallback(processedIncrement)
		processed += processedIncrement
	}

	return matchedKeys, nil
}

// processKeyChunkOptimizedFixed uses a more efficient approach to process keys
// It checks metadata first where available to reduce API calls
// This version fixes race conditions using proper synchronization
func processKeyChunkOptimizedFixed(client *api.Client, accountID, namespaceID string,
	chunkKeys []KeyValuePair, tagField, tagValue string, concurrency int,
	progressCallback func(processed int)) ([]string, error) {

	// Create a mutex for thread safety
	var mu sync.Mutex
	matchedKeys := []string{}

	// Create worker pool for parallel processing
	workChan := make(chan KeyValuePair, len(chunkKeys))
	resultChan := make(chan struct {
		key       string
		matches   bool
		processed int
	}, len(chunkKeys))

	// Launch worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for key := range workChan {
				var matches bool
				processedCount := 1 // Default processing increment

				// First check if metadata is already in the list response
				if key.Metadata != nil {
					// Check if metadata has our tag field
					if fieldValue, ok := (*key.Metadata)[tagField]; ok {
						// We found the field in metadata!
						foundTagStr, isString := fieldValue.(string)
						if isString && (tagValue == "" || foundTagStr == tagValue) {
							matches = true
						}

						// Report result - we found metadata in the list response
						resultChan <- struct {
							key       string
							matches   bool
							processed int
						}{
							key:       key.Key,
							matches:   matches,
							processed: processedCount,
						}
						continue
					}
				}

				// If metadata wasn't in list response or didn't have our field,
				// try a separate API call for metadata
				encodedKey := url.PathEscape(key.Key)
				metadataPath := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/metadata/%s",
					accountID, namespaceID, encodedKey)

				// Get metadata
				metadataResp, metadataErr := client.Request(http.MethodGet, metadataPath, nil, nil)

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

							// Report result - we don't need to check the value if we found metadata
							resultChan <- struct {
								key       string
								matches   bool
								processed int
							}{
								key:       key.Key,
								matches:   matches,
								processed: processedCount,
							}
							continue
						}
					}
				}

				// If we still didn't find metadata, fall back to getting the value
				// Add a small delay to avoid rate limiting
				time.Sleep(time.Duration(10) * time.Millisecond)

				// Get the value
				value, err := GetValue(client, accountID, namespaceID, key.Key)
				if err != nil {
					// Report as processed but not matched
					resultChan <- struct {
						key       string
						matches   bool
						processed int
					}{
						key:       key.Key,
						matches:   false,
						processed: processedCount,
					}
					continue
				}

				// Try to parse the value as JSON
				var valueMap map[string]interface{}
				if err := json.Unmarshal([]byte(value), &valueMap); err != nil {
					// Not valid JSON, report as processed but not matched
					resultChan <- struct {
						key       string
						matches   bool
						processed int
					}{
						key:       key.Key,
						matches:   false,
						processed: processedCount,
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
				resultChan <- struct {
					key       string
					matches   bool
					processed int
				}{
					key:       key.Key,
					matches:   matches,
					processed: processedCount,
				}
			}
		}()
	}

	// Feed keys to workers
	go func() {
		for _, key := range chunkKeys {
			workChan <- key
		}
		close(workChan)
	}()

	// Collect results
	processed := 0
	pendingItems := len(chunkKeys)

	for pendingItems > 0 {
		result := <-resultChan
		pendingItems--

		// Update processing progress
		processed += result.processed

		// Batch progress updates to reduce callback overhead
		if result.processed > 0 && (processed%10 == 0 || pendingItems == 0) {
			progressCallback(result.processed)
		}

		// Add matched key to results
		if result.matches {
			mu.Lock()
			matchedKeys = append(matchedKeys, result.key)
			mu.Unlock()
		}
	}

	// Wait for all workers to finish
	wg.Wait()

	return matchedKeys, nil
}
