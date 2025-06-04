package kv

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	
	"cache-kv-purger/internal/api"
)

// OptimizedPurgeOptions provides options for optimized purge operations
type OptimizedPurgeOptions struct {
	TagField         string
	TagValue         string
	SearchValue      string
	IncludeMetadata  bool
	BatchSize        int
	Concurrency      int
	DryRun           bool
	ProgressCallback func(processed, deleted int)
	Context          context.Context
}

// OptimizedPurgeByMetadata performs high-performance metadata-based purge
func OptimizedPurgeByMetadata(client *api.Client, accountID, namespaceID string, options *OptimizedPurgeOptions) (int, error) {
	if options == nil {
		options = &OptimizedPurgeOptions{
			BatchSize:   100,
			Concurrency: 10,
		}
	}
	
	// Set defaults
	if options.BatchSize <= 0 {
		options.BatchSize = 100
	}
	if options.Concurrency <= 0 {
		options.Concurrency = 10
	}
	
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Channel for keys to delete
	keysToDelete := make(chan []string, options.Concurrency)
	errChan := make(chan error, 1)
	
	var totalDeleted int32
	var totalProcessed int32
	
	// Start deletion workers
	var wg sync.WaitGroup
	for i := 0; i < options.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for keys := range keysToDelete {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				if options.DryRun {
					atomic.AddInt32(&totalDeleted, int32(len(keys)))
					continue
				}
				
				// Delete batch of keys
				err := DeleteMultipleValues(client, accountID, namespaceID, keys)
				deleted := len(keys)
				if err != nil {
					select {
					case errChan <- fmt.Errorf("failed to delete batch: %w", err):
					default:
					}
					continue
				}
				
				atomic.AddInt32(&totalDeleted, int32(deleted))
			}
		}()
	}
	
	// Stream keys and filter by metadata
	streamErr := StreamListKeysEnhanced(client, accountID, namespaceID, &EnhancedListOptions{
		IncludeMetadata: true,
		ParallelPages:   3,
		Context:         ctx,
		StreamCallback: func(keys []KeyValuePair) error {
			var batch []string
			
			for _, key := range keys {
				atomic.AddInt32(&totalProcessed, 1)
				
				// Check if key matches criteria
				if matchesMetadataCriteria(key, options) {
					batch = append(batch, key.Key)
					
					// Send batch when full
					if len(batch) >= options.BatchSize {
						select {
						case keysToDelete <- batch:
							batch = nil
						case <-ctx.Done():
							return ctx.Err()
						}
					}
				}
			}
			
			// Send remaining keys
			if len(batch) > 0 {
				select {
				case keysToDelete <- batch:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			
			// Update progress
			if options.ProgressCallback != nil {
				options.ProgressCallback(int(atomic.LoadInt32(&totalProcessed)), int(atomic.LoadInt32(&totalDeleted)))
			}
			
			return nil
		},
	})
	
	// Close channel and wait for workers
	close(keysToDelete)
	wg.Wait()
	
	// Check for errors
	select {
	case err := <-errChan:
		return int(totalDeleted), err
	default:
		if streamErr != nil {
			return int(totalDeleted), streamErr
		}
		return int(totalDeleted), nil
	}
}

// matchesMetadataCriteria checks if a key matches the given criteria
func matchesMetadataCriteria(key KeyValuePair, options *OptimizedPurgeOptions) bool {
	if key.Metadata == nil {
		return false
	}
	
	// Check tag field/value
	if options.TagField != "" {
		value, exists := (*key.Metadata)[options.TagField]
		if !exists {
			return false
		}
		
		if options.TagValue != "" {
			// Convert to string and compare
			if strValue, ok := value.(string); ok {
				return strValue == options.TagValue
			}
			return false
		}
	}
	
	// Check search value (deep search in metadata)
	if options.SearchValue != "" {
		return searchInMetadata(key.Metadata, options.SearchValue)
	}
	
	return true
}

// searchInMetadata performs a deep search for a value in metadata
func searchInMetadata(metadata *KeyValueMetadata, searchValue string) bool {
	if metadata == nil {
		return false
	}
	
	return searchInMap(*metadata, searchValue)
}

// searchInMap recursively searches for a value in a map
func searchInMap(data map[string]interface{}, searchValue string) bool {
	for _, value := range data {
		switch v := value.(type) {
		case string:
			if v == searchValue {
				return true
			}
		case map[string]interface{}:
			if searchInMap(v, searchValue) {
				return true
			}
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok && str == searchValue {
					return true
				}
				if m, ok := item.(map[string]interface{}); ok && searchInMap(m, searchValue) {
					return true
				}
			}
		}
	}
	return false
}

// OptimizedExportKeys exports keys with high performance
func OptimizedExportKeys(client *api.Client, accountID, namespaceID string, keys []KeyValuePair, includeMetadata bool) ([]map[string]interface{}, error) {
	if len(keys) == 0 {
		return []map[string]interface{}{}, nil
	}
	
	// Pre-allocate result slice
	results := make([]map[string]interface{}, len(keys))
	
	// Create worker pool
	type workItem struct {
		index int
		key   KeyValuePair
	}
	
	type resultItem struct {
		index  int
		result map[string]interface{}
		err    error
	}
	
	workChan := make(chan workItem, len(keys))
	resultChan := make(chan resultItem, len(keys))
	
	// Start workers (no artificial delays)
	concurrency := 20 // Increased from 10
	var wg sync.WaitGroup
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for work := range workChan {
				result := map[string]interface{}{
					"key": work.key.Key,
				}
				
				if includeMetadata && work.key.Metadata == nil {
					// Fetch metadata if not available
					kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, work.key.Key)
					if err != nil {
						resultChan <- resultItem{
							index: work.index,
							err:   err,
						}
						continue
					}
					
					result["value"] = kvp.Value
					if kvp.Metadata != nil {
						result["metadata"] = *kvp.Metadata
					}
				} else {
					// Get just the value
					value, err := GetValue(client, accountID, namespaceID, work.key.Key)
					if err != nil {
						resultChan <- resultItem{
							index: work.index,
							err:   err,
						}
						continue
					}
					result["value"] = value
					
					if includeMetadata && work.key.Metadata != nil {
						result["metadata"] = *work.key.Metadata
					}
				}
				
				if work.key.Expiration > 0 {
					result["expiration"] = work.key.Expiration
				}
				
				resultChan <- resultItem{
					index:  work.index,
					result: result,
				}
			}
		}()
	}
	
	// Queue work
	for i, key := range keys {
		workChan <- workItem{
			index: i,
			key:   key,
		}
	}
	close(workChan)
	
	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	var firstErr error
	for result := range resultChan {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		} else {
			results[result.index] = result.result
		}
	}
	
	return results, firstErr
}