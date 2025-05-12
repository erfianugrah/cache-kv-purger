package common

// GenericBatchProcessor handles batch processing with configurable concurrency using Go generics
// This is an enhanced version of the original BatchProcessor that can handle any type
type GenericBatchProcessor[T any, R any] struct {
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

// NewGenericBatchProcessor creates a new batch processor with default settings
func NewGenericBatchProcessor[T any, R any]() *GenericBatchProcessor[T, R] {
	return &GenericBatchProcessor[T, R]{
		BatchSize:        100, // Maximum items per API request (Cloudflare limit)
		Concurrency:      50,  // Enterprise tier concurrency
		ProgressCallback: func(completed, total, successful int) {},
	}
}

// WithBatchSize sets the batch size and returns the processor for chaining
func (p *GenericBatchProcessor[T, R]) WithBatchSize(size int) *GenericBatchProcessor[T, R] {
	if size > 0 {
		p.BatchSize = size
	}
	return p
}

// WithConcurrency sets the concurrency level and returns the processor for chaining
func (p *GenericBatchProcessor[T, R]) WithConcurrency(concurrency int) *GenericBatchProcessor[T, R] {
	if concurrency > 0 {
		p.Concurrency = concurrency
	}
	return p
}

// WithProgressCallback sets the progress callback and returns the processor for chaining
func (p *GenericBatchProcessor[T, R]) WithProgressCallback(callback func(completed, total, successful int)) *GenericBatchProcessor[T, R] {
	if callback != nil {
		p.ProgressCallback = callback
	}
	return p
}

// ProcessItems processes a slice of items in batches
// - items: The items to process
// - processor: A function that processes a batch of items and returns the results or an error
// Returns the successful results and any errors encountered
func (p *GenericBatchProcessor[T, R]) ProcessItems(items []T, processor func([]T) ([]R, error)) ([]R, []error) {
	if len(items) == 0 {
		return nil, nil
	}

	// Create batches
	var batches [][]T
	for i := 0; i < len(items); i += p.BatchSize {
		end := i + p.BatchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex int
		results    []R
		err        error
	}

	resultChan := make(chan batchResult, len(batches))

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, p.Concurrency)

	// Process all batches concurrently with semaphore control
	for idx, batch := range batches {
		// Acquire semaphore slot
		sem <- struct{}{}

		// Launch a goroutine to process this batch
		go func(batchIdx int, batchItems []T) {
			defer func() { <-sem }() // Release semaphore when done

			// Process this batch
			results, err := processor(batchItems)

			// Send result back through channel
			if err != nil {
				resultChan <- batchResult{
					batchIndex: batchIdx,
					results:    nil,
					err:        err,
				}
				return
			}

			resultChan <- batchResult{
				batchIndex: batchIdx,
				results:    results,
				err:        nil,
			}
		}(idx, batch)
	}

	// Collect results
	var successful []R
	var errors []error

	// Track progress
	completed := 0
	successCount := 0

	// Collect results from all batches
	for i := 0; i < len(batches); i++ {
		result := <-resultChan

		// Save error or success
		if result.err != nil {
			errors = append(errors, result.err)
		} else if result.results != nil {
			successful = append(successful, result.results...)
			successCount += len(result.results)
		}

		// Update progress
		completed++

		// Call progress callback
		p.ProgressCallback(completed, len(batches), successCount)
	}

	return successful, errors
}

// GenericSplitIntoBatches splits a slice into batches of the specified size
// This is a utility function that works with any type, not just strings
func GenericSplitIntoBatches[T any](items []T, batchSize int) [][]T {
	// Calculate number of batches
	numBatches := (len(items) + batchSize - 1) / batchSize

	// Create batches
	batches := make([][]T, 0, numBatches)
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}