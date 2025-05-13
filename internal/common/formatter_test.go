package common

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestToJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expectedErr bool
	}{
		{
			name: "Simple struct",
			input: struct {
				Name  string
				Value int
			}{
				Name:  "test",
				Value: 123,
			},
			expectedErr: false,
		},
		{
			name: "Map string interface",
			input: map[string]interface{}{
				"name":  "test",
				"value": 123,
			},
			expectedErr: false,
		},
		{
			name:        "Simple slice",
			input:       []string{"one", "two", "three"},
			expectedErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function
			result, err := ToJSON(tc.input)

			// Check error status
			if tc.expectedErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectedErr && err != nil {
				t.Errorf("Did not expect error but got: %v", err)
			}

			// Verify the result is valid JSON
			if err == nil {
				var js interface{}
				err = json.Unmarshal(result, &js)
				if err != nil {
					t.Errorf("Result is not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestFormatKeyValueTable(t *testing.T) {
	// Capture stdout to verify output
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Prepare test data
	data := map[string]string{
		"Name":  "Test Name",
		"Value": "Test Value",
		"Empty": "",
	}

	// Call the function
	FormatKeyValueTable(data)

	// Restore stdout
	w.Close()
	os.Stdout = originalStdout

	// Read the captured output
	var output bytes.Buffer
	io.Copy(&output, r)
	outputStr := output.String()

	// Verify the output contains the expected content
	for k, v := range data {
		if !strings.Contains(outputStr, k) {
			t.Errorf("Output should contain key %q, got: %s", k, outputStr)
		}
		if v != "" && !strings.Contains(outputStr, v) {
			t.Errorf("Output should contain value %q, got: %s", v, outputStr)
		}
	}
}

func TestFormatTable(t *testing.T) {
	// Capture stdout to verify output
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Prepare test data
	headers := []string{"ID", "Name", "Status"}
	rows := [][]string{
		{"1", "Item A", "Active"},
		{"2", "Item B", "Inactive"},
	}

	// Call the function
	FormatTable(headers, rows)

	// Restore stdout
	w.Close()
	os.Stdout = originalStdout

	// Read the captured output
	var output bytes.Buffer
	io.Copy(&output, r)
	outputStr := output.String()

	// Verify the output contains all headers and data
	for _, header := range headers {
		if !strings.Contains(outputStr, header) {
			t.Errorf("Output should contain header %q, got: %s", header, outputStr)
		}
	}

	for _, row := range rows {
		for _, cell := range row {
			if !strings.Contains(outputStr, cell) {
				t.Errorf("Output should contain value %q, got: %s", cell, outputStr)
			}
		}
	}
}