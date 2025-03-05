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