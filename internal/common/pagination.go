package common

import (
	"fmt"
	"time"
)

// PaginationOptions defines common options for pagination operations
type PaginationOptions struct {
	// Debug enables detailed debug logging
	Debug bool
	
	// Verbose enables summary and progress information
	Verbose bool
	
	// MaxRetries is the number of times to retry on failure
	MaxRetries int
	
	// Timeout is the maximum time to spend on the entire pagination operation
	Timeout time.Duration
	
	// PageLimit is the maximum number of pages to fetch (0 = no limit)
	PageLimit int
	
	// LogPrefix is a prefix added to all log messages (e.g., "Namespaces: ")
	LogPrefix string
	
	// BatchSize is the size of each page to fetch
	BatchSize int
}

// PaginationResult captures the results and metadata from a pagination operation
type PaginationResult struct {
	// PageCount is the number of pages fetched
	PageCount int
	
	// ItemCount is the total number of items fetched
	ItemCount int
	
	// Warnings contains any non-fatal issues encountered during pagination
	Warnings []string
	
	// StartTime is when the pagination operation started
	StartTime time.Time
	
	// EndTime is when the pagination operation completed
	EndTime time.Time
}

// PaginationLogger provides standard logging functions for pagination operations
type PaginationLogger struct {
	options *PaginationOptions
	result  *PaginationResult
}

// NewPaginationLogger creates a new pagination logger
func NewPaginationLogger(options *PaginationOptions, result *PaginationResult) *PaginationLogger {
	return &PaginationLogger{
		options: options,
		result:  result,
	}
}

// Debug logs a debug message
func (l *PaginationLogger) Debug(format string, args ...interface{}) {
	if l.options.Debug {
		prefix := ""
		if l.options.LogPrefix != "" {
			prefix = l.options.LogPrefix + " "
		}
		fmt.Printf("[DEBUG] %s"+format+"\n", append([]interface{}{prefix}, args...)...)
	}
}

// Verbose logs a verbose message
func (l *PaginationLogger) Verbose(format string, args ...interface{}) {
	if l.options.Verbose {
		prefix := ""
		if l.options.LogPrefix != "" {
			prefix = l.options.LogPrefix + " "
		}
		fmt.Printf("[INFO] %s"+format+"\n", append([]interface{}{prefix}, args...)...)
	}
}

// Warning adds a warning to the result and optionally logs it
func (l *PaginationLogger) Warning(warning string) {
	l.result.Warnings = append(l.result.Warnings, warning)
	l.Verbose("WARNING: %s", warning)
}

// PaginationHandler provides a standard interface for pagination operations
type PaginationHandler interface {
	// FetchPage fetches a single page of results starting at the given cursor
	// Returns the items for this page, the next cursor, a completion flag, and any error
	FetchPage(cursor string) (items interface{}, nextCursor string, complete bool, err error)
	
	// ProcessItems processes the items returned by FetchPage
	// This is called for each page and allows custom handling of items
	ProcessItems(items interface{}) error
}

// ExecutePagination performs a pagination operation with consistent handling
func ExecutePagination(handler PaginationHandler, options *PaginationOptions) (*PaginationResult, error) {
	// Initialize default options if not provided
	if options == nil {
		options = &PaginationOptions{
			MaxRetries: 3,
			Timeout:    30 * time.Second,
		}
	}
	
	// Initialize result
	result := &PaginationResult{
		StartTime: time.Now(),
	}
	
	// Set up logger
	logger := NewPaginationLogger(options, result)
	
	logger.Debug("Starting pagination operation")
	logger.Verbose("Beginning pagination...")
	
	var cursor string
	var seenCursors = make(map[string]bool)
	
	// Set an overall timeout if specified
	var timeoutChan <-chan time.Time
	if options.Timeout > 0 {
		timer := time.NewTimer(options.Timeout)
		defer timer.Stop()
		timeoutChan = timer.C
	}
	
	logger.Debug("Beginning pagination loop")
	
	// Loop for pagination
	for {
		// Check for timeout
		if options.Timeout > 0 {
			select {
			case <-timeoutChan:
				logger.Warning(fmt.Sprintf("Operation timed out after %v, results may be incomplete", options.Timeout))
				goto FINISH
			default:
				// Continue with the loop
			}
		}
		
		// Check if we've hit the page limit
		if options.PageLimit > 0 && result.PageCount >= options.PageLimit {
			logger.Warning(fmt.Sprintf("Reached configured page limit (%d), results may be incomplete", options.PageLimit))
			break
		}
		
		// Log cursor information
		if cursor != "" {
			logger.Debug("Fetching page with cursor: %s", cursor)
		} else {
			logger.Debug("Fetching first page")
		}
		
		// Fetch the page
		var items interface{}
		var nextCursor string
		var complete bool
		var err error
		
		retries := options.MaxRetries
		for {
			items, nextCursor, complete, err = handler.FetchPage(cursor)
			if err == nil {
				break
			}
			
			// Check if we can retry
			if retries > 0 {
				retries--
				logger.Verbose("Request failed, retrying (%d retries left): %v", retries, err)
				time.Sleep(1 * time.Second) // Simple backoff
				continue
			}
			
			// No more retries, return the error
			result.EndTime = time.Now()
			return result, fmt.Errorf("pagination failed after %d retries: %w", options.MaxRetries, err)
		}
		
		// Increment page counter
		result.PageCount++
		
		// Process the items
		if err := handler.ProcessItems(items); err != nil {
			result.EndTime = time.Now()
			return result, fmt.Errorf("failed to process items: %w", err)
		}
		
		// Check for completion flag
		if complete {
			logger.Debug("Pagination complete (completion flag set)")
			break
		}
		
		// Check if we need to fetch more pages
		cursor = nextCursor
		if cursor == "" {
			logger.Debug("No more pages (empty cursor)")
			break
		}
		
		// Detect cursor loops (can happen with eventual consistency issues)
		if seenCursors[cursor] {
			logger.Warning(fmt.Sprintf("Detected cursor loop (%s), breaking pagination to avoid infinite loop", cursor))
			break
		}
		
		// Record the cursor we've seen
		seenCursors[cursor] = true
		
		// Simple output of progress if verbose
		if options.Verbose && result.PageCount > 1 {
			elapsedTime := time.Since(result.StartTime)
			logger.Verbose("Pagination progress: %d pages, %v elapsed", 
				result.PageCount, elapsedTime.Round(time.Millisecond))
		}
	}
	
FINISH:
	result.EndTime = time.Now()
	elapsedTime := result.EndTime.Sub(result.StartTime)
	
	logger.Debug("Pagination complete: fetched %d items in %d pages", 
		result.ItemCount, result.PageCount)
	logger.Verbose("Operation completed in %v", elapsedTime.Round(time.Millisecond))
	
	return result, nil
}