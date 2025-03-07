package cache

import (
	"encoding/json"
	"fmt"
	"net/http"

	"cache-kv-purger/internal/api"
)

// PurgeOptions represents options for purging cache
type PurgeOptions struct {
	// PurgeEverything indicates whether to purge the entire cache
	PurgeEverything bool `json:"purge_everything,omitempty"`

	// Files is a list of URLs to individually purge
	Files []string `json:"files,omitempty"`

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
	if !options.PurgeEverything &&
		len(options.Files) == 0 &&
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

// PurgePrefixes purges files with specific URI prefixes from a zone
func PurgePrefixes(client *api.Client, zoneID string, prefixes []string) (*PurgeResponse, error) {
	options := PurgeOptions{
		Prefixes: prefixes,
	}
	return PurgeCache(client, zoneID, options)
}

// PurgeTagsInBatches purges tags in batches of 30 or fewer to comply with Cloudflare API limits
// The function takes a progressCallback that receives updates on completed/total batches
func PurgeTagsInBatches(client *api.Client, zoneID string, tags []string, progressCallback func(completed, total, successful int)) ([]string, []error) {
	if zoneID == "" {
		return nil, []error{fmt.Errorf("zone ID is required")}
	}

	if len(tags) == 0 {
		return nil, []error{fmt.Errorf("at least one tag is required")}
	}

	// Define batch size (Cloudflare API limit is 30 tags per request)
	batchSize := 30

	// Calculate number of batches
	numBatches := (len(tags) + batchSize - 1) / batchSize

	// Initialize results
	successful := make([]string, 0)
	var errors []error

	// Process each batch
	for i := 0; i < len(tags); i += batchSize {
		// Get the current batch
		end := i + batchSize
		if end > len(tags) {
			end = len(tags)
		}
		currentBatch := tags[i:end]

		// Log the batch if verbose is enabled
		batchNum := i/batchSize + 1

		// Purge this batch of tags
		_, err := PurgeTags(client, zoneID, currentBatch)

		// Call progress callback if provided
		if progressCallback != nil {
			progressCallback(batchNum, numBatches, len(successful))
		}

		// Handle error
		if err != nil {
			errors = append(errors, fmt.Errorf("batch %d failed: %w", batchNum, err))
			continue
		}

		// Add successfully purged tags to the result
		successful = append(successful, currentBatch...)
	}

	return successful, errors
}

// PurgeTagsAcrossZonesInBatches purges tags from multiple zones in batches
// Useful for purging the same set of tags across multiple zones
func PurgeTagsAcrossZonesInBatches(client *api.Client, zoneIDs []string, tags []string,
	progressCallback func(zoneIndex, totalZones, batchesDone, totalBatches, successful int)) (map[string][]string, map[string][]error) {

	if len(zoneIDs) == 0 {
		return nil, map[string][]error{"error": {fmt.Errorf("at least one zone ID is required")}}
	}

	if len(tags) == 0 {
		return nil, map[string][]error{"error": {fmt.Errorf("at least one tag is required")}}
	}

	// Initialize results for each zone
	successfulByZone := make(map[string][]string)
	errorsByZone := make(map[string][]error)

	// Calculate total number of batches across all zones
	batchSize := 30
	batchesPerZone := (len(tags) + batchSize - 1) / batchSize
	totalBatches := batchesPerZone * len(zoneIDs)

	// Track overall progress
	batchesDone := 0

	// Process each zone
	for zoneIndex, zoneID := range zoneIDs {
		// Create a zone-specific progress callback that updates the overall progress
		zoneProgressCallback := func(batchCompleted, batchTotal, successfulCount int) {
			// Update overall progress
			batchesDone++

			// Call the parent progress callback if provided
			if progressCallback != nil {
				progressCallback(zoneIndex+1, len(zoneIDs), batchesDone, totalBatches, successfulCount)
			}
		}

		// Purge tags for this zone
		successful, errors := PurgeTagsInBatches(client, zoneID, tags, zoneProgressCallback)

		// Store results for this zone
		if len(successful) > 0 {
			successfulByZone[zoneID] = successful
		}

		if len(errors) > 0 {
			errorsByZone[zoneID] = errors
		}
	}

	return successfulByZone, errorsByZone
}
