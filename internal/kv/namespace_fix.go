package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cache-kv-purger/internal/api"
)

// ListNamespacesOptions provides advanced options for listing namespaces
type ListNamespacesOptions struct {
	Debug      bool // Enable debug output
	Verbose    bool // Enable verbose output
	MaxRetries int  // Maximum number of retries for failed requests
	Timeout    time.Duration // Timeout for the entire operation
	PageLimit  int  // Maximum number of pages to fetch (0 = no limit)
}

// ListNamespacesResult contains the results of a namespace listing operation
type ListNamespacesResult struct {
	Namespaces []Namespace // The list of namespaces
	TotalCount int         // Total count of namespaces found
	PageCount  int         // Number of pages fetched
	Warnings   []string    // Any warnings that occurred during listing
}

// EnhancedListNamespaces lists all KV namespaces with improved pagination handling
func EnhancedListNamespaces(client *api.Client, accountID string, options *ListNamespacesOptions) (*ListNamespacesResult, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	
	// Initialize default options if needed
	if options == nil {
		options = &ListNamespacesOptions{
			MaxRetries: 3,
			Timeout:    30 * time.Second,
		}
	}
	
	// Initialize debugging functions
	debug := func(format string, args ...interface{}) {
		if options.Debug {
			fmt.Printf("[DEBUG] "+format+"\n", args...)
		}
	}
	
	verbose := func(format string, args ...interface{}) {
		if options.Verbose {
			fmt.Printf("[INFO] "+format+"\n", args...)
		}
	}
	
	debug("Starting enhanced namespace listing for account: %s", accountID)
	verbose("Fetching namespaces...")
	
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces", accountID)
	
	var allNamespaces []Namespace
	var cursor string
	var seenCursors = make(map[string]bool)
	var warnings []string
	pageCount := 0
	startTime := time.Now()
	
	// Set an overall timeout if specified
	var timeoutChan <-chan time.Time
	if options.Timeout > 0 {
		timer := time.NewTimer(options.Timeout)
		defer timer.Stop()
		timeoutChan = timer.C
	}
	
	debug("Beginning pagination loop")
	
	// Loop for pagination
	for {
		// Check for timeout
		if options.Timeout > 0 {
			select {
			case <-timeoutChan:
				warning := fmt.Sprintf("Operation timed out after %v, results may be incomplete", options.Timeout)
				warnings = append(warnings, warning)
				verbose(warning)
				goto FINISH
			default:
				// Continue with the loop
			}
		}
		
		// Check if we've hit the page limit
		if options.PageLimit > 0 && pageCount >= options.PageLimit {
			warning := fmt.Sprintf("Reached configured page limit (%d), results may be incomplete", options.PageLimit)
			warnings = append(warnings, warning)
			verbose(warning)
			break
		}
		
		// Set up query parameters for pagination if we have a cursor
		var queryParams url.Values
		if cursor != "" {
			queryParams = url.Values{}
			queryParams.Set("cursor", cursor)
			debug("Fetching page with cursor: %s", cursor)
		} else {
			debug("Fetching first page")
		}
		
		respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
		if err != nil {
			// Check if this is a retryable error and we have retries left
			if options.MaxRetries > 0 {
				options.MaxRetries--
				verbose("Request failed, retrying (%d retries left): %v", options.MaxRetries, err)
				time.Sleep(1 * time.Second) // Simple backoff
				continue
			}
			return nil, fmt.Errorf("failed to request namespaces: %w", err)
		}
		
		var nsResp NamespacesResponse
		if err := json.Unmarshal(respBody, &nsResp); err != nil {
			return nil, fmt.Errorf("failed to parse API response: %w", err)
		}
		
		if !nsResp.Success {
			errorStr := "API reported failure"
			if len(nsResp.Errors) > 0 {
				errorStr = nsResp.Errors[0].Message
			}
			return nil, fmt.Errorf("failed to list namespaces: %s", errorStr)
		}
		
		// Increment page counter
		pageCount++
		
		// Get current page information
		currentPageCount := len(nsResp.Result)
		currentPageCursor := nsResp.ResultInfo.Cursor
		
		debug("Page %d: Retrieved %d namespaces", pageCount, currentPageCount)
		verbose("Retrieved %d namespaces (total so far: %d)", currentPageCount, len(allNamespaces)+currentPageCount)
		
		// Check for suspicious empty results
		if currentPageCount == 0 && currentPageCursor != "" {
			warning := "Received empty result set with non-empty cursor, possible API inconsistency"
			warnings = append(warnings, warning)
			verbose(warning)
		}
		
		// Append results from this page
		allNamespaces = append(allNamespaces, nsResp.Result...)
		
		// Check if we need to fetch more pages
		cursor = currentPageCursor
		if cursor == "" {
			debug("No more pages (empty cursor)")
			break
		}
		
		// Detect cursor loops (can happen with eventual consistency issues)
		if seenCursors[cursor] {
			warning := fmt.Sprintf("Detected cursor loop (%s), breaking pagination to avoid infinite loop", cursor)
			warnings = append(warnings, warning)
			verbose(warning)
			break
		}
		
		// Record the cursor we've seen
		seenCursors[cursor] = true
		
		// Simple output of progress if verbose
		if options.Verbose && pageCount > 1 {
			elapsedTime := time.Since(startTime)
			verbose("Pagination progress: %d pages, %d namespaces, %v elapsed", 
				pageCount, len(allNamespaces), elapsedTime.Round(time.Millisecond))
		}
	}
	
FINISH:
	debug("Pagination complete: retrieved %d namespaces in %d pages", len(allNamespaces), pageCount)
	verbose("Found %d namespaces in %d pages, took %v", 
		len(allNamespaces), pageCount, time.Since(startTime).Round(time.Millisecond))
	
	// Return the results with metadata
	result := &ListNamespacesResult{
		Namespaces: allNamespaces,
		TotalCount: len(allNamespaces),
		PageCount:  pageCount,
		Warnings:   warnings,
	}
	
	return result, nil
}

// ListNamespacesWithOutput is an enhanced version of ListNamespaces that provides better output
// and ensures all namespaces are retrieved through proper pagination
func ListNamespacesWithOutput(client *api.Client, accountID string, options *ListNamespacesOptions) ([]Namespace, error) {
	result, err := EnhancedListNamespaces(client, accountID, options)
	if err != nil {
		return nil, err
	}
	
	// Print any warnings
	for _, warning := range result.Warnings {
		fmt.Printf("WARNING: %s\n", warning)
	}
	
	// Print pagination info if more than one page
	if result.PageCount > 1 {
		if options != nil && options.Verbose {
			fmt.Printf("Retrieved %d namespaces from %d pages\n", result.TotalCount, result.PageCount)
		} else {
			fmt.Printf("Retrieved %d namespaces\n", result.TotalCount)
		}
	}
	
	return result.Namespaces, nil
}