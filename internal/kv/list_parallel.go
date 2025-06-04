package kv

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	
	"cache-kv-purger/internal/api"
)

// ParallelListOptions provides options for parallel list operations
type ParallelListOptions struct {
	Prefix           string
	BatchSize        int
	MaxPages         int
	ParallelRequests int
	IncludeMetadata  bool
	Context          context.Context
}

// ParallelListAllKeys fetches all keys using parallel pagination
func ParallelListAllKeys(client *api.Client, accountID, namespaceID string, options *ParallelListOptions) ([]KeyValuePair, error) {
	if options == nil {
		options = &ParallelListOptions{
			BatchSize:        1000,
			ParallelRequests: 5,
		}
	}
	
	// Set defaults
	if options.BatchSize <= 0 || options.BatchSize > 1000 {
		options.BatchSize = 1000
	}
	if options.ParallelRequests <= 0 {
		options.ParallelRequests = 5
	}
	
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	// First, get the initial page to understand the total count
	firstPageOpts := &EnhancedListOptions{
		Prefix:          options.Prefix,
		Limit:           options.BatchSize,
		IncludeMetadata: options.IncludeMetadata,
		Context:         ctx,
	}
	
	firstResult, err := ListKeysEnhanced(client, accountID, namespaceID, firstPageOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch first page: %w", err)
	}
	
	// If there's only one page, return immediately
	if !firstResult.HasMore {
		return firstResult.Keys, nil
	}
	
	// Estimate total pages based on first page
	estimatedPages := (firstResult.TotalCount + options.BatchSize - 1) / options.BatchSize
	if options.MaxPages > 0 && estimatedPages > options.MaxPages {
		estimatedPages = options.MaxPages
	}
	
	// Create channels for work distribution
	type pageWork struct {
		cursor   string
		pageNum  int
	}
	
	type pageResult struct {
		keys    []KeyValuePair
		cursor  string
		pageNum int
		err     error
	}
	
	workChan := make(chan pageWork, estimatedPages)
	resultChan := make(chan pageResult, estimatedPages)
	
	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < options.ParallelRequests; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for work := range workChan {
				select {
				case <-ctx.Done():
					resultChan <- pageResult{err: ctx.Err()}
					return
				default:
				}
				
				// Fetch the page
				pageOpts := &EnhancedListOptions{
					Prefix:          options.Prefix,
					Limit:           options.BatchSize,
					Cursor:          work.cursor,
					IncludeMetadata: options.IncludeMetadata,
					Context:         ctx,
				}
				
				result, err := ListKeysEnhanced(client, accountID, namespaceID, pageOpts)
				if err != nil {
					resultChan <- pageResult{
						pageNum: work.pageNum,
						err:     fmt.Errorf("worker %d failed to fetch page %d: %w", workerID, work.pageNum, err),
					}
					return
				}
				
				resultChan <- pageResult{
					keys:    result.Keys,
					cursor:  result.Cursor,
					pageNum: work.pageNum,
					err:     nil,
				}
				
				// If there's another page, queue it
				if result.HasMore && result.Cursor != "" {
					select {
					case workChan <- pageWork{cursor: result.Cursor, pageNum: work.pageNum + 1}:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}
	
	// Queue the first cursor (from first page)
	workChan <- pageWork{cursor: firstResult.Cursor, pageNum: 2}
	
	// Collect results
	allKeys := make([]KeyValuePair, 0, firstResult.TotalCount)
	allKeys = append(allKeys, firstResult.Keys...)
	
	pagesReceived := 1
	expectedPages := estimatedPages
	
	// Close work channel when done
	go func() {
		// Wait for all pages to be processed
		for pagesReceived < expectedPages {
			time.Sleep(10 * time.Millisecond)
		}
		close(workChan)
		wg.Wait()
		close(resultChan)
	}()
	
	// Process results
	pageMap := make(map[int][]KeyValuePair)
	var lastErr error
	
	for result := range resultChan {
		if result.err != nil {
			lastErr = result.err
			continue
		}
		
		pageMap[result.pageNum] = result.keys
		pagesReceived++
		
		// If we know this is the last page, update expected count
		if result.cursor == "" {
			expectedPages = result.pageNum
		}
	}
	
	// Assemble results in order
	for i := 2; i <= expectedPages; i++ {
		if keys, ok := pageMap[i]; ok {
			allKeys = append(allKeys, keys...)
		}
	}
	
	if lastErr != nil {
		return allKeys, fmt.Errorf("parallel list completed with errors: %w", lastErr)
	}
	
	return allKeys, nil
}

// StreamParallelListKeys streams keys using parallel pagination
func StreamParallelListKeys(client *api.Client, accountID, namespaceID string, options *ParallelListOptions, callback func([]KeyValuePair) error) error {
	if options == nil {
		options = &ParallelListOptions{
			BatchSize:        1000,
			ParallelRequests: 5,
		}
	}
	
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Channel for streaming results
	resultChan := make(chan []KeyValuePair, options.ParallelRequests*2)
	errChan := make(chan error, 1)
	
	var totalFetched int32
	
	// Start consumer
	var consumerWG sync.WaitGroup
	consumerWG.Add(1)
	go func() {
		defer consumerWG.Done()
		
		for keys := range resultChan {
			if err := callback(keys); err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
			atomic.AddInt32(&totalFetched, int32(len(keys)))
		}
	}()
	
	// Start parallel fetching
	go func() {
		keys, err := ParallelListAllKeys(client, accountID, namespaceID, options)
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
			close(resultChan)
			return
		}
		
		// Stream keys in batches
		batchSize := 100
		for i := 0; i < len(keys); i += batchSize {
			end := i + batchSize
			if end > len(keys) {
				end = len(keys)
			}
			
			select {
			case resultChan <- keys[i:end]:
			case <-ctx.Done():
				close(resultChan)
				return
			}
		}
		
		close(resultChan)
	}()
	
	// Wait for consumer to finish
	consumerWG.Wait()
	
	// Check for errors
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}