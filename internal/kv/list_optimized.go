package kv

import (
	"strings"
	"sync"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// ListAllKeysOptimized lists all keys with memory-efficient processing
func ListAllKeysOptimized(client *api.Client, accountID, namespaceID string, options *ListKeysOptions) ([]KeyValuePair, error) {
	if options == nil {
		options = &ListKeysOptions{
			Limit: 1000,
		}
	}

	// Pre-allocate result slice based on expected size
	// Use large slice pool for better memory reuse
	resultSlice := common.MemoryPools.GetLargeSlice()
	defer func() {
		// Don't return to pool - we're returning this data
		// But we could consider copying to right-sized slice if much smaller
	}()

	var cursor string
	for {
		// Set cursor for pagination
		options.Cursor = cursor

		// List keys
		result, err := ListKeysWithOptions(client, accountID, namespaceID, options)
		if err != nil {
			return nil, err
		}

		// Pre-allocate space if this is the first batch (estimate)
		if cursor == "" && len(result.Keys) > 0 {
			// Estimate total based on first batch
			estimatedTotal := len(result.Keys) * 10 // Rough estimate
			if cap(*resultSlice) < estimatedTotal {
				tempKeys := make([]string, 0, estimatedTotal)
				*resultSlice = append(*resultSlice, tempKeys...)
				*resultSlice = (*resultSlice)[:0] // Reset length but keep capacity
			}
		}

		// Convert and append keys efficiently
		for i := range result.Keys {
			*resultSlice = append(*resultSlice, result.Keys[i].Key)
		}

		// Check if we have more pages
		if result.Cursor == "" {
			break
		}
		cursor = result.Cursor
	}

	// Convert to KeyValuePair slice
	finalResult := make([]KeyValuePair, len(*resultSlice))
	for i, key := range *resultSlice {
		finalResult[i] = KeyValuePair{Key: key}
	}

	return finalResult, nil
}

// BuildPathOptimized builds API paths using pooled string builders
func BuildPathOptimized(parts ...string) string {
	sb := common.MemoryPools.GetStringBuilder()
	defer common.MemoryPools.PutStringBuilder(sb)

	for i, part := range parts {
		if i > 0 {
			sb.WriteString("/")
		}
		sb.WriteString(part)
	}

	return sb.String()
}

// FormatKeysOptimized formats keys efficiently using pooled builders
func FormatKeysOptimized(keys []string, format string) string {
	if len(keys) == 0 {
		return ""
	}

	sb := common.MemoryPools.GetStringBuilder()
	defer common.MemoryPools.PutStringBuilder(sb)

	switch format {
	case "json":
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
	case "csv":
		for i, key := range keys {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(key)
		}
	default:
		for i, key := range keys {
			if i > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(key)
		}
	}

	return sb.String()
}

// ProcessKeysInBatchesOptimized processes keys in batches with minimal memory allocation
func ProcessKeysInBatchesOptimized(keys []string, batchSize int, processor func(batch []string) error) error {
	if len(keys) == 0 {
		return nil
	}

	if batchSize <= 0 {
		batchSize = 100
	}

	// Use pooled slice for batches
	batchSlice := common.MemoryPools.GetSmallSlice()
	defer common.MemoryPools.PutSmallSlice(batchSlice)

	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		// Reset batch slice and copy keys
		*batchSlice = (*batchSlice)[:0]
		*batchSlice = append(*batchSlice, keys[i:end]...)

		if err := processor(*batchSlice); err != nil {
			return err
		}
	}

	return nil
}

// JoinPathsOptimized joins paths efficiently
func JoinPathsOptimized(paths []string) string {
	if len(paths) == 0 {
		return ""
	}

	// Calculate total length to minimize allocations
	totalLen := 0
	for i, path := range paths {
		totalLen += len(path)
		if i > 0 {
			totalLen++ // for separator
		}
	}

	// Use string builder with pre-allocated capacity
	var sb strings.Builder
	sb.Grow(totalLen)

	for i, path := range paths {
		if i > 0 {
			sb.WriteString("/")
		}
		sb.WriteString(path)
	}

	return sb.String()
}

// FilterKeysOptimized filters keys with minimal allocations
func FilterKeysOptimized(keys []string, predicate func(string) bool) []string {
	if len(keys) == 0 {
		return nil
	}

	// Use pooled slice for results
	resultSlice := common.MemoryPools.GetLargeSlice()
	
	for _, key := range keys {
		if predicate(key) {
			*resultSlice = append(*resultSlice, key)
		}
	}

	// Copy to right-sized slice
	filtered := make([]string, len(*resultSlice))
	copy(filtered, *resultSlice)
	
	// Return pooled slice
	common.MemoryPools.PutLargeSlice(resultSlice)

	return filtered
}

// BatchWorkItem represents a reusable work item for batch processing
type BatchWorkItem struct {
	Keys   []string
	Index  int
	Result interface{}
	Error  error
}

// BatchWorkPool provides pooled work items for batch processing
var BatchWorkPool = sync.Pool{
	New: func() interface{} {
		return &BatchWorkItem{
			Keys: make([]string, 0, 100),
		}
	},
}

// GetBatchWorkItem gets a work item from the pool
func GetBatchWorkItem() *BatchWorkItem {
	item := BatchWorkPool.Get().(*BatchWorkItem)
	item.Keys = item.Keys[:0]
	item.Index = 0
	item.Result = nil
	item.Error = nil
	return item
}

// PutBatchWorkItem returns a work item to the pool
func PutBatchWorkItem(item *BatchWorkItem) {
	if cap(item.Keys) > 1000 { // Don't pool if it grew too large
		return
	}
	BatchWorkPool.Put(item)
}