package api

import (
	"testing"
)

func TestParseAPIResponse(t *testing.T) {
	// Create test response data
	successResp := `{"success": true, "result": {"id": "123", "name": "test"}}`
	errorResp := `{"success": false, "errors": [{"code": 1003, "message": "Invalid request"}]}`
	malformedResp := `{"success": true, "result": {"id":`

	t.Run("Success response", func(t *testing.T) {
		// Define target struct
		var result struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		// Parse the response
		resp, err := ParseAPIResponse([]byte(successResp), &result)

		// Verify
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !resp.Success {
			t.Errorf("Expected success to be true")
		}

		if result.ID != "123" {
			t.Errorf("Expected ID 123, got: %s", result.ID)
		}

		if result.Name != "test" {
			t.Errorf("Expected Name 'test', got: %s", result.Name)
		}
	})

	t.Run("Error response", func(t *testing.T) {
		// Define target struct
		var result struct {
			ID string `json:"id"`
		}

		// Parse the response
		resp, err := ParseAPIResponse([]byte(errorResp), &result)

		// Verify
		if err == nil {
			t.Errorf("Expected error, got nil")
		}

		if resp == nil {
			t.Errorf("Expected response object, got nil")
		} else if resp.Success {
			t.Errorf("Expected success to be false")
		}

		// Check if the error contains the code and message
		if err != nil && !testContains(err.Error(), "Invalid request") {
			t.Errorf("Error doesn't contain expected message: %v", err)
		}
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		// Define target struct
		var result struct {
			ID string `json:"id"`
		}

		// Parse the response
		resp, err := ParseAPIResponse([]byte(malformedResp), &result)

		// Verify
		if err == nil {
			t.Errorf("Expected error for malformed JSON, got nil")
		}

		if resp != nil {
			t.Errorf("Expected nil response for malformed JSON, got non-nil")
		}

		// Just check if there's an error with "JSON" in the message
		if !testContains(err.Error(), "unexpected end of JSON input") {
			t.Errorf("Expected JSON syntax error message, got: %v", err)
		}
	})
}

func TestParsePaginatedResponse(t *testing.T) {
	// Create test response data with pagination info
	paginatedResp := `{
		"success": true,
		"result": [{"id": "123", "name": "test1"}, {"id": "456", "name": "test2"}],
		"result_info": {
			"page": 1,
			"per_page": 20,
			"total_pages": 2,
			"count": 2,
			"total_count": 30
		}
	}`

	t.Run("Paginated success response", func(t *testing.T) {
		// Define target struct
		var result []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		// Parse the response
		resp, err := ParsePaginatedResponse([]byte(paginatedResp), &result)

		// Verify
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}

		if !resp.Success {
			t.Errorf("Expected success to be true")
		}

		// Check pagination info
		if resp.ResultInfo.Page != 1 {
			t.Errorf("Expected page 1, got: %d", resp.ResultInfo.Page)
		}

		if resp.ResultInfo.TotalPages != 2 {
			t.Errorf("Expected 2 total pages, got: %d", resp.ResultInfo.TotalPages)
		}

		// Check result data
		if len(result) != 2 {
			t.Errorf("Expected 2 items, got: %d", len(result))
		}

		if result[0].ID != "123" {
			t.Errorf("Expected first ID 123, got: %s", result[0].ID)
		}

		if result[1].ID != "456" {
			t.Errorf("Expected second ID 456, got: %s", result[1].ID)
		}
	})
}

func TestFormatAPIError(t *testing.T) {
	t.Run("Single error", func(t *testing.T) {
		resp := &APIResponse{
			Success: false,
			Errors: []Error{
				{Code: 1003, Message: "Invalid request"},
			},
		}

		err := FormatAPIError(resp)
		if err == nil {
			t.Errorf("Expected error, got nil")
		}

		if !contains(err.Error(), "1003") || !contains(err.Error(), "Invalid request") {
			t.Errorf("Error doesn't contain expected details: %v", err)
		}
	})

	t.Run("Multiple errors", func(t *testing.T) {
		resp := &APIResponse{
			Success: false,
			Errors: []Error{
				{Code: 1003, Message: "Invalid request"},
				{Code: 1004, Message: "Authentication failed"},
			},
		}

		err := FormatAPIError(resp)
		if err == nil {
			t.Errorf("Expected error, got nil")
		}

		if !contains(err.Error(), "1003") || !contains(err.Error(), "Invalid request") ||
			!contains(err.Error(), "1004") || !contains(err.Error(), "Authentication failed") {
			t.Errorf("Error doesn't contain expected details: %v", err)
		}
	})

	t.Run("Success response", func(t *testing.T) {
		resp := &APIResponse{
			Success: true,
		}

		err := FormatAPIError(resp)
		if err != nil {
			t.Errorf("Expected nil error for success response, got: %v", err)
		}
	})

	t.Run("Nil response", func(t *testing.T) {
		err := FormatAPIError(nil)
		if err == nil {
			t.Errorf("Expected error for nil response, got nil")
		}

		if !contains(err.Error(), "nil response") {
			t.Errorf("Error doesn't contain expected message: %v", err)
		}
	})
}

// Helper function to check if string contains a substring
func testContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
