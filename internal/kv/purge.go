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
		concurrency = 20 // Default concurrency
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
		matchedKeynames, err := processMetadataOnlyChunk(client, accountID, namespaceID, chunkKeys,
			metadataField, metadataValue, func(processed int) {
				totalProcessed = i + processed
				progressCallback(totalKeys, totalProcessed, len(allMatchedKeys), totalKeys)
			})

		if err != nil {
			return allMatchedKeys, fmt.Errorf("error processing chunk %d-%d: %w", i, end-1, err)
		}

		// Get full key-value pairs for matched keys
		for _, keyName := range matchedKeynames {
			// Try to find the original key in our chunk to preserve metadata
			var matchedKey *KeyValuePair
			for _, key := range chunkKeys {
				if key.Key == keyName {
					keyCopy := key // Make a copy to avoid slice reference issues
					matchedKey = &keyCopy
					break
				}
			}

			// If we found the key with metadata in our chunk, use it directly
			if matchedKey != nil {
				allMatchedKeys = append(allMatchedKeys, *matchedKey)
			} else {
				// Otherwise get the key with metadata (fallback, should rarely happen)
				kvPair, err := GetKeyWithMetadata(client, accountID, namespaceID, keyName)
				if err != nil {
					// Just log and continue if we can't get full details for this key
					continue
				}
				allMatchedKeys = append(allMatchedKeys, *kvPair)
			}
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

	// Process all keys in memory using metadata from list response when available
	var matchingKeys []string

	for _, key := range keys {
		// First check if metadata is already available in the response
		if key.Metadata != nil {
			// Check if the field exists and matches the value
			fieldValue, exists := (*key.Metadata)[metadataField]
			if exists {
				// Check if the field value matches (empty value matches anything)
				fieldValueStr := fmt.Sprintf("%v", fieldValue)
				if metadataValue == "" || fieldValueStr == metadataValue {
					matchingKeys = append(matchingKeys, key.Key)
				}
			}
		}

		// Update progress
		progressCallback(totalKeys, totalKeys, len(matchingKeys), 0, totalKeys)
	}

	// If we still need to check keys without metadata, use FetchAllMetadata for remaining keys
	// This is an optimization when list response doesn't include metadata
	if len(matchingKeys) == 0 {
		// Fetch metadata for keys without metadata in response
		var keysNeedingMetadata []KeyValuePair
		for _, key := range keys {
			if key.Metadata == nil {
				keysNeedingMetadata = append(keysNeedingMetadata, key)
			}
		}

		if len(keysNeedingMetadata) > 0 {
			metadataProgress := func(fetched, total int) {
				progressCallback(totalKeys, totalKeys, len(matchingKeys)+fetched/2, 0, total)
			}

			// Use the FetchAllMetadata from export.go
			allMetadata, err := FetchAllMetadata(client, accountID, namespaceID, keysNeedingMetadata, concurrency, metadataProgress)
			if err != nil {
				// Continue with the keys we already matched from the list response
				// Just log the error as this is a fallback mechanism
				fmt.Printf("Warning: Failed to fetch additional metadata: %v\n", err)
			} else {
				// Check additional keys with fetched metadata
				for _, key := range keysNeedingMetadata {
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
				}
			}
		}
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
	var mu sync.Mutex
	matchedKeys := []string{}
	processed := 0

	// Process each key in the chunk checking for metadata
	for _, key := range chunkKeys {
		processed++

		// Report progress
		progressCallback(processed)

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
	}

	return matchedKeys, nil
}

// processKeyChunkOptimized uses a more efficient approach to process keys
// It checks metadata first where available to reduce API calls
func processKeyChunkOptimized(client *api.Client, accountID, namespaceID string,
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
		processed bool
	}, len(chunkKeys))

	// Launch worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for key := range workChan {
				var matches bool

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
							processed bool
						}{
							key:       key.Key,
							matches:   matches,
							processed: true,
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
								processed bool
							}{
								key:       key.Key,
								matches:   matches,
								processed: true,
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
					resultChan <- struct {
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
					key:       key.Key,
					matches:   matches,
					processed: true,
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

	for processed < len(chunkKeys) {
		result := <-resultChan
		processed++

		// Update progress after every 5 items to reduce output volume
		if processed%5 == 0 || processed == len(chunkKeys) {
			progressCallback(processed)
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

// Note: FetchAllMetadata function is defined in export.go and not duplicated here
