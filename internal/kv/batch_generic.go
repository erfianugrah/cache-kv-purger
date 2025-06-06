package kv

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// WriteMultipleValuesGeneric writes multiple values using GenericBatchProcessor
func WriteMultipleValuesGeneric(ctx context.Context, client *api.Client, accountID, namespaceID string, 
	items []BulkWriteItem, batchSize int, concurrency int, progressCallback func(completed, total int)) (int, error) {
	
	if len(items) == 0 {
		return 0, nil
	}

	// Set defaults
	if batchSize <= 0 {
		batchSize = 100 // Cloudflare limit
	}
	if concurrency <= 0 {
		concurrency = 10
	}

	// Track total successful writes
	var successCount int32

	// Create processor
	processor := common.NewGenericBatchProcessor[BulkWriteItem, int]().
		WithBatchSize(batchSize).
		WithConcurrency(concurrency).
		WithProgressCallback(func(completed, total, successful int) {
			if progressCallback != nil {
				progressCallback(completed, total)
			}
		})

	// Process function
	processBatch := func(batch []BulkWriteItem) ([]int, error) {
		// Check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Write the batch
		err := WriteMultipleValues(client, accountID, namespaceID, batch)
		if err != nil {
			// Try individual writes as fallback
			var batchSuccess int
			for _, item := range batch {
				writeOpts := &WriteOptions{}
				if item.Metadata != nil {
					writeOpts.Metadata = item.Metadata
				}
				if item.Expiration > 0 {
					writeOpts.Expiration = item.Expiration
				}
				if item.ExpirationTTL > 0 {
					writeOpts.ExpirationTTL = item.ExpirationTTL
				}
				
				err := WriteValue(client, accountID, namespaceID, item.Key, item.Value, writeOpts)
				if err == nil {
					batchSuccess++
				}
			}
			
			atomic.AddInt32(&successCount, int32(batchSuccess))
			
			// Return results (number of successful writes per item)
			results := make([]int, len(batch))
			for i := range results {
				if i < batchSuccess {
					results[i] = 1
				}
			}
			return results, nil
		}

		// All succeeded
		atomic.AddInt32(&successCount, int32(len(batch)))
		results := make([]int, len(batch))
		for i := range results {
			results[i] = 1
		}
		return results, nil
	}

	// Process items
	_, errors := processor.ProcessItems(items, processBatch)
	
	// Return success count even if there were some errors
	finalCount := int(atomic.LoadInt32(&successCount))
	
	if len(errors) > 0 && finalCount == 0 {
		// All failed
		return 0, fmt.Errorf("all write operations failed: %v", errors[0])
	}
	
	return finalCount, nil
}

// DeleteMultipleValuesGeneric deletes multiple values using GenericBatchProcessor
func DeleteMultipleValuesGeneric(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, batchSize int, concurrency int, progressCallback func(completed, total int)) (int, error) {
	
	if len(keys) == 0 {
		return 0, nil
	}

	// Set defaults
	if batchSize <= 0 {
		batchSize = 100
	}
	if concurrency <= 0 {
		concurrency = 10
	}

	// Track total successful deletes
	var successCount int32

	// Create processor
	processor := common.NewGenericBatchProcessor[string, int]().
		WithBatchSize(batchSize).
		WithConcurrency(concurrency).
		WithProgressCallback(func(completed, total, successful int) {
			if progressCallback != nil {
				progressCallback(completed, total)
			}
		})

	// Process function
	processBatch := func(batch []string) ([]int, error) {
		// Check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Use optimized deletion with binary search fallback
		err := DeleteMultipleValuesOptimized(client, accountID, namespaceID, batch, false)
		
		var batchSuccess int
		if err == nil {
			batchSuccess = len(batch)
		} else {
			// Count partial successes
			for _, key := range batch {
				if err := DeleteValue(client, accountID, namespaceID, key); err == nil {
					batchSuccess++
				}
			}
		}
		
		atomic.AddInt32(&successCount, int32(batchSuccess))
		
		// Return results
		results := make([]int, len(batch))
		for i := range results {
			if i < batchSuccess {
				results[i] = 1
			}
		}
		return results, nil
	}

	// Process items
	_, errors := processor.ProcessItems(keys, processBatch)
	
	// Return success count
	finalCount := int(atomic.LoadInt32(&successCount))
	
	if len(errors) > 0 && finalCount == 0 {
		return 0, fmt.Errorf("all delete operations failed: %v", errors[0])
	}
	
	return finalCount, nil
}

// FetchMetadataGeneric fetches metadata using GenericBatchProcessor
func FetchMetadataGeneric(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, concurrency int, progressCallback func(fetched, total int)) (map[string]*KeyValueMetadata, error) {
	
	if len(keys) == 0 {
		return make(map[string]*KeyValueMetadata), nil
	}

	// Set defaults
	if concurrency <= 0 {
		concurrency = 50
	}

	// Result map
	results := make(map[string]*KeyValueMetadata)
	var mu sync.Mutex

	// Create processor - process one key at a time for metadata
	processor := common.NewGenericBatchProcessor[string, *KeyValuePair]().
		WithBatchSize(1). // One key at a time for metadata
		WithConcurrency(concurrency).
		WithProgressCallback(func(completed, total, successful int) {
			if progressCallback != nil {
				progressCallback(completed, total)
			}
		})

	// Process function
	processKey := func(batch []string) ([]*KeyValuePair, error) {
		// Should only have one key
		if len(batch) != 1 {
			return nil, fmt.Errorf("expected single key, got %d", len(batch))
		}
		
		key := batch[0]
		
		// Check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Fetch metadata
		kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, key)
		if err != nil {
			return nil, err
		}

		// Store result if we got metadata
		if kvp != nil && kvp.Metadata != nil {
			mu.Lock()
			results[key] = kvp.Metadata
			mu.Unlock()
		}

		return []*KeyValuePair{kvp}, nil
	}

	// Process items
	_, errors := processor.ProcessItems(keys, processKey)
	
	// Return results even if there were some errors
	if len(errors) > 0 && len(results) == 0 {
		return results, fmt.Errorf("failed to fetch any metadata: %v", errors[0])
	}
	
	return results, nil
}

// TODO: ExportKeysGeneric - Commented out until ExportedKey type is defined
// func ExportKeysGeneric(ctx context.Context, client *api.Client, accountID, namespaceID string,
// 	keys []KeyValuePair, includeValues bool, concurrency int, 
// 	progressCallback func(exported, total int)) ([]ExportedKey, error) {
// 	// Implementation commented out until ExportedKey type is defined
// }

// PurgeByMetadataGeneric purges keys by metadata using GenericBatchProcessor
func PurgeByMetadataGeneric(ctx context.Context, client *api.Client, accountID, namespaceID string,
	keys []string, metadataField, metadataValue string, batchSize int, concurrency int,
	progressCallback func(checked, matched, deleted, total int)) (int, error) {
	
	if len(keys) == 0 {
		return 0, nil
	}

	// Set defaults
	if batchSize <= 0 {
		batchSize = 100
	}
	if concurrency <= 0 {
		concurrency = 50
	}

	// Track progress
	var checked, matched, deleted int32

	// First, find all matching keys
	var matchingKeys []string
	var matchMu sync.Mutex

	// Create processor for checking metadata
	checkProcessor := common.NewGenericBatchProcessor[string, bool]().
		WithBatchSize(1). // Check one at a time
		WithConcurrency(concurrency).
		WithProgressCallback(func(completed, total, successful int) {
			atomic.StoreInt32(&checked, int32(completed))
			if progressCallback != nil {
				progressCallback(int(checked), int(matched), int(deleted), len(keys))
			}
		})

	// Check function
	checkMetadata := func(batch []string) ([]bool, error) {
		key := batch[0]
		
		// Check context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Get metadata
		kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, key)
		if err != nil {
			return []bool{false}, nil // Skip on error
		}

		// Check if metadata matches
		if kvp != nil && kvp.Metadata != nil {
			if CheckMetadataMatch(kvp.Metadata, metadataField, metadataValue) {
				matchMu.Lock()
				matchingKeys = append(matchingKeys, key)
				atomic.AddInt32(&matched, 1)
				matchMu.Unlock()
				return []bool{true}, nil
			}
		}

		return []bool{false}, nil
	}

	// Check all keys
	_, _ = checkProcessor.ProcessItems(keys, checkMetadata)

	// Now delete matching keys
	if len(matchingKeys) > 0 {
		deletedCount, err := DeleteMultipleValuesGeneric(ctx, client, accountID, namespaceID, 
			matchingKeys, batchSize, concurrency, 
			func(completed, total int) {
				atomic.StoreInt32(&deleted, int32(completed))
				if progressCallback != nil {
					progressCallback(int(checked), int(matched), int(deleted), len(keys))
				}
			})
		
		return deletedCount, err
	}

	return 0, nil
}

// Helper function to check metadata match
func CheckMetadataMatch(metadata *KeyValueMetadata, field, value string) bool {
	if metadata == nil {
		return false
	}
	
	// Convert to map for easier access
	metadataMap := map[string]interface{}(*metadata)
	
	// Check for exact match
	if val, exists := metadataMap[field]; exists {
		if strVal, ok := val.(string); ok && strVal == value {
			return true
		}
	}
	
	// Check nested fields (simplified)
	for _, v := range metadataMap {
		if nestedMap, ok := v.(map[string]interface{}); ok {
			if val, exists := nestedMap[field]; exists {
				if strVal, ok := val.(string); ok && strVal == value {
					return true
				}
			}
		}
	}
	
	return false
}