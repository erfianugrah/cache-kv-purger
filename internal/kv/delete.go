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

	fmt.Printf("[DEBUG] DeleteMultipleValues called with %d keys\n", len(keys))

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/bulk/delete", accountID, namespaceID)

	// API expects an array of strings, not objects with 'name' property
	fmt.Printf("[VERBOSE] Sending bulk delete request to %s with %d keys\n", path, len(keys))

	// Send the keys directly as an array of strings
	respBody, err := client.Request(http.MethodPost, path, nil, keys)
	if err != nil {
		fmt.Printf("[ERROR] Bulk delete request failed: %v\n", err)

		// Fall back to individual deletions if bulk delete fails
		fmt.Printf("[VERBOSE] Falling back to individual deletions for %d keys\n", len(keys))
		fallbackErrors := 0
		for i, key := range keys {
			if i%100 == 0 {
				fmt.Printf("[DEBUG] Performing individual deletion %d/%d\n", i+1, len(keys))
			}
			if deleteErr := DeleteValue(client, accountID, namespaceID, key); deleteErr != nil {
				fallbackErrors++
				fmt.Printf("[ERROR] Individual deletion failed for key %s: %v\n", key, deleteErr)
			}
		}

		// If all individual deletes failed too, return the original error
		if fallbackErrors == len(keys) {
			fmt.Printf("[ERROR] All %d individual deletions failed\n", len(keys))
			return err
		}

		// Otherwise we succeeded with individual deletes
		fmt.Printf("[INFO] Completed with %d/%d successful individual deletions\n", len(keys)-fallbackErrors, len(keys))
		return nil
	}

	var resp api.APIResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		fmt.Printf("[ERROR] Failed to parse API response: %v\n", err)
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	fmt.Printf("[DEBUG] API response: success=%v, errors=%v\n", resp.Success, len(resp.Errors))

	if !resp.Success {
		errorStr := "API reported failure"
		if len(resp.Errors) > 0 {
			errorStr = resp.Errors[0].Message
			fmt.Printf("[ERROR] API reported error: %s\n", errorStr)
		}

		// Try individual deletions as fallback
		fmt.Printf("[VERBOSE] API reported failure, falling back to individual deletions for %d keys\n", len(keys))
		fallbackErrors := 0
		for i, key := range keys {
			if i%100 == 0 {
				fmt.Printf("[DEBUG] Performing individual deletion %d/%d\n", i+1, len(keys))
			}
			if deleteErr := DeleteValue(client, accountID, namespaceID, key); deleteErr != nil {
				fallbackErrors++
				fmt.Printf("[ERROR] Individual deletion failed for key %s: %v\n", key, deleteErr)
			}
		}

		// If all individual deletes failed too, return the original error
		if fallbackErrors == len(keys) {
			fmt.Printf("[ERROR] All %d individual deletions failed\n", len(keys))
			return fmt.Errorf("failed to delete multiple values: %s", errorStr)
		}

		// Otherwise we succeeded with individual deletes
		fmt.Printf("[INFO] Completed with %d/%d successful individual deletions\n", len(keys)-fallbackErrors, len(keys))
		return nil
	}

	fmt.Printf("[INFO] Bulk delete of %d keys completed successfully\n", len(keys))
	return nil
}

// Note: DeleteMultipleValuesInBatches is now moved to batch.go
