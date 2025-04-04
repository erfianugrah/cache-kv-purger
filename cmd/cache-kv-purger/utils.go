package main

// splitIntoBatches splits a slice into batches of the specified size
func splitIntoBatches(items []string, batchSize int) [][]string {
	// Calculate number of batches
	numBatches := (len(items) + batchSize - 1) / batchSize

	// Create batches
	batches := make([][]string, 0, numBatches)
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// removeDuplicates removes duplicate strings from a slice while preserving order
func removeDuplicates(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
