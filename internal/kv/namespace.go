package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"cache-kv-purger/internal/api"
)

// Namespace represents a KV namespace
type Namespace struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	SupportURL string `json:"support_url,omitempty"`
}

// NamespaceResponse represents a response containing namespace information
type NamespaceResponse struct {
	Success  bool        `json:"success"`
	Errors   []api.Error `json:"errors,omitempty"`
	Messages []string    `json:"messages,omitempty"`
	Result   Namespace   `json:"result"`
}

// NamespacesResponse represents a response containing multiple namespaces
type NamespacesResponse struct {
	Success    bool        `json:"success"`
	Errors     []api.Error `json:"errors,omitempty"`
	Messages   []string    `json:"messages,omitempty"`
	ResultInfo struct {
		Cursor string `json:"cursor"`
		Count  int    `json:"count"`
	} `json:"result_info"`
	Result []Namespace `json:"result"`
}

// ListNamespaces lists all KV namespaces for an account with automatic pagination
func ListNamespaces(client *api.Client, accountID string) ([]Namespace, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces", accountID)

	var allNamespaces []Namespace
	var cursor string

	for {
		// Set up query parameters for pagination if we have a cursor
		var queryParams url.Values
		if cursor != "" {
			queryParams = url.Values{}
			queryParams.Set("cursor", cursor)
		}

		respBody, err := client.Request(http.MethodGet, path, queryParams, nil)
		if err != nil {
			return nil, err
		}

		var nsResp NamespacesResponse
		if err := json.Unmarshal(respBody, &nsResp); err != nil {
			return nil, fmt.Errorf("failed to parse API response: %w", err)
		}

		if !nsResp.Success {
			errorStr := "API reported failure"
			if len(nsResp.Errors) > 0 {
				errorStr = nsResp.Errors[0].Message
			}
			return nil, fmt.Errorf("failed to list namespaces: %s", errorStr)
		}

		// Append results from this page
		allNamespaces = append(allNamespaces, nsResp.Result...)

		// Check if we need to fetch more pages
		cursor = nsResp.ResultInfo.Cursor
		if cursor == "" {
			break
		}
	}

	return allNamespaces, nil
}

// GetNamespace gets details of a specific namespace
func GetNamespace(client *api.Client, accountID, namespaceID string) (*Namespace, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s", accountID, namespaceID)

	respBody, err := client.Request(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var nsResp NamespaceResponse
	if err := json.Unmarshal(respBody, &nsResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !nsResp.Success {
		errorStr := "API reported failure"
		if len(nsResp.Errors) > 0 {
			errorStr = nsResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to get namespace: %s", errorStr)
	}

	return &nsResp.Result, nil
}

// CreateNamespace creates a new KV namespace
func CreateNamespace(client *api.Client, accountID, title string) (*Namespace, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces", accountID)

	requestBody := map[string]string{
		"title": title,
	}

	respBody, err := client.Request(http.MethodPost, path, nil, requestBody)
	if err != nil {
		return nil, err
	}

	var nsResp NamespaceResponse
	if err := json.Unmarshal(respBody, &nsResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !nsResp.Success {
		errorStr := "API reported failure"
		if len(nsResp.Errors) > 0 {
			errorStr = nsResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to create namespace: %s", errorStr)
	}

	return &nsResp.Result, nil
}

// DeleteNamespace deletes a KV namespace
func DeleteNamespace(client *api.Client, accountID, namespaceID string) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return fmt.Errorf("namespace ID is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s", accountID, namespaceID)

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
		return fmt.Errorf("failed to delete namespace: %s", errorStr)
	}

	return nil
}

// FindNamespaceByTitle finds a namespace by its title
func FindNamespaceByTitle(client *api.Client, accountID, title string) (*Namespace, error) {
	namespaces, err := ListNamespaces(client, accountID)
	if err != nil {
		return nil, err
	}

	for _, ns := range namespaces {
		if ns.Title == title {
			return &ns, nil
		}
	}

	return nil, fmt.Errorf("namespace with title '%s' not found", title)
}

// DeleteMultipleNamespaces deletes multiple KV namespaces
func DeleteMultipleNamespaces(client *api.Client, accountID string, namespaceIDs []string) ([]string, []error) {
	if accountID == "" {
		return nil, []error{fmt.Errorf("account ID is required")}
	}
	if len(namespaceIDs) == 0 {
		return nil, []error{fmt.Errorf("at least one namespace ID is required")}
	}

	var successIDs []string
	var errors []error

	// Process each namespace deletion separately since the API doesn't support bulk namespace deletion
	for _, nsID := range namespaceIDs {
		if nsID == "" {
			errors = append(errors, fmt.Errorf("empty namespace ID provided"))
			continue
		}

		err := DeleteNamespace(client, accountID, nsID)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to delete namespace %s: %w", nsID, err))
		} else {
			successIDs = append(successIDs, nsID)
		}
	}

	return successIDs, errors
}

// DeleteMultipleNamespacesWithProgress deletes multiple KV namespaces with progress callback
func DeleteMultipleNamespacesWithProgress(client *api.Client, accountID string, namespaceIDs []string, progressCallback func(completed, total, success, failed int)) ([]string, []error) {
	if accountID == "" {
		return nil, []error{fmt.Errorf("account ID is required")}
	}
	if len(namespaceIDs) == 0 {
		return nil, []error{fmt.Errorf("at least one namespace ID is required")}
	}

	var successIDs []string
	var errors []error
	totalCount := len(namespaceIDs)

	// Process each namespace deletion separately
	for i, nsID := range namespaceIDs {
		if nsID == "" {
			errors = append(errors, fmt.Errorf("empty namespace ID provided"))

			if progressCallback != nil {
				progressCallback(i+1, totalCount, len(successIDs), len(errors))
			}
			continue
		}

		err := DeleteNamespace(client, accountID, nsID)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to delete namespace %s: %w", nsID, err))
		} else {
			successIDs = append(successIDs, nsID)
		}

		if progressCallback != nil {
			progressCallback(i+1, totalCount, len(successIDs), len(errors))
		}
	}

	return successIDs, errors
}

// FindNamespacesByPattern finds namespaces with titles matching a regex pattern
func FindNamespacesByPattern(client *api.Client, accountID string, pattern string) ([]Namespace, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if pattern == "" {
		return nil, fmt.Errorf("pattern is required")
	}

	// Get all namespaces first
	namespaces, err := ListNamespaces(client, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	// If no pattern, return all namespaces
	if pattern == "*" {
		return namespaces, nil
	}

	// Compile regex
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Filter namespaces based on the pattern
	var matchingNamespaces []Namespace
	for _, ns := range namespaces {
		if regex.MatchString(ns.Title) {
			matchingNamespaces = append(matchingNamespaces, ns)
		}
	}

	return matchingNamespaces, nil
}

// RenameNamespace renames a KV namespace
func RenameNamespace(client *api.Client, accountID, namespaceID, newTitle string) (*Namespace, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, fmt.Errorf("namespace ID is required")
	}
	if newTitle == "" {
		return nil, fmt.Errorf("new title is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s", accountID, namespaceID)

	requestBody := map[string]string{
		"title": newTitle,
	}

	respBody, err := client.Request(http.MethodPut, path, nil, requestBody)
	if err != nil {
		return nil, err
	}

	var nsResp NamespaceResponse
	if err := json.Unmarshal(respBody, &nsResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !nsResp.Success {
		errorStr := "API reported failure"
		if len(nsResp.Errors) > 0 {
			errorStr = nsResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to rename namespace: %s", errorStr)
	}

	return &nsResp.Result, nil
}
