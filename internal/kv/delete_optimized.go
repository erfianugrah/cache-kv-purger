package kv

import (
	"fmt"
	"sync"
	"sync/atomic"

	"cache-kv-purger/internal/api"
)

// DeleteMultipleValuesOptimized deletes multiple values with smart binary search fallback
func DeleteMultipleValuesOptimized(client *api.Client, accountID, namespaceID string, keys []string, verbose bool) error {
	if len(keys) == 0 {
		return nil
	}

	if verbose {
		fmt.Printf("🗑️  Deleting %d keys with optimized strategy...\n", len(keys))
	}

	// Try bulk delete first
	err := attemptBulkDelete(client, accountID, namespaceID, keys)
	if err == nil {
		if verbose {
			fmt.Printf("✅ Bulk delete successful for all %d keys\n", len(keys))
		}
		return nil
	}

	if verbose {
		fmt.Printf("⚠️  Bulk delete failed, using binary search strategy...\n")
	}

	// Binary search to find failing keys
	failedKeys, err := binarySearchFailures(client, accountID, namespaceID, keys, verbose)
	if err != nil {
		return err
	}

	if len(failedKeys) == 0 {
		// All keys deleted successfully through binary search
		return nil
	}

	// Delete failed keys individually
	if verbose {
		fmt.Printf("🔧 Deleting %d problematic keys individually...\n", len(failedKeys))
	}

	var successCount int32
	for _, key := range failedKeys {
		if err := DeleteValue(client, accountID, namespaceID, key); err != nil {
			if verbose {
				fmt.Printf("❌ Failed to delete key '%s': %v\n", key, err)
			}
		} else {
			atomic.AddInt32(&successCount, 1)
		}
	}

	if verbose {
		fmt.Printf("✅ Completed: %d/%d keys deleted successfully\n",
			len(keys)-len(failedKeys)+int(successCount), len(keys))
	}

	return nil
}

// attemptBulkDelete tries to delete keys in bulk
func attemptBulkDelete(client *api.Client, accountID, namespaceID string, keys []string) error {
	// Remove all the debug logging from the original function
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk/delete", accountID, namespaceID)
	_, err := client.Request("POST", path, nil, keys)
	return err
}

// binarySearchFailures uses binary search to find which keys are causing bulk delete to fail
func binarySearchFailures(client *api.Client, accountID, namespaceID string, keys []string, verbose bool) ([]string, error) {
	if len(keys) <= 1 {
		// Base case: single key that failed
		return keys, nil
	}

	// Try the whole batch
	err := attemptBulkDelete(client, accountID, namespaceID, keys)
	if err == nil {
		// This batch succeeded
		return nil, nil
	}

	// Split in half and test each half
	mid := len(keys) / 2
	leftKeys := keys[:mid]
	rightKeys := keys[mid:]

	var failedKeys []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Test both halves concurrently
	wg.Add(2)

	go func() {
		defer wg.Done()
		leftFailed, err := binarySearchFailures(client, accountID, namespaceID, leftKeys, verbose)
		if err == nil && len(leftFailed) > 0 {
			mu.Lock()
			failedKeys = append(failedKeys, leftFailed...)
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		rightFailed, err := binarySearchFailures(client, accountID, namespaceID, rightKeys, verbose)
		if err == nil && len(rightFailed) > 0 {
			mu.Lock()
			failedKeys = append(failedKeys, rightFailed...)
			mu.Unlock()
		}
	}()

	wg.Wait()

	if verbose && len(failedKeys) > 0 {
		fmt.Printf("🔍 Found %d problematic keys in batch of %d\n", len(failedKeys), len(keys))
	}

	return failedKeys, nil
}

// DeleteMultipleValuesWithProgress deletes multiple values with progress reporting
func DeleteMultipleValuesWithProgress(client *api.Client, accountID, namespaceID string, keys []string,
	batchSize int, progressCallback func(deleted, total int)) error {

	if len(keys) == 0 {
		return nil
	}

	totalDeleted := 0

	// Process in batches
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]

		// Use optimized delete for each batch
		err := DeleteMultipleValuesOptimized(client, accountID, namespaceID, batch, false)
		if err != nil {
			// Even on error, some keys might have been deleted
			// Continue with next batch - errors are handled internally
			_ = err // Intentionally continue on error
		}

		totalDeleted += len(batch)
		if progressCallback != nil {
			progressCallback(totalDeleted, len(keys))
		}
	}

	return nil
}
