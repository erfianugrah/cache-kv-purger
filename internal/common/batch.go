package common

// BatchResult represents the result of processing a batch
type BatchResult struct {
	BatchIndex int
	Success    bool
	Data       interface{}
	Error      error
}

// BatchProcessor handles batch processing with configurable concurrency
// DEPRECATED: Use BatchProcessor[T, R] from batchprocessor.go instead
// This version is kept for backward compatibility
type BatchProcessor struct {
	// Batch size for processing
	BatchSize int

	// Maximum number of concurrent operations
	Concurrency int

	// Progress callback to report progress
	// completed: number of completed batches
	// total: total number of batches
	// successful: number of successful items
	ProgressCallback func(completed, total, successful int)
}

// NewBatchProcessor creates a new batch processor with default settings
func NewBatchProcessor() *BatchProcessor {
	return &BatchProcessor{
		BatchSize:        100, // Maximum items per API request (Cloudflare limit)
		Concurrency:      50,  // Enterprise tier concurrency
		ProgressCallback: func(completed, total, successful int) {},
	}
}

// WithBatchSize sets the batch size and returns the processor for chaining
func (p *BatchProcessor) WithBatchSize(size int) *BatchProcessor {
	if size > 0 {
		p.BatchSize = size
	}
	return p
}

// WithConcurrency sets the concurrency level and returns the processor for chaining
func (p *BatchProcessor) WithConcurrency(concurrency int) *BatchProcessor {
	if concurrency > 0 {
		p.Concurrency = concurrency
	}
	return p
}

// WithProgressCallback sets the progress callback and returns the processor for chaining
func (p *BatchProcessor) WithProgressCallback(callback func(completed, total, successful int)) *BatchProcessor {
	if callback != nil {
		p.ProgressCallback = callback
	}
	return p
}

// ProcessStrings processes a slice of strings in batches
// - items: The string items to process
// - processor: A function that processes a batch of strings and returns the results or an error
// Returns the successful results and any errors encountered
func (p *BatchProcessor) ProcessStrings(items []string, processor func([]string) ([]string, error)) ([]string, []error) {
	// Use the generic implementation with a backward-compatible wrapper
	genericProcessor := NewGenericBatchProcessor[string, string]().
		WithBatchSize(p.BatchSize).
		WithConcurrency(p.Concurrency).
		WithProgressCallback(p.ProgressCallback)

	return genericProcessor.ProcessItems(items, processor)
}

// SplitIntoBatches splits a slice into batches of the specified size
// This is a utility function for simple batch creation without needing the full BatchProcessor
func SplitIntoBatches(items []string, batchSize int) [][]string {
	return GenericSplitIntoBatches(items, batchSize)
}

// RemoveDuplicates removes duplicate strings from a slice while preserving order
func RemoveDuplicates(items []string) []string {
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
