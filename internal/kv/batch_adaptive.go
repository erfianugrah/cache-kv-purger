package kv

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// AdaptiveBatchOptions configures adaptive batch processing
type AdaptiveBatchOptions struct {
	InitialWorkers   int
	MinWorkers       int
	MaxWorkers       int
	BatchSize        int
	ProgressCallback func(processed, total int, workers int)
}

// DefaultAdaptiveBatchOptions returns sensible defaults
func DefaultAdaptiveBatchOptions() *AdaptiveBatchOptions {
	return &AdaptiveBatchOptions{
		InitialWorkers: 10,
		MinWorkers:     1,
		MaxWorkers:     100,
		BatchSize:      100,
	}
}

// DeleteMultipleValuesAdaptive deletes values with adaptive concurrency
func DeleteMultipleValuesAdaptive(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, options *AdaptiveBatchOptions) (int, error) {

	if len(keys) == 0 {
		return 0, nil
	}

	if options == nil {
		options = DefaultAdaptiveBatchOptions()
	}

	// Create work items
	type deleteWork struct {
		keys  []string
		index int
	}

	// Create adaptive worker pool
	workerFunc := func(ctx context.Context, work interface{}) (interface{}, error) {
		dw := work.(deleteWork)

		// Try bulk delete first
		err := attemptBulkDelete(client, accountID, namespaceID, dw.keys)
		if err == nil {
			return len(dw.keys), nil
		}

		// On failure, use optimized deletion with binary search
		successCount := 0
		err = DeleteMultipleValuesOptimized(client, accountID, namespaceID, dw.keys, false)
		if err == nil {
			successCount = len(dw.keys)
		} else {
			// Count partial successes
			for _, key := range dw.keys {
				if err := DeleteValue(client, accountID, namespaceID, key); err == nil {
					successCount++
				}
			}
		}

		return successCount, nil
	}

	pool := common.NewAdaptiveWorkerPool(ctx, options.MinWorkers, options.MaxWorkers, workerFunc)
	defer pool.Close()

	// Submit work in batches
	totalBatches := (len(keys) + options.BatchSize - 1) / options.BatchSize
	for i := 0; i < len(keys); i += options.BatchSize {
		end := i + options.BatchSize
		if end > len(keys) {
			end = len(keys)
		}

		work := deleteWork{
			keys:  keys[i:end],
			index: i / options.BatchSize,
		}

		if err := pool.Submit(work); err != nil {
			return 0, err
		}
	}

	// Collect results
	var totalDeleted int32
	processedBatches := 0

	// Start result collector
	done := make(chan bool)
	go func() {
		for processedBatches < totalBatches {
			select {
			case result := <-pool.Results():
				if count, ok := result.(int); ok {
					atomic.AddInt32(&totalDeleted, int32(count))
				}
				processedBatches++

				if options.ProgressCallback != nil {
					// Get current worker count from the pool
					currentWorkers := options.InitialWorkers // Simplified for now
					options.ProgressCallback(int(atomic.LoadInt32(&totalDeleted)), len(keys), currentWorkers)
				}

			case err := <-pool.Errors():
				if err != nil {
					// Log error but continue processing
					fmt.Printf("Batch deletion error: %v\n", err)
				}
				processedBatches++

			case <-ctx.Done():
				done <- false
				return
			}
		}
		done <- true
	}()

	// Wait for completion
	success := <-done
	if !success {
		return int(atomic.LoadInt32(&totalDeleted)), ctx.Err()
	}

	return int(atomic.LoadInt32(&totalDeleted)), nil
}

// ListAllKeysAdaptive lists all keys with adaptive concurrency
func ListAllKeysAdaptive(ctx context.Context, client *api.Client, accountID, namespaceID string,
	options *ListKeysOptions, adaptiveOpts *AdaptiveBatchOptions) ([]KeyValuePair, error) {

	if options == nil {
		options = &ListKeysOptions{
			Limit: 1000,
		}
	}

	if adaptiveOpts == nil {
		adaptiveOpts = &AdaptiveBatchOptions{
			InitialWorkers: 1, // Listing is sequential due to cursor
			MinWorkers:     1,
			MaxWorkers:     1,
			BatchSize:      1000,
		}
	}

	// For listing, we can't parallelize due to cursor-based pagination
	// But we can use adaptive concurrency for metadata fetching if needed
	allKeys := make([]KeyValuePair, 0, options.Limit*10)
	var cursor string

	for {
		options.Cursor = cursor

		result, err := ListKeysWithOptions(client, accountID, namespaceID, options)
		if err != nil {
			return nil, err
		}

		allKeys = append(allKeys, result.Keys...)

		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor

		if adaptiveOpts.ProgressCallback != nil {
			adaptiveOpts.ProgressCallback(len(allKeys), -1, 1) // -1 indicates unknown total
		}
	}

	return allKeys, nil
}

// BatchFetchMetadataAdaptive fetches metadata with adaptive concurrency
func BatchFetchMetadataAdaptive(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, options *AdaptiveBatchOptions) (map[string]*KeyValueMetadata, error) {

	if len(keys) == 0 {
		return make(map[string]*KeyValueMetadata), nil
	}

	if options == nil {
		options = &AdaptiveBatchOptions{
			InitialWorkers: 20,
			MinWorkers:     5,
			MaxWorkers:     100,
			BatchSize:      1, // One key per work item for metadata
		}
	}

	// Create result map
	results := make(map[string]*KeyValueMetadata)
	var resultMu sync.Mutex

	// Worker function
	workerFunc := func(ctx context.Context, work interface{}) (interface{}, error) {
		key := work.(string)

		kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, key)
		if err != nil {
			return nil, err
		}

		if kvp != nil && kvp.Metadata != nil {
			resultMu.Lock()
			results[key] = kvp.Metadata
			resultMu.Unlock()
		}

		return key, nil
	}

	// Create adaptive pool
	pool := common.NewAdaptiveWorkerPool(ctx, options.MinWorkers, options.MaxWorkers, workerFunc)
	defer pool.Close()

	// Submit all keys
	for _, key := range keys {
		if err := pool.Submit(key); err != nil {
			return results, err
		}
	}

	// Collect results
	processed := 0
	for processed < len(keys) {
		select {
		case <-pool.Results():
			processed++
			if options.ProgressCallback != nil {
				// Get current worker count
				currentWorkers := options.InitialWorkers // Simplified for now
				options.ProgressCallback(processed, len(keys), currentWorkers)
			}

		case <-pool.Errors():
			processed++
			// Continue on error

		case <-ctx.Done():
			return results, ctx.Err()
		}
	}

	return results, nil
}

// PurgeKeysAdaptive purges keys with adaptive concurrency
func PurgeKeysAdaptive(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, options *AdaptiveBatchOptions) (int, error) {

	// Use the adaptive deletion function
	return DeleteMultipleValuesAdaptive(ctx, client, accountID, namespaceID, keys, options)
}

// AdaptiveConcurrencyDemo demonstrates adaptive concurrency
func AdaptiveConcurrencyDemo(client *api.Client, accountID, namespaceID string, keyCount int) {
	fmt.Println("🎯 Adaptive Concurrency Demo")
	fmt.Println("============================")

	// List some keys first
	listOpts := &ListKeysOptions{
		Limit: keyCount,
	}

	fmt.Printf("Listing %d keys...\n", keyCount)
	keys, err := ListKeysWithOptions(client, accountID, namespaceID, listOpts)
	if err != nil {
		fmt.Printf("Error listing keys: %v\n", err)
		return
	}

	if len(keys.Keys) == 0 {
		fmt.Println("No keys found")
		return
	}

	// Extract key names
	keyNames := make([]string, len(keys.Keys))
	for i, k := range keys.Keys {
		keyNames[i] = k.Key
	}

	fmt.Printf("\nFetching metadata for %d keys with adaptive concurrency...\n", len(keyNames))

	// Create adaptive options with progress callback
	adaptiveOpts := &AdaptiveBatchOptions{
		InitialWorkers: 10,
		MinWorkers:     1,
		MaxWorkers:     50,
		ProgressCallback: func(processed, total, workers int) {
			fmt.Printf("\rProgress: %d/%d keys (Workers: %d)", processed, total, workers)
		},
	}

	ctx := context.Background()
	start := time.Now()

	results, err := BatchFetchMetadataAdaptive(ctx, client, accountID, namespaceID, keyNames, adaptiveOpts)
	if err != nil {
		fmt.Printf("\nError: %v\n", err)
		return
	}

	duration := time.Since(start)
	fmt.Printf("\n✅ Fetched metadata for %d keys in %v\n", len(results), duration)
	fmt.Printf("📊 Performance: %.2f keys/second\n", float64(len(results))/duration.Seconds())
}
