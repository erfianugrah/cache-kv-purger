package kv

import (
	"cache-kv-purger/internal/api"
	"fmt"
	"sync"
)

// WriteMultipleValuesInBatches writes multiple values to a KV namespace in batches
// This is useful when you have a large number of items to write
func WriteMultipleValuesInBatches(client *api.Client, accountID, namespaceID string, items []BulkWriteItem, batchSize int, progressCallback func(completed, total int)) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	if batchSize <= 0 {
		batchSize = 10000 // Maximum batch size supported by API
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
		batchSize = 10000 // Maximum batch size supported by API
	} else if batchSize > 10000 {
		batchSize = 10000 // Max batch size supported by API
	}

	// Set reasonable concurrency
	if concurrency <= 0 {
		concurrency = 20 // Default concurrency
	}
	if concurrency > 100 {
		concurrency = 100 // Cap concurrency to avoid overwhelming API
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

// DeleteMultipleValuesInBatches deletes multiple values from a KV namespace in batches
// This is the sequential version that processes batches one at a time
func DeleteMultipleValuesInBatches(client *api.Client, accountID, namespaceID string, keys []string, batchSize int, progressCallback func(completed, total int)) error {
	if len(keys) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = 10000 // Maximum batch size supported by API
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

// DeleteMultipleValuesConcurrently deletes multiple values from a KV namespace using concurrent batch operations
// This is optimized for high throughput with a high API rate limit
func DeleteMultipleValuesConcurrently(client *api.Client, accountID, namespaceID string, keys []string, batchSize int, concurrency int, progressCallback func(completed, total int)) (int, []error) {
	if len(keys) == 0 {
		return 0, nil
	}

	if batchSize <= 0 {
		batchSize = 10000 // Maximum batch size supported by API
	}

	// Set reasonable concurrency
	if concurrency <= 0 {
		concurrency = 20 // Default concurrency
	}
	if concurrency > 100 {
		concurrency = 100 // Cap concurrency to avoid overwhelming API
	}

	// Simple progress reporting if none provided
	if progressCallback == nil {
		progressCallback = func(completed, total int) {}
	}

	totalItems := len(keys)

	// Create work items for all batches
	type batchWork struct {
		batchIndex int
		batchItems []string
	}

	var batches []batchWork
	for i := 0; i < totalItems; i += batchSize {
		end := i + batchSize
		if end > totalItems {
			end = totalItems
		}

		batch := keys[i:end]
		batches = append(batches, batchWork{
			batchIndex: i / batchSize,
			batchItems: batch,
		})
	}

	// Verbose logging about how many batches we're creating
	fmt.Printf("[DEBUG] Created %d batches for %d keys with batch size %d\n", len(batches), totalItems, batchSize)

	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex int
		success    bool
		err        error
	}

	resultChan := make(chan batchResult, len(batches))

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, concurrency)

	// Process all batches
	for _, batch := range batches {
		// Acquire semaphore slot (or wait if at capacity)
		sem <- struct{}{}

		// Launch a goroutine to process this batch
		go func(b batchWork) {
			defer func() { <-sem }() // Release semaphore when done

			fmt.Printf("[VERBOSE] Processing batch %d with %d keys\n", b.batchIndex+1, len(b.batchItems))

			// Delete this batch
			err := DeleteMultipleValues(client, accountID, namespaceID, b.batchItems)

			// Send result back through channel
			if err != nil {
				fmt.Printf("[ERROR] Batch %d failed: %v\n", b.batchIndex+1, err)
				resultChan <- batchResult{
					batchIndex: b.batchIndex,
					success:    false,
					err:        fmt.Errorf("batch %d failed: %w", b.batchIndex+1, err),
				}
				return
			}

			fmt.Printf("[VERBOSE] Batch %d completed successfully\n", b.batchIndex+1)
			resultChan <- batchResult{
				batchIndex: b.batchIndex,
				success:    true,
				err:        nil,
			}
		}(batch)
	}

	// Collect results
	successCount := 0
	var errors []error

	// Track progress for callback
	completed := 0

	// Collect results from all batches
	for i := 0; i < len(batches); i++ {
		result := <-resultChan

		// Track successful batches
		if result.success {
			successCount += len(batches[result.batchIndex].batchItems)
		} else if result.err != nil {
			errors = append(errors, result.err)
		}

		// Update progress
		completed++

		// Call progress callback
		progressCallback(completed, len(batches))
		fmt.Printf("[DEBUG] Completed %d/%d batches, success count: %d\n", completed, len(batches), successCount)
	}

	return successCount, errors
}
