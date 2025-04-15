package common

// BatchResult represents the result of processing a batch
type BatchResult struct {
	BatchIndex int
	Success    bool
	Data       interface{}
	Error      error
}

// BatchProcessor handles batch processing with configurable concurrency
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
	if len(items) == 0 {
		return nil, nil
	}

	// Create batches
	var batches [][]string
	for i := 0; i < len(items); i += p.BatchSize {
		end := i + p.BatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	// Process each batch (simplified version without concurrency for now)
	var successful []string
	var errors []error

	for batchIdx, batch := range batches {
		results, err := processor(batch)
		if err != nil {
			errors = append(errors, err)
		} else {
			successful = append(successful, results...)
		}

		p.ProgressCallback(batchIdx+1, len(batches), len(successful))
	}

	return successful, errors
}

// SplitIntoBatches splits a slice into batches of the specified size
// This is a utility function for simple batch creation without needing the full BatchProcessor
func SplitIntoBatches(items []string, batchSize int) [][]string {
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
