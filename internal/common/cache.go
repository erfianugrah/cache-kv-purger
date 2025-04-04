package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateFilePath checks if a file exists and returns its absolute path
func ValidateFilePath(path string) (string, error) {
	// Check if path is empty
	if path == "" {
		return "", fmt.Errorf("file path is required")
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	return absPath, nil
}

// ValidateURLs validates a list of URLs
func ValidateURLs(urls []string) ([]string, []error) {
	var validURLs []string
	var errors []error

	for _, url := range urls {
		// Ensure URL has protocol
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}

		// Additional validation could be done here
		validURLs = append(validURLs, url)
	}

	return validURLs, errors
}

// ExtractHostFromURL extracts the hostname from a URL
func ExtractHostFromURL(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	// Extract hostname (everything before the first slash)
	hostname := url
	if idx := strings.Index(url, "/"); idx > 0 {
		hostname = url[:idx]
	}

	return hostname
}

// GroupURLsByHost groups URLs by hostname
func GroupURLsByHost(urls []string) map[string][]string {
	result := make(map[string][]string)

	for _, url := range urls {
		hostname := ExtractHostFromURL(url)
		result[hostname] = append(result[hostname], url)
	}

	return result
}
