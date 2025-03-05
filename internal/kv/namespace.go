package kv

import (
	"encoding/json"
	"fmt"
	"net/http"

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
	api.APIResponse
	Result Namespace `json:"result"`
}

// NamespacesResponse represents a response containing multiple namespaces
type NamespacesResponse struct {
	api.APIResponse
	Result []Namespace `json:"result"`
}

// ListNamespaces lists all KV namespaces for an account
func ListNamespaces(client *api.Client, accountID string) ([]Namespace, error) {
	if accountID == "" {
		return nil, fmt.Errorf("account ID is required")
	}

	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces", accountID)
	
	respBody, err := client.Request(http.MethodGet, path, nil, nil)
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

	return nsResp.Result, nil
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