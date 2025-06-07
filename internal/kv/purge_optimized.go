package kv

import (
	"context"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// PurgeOptimizedOptions provides configuration for optimized purge operations
type PurgeOptimizedOptions struct {
	BatchSize        int
	Concurrency      int
	UseStreaming     bool
	ProgressCallback func(processed, total int)
}

// PurgeByPrefixOptimized deletes keys by prefix with minimal memory usage
func PurgeByPrefixOptimized(ctx context.Context, client *api.Client, accountID, namespaceID, prefix string,
	options *PurgeOptimizedOptions) (int, error) {

	if options == nil {
		options = &PurgeOptimizedOptions{
			BatchSize:    1000,
			Concurrency:  50,
			UseStreaming: true,
		}
	}

	deletedCount := 0

	// Use streaming to avoid loading all keys into memory
	if options.UseStreaming {
		// Stream keys and delete in batches
		listOpts := &ListKeysOptions{
			Prefix: prefix,
			Limit:  options.BatchSize,
		}

		streamOpts := &StreamingListOptions{
			BufferSize: options.BatchSize,
		}

		keyChan, errChan, err := StreamKeys(ctx, client, accountID, namespaceID, listOpts, streamOpts)
		if err != nil {
			return 0, err
		}

		// Collect keys in batches and delete
		batch := common.MemoryPools.GetLargeSlice()
		defer common.MemoryPools.PutLargeSlice(batch)

		for {
			select {
			case key, ok := <-keyChan:
				if !ok {
					// Channel closed, process final batch
					if len(*batch) > 0 {
						count := deleteKeyBatchOptimized(client, accountID, namespaceID, *batch)
						deletedCount += count
					}
					return deletedCount, nil
				}

				*batch = append(*batch, key.Key)

				// Process batch when full
				if len(*batch) >= options.BatchSize {
					count := deleteKeyBatchOptimized(client, accountID, namespaceID, *batch)
					deletedCount += count

					// Report progress
					if options.ProgressCallback != nil {
						options.ProgressCallback(deletedCount, -1) // -1 indicates unknown total
					}

					// Reset batch
					*batch = (*batch)[:0]
				}

			case err := <-errChan:
				if err != nil {
					return deletedCount, err
				}

			case <-ctx.Done():
				return deletedCount, ctx.Err()
			}
		}
	}

	// Non-streaming fallback
	return purgeByPrefixNonStreaming(client, accountID, namespaceID, prefix, options)
}

// deleteKeyBatchOptimized deletes a batch of keys with optimized memory usage
func deleteKeyBatchOptimized(client *api.Client, accountID, namespaceID string, keys []string) int {
	if len(keys) == 0 {
		return 0
	}

	// Try bulk delete first
	err := DeleteMultipleValuesOptimized(client, accountID, namespaceID, keys, false)
	if err == nil {
		return len(keys)
	}

	// On failure, count successful individual deletes
	successCount := 0
	for _, key := range keys {
		if err := DeleteValue(client, accountID, namespaceID, key); err == nil {
			successCount++
		}
	}

	return successCount
}

// purgeByPrefixNonStreaming handles non-streaming purge operations
func purgeByPrefixNonStreaming(client *api.Client, accountID, namespaceID, prefix string,
	options *PurgeOptimizedOptions) (int, error) {

	// List all keys with prefix
	listOpts := &ListKeysOptions{
		Prefix: prefix,
		Limit:  1000,
	}

	allKeys, err := ListAllKeysWithOptions(client, accountID, namespaceID, listOpts, nil)
	if err != nil {
		return 0, err
	}

	// Extract key names using pooled slice
	keySlice := common.MemoryPools.GetLargeSlice()
	defer common.MemoryPools.PutLargeSlice(keySlice)

	for _, kv := range allKeys {
		*keySlice = append(*keySlice, kv.Key)
	}

	// Process in batches
	deletedCount := 0
	err = ProcessKeysInBatchesOptimized(*keySlice, options.BatchSize, func(batch []string) error {
		count := deleteKeyBatchOptimized(client, accountID, namespaceID, batch)
		deletedCount += count

		if options.ProgressCallback != nil {
			options.ProgressCallback(deletedCount, len(*keySlice))
		}

		return nil
	})

	return deletedCount, err
}

// FindKeysOptimized searches for keys with minimal memory allocation
func FindKeysOptimized(client *api.Client, accountID, namespaceID string,
	searchFunc func(key string, metadata *KeyValueMetadata) bool) ([]string, error) {

	// Use streaming to minimize memory usage
	resultSlice := common.MemoryPools.GetLargeSlice()
	defer func() {
		// Don't return to pool - we're returning this data
	}()

	// Stream all keys
	listOpts := &ListKeysOptions{
		Limit: 1000,
	}

	var cursor string
	for {
		listOpts.Cursor = cursor

		result, err := ListKeysWithOptions(client, accountID, namespaceID, listOpts)
		if err != nil {
			return nil, err
		}

		// Check each key
		for _, key := range result.Keys {
			// Fetch metadata if needed and not present
			var metadata *KeyValueMetadata
			if key.Metadata != nil {
				metadata = key.Metadata
			} else {
				// Fetch metadata for this key
				kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, key.Key)
				if err == nil && kvp != nil {
					metadata = kvp.Metadata
				}
			}

			// Apply search function
			if searchFunc(key.Key, metadata) {
				*resultSlice = append(*resultSlice, key.Key)
			}
		}

		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor
	}

	// Copy to right-sized result
	finalResult := make([]string, len(*resultSlice))
	copy(finalResult, *resultSlice)

	return finalResult, nil
}

// BuildBulkDeleteRequestOptimized builds a bulk delete request with minimal allocations
func BuildBulkDeleteRequestOptimized(keys []string) []byte {
	// Calculate required size
	totalSize := 2 // for "[]"
	for i, key := range keys {
		if i > 0 {
			totalSize++ // for comma
		}
		totalSize += len(key) + 2 // for quotes
	}

	// Use pooled string builder
	sb := common.MemoryPools.GetStringBuilder()
	defer common.MemoryPools.PutStringBuilder(sb)
	sb.Grow(totalSize)

	sb.WriteString("[")
	for i, key := range keys {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"`)
		sb.WriteString(key)
		sb.WriteString(`"`)
	}
	sb.WriteString("]")

	return []byte(sb.String())
}

// ProcessMetadataOptimized processes metadata with reduced memory allocation
func ProcessMetadataOptimized(metadata map[string]interface{}, processor func(key string, value interface{}) error) error {
	// Reuse string for path building
	pathBuilder := common.MemoryPools.GetStringBuilder()
	defer common.MemoryPools.PutStringBuilder(pathBuilder)

	var processMap func(m map[string]interface{}, prefix string) error
	processMap = func(m map[string]interface{}, prefix string) error {
		for k, v := range m {
			// Build current path
			pathBuilder.Reset()
			if prefix != "" {
				pathBuilder.WriteString(prefix)
				pathBuilder.WriteString(".")
			}
			pathBuilder.WriteString(k)
			currentPath := pathBuilder.String()

			// Process based on type
			switch val := v.(type) {
			case map[string]interface{}:
				if err := processMap(val, currentPath); err != nil {
					return err
				}
			default:
				if err := processor(currentPath, val); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return processMap(metadata, "")
}

// InterningStringPool provides string interning for frequently used strings
var metadataKeyPool = common.NewStringPool(10000)

// InternMetadataKey returns an interned version of a metadata key
func InternMetadataKey(key string) string {
	return metadataKeyPool.Intern(key)
}

// OptimizeMetadataStructure reduces memory usage of metadata structures
func OptimizeMetadataStructure(metadata map[string]interface{}) map[string]interface{} {
	optimized := make(map[string]interface{}, len(metadata))

	for k, v := range metadata {
		// Intern frequently used keys
		internedKey := InternMetadataKey(k)

		// Recursively optimize nested maps
		if nestedMap, ok := v.(map[string]interface{}); ok {
			optimized[internedKey] = OptimizeMetadataStructure(nestedMap)
		} else {
			optimized[internedKey] = v
		}
	}

	return optimized
}
