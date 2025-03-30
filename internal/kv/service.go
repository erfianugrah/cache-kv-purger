package kv

import (
	"context"
	"fmt"
	"regexp"

	"cache-kv-purger/internal/api"
)

// KVService provides a unified interface for KV operations
type KVService interface {
	// Namespace operations
	ListNamespaces(ctx context.Context, accountID string) ([]Namespace, error)
	CreateNamespace(ctx context.Context, accountID, title string) (*Namespace, error)
	RenameNamespace(ctx context.Context, accountID, namespaceID, newTitle string) (*Namespace, error)
	DeleteNamespace(ctx context.Context, accountID, namespaceID string) error
	FindNamespaceByTitle(ctx context.Context, accountID, title string) (*Namespace, error)
	FindNamespacesByPattern(ctx context.Context, accountID, pattern string) ([]Namespace, error)

	// Namespace resolution
	ResolveNamespaceID(ctx context.Context, accountID, nameOrID string) (string, error)

	// Basic operations
	List(ctx context.Context, accountID, namespaceID string, options ListOptions) (*ListKeysResult, error)
	Get(ctx context.Context, accountID, namespaceID, key string, options ServiceGetOptions) (*KeyValuePair, error)
	Put(ctx context.Context, accountID, namespaceID, key, value string, options WriteOptions) error
	Delete(ctx context.Context, accountID, namespaceID, key string) error
	Exists(ctx context.Context, accountID, namespaceID, key string) (bool, error)

	// Bulk operations
	BulkGet(ctx context.Context, accountID, namespaceID string, keys []string, options BulkGetOptions) ([]KeyValuePair, error)
	BulkPut(ctx context.Context, accountID, namespaceID string, items []BulkWriteItem, options BulkWriteOptions) (int, error)
	BulkDelete(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error)

	// Search operations
	Search(ctx context.Context, accountID, namespaceID string, options SearchOptions) ([]KeyValuePair, error)
}

// ListOptions represents options for listing keys
type ListOptions struct {
	Limit          int
	Cursor         string
	Prefix         string
	Pattern        string
	IncludeValues  bool
	IncludeMetadata bool
}

// ServiceGetOptions represents options for reading a value (service-specific type)
type ServiceGetOptions struct {
	IncludeMetadata bool
}

// BulkGetOptions represents options for bulk reading values
type BulkGetOptions struct {
	IncludeMetadata bool
	BatchSize       int
	Concurrency     int
	Prefix          string
	Pattern         string
}

// BulkWriteOptions represents options for bulk write operations
type BulkWriteOptions struct {
	BatchSize   int
	Concurrency int
}

// BulkDeleteOptions represents options for bulk delete operations
type BulkDeleteOptions struct {
	BatchSize   int
	Concurrency int
	DryRun      bool
	Force       bool
	Prefix      string
	Pattern     string
	TagField    string
	TagValue    string
	SearchValue string
}

// SearchOptions represents options for searching keys
type SearchOptions struct {
	TagField        string
	TagValue        string
	SearchValue     string
	IncludeMetadata bool
	BatchSize       int
	Concurrency     int
}

// CloudflareKVService implements the KVService interface using Cloudflare API
type CloudflareKVService struct {
	client *api.Client
}

// NewKVService creates a new KV service
func NewKVService(client *api.Client) KVService {
	return &CloudflareKVService{
		client: client,
	}
}

// ListNamespaces lists all KV namespaces for an account
func (s *CloudflareKVService) ListNamespaces(ctx context.Context, accountID string) ([]Namespace, error) {
	// Call existing function
	return ListNamespaces(s.client, accountID)
}

// CreateNamespace creates a new KV namespace
func (s *CloudflareKVService) CreateNamespace(ctx context.Context, accountID, title string) (*Namespace, error) {
	// Call existing function
	return CreateNamespace(s.client, accountID, title)
}

// RenameNamespace renames a KV namespace
func (s *CloudflareKVService) RenameNamespace(ctx context.Context, accountID, namespaceID, newTitle string) (*Namespace, error) {
	// Call existing function
	return RenameNamespace(s.client, accountID, namespaceID, newTitle)
}

// DeleteNamespace deletes a KV namespace
func (s *CloudflareKVService) DeleteNamespace(ctx context.Context, accountID, namespaceID string) error {
	// Call existing function
	return DeleteNamespace(s.client, accountID, namespaceID)
}

// FindNamespaceByTitle finds a namespace by its title
func (s *CloudflareKVService) FindNamespaceByTitle(ctx context.Context, accountID, title string) (*Namespace, error) {
	// Call existing function
	return FindNamespaceByTitle(s.client, accountID, title)
}

// FindNamespacesByPattern finds namespaces with titles matching a regex pattern
func (s *CloudflareKVService) FindNamespacesByPattern(ctx context.Context, accountID, pattern string) ([]Namespace, error) {
	// Call existing function
	return FindNamespacesByPattern(s.client, accountID, pattern)
}

// ResolveNamespaceID resolves a namespace name or ID to its ID
func (s *CloudflareKVService) ResolveNamespaceID(ctx context.Context, accountID, nameOrID string) (string, error) {
	// Check if it's already an ID (probably a UUID format)
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{32}$|^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if uuidPattern.MatchString(nameOrID) {
		return nameOrID, nil
	}

	// Try to find the namespace by title
	namespace, err := s.FindNamespaceByTitle(ctx, accountID, nameOrID)
	if err != nil {
		return "", fmt.Errorf("namespace '%s' not found: %w", nameOrID, err)
	}

	return namespace.ID, nil
}

// List lists keys in a KV namespace
func (s *CloudflareKVService) List(ctx context.Context, accountID, namespaceID string, options ListOptions) (*ListKeysResult, error) {
	// Convert options to the format expected by the existing function
	listOptions := &ListKeysOptions{
		Limit:  options.Limit,
		Cursor: options.Cursor,
		Prefix: options.Prefix,
	}

	return ListKeysWithOptions(s.client, accountID, namespaceID, listOptions)
}

// Get gets a value for a key
func (s *CloudflareKVService) Get(ctx context.Context, accountID, namespaceID, key string, options ServiceGetOptions) (*KeyValuePair, error) {
	if options.IncludeMetadata {
		return GetKeyWithMetadata(s.client, accountID, namespaceID, key)
	}

	// Just get the value without metadata
	value, err := GetValue(s.client, accountID, namespaceID, key)
	if err != nil {
		return nil, err
	}

	return &KeyValuePair{
		Key:   key,
		Value: value,
	}, nil
}

// Put puts a value for a key
func (s *CloudflareKVService) Put(ctx context.Context, accountID, namespaceID, key, value string, options WriteOptions) error {
	return WriteValue(s.client, accountID, namespaceID, key, value, &options)
}

// Delete deletes a value for a key
func (s *CloudflareKVService) Delete(ctx context.Context, accountID, namespaceID, key string) error {
	return DeleteValue(s.client, accountID, namespaceID, key)
}

// Exists checks if a key exists
func (s *CloudflareKVService) Exists(ctx context.Context, accountID, namespaceID, key string) (bool, error) {
	return KeyExists(s.client, accountID, namespaceID, key)
}

// BulkGet gets multiple values in bulk
func (s *CloudflareKVService) BulkGet(ctx context.Context, accountID, namespaceID string, keys []string, options BulkGetOptions) ([]KeyValuePair, error) {
	// This is a more complex operation that needs to be implemented
	// For now, we'll just get each key individually
	result := make([]KeyValuePair, 0, len(keys))
	
	for _, key := range keys {
		kv, err := s.Get(ctx, accountID, namespaceID, key, ServiceGetOptions{
			IncludeMetadata: options.IncludeMetadata,
		})
		if err != nil {
			// Skip keys that don't exist or have errors
			continue
		}
		result = append(result, *kv)
	}
	
	return result, nil
}

// BulkPut puts multiple values in bulk
func (s *CloudflareKVService) BulkPut(ctx context.Context, accountID, namespaceID string, items []BulkWriteItem, options BulkWriteOptions) (int, error) {
	if options.Concurrency > 0 {
		return WriteMultipleValuesConcurrently(s.client, accountID, namespaceID, items, options.BatchSize, options.Concurrency, nil)
	}
	
	return WriteMultipleValuesInBatches(s.client, accountID, namespaceID, items, options.BatchSize, nil)
}

// BulkDelete deletes multiple values in bulk
func (s *CloudflareKVService) BulkDelete(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
	if options.DryRun {
		// Just return the count without deleting
		return len(keys), nil
	}
	
	// If we have tag-based filtering or search, use the appropriate functions
	if options.TagField != "" || options.SearchValue != "" {
		return s.bulkDeleteWithAdvancedFiltering(ctx, accountID, namespaceID, keys, options)
	}
	
	// Otherwise do a simple batch delete
	err := DeleteMultipleValuesInBatches(s.client, accountID, namespaceID, keys, options.BatchSize, nil)
	if err != nil {
		return 0, err
	}
	
	return len(keys), nil
}

// bulkDeleteWithAdvancedFiltering handles complex delete operations with filtering
func (s *CloudflareKVService) bulkDeleteWithAdvancedFiltering(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
	// This will use the appropriate purge function based on the options
	if options.SearchValue != "" {
		// Use smart purge by value
		return SmartPurgeByValue(s.client, accountID, namespaceID, options.SearchValue, 
			options.BatchSize, options.Concurrency, options.DryRun, nil)
	} else if options.TagField != "" {
		// Use tag-based purge
		return PurgeByMetadataOnly(s.client, accountID, namespaceID, options.TagField, options.TagValue,
			options.BatchSize, options.Concurrency, options.DryRun, nil)
	}
	
	// Shouldn't reach here but just in case
	return 0, fmt.Errorf("invalid advanced filtering options")
}

// Search searches for keys with specific criteria
func (s *CloudflareKVService) Search(ctx context.Context, accountID, namespaceID string, options SearchOptions) ([]KeyValuePair, error) {
	if options.SearchValue != "" {
		// Use smart search
		return SmartFindKeysWithValue(s.client, accountID, namespaceID, options.SearchValue,
			options.BatchSize, options.Concurrency, nil)
	} else if options.TagField != "" {
		// Use tag-based search
		return StreamingFilterKeysByMetadata(s.client, accountID, namespaceID, options.TagField, 
			options.TagValue, options.BatchSize, options.Concurrency, nil)
	}
	
	return nil, fmt.Errorf("search requires either SearchValue or TagField to be specified")
}