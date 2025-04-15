package main

import "cache-kv-purger/internal/common"

// DEPRECATED: Use common.SplitIntoBatches instead
// This is kept for backward compatibility but will be removed in future versions
func splitIntoBatches(items []string, batchSize int) [][]string {
	return common.SplitIntoBatches(items, batchSize)
}

// DEPRECATED: Use common.RemoveDuplicates instead
// This is kept for backward compatibility but will be removed in future versions
func removeDuplicates(items []string) []string {
	return common.RemoveDuplicates(items)
}
