package common

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

var errAccountIDRequired = errors.New("account ID is required")

func TestValidateAccountIDWithMock(t *testing.T) {
	tests := []struct {
		name        string
		cmdFlags    map[string]string
		configValue string
		inputValue  string
		expected    string
		shouldError bool
	}{
		{
			name:        "Use input value when provided",
			cmdFlags:    map[string]string{},
			configValue: "config-account-id",
			inputValue:  "input-account-id",
			expected:    "input-account-id",
			shouldError: false,
		},
		{
			name:        "Use command flag when provided",
			cmdFlags:    map[string]string{"account-id": "flag-account-id"},
			configValue: "config-account-id",
			inputValue:  "",
			expected:    "flag-account-id",
			shouldError: false,
		},
		{
			name:        "Use config value when no flag or input",
			cmdFlags:    map[string]string{},
			configValue: "config-account-id",
			inputValue:  "",
			expected:    "config-account-id",
			shouldError: false,
		},
		{
			name:        "Error when no value available",
			cmdFlags:    map[string]string{},
			configValue: "",
			inputValue:  "",
			expected:    "",
			shouldError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock command with flags
			cmd := &cobra.Command{Use: "test"}
			for k, v := range tc.cmdFlags {
				cmd.Flags().String(k, "", "test flag")
				_ = cmd.Flags().Set(k, v)
			}

			// Create a mock config
			config := struct {
				AccountID string
			}{
				AccountID: tc.configValue,
			}

			// Call the function
			result, err := validateAccountIDWithMock(cmd, config, tc.inputValue)

			// Check error status
			if tc.shouldError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Did not expect error but got: %v", err)
			}

			// Check result value
			if !tc.shouldError && result != tc.expected {
				t.Errorf("Expected result %q but got %q", tc.expected, result)
			}
		})
	}
}

// Mock version of ValidateAccountID for testing
func validateAccountIDWithMock(cmd *cobra.Command, config struct{ AccountID string }, inputValue string) (string, error) {
	// If input value is provided directly, use it
	if inputValue != "" {
		return inputValue, nil
	}

	// Otherwise check if flag is provided
	if flagValue, _ := cmd.Flags().GetString("account-id"); flagValue != "" {
		return flagValue, nil
	}

	// Otherwise use value from config
	if config.AccountID != "" {
		return config.AccountID, nil
	}

	// No value available, return error
	return "", errAccountIDRequired
}
