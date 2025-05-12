package api

import (
	"encoding/json"
	"fmt"
)

// ParseAPIResponse parses a generic Cloudflare API response
// - respBody: The raw response bytes from the API
// - resultTarget: Optional pointer to a struct where the "result" field will be unmarshaled
// Returns: The base APIResponse and any error that occurred
func ParseAPIResponse(respBody []byte, resultTarget interface{}) (*APIResponse, error) {
	// First unmarshal into a base APIResponse to check success and errors
	var baseResp APIResponse
	if err := json.Unmarshal(respBody, &baseResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Check if the API reported an error
	if !baseResp.Success {
		errorStr := "API reported failure"
		if len(baseResp.Errors) > 0 {
			errorStr = baseResp.Errors[0].Message
		}
		return &baseResp, fmt.Errorf("API error: %s", errorStr)
	}

	// If the caller wants the result field parsed into a target struct
	if resultTarget != nil {
		// Parse again, but this time into a map to extract the "result" field
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(respBody, &rawMap); err != nil {
			return &baseResp, fmt.Errorf("failed to parse API response for result: %w", err)
		}

		// Check if the result field exists
		resultJson, ok := rawMap["result"]
		if !ok {
			return &baseResp, fmt.Errorf("API response missing 'result' field")
		}

		// Unmarshal the result field into the target
		if err := json.Unmarshal(resultJson, resultTarget); err != nil {
			return &baseResp, fmt.Errorf("failed to parse result data: %w", err)
		}
	}

	return &baseResp, nil
}

// ParsePaginatedResponse parses a paginated Cloudflare API response
// - respBody: The raw response bytes from the API
// - resultTarget: Optional pointer to a struct where the "result" field will be unmarshaled
// Returns: The PaginatedResponse and any error that occurred
func ParsePaginatedResponse(respBody []byte, resultTarget interface{}) (*PaginatedResponse, error) {
	// First unmarshal into a base PaginatedResponse to check success and get pagination info
	var paginatedResp PaginatedResponse
	if err := json.Unmarshal(respBody, &paginatedResp); err != nil {
		return nil, fmt.Errorf("failed to parse paginated API response: %w", err)
	}

	// Check if the API reported an error
	if !paginatedResp.Success {
		errorStr := "API reported failure"
		if len(paginatedResp.Errors) > 0 {
			errorStr = paginatedResp.Errors[0].Message
		}
		return &paginatedResp, fmt.Errorf("API error: %s", errorStr)
	}

	// If the caller wants the result field parsed into a target struct
	if resultTarget != nil {
		// Parse again, but this time into a map to extract the "result" field
		var rawMap map[string]json.RawMessage
		if err := json.Unmarshal(respBody, &rawMap); err != nil {
			return &paginatedResp, fmt.Errorf("failed to parse API response for result: %w", err)
		}

		// Check if the result field exists
		resultJson, ok := rawMap["result"]
		if !ok {
			return &paginatedResp, fmt.Errorf("API response missing 'result' field")
		}

		// Unmarshal the result field into the target
		if err := json.Unmarshal(resultJson, resultTarget); err != nil {
			return &paginatedResp, fmt.Errorf("failed to parse result data: %w", err)
		}
	}

	return &paginatedResp, nil
}

// FormatAPIError creates a formatted error message from an API response
func FormatAPIError(resp *APIResponse) error {
	if resp == nil {
		return fmt.Errorf("unknown API error (nil response)")
	}

	if resp.Success {
		return nil
	}

	if len(resp.Errors) > 0 {
		// If we have multiple errors, include all of them
		if len(resp.Errors) > 1 {
			errMsgs := make([]string, 0, len(resp.Errors))
			for _, err := range resp.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("[%d] %s", err.Code, err.Message))
			}
			return fmt.Errorf("multiple API errors: %v", errMsgs)
		}
		
		// Single error case
		err := resp.Errors[0]
		return fmt.Errorf("API error [%d]: %s", err.Code, err.Message)
	}

	// Success is false but no errors provided
	return fmt.Errorf("API reported failure without error details")
}