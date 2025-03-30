package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"cache-kv-purger/internal/api"
)

// DeleteValue deletes a value from a KV namespace
func DeleteValue(client *api.Client, accountID, namespaceID, key string) error {
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

	respBody, err := client.Request(http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
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
		return fmt.Errorf("failed to delete value: %s", errorStr)
	}

	return nil
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

// Note: DeleteMultipleValuesInBatches is now moved to batch.go