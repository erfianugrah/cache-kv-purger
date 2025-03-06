package kv

import (
	"cache-kv-purger/internal/api"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

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
		matchedKeys, err := processMetadataOnlyChunk(client, accountID, namespaceID, chunkKeys,
			metadataField, metadataValue, func(processed int) {
				totalProcessed = i + processed
				progressCallback(totalKeys, totalProcessed, len(allMatchedKeys), totalKeys)
			})

		if err != nil {
			return allMatchedKeys, fmt.Errorf("error processing chunk %d-%d: %w", i, end-1, err)
		}

		// Get full key-value pairs for matched keys
		for _, keyName := range matchedKeys {
			kvPair, err := GetKeyWithMetadata(client, accountID, namespaceID, keyName)
			if err != nil {
				continue
			}
			allMatchedKeys = append(allMatchedKeys, *kvPair)
		}

		// Update progress
		progressCallback(totalKeys, totalProcessed, len(allMatchedKeys), totalKeys)
	}

	return allMatchedKeys, nil
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
	resultChan := make(chan struct {
		key       string
		matches   bool
		processed bool
	}, len(chunkKeys))

	// Use extremely high concurrency - Cloudflare's API seems to be rate limiting, so we'll try to max out our requests
	// Process all keys in parallel for maximum throughput
	concurrency := len(chunkKeys) // Process every key in its own goroutine for maximum parallelism
	if concurrency > 1000 {       // But cap at 1000 to avoid system resource issues
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
				resultChan <- struct {
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
	resultChan := make(chan struct {
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
									resultChan <- struct {
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
							resultChan <- struct {
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
							resultChan <- struct {
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
						resultChan <- struct {
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
					resultChan <- struct {
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
		if processed%5 == 0 || processed == len(chunkKeys) {
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
