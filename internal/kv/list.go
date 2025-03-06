package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

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

// ListAllKeys lists all keys in a KV namespace, handling pagination automatically
func ListAllKeys(client *api.Client, accountID, namespaceID string, progressCallback func(fetched, total int)) ([]KeyValuePair, error) {
	var allKeys []KeyValuePair
	options := &ListKeysOptions{
		Limit: 1000, // Maximum limit per request
	}

	totalFetched := 0

	for {
		result, err := ListKeysWithOptions(client, accountID, namespaceID, options)
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
		options.Cursor = result.Cursor
	}

	return allKeys, nil
}

// WriteMultipleValues writes multiple values to a KV namespace
func WriteMultipleValues(client *api.Client, accountID, namespaceID string, items []BulkWriteItem) error {
	result, err := WriteMultipleValuesWithResult(client, accountID, namespaceID, items)
	if err != nil {
		return err
	}

	// If there were any errors in individual items, report them
	if result.Result.ErrorCount > 0 {
		errorMsgs := make([]string, len(result.Result.Errors))
		for i, err := range result.Result.Errors {
			errorMsgs[i] = fmt.Sprintf("key '%s': %s", err.Key, err.Error)
		}
		return fmt.Errorf("bulk write partially failed (%d/%d succeeded): %s",
			result.Result.SuccessCount,
			result.Result.SuccessCount+result.Result.ErrorCount,
			strings.Join(errorMsgs, "; "))
	}

	return nil
}

// WriteMultipleValuesWithResult writes multiple values to a KV namespace and returns detailed results
func WriteMultipleValuesWithResult(client *api.Client, accountID, namespaceID string, items []BulkWriteItem) (*BulkWriteResult, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}
	if len(items) > 10000 {
		return nil, fmt.Errorf("maximum of 10000 items can be written in a single bulk operation")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk", accountID, namespaceID)

	respBody, err := client.Request(http.MethodPut, path, nil, items)
	if err != nil {
		return nil, err
	}

	var resp BulkWriteResult
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !resp.Success {
		errorStr := "API reported failure"
		if len(resp.Errors) > 0 {
			errorStr = resp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to write multiple values: %s", errorStr)
	}

	return &resp, nil
}

// DeleteMultipleValues deletes multiple values from a KV namespace
func DeleteMultipleValues(client *api.Client, accountID, namespaceID string, keys []string) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return fmt.Errorf("namespace ID is required")
	}
	if len(keys) == 0 {
		return fmt.Errorf("at least one key is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk", accountID, namespaceID)

	// API expects an array of objects with 'name' property
	keyObjects := make([]KeyName, len(keys))
	for i, key := range keys {
		keyObjects[i] = KeyName{Name: key}
	}

	respBody, err := client.Request(http.MethodDelete, path, nil, keyObjects)
	if err != nil {
		// Fall back to individual deletions if bulk delete fails
		fallbackErrors := 0
		for _, key := range keys {
			if deleteErr := DeleteValue(client, accountID, namespaceID, key); deleteErr != nil {
				fallbackErrors++
			}
		}

		// If all individual deletes failed too, return the original error
		if fallbackErrors == len(keys) {
			return err
		}

		// Otherwise we succeeded with individual deletes
		return nil
	}

	var resp api.APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !resp.Success {
		errorStr := "API reported failure"
		if len(resp.Errors) > 0 {
			errorStr = resp.Errors[0].Message
		}

		// Try individual deletions as fallback
		fallbackErrors := 0
		for _, key := range keys {
			if deleteErr := DeleteValue(client, accountID, namespaceID, key); deleteErr != nil {
				fallbackErrors++
			}
		}

		// If all individual deletes failed too, return the original error
		if fallbackErrors == len(keys) {
			return fmt.Errorf("failed to delete multiple values: %s", errorStr)
		}

		// Otherwise we succeeded with individual deletes
		return nil
	}

	return nil
}
