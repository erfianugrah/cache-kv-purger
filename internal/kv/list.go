package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"cache-kv-purger/internal/api"
)

// ListKeys lists all keys in a KV namespace
func ListKeys(client *api.Client, accountID, namespaceID string) ([]KeyValuePair, error) {
	result, err := ListKeysWithOptions(client, accountID, namespaceID, nil)
	if err != nil {
		return nil, err
	}
	return result.Keys, nil
}

// ListKeysWithOptions lists keys in a KV namespace with advanced options and pagination
func ListKeysWithOptions(client *api.Client, accountID, namespaceID string, options *ListKeysOptions) (*ListKeysResult, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/keys", accountID, namespaceID)

	// Add query parameters if options are provided
	var queryParams url.Values
	if options != nil {
		queryParams = url.Values{}

		if options.Limit > 0 {
			// Cloudflare API requires a minimum limit of 10
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
	}

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

	// Prepare result
	result := &ListKeysResult{
		Keys:    keysResp.Result,
		Cursor:  keysResp.ResultInfo.Cursor,
		HasMore: keysResp.ResultInfo.Cursor != "",
	}

	return result, nil
}

// ListAllKeysWithOptions lists all keys in a KV namespace, handling pagination automatically with custom options
func ListAllKeysWithOptions(client *api.Client, accountID, namespaceID string, options *ListKeysOptions, progressCallback func(fetched, total int)) ([]KeyValuePair, error) {
	var allKeys []KeyValuePair

	// Create options if not provided
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

	// Use a copy of options so we don't modify the original
	requestOptions := *options

	totalFetched := 0

	for {
		result, err := ListKeysWithOptions(client, accountID, namespaceID, &requestOptions)
		if err != nil {
			return nil, err
		}

		allKeys = append(allKeys, result.Keys...)
		totalFetched += len(result.Keys)

		if progressCallback != nil {
			progressCallback(totalFetched, -1) // -1 means total unknown
		}

		if !result.HasMore {
			break
		}

		// Update cursor for next request
		requestOptions.Cursor = result.Cursor
	}

	return allKeys, nil
}

// ListAllKeys lists all keys in a KV namespace, handling pagination automatically (legacy function)
func ListAllKeys(client *api.Client, accountID, namespaceID string, progressCallback func(fetched, total int)) ([]KeyValuePair, error) {
	return ListAllKeysWithOptions(client, accountID, namespaceID, nil, progressCallback)
}
