package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"cache-kv-purger/internal/api"
)

// WriteValue writes a value to a KV namespace
func WriteValue(client *api.Client, accountID, namespaceID, key, value string, options *WriteOptions) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return fmt.Errorf("namespace ID is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	// URL encode the key
	encodedKey := url.PathEscape(key)
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/values/%s", accountID, namespaceID, encodedKey)

	var query url.Values
	if options != nil {
		query = url.Values{}

		if options.Expiration > 0 {
			query.Set("expiration", fmt.Sprintf("%d", options.Expiration))
		}

		if options.ExpirationTTL > 0 {
			query.Set("expiration_ttl", fmt.Sprintf("%d", options.ExpirationTTL))
		}

		if options.Metadata != nil {
			metadataJSON, err := json.Marshal(options.Metadata)
			if err != nil {
				return fmt.Errorf("failed to encode metadata: %w", err)
			}
			query.Set("metadata", string(metadataJSON))
		}
	}

	respBody, err := client.Request(http.MethodPut, path, query, []byte(value))
	if err != nil {
		return err
	}

	var apiResp api.APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		errorStr := "API reported failure"
		if len(apiResp.Errors) > 0 {
			errorStr = apiResp.Errors[0].Message
		}
		return fmt.Errorf("failed to write value: %s", errorStr)
	}

	return nil
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
			errorMsgs)
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
