package kv

import (
	"fmt"
	"strings"
	"sync"
	"time"
	
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// EnhancedSearchOptions provides advanced options for searching keys
type EnhancedSearchOptions struct {
	SearchValue     string // Value to search for
	TagField        string // Metadata field to filter by
	TagValue        string // Value to match in the tag field
	IncludeMetadata bool   // Include metadata with keys
	BatchSize       int    // Batch size for bulk operations
	Concurrency     int    // Number of concurrent operations
	Timeout         time.Duration // Overall timeout for the operation
	Verbose         bool   // Enable verbose output
	Debug           bool   // Enable debug output
}

// EnhancedStreamingFilterKeysByMetadata is an improved version of StreamingFilterKeysByMetadata
// that uses the common pagination utilities
func EnhancedStreamingFilterKeysByMetadata(
	client *api.Client, 
	accountID, 
	namespaceID string, 
	tagField, 
	tagValue string, 
	options *EnhancedSearchOptions,
	progressCallback func(fetched, processed, matched int)) ([]KeyValuePair, error) {
	
	// Validate inputs
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if tagField == "" {
		return nil, fmt.Errorf("tag field is required")
	}
	
	// Set default options if not provided
	if options == nil {
		options = &EnhancedSearchOptions{
			BatchSize:   1000,
			Concurrency: 5,
			Timeout:     120 * time.Second,
		}
	}
	
	// Create pagination options from search options
	pagOptions := &common.PaginationOptions{
		Debug:     options.Debug,
		Verbose:   options.Verbose,
		MaxRetries: 3,
		Timeout:   options.Timeout,
		LogPrefix: "Search",
	}
	
	// Configure a standard key listing handler
	listHandler := &tagSearchHandler{
		client:         client,
		accountID:      accountID,
		namespaceID:    namespaceID,
		tagField:       tagField,
		tagValue:       tagValue,
		includeMetadata: options.IncludeMetadata,
		batchSize:      options.BatchSize,
		concurrency:    options.Concurrency,
		progressCallback: progressCallback,
		matchedKeys:    []KeyValuePair{},
		logger:         common.NewPaginationLogger(pagOptions, &common.PaginationResult{}),
	}
	
	// Execute pagination
	result, err := common.ExecutePagination(listHandler, pagOptions)
	if err != nil {
		return nil, err
	}
	
	// Show warnings if verbose
	if options.Verbose && len(result.Warnings) > 0 {
		fmt.Println("\nWarnings during search operation:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
	
	// Return the matched keys
	return listHandler.matchedKeys, nil
}

// tagSearchHandler implements the PaginationHandler interface for tag-based search
type tagSearchHandler struct {
	client          *api.Client
	accountID       string
	namespaceID     string
	tagField        string
	tagValue        string
	includeMetadata bool
	batchSize       int
	concurrency     int
	progressCallback func(fetched, processed, matched int)
	matchedKeys     []KeyValuePair
	fetchedCount    int
	processedCount  int
	logger          *common.PaginationLogger
	mu              sync.Mutex // Mutex to protect concurrent access to matchedKeys
}

// FetchPage fetches a single page of keys
func (h *tagSearchHandler) FetchPage(cursor string) (interface{}, string, bool, error) {
	// Create list options for this page
	listOptions := &ListKeysOptions{
		Limit:  h.batchSize,
		Cursor: cursor,
		// Note: ListKeysOptions doesn't have an IncludeMetadata field,
		// but ListKeysWithOptions will return metadata by default
	}
	
	// Call the API to fetch a page of keys
	result, err := ListKeysWithOptions(h.client, h.accountID, h.namespaceID, listOptions)
	if err != nil {
		return nil, "", false, err
	}
	
	// Update fetched count
	h.fetchedCount += len(result.Keys)
	
	h.logger.Debug("Fetched %d keys (total %d)", len(result.Keys), h.fetchedCount)
	
	// Return the results
	return result.Keys, result.Cursor, !result.HasMore, nil
}

// ProcessItems processes the keys returned by FetchPage
func (h *tagSearchHandler) ProcessItems(items interface{}) error {
	keys, ok := items.([]KeyValuePair)
	if !ok {
		return fmt.Errorf("unexpected item type in tag search")
	}
	
	// Process concurrently if concurrency > 1
	if h.concurrency > 1 && len(keys) > 10 {
		return h.processConcurrently(keys)
	}
	
	// Otherwise process serially
	return h.processSerially(keys)
}

// processSerially processes items in a single thread
func (h *tagSearchHandler) processSerially(keys []KeyValuePair) error {
	for _, key := range keys {
		if h.matchesCriteria(key) {
			// For non-metadata search, we need to fetch the full key if metadata wasn't requested originally
			if !h.includeMetadata {
				h.matchedKeys = append(h.matchedKeys, KeyValuePair{
					Key: key.Key,
				})
			} else {
				h.matchedKeys = append(h.matchedKeys, key)
			}
		}
		
		h.processedCount++
		
		// Call progress callback if provided
		if h.progressCallback != nil && h.processedCount%50 == 0 {
			h.progressCallback(h.fetchedCount, h.processedCount, len(h.matchedKeys))
		}
	}
	
	h.logger.Debug("Processed %d keys, matched %d", len(keys), len(h.matchedKeys))
	
	return nil
}

// processConcurrently processes items using multiple goroutines
func (h *tagSearchHandler) processConcurrently(keys []KeyValuePair) error {
	var wg sync.WaitGroup
	
	// Create a channel to distribute work
	keyChan := make(chan KeyValuePair, len(keys))
	for _, key := range keys {
		keyChan <- key
	}
	close(keyChan)
	
	// Create workers
	concurrency := h.concurrency
	if concurrency > len(keys) {
		concurrency = len(keys)
	}
	
	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			var localMatches []KeyValuePair
			var localProcessed int
			
			// Process keys from the channel
			for key := range keyChan {
				if h.matchesCriteria(key) {
					if !h.includeMetadata {
						localMatches = append(localMatches, KeyValuePair{
							Key: key.Key,
						})
					} else {
						localMatches = append(localMatches, key)
					}
				}
				
				localProcessed++
			}
			
			// Add local results to global results
			if len(localMatches) > 0 {
				h.mu.Lock()
				h.matchedKeys = append(h.matchedKeys, localMatches...)
				h.mu.Unlock()
			}
			
			// Update processed count
			h.mu.Lock()
			h.processedCount += localProcessed
			processCopy := h.processedCount
			matchCopy := len(h.matchedKeys)
			h.mu.Unlock()
			
			// Call progress callback if provided
			if h.progressCallback != nil {
				h.progressCallback(h.fetchedCount, processCopy, matchCopy)
			}
		}()
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	h.logger.Debug("Processed %d keys concurrently, matched %d", len(keys), len(h.matchedKeys))
	
	return nil
}

// matchesCriteria checks if a key matches the search criteria
func (h *tagSearchHandler) matchesCriteria(key KeyValuePair) bool {
	// If the key has no metadata, it can't match
	if key.Metadata == nil {
		return false
	}
	
	// Check if the field exists in metadata
	value, exists := (*key.Metadata)[h.tagField]
	if !exists {
		return false
	}
	
	// If no tag value was specified, just matching the field is enough
	if h.tagValue == "" {
		return true
	}
	
	// Check if the value matches
	valueStr := fmt.Sprintf("%v", value)
	return strings.Contains(valueStr, h.tagValue)
}

// EnhancedSmartFindKeysWithValue is an improved version of SmartFindKeysWithValue
// that uses the common pagination utilities for searching by value in metadata
func EnhancedSmartFindKeysWithValue(
	client *api.Client,
	accountID,
	namespaceID,
	searchValue string,
	options *EnhancedSearchOptions,
	progressCallback func(fetched, processed, matched int)) ([]KeyValuePair, error) {
	
	// Validate inputs
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if searchValue == "" {
		return nil, fmt.Errorf("search value is required")
	}
	
	// Set default options if not provided
	if options == nil {
		options = &EnhancedSearchOptions{
			BatchSize:   1000,
			Concurrency: 5,
			Timeout:     120 * time.Second,
		}
	}
	
	// Create pagination options from search options
	pagOptions := &common.PaginationOptions{
		Debug:     options.Debug,
		Verbose:   options.Verbose,
		MaxRetries: 3,
		Timeout:   options.Timeout,
		LogPrefix: "Deep Search",
	}
	
	// Configure a search handler
	searchHandler := &valueSearchHandler{
		client:          client,
		accountID:       accountID,
		namespaceID:     namespaceID,
		searchValue:     searchValue,
		includeMetadata: options.IncludeMetadata,
		batchSize:       options.BatchSize,
		concurrency:     options.Concurrency,
		progressCallback: progressCallback,
		matchedKeys:     []KeyValuePair{},
		logger:          common.NewPaginationLogger(pagOptions, &common.PaginationResult{}),
	}
	
	// Execute pagination
	result, err := common.ExecutePagination(searchHandler, pagOptions)
	if err != nil {
		return nil, err
	}
	
	// Show warnings if verbose
	if options.Verbose && len(result.Warnings) > 0 {
		fmt.Println("\nWarnings during deep search operation:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}
	
	// Return the matched keys
	return searchHandler.matchedKeys, nil
}

// valueSearchHandler implements the PaginationHandler interface for deep value search
type valueSearchHandler struct {
	client          *api.Client
	accountID       string
	namespaceID     string
	searchValue     string
	includeMetadata bool
	batchSize       int
	concurrency     int
	progressCallback func(fetched, processed, matched int)
	matchedKeys     []KeyValuePair
	fetchedCount    int
	processedCount  int
	logger          *common.PaginationLogger
	mu              sync.Mutex
}

// FetchPage fetches a single page of keys
func (h *valueSearchHandler) FetchPage(cursor string) (interface{}, string, bool, error) {
	// Create list options for this page
	listOptions := &ListKeysOptions{
		Limit:  h.batchSize,
		Cursor: cursor,
		// Note: ListKeysOptions doesn't have an IncludeMetadata field,
		// but ListKeysWithOptions will return metadata by default
	}
	
	// Call the API to fetch a page of keys
	result, err := ListKeysWithOptions(h.client, h.accountID, h.namespaceID, listOptions)
	if err != nil {
		return nil, "", false, err
	}
	
	// Update fetched count
	h.fetchedCount += len(result.Keys)
	
	h.logger.Debug("Fetched %d keys (total %d)", len(result.Keys), h.fetchedCount)
	
	// Return the results
	return result.Keys, result.Cursor, !result.HasMore, nil
}

// ProcessItems processes the keys returned by FetchPage
func (h *valueSearchHandler) ProcessItems(items interface{}) error {
	keys, ok := items.([]KeyValuePair)
	if !ok {
		return fmt.Errorf("unexpected item type in value search")
	}
	
	// Process concurrently if concurrency > 1
	if h.concurrency > 1 && len(keys) > 10 {
		return h.processConcurrently(keys)
	}
	
	// Otherwise process serially
	return h.processSerially(keys)
}

// processSerially processes items in a single thread
func (h *valueSearchHandler) processSerially(keys []KeyValuePair) error {
	for _, key := range keys {
		// Search both key and metadata
		if h.matchesCriteria(key) {
			if !h.includeMetadata {
				h.matchedKeys = append(h.matchedKeys, KeyValuePair{
					Key: key.Key,
				})
			} else {
				h.matchedKeys = append(h.matchedKeys, key)
			}
		}
		
		h.processedCount++
		
		// Call progress callback if provided
		if h.progressCallback != nil && h.processedCount%50 == 0 {
			h.progressCallback(h.fetchedCount, h.processedCount, len(h.matchedKeys))
		}
	}
	
	h.logger.Debug("Processed %d keys, matched %d", len(keys), len(h.matchedKeys))
	
	return nil
}

// processConcurrently processes items using multiple goroutines
func (h *valueSearchHandler) processConcurrently(keys []KeyValuePair) error {
	var wg sync.WaitGroup
	
	// Create a channel to distribute work
	keyChan := make(chan KeyValuePair, len(keys))
	for _, key := range keys {
		keyChan <- key
	}
	close(keyChan)
	
	// Create workers
	concurrency := h.concurrency
	if concurrency > len(keys) {
		concurrency = len(keys)
	}
	
	// Start workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			var localMatches []KeyValuePair
			var localProcessed int
			
			// Process keys from the channel
			for key := range keyChan {
				if h.matchesCriteria(key) {
					if !h.includeMetadata {
						localMatches = append(localMatches, KeyValuePair{
							Key: key.Key,
						})
					} else {
						localMatches = append(localMatches, key)
					}
				}
				
				localProcessed++
			}
			
			// Add local results to global results
			if len(localMatches) > 0 {
				h.mu.Lock()
				h.matchedKeys = append(h.matchedKeys, localMatches...)
				h.mu.Unlock()
			}
			
			// Update processed count
			h.mu.Lock()
			h.processedCount += localProcessed
			processCopy := h.processedCount
			matchCopy := len(h.matchedKeys)
			h.mu.Unlock()
			
			// Call progress callback if provided
			if h.progressCallback != nil {
				h.progressCallback(h.fetchedCount, processCopy, matchCopy)
			}
		}()
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	h.logger.Debug("Processed %d keys concurrently, matched %d", len(keys), len(h.matchedKeys))
	
	return nil
}

// matchesCriteria checks if a key matches the deep search criteria
func (h *valueSearchHandler) matchesCriteria(key KeyValuePair) bool {
	// First check the key name itself
	if strings.Contains(key.Key, h.searchValue) {
		return true
	}
	
	// If the key has no metadata, can't match in metadata
	if key.Metadata == nil {
		return false
	}
	
	// Check recursively in all metadata fields
	// First convert to string to search in
	metadataStr := fmt.Sprintf("%v", *key.Metadata)
	return strings.Contains(metadataStr, h.searchValue)
}