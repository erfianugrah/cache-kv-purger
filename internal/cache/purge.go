package cache

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cache-kv-purger/internal/api"
)

// FileWithHeaders represents a file URL with associated headers for purging
type FileWithHeaders struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// PurgeOptions represents options for purging cache
type PurgeOptions struct {
	// PurgeEverything indicates whether to purge the entire cache
	PurgeEverything bool `json:"purge_everything,omitempty"`

	// Files is a list of URLs to individually purge
	// This field supports both string URLs and FileWithHeaders objects
	// The API marshaling will handle this correctly based on the actual type
	Files interface{} `json:"files,omitempty"`

	// Tags is a list of cache tags to purge
	Tags []string `json:"tags,omitempty"`

	// Hosts is a list of hosts to purge
	Hosts []string `json:"hosts,omitempty"`

	// Prefixes is a list of URI prefixes to purge
	Prefixes []string `json:"prefixes,omitempty"`
}

// PurgeResponse represents the response from a cache purge request
type PurgeResponse struct {
	Success  bool        `json:"success"`
	Errors   []api.Error `json:"errors"`
	Messages []string    `json:"messages"`
	Result   struct {
		ID string `json:"id"`
	} `json:"result"`
}

// PurgeCache purges cache for a zone based on the provided options
func PurgeCache(client *api.Client, zoneID string, options PurgeOptions) (*PurgeResponse, error) {
	if zoneID == "" {
		return nil, fmt.Errorf("zone ID is required")
	}

	// Validate options
	filesEmpty := false
	switch v := options.Files.(type) {
	case []string:
		filesEmpty = len(v) == 0
	case []FileWithHeaders:
		filesEmpty = len(v) == 0
	case nil:
		filesEmpty = true
	default:
		// If it's some other type, assume it's not empty
		filesEmpty = false
	}

	if !options.PurgeEverything &&
		filesEmpty &&
		len(options.Tags) == 0 &&
		len(options.Hosts) == 0 &&
		len(options.Prefixes) == 0 {
		return nil, fmt.Errorf("at least one purge parameter (purge_everything, files, tags, hosts, prefixes) must be specified")
	}

	// Make the purge request
	path := fmt.Sprintf("/zones/%s/purge_cache", zoneID)
	respBody, err := client.Request(http.MethodPost, path, nil, options)
	if err != nil {
		return nil, err
	}

	// Parse the response
	var purgeResp PurgeResponse
	if err := json.Unmarshal(respBody, &purgeResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !purgeResp.Success {
		errorStr := "API reported failure"
		if len(purgeResp.Errors) > 0 {
			errorStr = purgeResp.Errors[0].Message
		}
		return nil, fmt.Errorf("cache purge failed: %s", errorStr)
	}

	return &purgeResp, nil
}

// PurgeEverything purges all files from a zone
func PurgeEverything(client *api.Client, zoneID string) (*PurgeResponse, error) {
	options := PurgeOptions{
		PurgeEverything: true,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgeFiles purges specific files from a zone
func PurgeFiles(client *api.Client, zoneID string, files []string) (*PurgeResponse, error) {
	options := PurgeOptions{
		Files: files,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgeFilesWithHeaders purges specific files with custom cache headers from a zone
// This implements the Cloudflare API feature to purge cache by URL with headers for variant matching
// Headers like CF-Device-Type, CF-IPCountry, Accept-Language, and Origin can be used to target
// specific cache variants.
func PurgeFilesWithHeaders(client *api.Client, zoneID string, files []FileWithHeaders) (*PurgeResponse, error) {
	options := PurgeOptions{
		Files: files,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgeFilesWithHeadersInBatches purges files with custom headers in batches to comply with Cloudflare API limits
// The batch size is set to 100 items per request (Cloudflare API limit)
// The function takes a progressCallback that receives updates on completed/total batches
func PurgeFilesWithHeadersInBatches(client *api.Client, zoneID string, files []FileWithHeaders,
	progressCallback func(completed, total, successful int), concurrencyOverride int) ([]FileWithHeaders, []error) {

	if zoneID == "" {
		return nil, []error{fmt.Errorf("zone ID is required")}
	}

	if len(files) == 0 {
		return nil, []error{fmt.Errorf("at least one file with headers is required")}
	}

	// Default batch size based on API limits
	batchSize := 100 // API has a limit of 100 items per purge request

	// Calculate number of batches (used for log progress updates)
	_ = (len(files) + batchSize - 1) / batchSize

	// Simple progress reporting if none provided
	if progressCallback == nil {
		progressCallback = func(completed, total, successful int) {}
	}

	// Create work items for all batches
	type batchWork struct {
		batchIndex int
		batchItems []FileWithHeaders
	}

	var batches []batchWork
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		batches = append(batches, batchWork{
			batchIndex: i / batchSize,
			batchItems: batch,
		})
	}

	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex int
		batchItems []FileWithHeaders
		err        error
	}

	resultChan := make(chan batchResult, len(batches))

	// Set concurrency based on override or default
	concurrency := 10 // Default concurrency
	if concurrencyOverride > 0 {
		concurrency = concurrencyOverride
	}

	// Cap concurrency based on account tier
	if concurrency > 50 {
		concurrency = 50 // Enterprise tier allows 50 requests per second
	}

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, concurrency)

	// Process all batches
	for _, batch := range batches {
		// Acquire semaphore slot (or wait if at capacity)
		sem <- struct{}{}

		// Launch a goroutine to process this batch
		go func(b batchWork) {
			defer func() { <-sem }() // Release semaphore when done

			// Purge this batch of files with headers
			_, err := PurgeFilesWithHeaders(client, zoneID, b.batchItems)

			// Send result back through channel
			if err != nil {
				resultChan <- batchResult{
					batchIndex: b.batchIndex,
					batchItems: nil,
					err:        fmt.Errorf("batch %d failed: %w", b.batchIndex+1, err),
				}
				return
			}

			resultChan <- batchResult{
				batchIndex: b.batchIndex,
				batchItems: b.batchItems,
				err:        nil,
			}
		}(batch)
	}

	// Collect results
	successful := make([]FileWithHeaders, 0)
	var errors []error

	// Track progress for callback
	completed := 0
	completedItems := 0

	// Collect results from all batches
	for i := 0; i < len(batches); i++ {
		result := <-resultChan

		// Save error or success
		if result.err != nil {
			errors = append(errors, result.err)
		} else if result.batchItems != nil {
			successful = append(successful, result.batchItems...)
		}

		// Update progress
		completed++
		if result.batchItems != nil {
			completedItems += len(result.batchItems)
		}

		// Call progress callback
		progressCallback(completed, len(batches), len(successful))
	}

	return successful, errors
}

// PurgeFilesWithHeadersAcrossZonesInBatches purges files with headers from multiple zones in batches
// Useful for purging the same set of files across multiple zones
func PurgeFilesWithHeadersAcrossZonesInBatches(client *api.Client, zoneIDs []string, files []FileWithHeaders,
	progressCallback func(zoneIndex, totalZones, batchesDone, totalBatches, successful int),
	concurrencyOverride int) (map[string][]FileWithHeaders, map[string][]error) {

	if len(zoneIDs) == 0 {
		return nil, map[string][]error{"error": {fmt.Errorf("at least one zone ID is required")}}
	}

	if len(files) == 0 {
		return nil, map[string][]error{"error": {fmt.Errorf("at least one file with headers is required")}}
	}

	// Simple progress reporting if none provided
	if progressCallback == nil {
		progressCallback = func(zoneIndex, totalZones, batchesDone, totalBatches, successfulCount int) {}
	}

	// Initialize results for each zone (don't need mutex as we're using a channel for results)
	successfulByZone := make(map[string][]FileWithHeaders)
	errorsByZone := make(map[string][]error)

	// Default batch size
	batchSize := 100 // API has a limit of 100 items per purge request

	// Calculate total number of batches across all zones
	batchesPerZone := (len(files) + batchSize - 1) / batchSize
	totalBatches := batchesPerZone * len(zoneIDs)

	// Create a result channel for completed zones (eliminates need for mutex)
	type zoneResult struct {
		zoneIndex  int
		zoneID     string
		successful []FileWithHeaders
		errors     []error
	}

	resultChan := make(chan zoneResult, len(zoneIDs))

	// Set concurrency based on override or default
	concurrency := 3 // Default maximum number of zones to process concurrently
	if concurrencyOverride > 0 {
		concurrency = concurrencyOverride
	}

	// Use a semaphore to limit concurrent zone processing
	sem := make(chan struct{}, concurrency)

	// Process all zones
	for i, zoneID := range zoneIDs {
		// Acquire semaphore slot
		sem <- struct{}{}

		// Launch a goroutine to process this zone
		go func(idx int, zID string) {
			defer func() { <-sem }() // Release semaphore when done

			// Counter for batches completed in this zone
			zoneProgress := 0

			// Create a zone-specific progress callback
			zoneProgressCallback := func(batchCompleted, batchTotal, successfulCount int) {
				// Update zone progress
				zoneProgress = batchCompleted

				// Call the parent progress callback
				progressCallback(idx+1, len(zoneIDs),
					(idx*batchesPerZone)+zoneProgress, // overall batches done
					totalBatches, successfulCount)
			}

			// Purge files with headers for this zone
			successful, errors := PurgeFilesWithHeadersInBatches(client, zID, files, zoneProgressCallback, concurrencyOverride)

			// Send result back through channel
			resultChan <- zoneResult{
				zoneIndex:  idx,
				zoneID:     zID,
				successful: successful,
				errors:     errors,
			}
		}(i, zoneID)
	}

	// Collect results from all zones
	for i := 0; i < len(zoneIDs); i++ {
		result := <-resultChan

		// Store results for this zone
		if len(result.successful) > 0 {
			successfulByZone[result.zoneID] = result.successful
		}

		if len(result.errors) > 0 {
			errorsByZone[result.zoneID] = result.errors
		}
	}

	return successfulByZone, errorsByZone
}

// PurgeTags purges files with specific cache tags from a zone
func PurgeTags(client *api.Client, zoneID string, tags []string) (*PurgeResponse, error) {
	options := PurgeOptions{
		Tags: tags,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgeHosts purges files from specific hosts in a zone
func PurgeHosts(client *api.Client, zoneID string, hosts []string) (*PurgeResponse, error) {
	options := PurgeOptions{
		Hosts: hosts,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgeHostsInBatches purges hosts in batches with concurrency support
// This is optimized for purging a large number of hosts
func PurgeHostsInBatches(client *api.Client, zoneID string, hosts []string,
	progressCallback func(completed, total, successful int), concurrencyOverride int) ([]string, []error) {

	if zoneID == "" {
		return nil, []error{fmt.Errorf("zone ID is required")}
	}

	if len(hosts) == 0 {
		return nil, []error{fmt.Errorf("at least one host is required")}
	}

	// Define batch size based on API limits
	batchSize := 100 // API has a limit of 100 items per purge request

	// Simple progress callback if none provided
	if progressCallback == nil {
		progressCallback = func(completed, total, successful int) {}
	}

	// Create work items for all batches
	type batchWork struct {
		batchIndex int
		batchItems []string
	}

	var batches []batchWork
	for i := 0; i < len(hosts); i += batchSize {
		end := i + batchSize
		if end > len(hosts) {
			end = len(hosts)
		}

		batch := hosts[i:end]
		batches = append(batches, batchWork{
			batchIndex: i / batchSize,
			batchItems: batch,
		})
	}

	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex int
		batchItems []string
		err        error
	}

	resultChan := make(chan batchResult, len(batches))

	// Set concurrency based on override or default
	concurrency := 10 // Default concurrency
	if concurrencyOverride > 0 {
		concurrency = concurrencyOverride
	}

	// Cap concurrency based on account tier
	if concurrency > 50 {
		concurrency = 50 // Enterprise tier allows 50 requests per second
	}

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, concurrency)

	// Process all batches
	for _, batch := range batches {
		// Acquire semaphore slot (or wait if at capacity)
		sem <- struct{}{}

		// Launch a goroutine to process this batch
		go func(b batchWork) {
			defer func() { <-sem }() // Release semaphore when done

			// Purge this batch of hosts
			_, err := PurgeHosts(client, zoneID, b.batchItems)

			// Send result back through channel
			if err != nil {
				resultChan <- batchResult{
					batchIndex: b.batchIndex,
					batchItems: nil,
					err:        fmt.Errorf("batch %d failed: %w", b.batchIndex+1, err),
				}
				return
			}

			resultChan <- batchResult{
				batchIndex: b.batchIndex,
				batchItems: b.batchItems,
				err:        nil,
			}
		}(batch)
	}

	// Collect results
	successful := make([]string, 0)
	var errors []error

	// Track progress for callback
	completed := 0

	// Collect results from all batches
	for i := 0; i < len(batches); i++ {
		result := <-resultChan

		// Save error or success
		if result.err != nil {
			errors = append(errors, result.err)
		} else if result.batchItems != nil {
			successful = append(successful, result.batchItems...)
		}

		// Update progress
		completed++

		// Call progress callback
		progressCallback(completed, len(batches), len(successful))
	}

	return successful, errors
}

// PurgePrefixes purges files with specific URI prefixes from a zone
func PurgePrefixes(client *api.Client, zoneID string, prefixes []string) (*PurgeResponse, error) {
	options := PurgeOptions{
		Prefixes: prefixes,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgePrefixesInBatches purges prefixes in batches with concurrency support
// This is optimized for purging a large number of prefixes
func PurgePrefixesInBatches(client *api.Client, zoneID string, prefixes []string,
	progressCallback func(completed, total, successful int), concurrencyOverride int) ([]string, []error) {

	if zoneID == "" {
		return nil, []error{fmt.Errorf("zone ID is required")}
	}

	if len(prefixes) == 0 {
		return nil, []error{fmt.Errorf("at least one prefix is required")}
	}

	// Define batch size based on API limits
	batchSize := 100 // API has a limit of 100 items per purge request

	// Simple progress callback if none provided
	if progressCallback == nil {
		progressCallback = func(completed, total, successful int) {}
	}

	// Create work items for all batches
	type batchWork struct {
		batchIndex int
		batchItems []string
	}

	var batches []batchWork
	for i := 0; i < len(prefixes); i += batchSize {
		end := i + batchSize
		if end > len(prefixes) {
			end = len(prefixes)
		}

		batch := prefixes[i:end]
		batches = append(batches, batchWork{
			batchIndex: i / batchSize,
			batchItems: batch,
		})
	}

	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex int
		batchItems []string
		err        error
	}

	resultChan := make(chan batchResult, len(batches))

	// Set concurrency based on override or default
	concurrency := 10 // Default concurrency
	if concurrencyOverride > 0 {
		concurrency = concurrencyOverride
	}

	// Cap concurrency based on account tier
	if concurrency > 50 {
		concurrency = 50 // Enterprise tier allows 50 requests per second
	}

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, concurrency)

	// Process all batches
	for _, batch := range batches {
		// Acquire semaphore slot (or wait if at capacity)
		sem <- struct{}{}

		// Launch a goroutine to process this batch
		go func(b batchWork) {
			defer func() { <-sem }() // Release semaphore when done

			// Purge this batch of prefixes
			_, err := PurgePrefixes(client, zoneID, b.batchItems)

			// Send result back through channel
			if err != nil {
				resultChan <- batchResult{
					batchIndex: b.batchIndex,
					batchItems: nil,
					err:        fmt.Errorf("batch %d failed: %w", b.batchIndex+1, err),
				}
				return
			}

			resultChan <- batchResult{
				batchIndex: b.batchIndex,
				batchItems: b.batchItems,
				err:        nil,
			}
		}(batch)
	}

	// Collect results
	successful := make([]string, 0)
	var errors []error

	// Track progress for callback
	completed := 0

	// Collect results from all batches
	for i := 0; i < len(batches); i++ {
		result := <-resultChan

		// Save error or success
		if result.err != nil {
			errors = append(errors, result.err)
		} else if result.batchItems != nil {
			successful = append(successful, result.batchItems...)
		}

		// Update progress
		completed++

		// Call progress callback
		progressCallback(completed, len(batches), len(successful))
	}

	return successful, errors
}

// PurgeTagsInBatches purges tags in batches of 30 or fewer to comply with Cloudflare API limits
// The function takes a progressCallback that receives updates on completed/total batches
// This version uses concurrency for faster processing when handling many batches
func PurgeTagsInBatches(client *api.Client, zoneID string, tags []string, progressCallback func(completed, total, successful int), concurrencyOverride int) ([]string, []error) {
	if zoneID == "" {
		return nil, []error{fmt.Errorf("zone ID is required")}
	}

	if len(tags) == 0 {
		return nil, []error{fmt.Errorf("at least one tag is required")}
	}

	// Define batch size based on API limits
	batchSize := 100 // API has a limit of 100 items per purge request

	// Calculate number of batches (this will be reflected in the length of the batches slice)

	// Simple progress callback if none provided
	if progressCallback == nil {
		progressCallback = func(completed, total, successful int) {}
	}

	// Create work items for all batches
	type batchWork struct {
		batchIndex int
		batchItems []string
	}

	var batches []batchWork
	for i := 0; i < len(tags); i += batchSize {
		end := i + batchSize
		if end > len(tags) {
			end = len(tags)
		}

		batch := tags[i:end]
		batches = append(batches, batchWork{
			batchIndex: i / batchSize,
			batchItems: batch,
		})
	}

	// Create a result channel for completed batches
	type batchResult struct {
		batchIndex int
		batchItems []string
		err        error
	}

	resultChan := make(chan batchResult, len(batches))

	// Set concurrency based on override or default
	concurrency := 10 // Default concurrency
	if concurrencyOverride > 0 {
		concurrency = concurrencyOverride
	}

	// Cap concurrency based on account tier
	if concurrency > 50 {
		concurrency = 50 // Enterprise tier allows 50 requests per second
	}

	// Use a semaphore to limit concurrent goroutines
	sem := make(chan struct{}, concurrency)

	// Process all batches
	for _, batch := range batches {
		// Acquire semaphore slot (or wait if at capacity)
		sem <- struct{}{}

		// Launch a goroutine to process this batch
		go func(b batchWork) {
			defer func() { <-sem }() // Release semaphore when done

			// Purge this batch of tags
			_, err := PurgeTags(client, zoneID, b.batchItems)

			// Send result back through channel
			if err != nil {
				resultChan <- batchResult{
					batchIndex: b.batchIndex,
					batchItems: nil,
					err:        fmt.Errorf("batch %d failed: %w", b.batchIndex+1, err),
				}
				return
			}

			resultChan <- batchResult{
				batchIndex: b.batchIndex,
				batchItems: b.batchItems,
				err:        nil,
			}
		}(batch)
	}

	// Collect results
	successful := make([]string, 0)
	var errors []error

	// Track progress for callback
	completed := 0

	// Collect results from all batches
	for i := 0; i < len(batches); i++ {
		result := <-resultChan

		// Save error or success
		if result.err != nil {
			errors = append(errors, result.err)
		} else if result.batchItems != nil {
			successful = append(successful, result.batchItems...)
		}

		// Update progress
		completed++

		// Call progress callback
		progressCallback(completed, len(batches), len(successful))
	}

	return successful, errors
}

// PurgeTagsAcrossZonesInBatches purges tags from multiple zones in batches
// Useful for purging the same set of tags across multiple zones
// This version uses concurrency for both zone-level and batch-level processing
func PurgeTagsAcrossZonesInBatches(client *api.Client, zoneIDs []string, tags []string,
	progressCallback func(zoneIndex, totalZones, batchesDone, totalBatches, successful int),
	batchConcurrency, zoneConcurrency int) (map[string][]string, map[string][]error) {

	if len(zoneIDs) == 0 {
		return nil, map[string][]error{"error": {fmt.Errorf("at least one zone ID is required")}}
	}

	if len(tags) == 0 {
		return nil, map[string][]error{"error": {fmt.Errorf("at least one tag is required")}}
	}

	// Simple progress reporting if none provided
	if progressCallback == nil {
		progressCallback = func(zoneIndex, totalZones, batchesDone, totalBatches, successfulCount int) {}
	}

	// Initialize results for each zone (don't need mutex as we're using a channel for results)
	successfulByZone := make(map[string][]string)
	errorsByZone := make(map[string][]error)

	// Default batch size
	batchSize := 100 // API has a limit of 100 items per purge request

	// Calculate total number of batches across all zones
	batchesPerZone := (len(tags) + batchSize - 1) / batchSize
	totalBatches := batchesPerZone * len(zoneIDs)

	// Create a result channel for completed zones (eliminates need for mutex)
	type zoneResult struct {
		zoneIndex  int
		zoneID     string
		successful []string
		errors     []error
	}

	resultChan := make(chan zoneResult, len(zoneIDs))

	// Set concurrency based on override or default
	concurrency := 3 // Default maximum number of zones to process concurrently
	if zoneConcurrency > 0 {
		concurrency = zoneConcurrency
	}

	// Use a semaphore to limit concurrent zone processing
	sem := make(chan struct{}, concurrency)

	// Process all zones
	for i, zoneID := range zoneIDs {
		// Acquire semaphore slot
		sem <- struct{}{}

		// Launch a goroutine to process this zone
		go func(idx int, zID string) {
			defer func() { <-sem }() // Release semaphore when done

			// Counter for batches completed in this zone
			zoneProgress := 0

			// Create a zone-specific progress callback
			zoneProgressCallback := func(batchCompleted, batchTotal, successfulCount int) {
				// Update zone progress
				zoneProgress = batchCompleted

				// Call the parent progress callback
				progressCallback(idx+1, len(zoneIDs),
					(idx*batchesPerZone)+zoneProgress, // overall batches done
					totalBatches, successfulCount)
			}

			// Purge tags for this zone
			successful, errors := PurgeTagsInBatches(client, zID, tags, zoneProgressCallback, batchConcurrency)

			// Send result back through channel
			resultChan <- zoneResult{
				zoneIndex:  idx,
				zoneID:     zID,
				successful: successful,
				errors:     errors,
			}
		}(i, zoneID)
	}

	// Collect results from all zones
	for i := 0; i < len(zoneIDs); i++ {
		result := <-resultChan

		// Store results for this zone
		if len(result.successful) > 0 {
			successfulByZone[result.zoneID] = result.successful
		}

		if len(result.errors) > 0 {
			errorsByZone[result.zoneID] = result.errors
		}
	}

	return successfulByZone, errorsByZone
}
