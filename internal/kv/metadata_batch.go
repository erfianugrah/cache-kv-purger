package kv

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"cache-kv-purger/internal/api"
)

// BatchMetadataOptions configures batch metadata fetching
type BatchMetadataOptions struct {
	BatchSize       int
	Concurrency     int
	Timeout         time.Duration
	RetryFailures   bool
	ProgressCallback func(fetched, total int)
}

// DefaultBatchMetadataOptions returns sensible defaults
func DefaultBatchMetadataOptions() *BatchMetadataOptions {
	return &BatchMetadataOptions{
		BatchSize:     100,
		Concurrency:   50,
		Timeout:       5 * time.Minute,
		RetryFailures: true,
	}
}

// MetadataResult holds the result of a metadata fetch
type MetadataResult struct {
	Key      string
	Metadata *KeyValueMetadata
	Error    error
}

// BatchFetchMetadataOptimized fetches metadata for multiple keys with optimizations
func BatchFetchMetadataOptimized(ctx context.Context, client *api.Client, accountID, namespaceID string, 
	keys []string, options *BatchMetadataOptions) (map[string]*KeyValueMetadata, error) {
	
	if len(keys) == 0 {
		return make(map[string]*KeyValueMetadata), nil
	}

	if options == nil {
		options = DefaultBatchMetadataOptions()
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	// Result map
	results := make(map[string]*KeyValueMetadata)
	var mu sync.Mutex

	// Progress tracking
	var completed int32
	totalKeys := len(keys)

	// Create work channel
	workChan := make(chan string, len(keys))
	for _, key := range keys {
		workChan <- key
	}
	close(workChan)

	// Create result channel
	resultChan := make(chan MetadataResult, options.Concurrency)

	// Worker pool
	var wg sync.WaitGroup
	for i := 0; i < options.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for key := range workChan {
				select {
				case <-ctx.Done():
					return
				default:
					// Fetch metadata for this key
					metadata, err := fetchSingleMetadata(client, accountID, namespaceID, key)
					
					result := MetadataResult{
						Key:      key,
						Metadata: metadata,
						Error:    err,
					}
					
					select {
					case resultChan <- result:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}

	// Result collector
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Failed keys for retry
	var failedKeys []string

	// Collect results
	for result := range resultChan {
		if result.Error != nil {
			if options.RetryFailures {
				failedKeys = append(failedKeys, result.Key)
			}
		} else if result.Metadata != nil {
			mu.Lock()
			results[result.Key] = result.Metadata
			mu.Unlock()
		}

		// Update progress
		current := atomic.AddInt32(&completed, 1)
		if options.ProgressCallback != nil && current%100 == 0 {
			options.ProgressCallback(int(current), totalKeys)
		}
	}

	// Final progress update
	if options.ProgressCallback != nil {
		options.ProgressCallback(int(completed), totalKeys)
	}

	// Retry failed keys with exponential backoff
	if options.RetryFailures && len(failedKeys) > 0 {
		retryResults := retryFailedMetadata(ctx, client, accountID, namespaceID, failedKeys, options)
		
		// Merge retry results
		mu.Lock()
		for key, metadata := range retryResults {
			results[key] = metadata
		}
		mu.Unlock()
	}

	return results, nil
}

// fetchSingleMetadata fetches metadata for a single key
func fetchSingleMetadata(client *api.Client, accountID, namespaceID, key string) (*KeyValueMetadata, error) {
	// Fetch the key with metadata
	kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, key)
	if err != nil {
		return nil, err
	}
	return kvp.Metadata, nil
}

// retryFailedMetadata retries failed metadata fetches with exponential backoff
func retryFailedMetadata(ctx context.Context, client *api.Client, accountID, namespaceID string,
	failedKeys []string, options *BatchMetadataOptions) map[string]*KeyValueMetadata {
	
	results := make(map[string]*KeyValueMetadata)
	maxRetries := 3
	
	for _, key := range failedKeys {
		var metadata *KeyValueMetadata
		var err error
		
		// Exponential backoff retry
		for attempt := 0; attempt < maxRetries; attempt++ {
			select {
			case <-ctx.Done():
				return results
			default:
				metadata, err = fetchSingleMetadata(client, accountID, namespaceID, key)
				if err == nil && metadata != nil {
					results[key] = metadata
					break
				}
				
				// Exponential backoff: 100ms, 200ms, 400ms
				if attempt < maxRetries-1 {
					backoff := time.Duration(100<<uint(attempt)) * time.Millisecond
					time.Sleep(backoff)
				}
			}
		}
	}
	
	return results
}

// StreamBatchMetadata fetches metadata in a streaming fashion
func StreamBatchMetadata(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, options *BatchMetadataOptions) (<-chan MetadataResult, error) {
	
	if options == nil {
		options = DefaultBatchMetadataOptions()
	}

	// Create result channel
	resultChan := make(chan MetadataResult, options.Concurrency)

	// Create work channel
	workChan := make(chan string, len(keys))
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < options.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for key := range workChan {
				select {
				case <-ctx.Done():
					return
				default:
					// Fetch metadata
					metadata, err := fetchSingleMetadata(client, accountID, namespaceID, key)
					
					result := MetadataResult{
						Key:      key,
						Metadata: metadata,
						Error:    err,
					}
					
					select {
					case resultChan <- result:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}

	// Feed work
	go func() {
		defer close(workChan)
		for _, key := range keys {
			select {
			case workChan <- key:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Close result channel when done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	return resultChan, nil
}

// OptimizeMetadataFetching checks if metadata is already available before fetching
func OptimizeMetadataFetching(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []KeyValuePair, options *BatchMetadataOptions) (map[string]*KeyValueMetadata, error) {
	
	// Separate keys that need metadata fetch
	var keysNeedingFetch []string
	existingMetadata := make(map[string]*KeyValueMetadata)
	
	for _, kvp := range keys {
		if kvp.Metadata != nil {
			// Already have metadata
			existingMetadata[kvp.Key] = kvp.Metadata
		} else {
			// Need to fetch
			keysNeedingFetch = append(keysNeedingFetch, kvp.Key)
		}
	}
	
	// Fetch missing metadata
	if len(keysNeedingFetch) > 0 {
		fetchedMetadata, err := BatchFetchMetadataOptimized(ctx, client, accountID, namespaceID, keysNeedingFetch, options)
		if err != nil {
			return existingMetadata, err
		}
		
		// Merge results
		for key, metadata := range fetchedMetadata {
			existingMetadata[key] = metadata
		}
	}
	
	return existingMetadata, nil
}