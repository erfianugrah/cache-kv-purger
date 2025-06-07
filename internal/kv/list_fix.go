package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// ListKeysWithPagination is an enhanced version of ListKeysWithOptions that provides
// better pagination handling and debugging capabilities
func ListKeysWithPagination(client *api.Client, accountID, namespaceID string, options *ListKeysOptions,
	pagOptions *common.PaginationOptions) (*common.PaginationResult, []KeyValuePair, error) {

	if accountID == "" {
		return nil, nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, nil, fmt.Errorf("namespace ID is required")
	}

	// Initialize pagination options if not provided
	if pagOptions == nil {
		pagOptions = &common.PaginationOptions{
			MaxRetries: 3,
			Timeout:    30 * time.Second,
			LogPrefix:  "Keys",
		}
	}

	// Use default options if not provided
	if options == nil {
		options = &ListKeysOptions{
			Limit: 1000, // Maximum limit per request
		}
	} else if options.Limit == 0 {
		options.Limit = 1000 // Ensure a reasonable default
	}

	// Cloudflare API requires a minimum limit of 10
	if options.Limit < 10 {
		options.Limit = 10
	}

	// Create a copy of options to use throughout the pagination
	requestOptions := *options

	// Create a key listing handler to use with the pagination utility
	handler := &keyListingHandler{
		client:      client,
		accountID:   accountID,
		namespaceID: namespaceID,
		options:     &requestOptions,
		allKeys:     []KeyValuePair{},
	}

	// Execute pagination
	result, err := common.ExecutePagination(handler, pagOptions)
	if err != nil {
		return result, nil, err
	}

	// Update the item count to reflect the actual number of keys
	result.ItemCount = len(handler.allKeys)

	return result, handler.allKeys, nil
}

// keyListingHandler implements the PaginationHandler interface for key listing
type keyListingHandler struct {
	client      *api.Client
	accountID   string
	namespaceID string
	options     *ListKeysOptions
	allKeys     []KeyValuePair
}

// FetchPage fetches a single page of keys
func (h *keyListingHandler) FetchPage(cursor string) (interface{}, string, bool, error) {
	// Update the cursor in our options
	h.options.Cursor = cursor

	// Call the API to fetch a page of keys
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/keys", h.accountID, h.namespaceID)

	// Add query parameters
	queryParams := url.Values{}
	if h.options.Limit > 0 {
		// Cloudflare API requires a minimum limit of 10
		if h.options.Limit < 10 {
			h.options.Limit = 10
		}
		queryParams.Set("limit", fmt.Sprintf("%d", h.options.Limit))
	}
	if h.options.Cursor != "" {
		queryParams.Set("cursor", h.options.Cursor)
	}
	if h.options.Prefix != "" {
		queryParams.Set("prefix", h.options.Prefix)
	}

	// Make the API request
	respBody, err := h.client.Request(http.MethodGet, path, queryParams, nil)
	if err != nil {
		return nil, "", false, err
	}

	// Parse the response
	var keysResp KeyValuesResponse
	if err := json.Unmarshal(respBody, &keysResp); err != nil {
		return nil, "", false, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !keysResp.Success {
		errorStr := "API reported failure"
		if len(keysResp.Errors) > 0 {
			errorStr = keysResp.Errors[0].Message
		}
		return nil, "", false, fmt.Errorf("failed to list keys: %s", errorStr)
	}

	// Get cursor and completion status
	nextCursor := keysResp.ResultInfo.Cursor
	isComplete := nextCursor == ""

	// Return the results
	return keysResp.Result, nextCursor, isComplete, nil
}

// ProcessItems processes the keys returned by FetchPage
func (h *keyListingHandler) ProcessItems(items interface{}) error {
	keys, ok := items.([]KeyValuePair)
	if !ok {
		return fmt.Errorf("unexpected item type in key listing")
	}

	// Append the keys to our collection
	h.allKeys = append(h.allKeys, keys...)
	return nil
}

// EnhancedListAllKeys lists all keys in a KV namespace with improved pagination
// It replaces the old ListAllKeysWithOptions function with more reliable pagination
func EnhancedListAllKeys(client *api.Client, accountID, namespaceID string,
	options *ListKeysOptions, pagOptions *common.PaginationOptions, progressCallback func(fetched, total int)) ([]KeyValuePair, error) {

	// Configure pagination options
	if pagOptions == nil {
		pagOptions = &common.PaginationOptions{
			MaxRetries: 3,
			Timeout:    120 * time.Second, // Longer timeout for potentially large key sets
			LogPrefix:  "Keys",
		}
	}

	// Execute the paginated request
	result, keys, err := ListKeysWithPagination(client, accountID, namespaceID, options, pagOptions)
	if err != nil {
		return nil, err
	}

	// Call the progress callback if provided
	if progressCallback != nil {
		progressCallback(len(keys), -1) // -1 means total unknown
	}

	// Log any warnings if verbose
	if pagOptions.Verbose && len(result.Warnings) > 0 {
		fmt.Println("\nWarnings during key listing:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	return keys, nil
}
