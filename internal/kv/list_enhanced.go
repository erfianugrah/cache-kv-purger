package kv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	
	"cache-kv-purger/internal/api"
)

// EnhancedListOptions provides advanced options for listing keys with optimizations
type EnhancedListOptions struct {
	Prefix           string
	Limit            int
	Cursor           string
	IncludeMetadata  bool
	IncludeValues    bool
	ParallelPages    int    // Number of pages to fetch in parallel
	StreamCallback   func(keys []KeyValuePair) error
	ProgressCallback func(fetched int, total int)
	Context          context.Context
}

// ListKeysEnhanced performs optimized key listing with metadata included
func ListKeysEnhanced(client *api.Client, accountID, namespaceID string, options *EnhancedListOptions) (*ListKeysResult, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/keys", accountID, namespaceID)
	
	// Build query parameters
	queryParams := url.Values{}
	if options != nil {
		if options.Limit > 0 {
			if options.Limit < 10 {
				options.Limit = 10
			}
			queryParams.Set("limit", fmt.Sprintf("%d", options.Limit))
		}
		
		if options.Cursor != "" {
			queryParams.Set("cursor", options.Cursor)
		}
		
		if options.Prefix != "" {
			queryParams.Set("prefix", options.Prefix)
		}
		
		// Request metadata to be included in the response
		if options.IncludeMetadata {
			queryParams.Set("include", "metadata")
		}
		
		// Request values if needed (note: this might not be supported by all endpoints)
		if options.IncludeValues {
			queryParams.Set("include", "metadata,value")
		}
	}
	
	// Make the request
	respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
	
	if err != nil {
		return nil, err
	}
	
	var keysResp KeyValuesResponse
	if err := json.Unmarshal(respBody, &keysResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}
	
	if !keysResp.Success {
		errorStr := "API reported failure"
		if len(keysResp.Errors) > 0 {
			errorStr = keysResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to list keys: %s", errorStr)
	}
	
	return &ListKeysResult{
		Keys:       keysResp.Result,
		Cursor:     keysResp.ResultInfo.Cursor,
		HasMore:    keysResp.ResultInfo.Cursor != "",
		TotalCount: keysResp.ResultInfo.Count,
	}, nil
}

// StreamListKeysEnhanced streams keys with optimized performance
func StreamListKeysEnhanced(client *api.Client, accountID, namespaceID string, options *EnhancedListOptions) error {
	if options == nil {
		options = &EnhancedListOptions{
			Limit:         1000,
			ParallelPages: 1,
		}
	}
	
	if options.Limit == 0 {
		options.Limit = 1000
	}
	
	if options.ParallelPages <= 0 {
		options.ParallelPages = 1
	}
	
	// For single page streaming
	if options.ParallelPages == 1 {
		return streamSequential(client, accountID, namespaceID, options)
	}
	
	// For parallel page streaming
	return streamParallel(client, accountID, namespaceID, options)
}

// streamSequential handles sequential page streaming
func streamSequential(client *api.Client, accountID, namespaceID string, options *EnhancedListOptions) error {
	var totalFetched int
	cursor := options.Cursor
	
	for {
		// Create options for this page
		pageOpts := *options
		pageOpts.Cursor = cursor
		
		result, err := ListKeysEnhanced(client, accountID, namespaceID, &pageOpts)
		if err != nil {
			return err
		}
		
		totalFetched += len(result.Keys)
		
		// Call progress callback
		if options.ProgressCallback != nil {
			options.ProgressCallback(totalFetched, result.TotalCount)
		}
		
		// Call stream callback
		if options.StreamCallback != nil {
			if err := options.StreamCallback(result.Keys); err != nil {
				return err
			}
		}
		
		if !result.HasMore {
			break
		}
		
		cursor = result.Cursor
	}
	
	return nil
}

// streamParallel handles parallel page streaming
func streamParallel(client *api.Client, accountID, namespaceID string, options *EnhancedListOptions) error {
	// First, get the initial page to understand pagination
	firstPageOpts := *options
	firstPageOpts.ParallelPages = 1
	
	firstResult, err := ListKeysEnhanced(client, accountID, namespaceID, &firstPageOpts)
	if err != nil {
		return err
	}
	
	// Process first page
	if options.StreamCallback != nil {
		if err := options.StreamCallback(firstResult.Keys); err != nil {
			return err
		}
	}
	
	if !firstResult.HasMore {
		return nil
	}
	
	// Set up parallel fetching
	type pageResult struct {
		keys   []KeyValuePair
		cursor string
		err    error
		pageNum int
	}
	
	resultChan := make(chan pageResult, options.ParallelPages)
	workChan := make(chan string, options.ParallelPages*2)
	
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	var wg sync.WaitGroup
	var totalFetched int32
	atomic.AddInt32(&totalFetched, int32(len(firstResult.Keys)))
	
	// Start workers
	for i := 0; i < options.ParallelPages; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for cursor := range workChan {
				select {
				case <-ctx.Done():
					return
				default:
				}
				
				pageOpts := *options
				pageOpts.Cursor = cursor
				pageOpts.Context = ctx
				
				result, err := ListKeysEnhanced(client, accountID, namespaceID, &pageOpts)
				if err != nil {
					resultChan <- pageResult{err: err}
					return
				}
				
				resultChan <- pageResult{
					keys:   result.Keys,
					cursor: result.Cursor,
					pageNum: workerID,
				}
				
				// If there are more pages, add to work queue
				if result.HasMore && result.Cursor != "" {
					select {
					case workChan <- result.Cursor:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}
	
	// Start with the cursor from first page
	workChan <- firstResult.Cursor
	
	// Process results
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	for result := range resultChan {
		if result.err != nil {
			close(workChan)
			return result.err
		}
		
		atomic.AddInt32(&totalFetched, int32(len(result.keys)))
		
		// Call progress callback
		if options.ProgressCallback != nil {
			options.ProgressCallback(int(atomic.LoadInt32(&totalFetched)), firstResult.TotalCount)
		}
		
		// Call stream callback
		if options.StreamCallback != nil {
			if err := options.StreamCallback(result.keys); err != nil {
				close(workChan)
				return err
			}
		}
	}
	
	close(workChan)
	return nil
}

// BulkGetMetadata fetches metadata for multiple keys in a single request
func BulkGetMetadata(client *api.Client, accountID, namespaceID string, keys []string) (map[string]*KeyValueMetadata, error) {
	if len(keys) == 0 {
		return make(map[string]*KeyValueMetadata), nil
	}
	
	// Cloudflare KV API might not support bulk metadata fetch directly,
	// so we'll implement a parallel fetch strategy
	results := make(map[string]*KeyValueMetadata)
	var mu sync.Mutex
	var wg sync.WaitGroup
	
	// Limit concurrency
	semaphore := make(chan struct{}, 10)
	errChan := make(chan error, len(keys))
	
	for _, key := range keys {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Get key with metadata
			kvp, err := GetKeyWithMetadata(client, accountID, namespaceID, k)
			if err != nil {
				errChan <- fmt.Errorf("failed to get metadata for key %s: %w", k, err)
				return
			}
			
			mu.Lock()
			results[k] = kvp.Metadata
			mu.Unlock()
		}(key)
	}
	
	wg.Wait()
	close(errChan)
	
	// Check for errors
	for err := range errChan {
		return results, err
	}
	
	return results, nil
}

// OptimizedGetKeyWithMetadata fetches key value and metadata in the most efficient way
func OptimizedGetKeyWithMetadata(client *api.Client, accountID, namespaceID, key string) (*KeyValuePair, error) {
	// Try to get both value and metadata in a single request if the API supports it
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, url.QueryEscape(key))
	
	queryParams := url.Values{}
	queryParams.Set("include", "metadata")
	
	respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
	if err != nil {
		// Fallback to separate requests
		return GetKeyWithMetadata(client, accountID, namespaceID, key)
	}
	
	// Try to parse enhanced response
	var enhancedResp struct {
		Value    string            `json:"value"`
		Metadata *KeyValueMetadata `json:"metadata"`
	}
	
	if err := json.Unmarshal(respBody, &enhancedResp); err == nil && enhancedResp.Metadata != nil {
		// Successfully got both in one request
		return &KeyValuePair{
			Key:      key,
			Value:    enhancedResp.Value,
			Metadata: enhancedResp.Metadata,
		}, nil
	}
	
	// Fallback to separate requests
	return GetKeyWithMetadata(client, accountID, namespaceID, key)
}