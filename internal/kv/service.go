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
	Limit           int
	Cursor          string
	Prefix          string
	Pattern         string
	IncludeValues   bool
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
	BatchSize       int
	Concurrency     int
	DryRun          bool
	Force           bool
	Verbose         bool // Enable verbose output
	Debug           bool // Enable debug output (more detailed than verbose)
	Prefix          string
	PrefixSpecified bool // Whether prefix was explicitly specified, even if empty
	AllKeys         bool // Whether to delete all keys in the namespace
	Pattern         string
	TagField        string
	TagValue        string
	SearchValue     string
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
	// Define debug functions that respect verbosity flags
	verbose := func(format string, args ...interface{}) {
		// Print verbose information in verbose mode
		if options.Verbose {
			fmt.Printf("[VERBOSE] "+format+"\n", args...)
		}
	}
	
	debug := func(format string, args ...interface{}) {
		// Only print debug information in debug mode
		if options.Debug {
			fmt.Printf("[DEBUG] "+format+"\n", args...)
		}
	}
	// Handle filtering first to get an accurate count for dry run
	var keysToDelete []string

	// If keys are provided, use them directly
	if len(keys) > 0 {
		keysToDelete = keys
		debug("Using provided keys: %d keys", len(keysToDelete))
	} else {
		// Otherwise check for filtering criteria:
		// 1. Explicit --all-keys flag
		// 2. Non-empty prefix filtering
		// 3. Empty prefix specified with --prefix ""
		// 4. Pattern-based filtering
		shouldListAllKeys := options.AllKeys || options.Prefix != "" || options.PrefixSpecified || options.Pattern != ""

		if shouldListAllKeys {
			debug("Finding keys with criteria: prefix='%s', pattern='%s', allKeys=%v",
				options.Prefix, options.Pattern, options.AllKeys)

			// Use existing pagination-aware function to list keys
			listOptions := &ListKeysOptions{
				Prefix: options.Prefix,
				// Pattern is handled separately, not directly in the listing API
			}

			allKeys, err := ListAllKeysWithOptions(s.client, accountID, namespaceID, listOptions, nil)
			if err != nil {
				return 0, fmt.Errorf("failed to list keys: %w", err)
			}

			verbose("Found %d keys matching criteria", len(allKeys))
			debug("Matched keys count: %d, proceeding with deletion", len(allKeys))

			// Extract key names
			keysToDelete = make([]string, len(allKeys))
			for i, key := range allKeys {
				keysToDelete[i] = key.Key
			}
		} else {
			verbose("No keys or filtering criteria provided")
			debug("Empty criteria, no keys to process")
		}
	}

	// If we have tag-based filtering or search, use the appropriate functions
	if options.TagField != "" || options.SearchValue != "" {
		verbose("Using advanced filtering with tag field '%s' or search value '%s'",
			options.TagField, options.SearchValue)
		debug("Starting advanced filtering process with field='%s', value='%s'", 
			options.TagField, options.SearchValue)

		// If dry run, simulate count for advanced filtering
		if options.DryRun {
			verbose("Dry run, would process %d keys with advanced filtering", len(keysToDelete))
			debug("Dry run mode, skipping actual deletion for %d keys", len(keysToDelete))
			return len(keysToDelete), nil
		}

		return s.bulkDeleteWithAdvancedFiltering(ctx, accountID, namespaceID, keys, options)
	}

	// If dry run, return the count without deleting
	if options.DryRun {
		verbose("Dry run, would delete %d keys", len(keysToDelete))
		debug("Dry run mode active, skipping actual deletion")
		return len(keysToDelete), nil
	}

	// If we have no keys to delete after all filtering, just return 0
	if len(keysToDelete) == 0 {
		verbose("No keys to delete after filtering")
		debug("Filter result: 0 keys matched criteria")
		return 0, nil
	}

	verbose("Deleting %d keys", len(keysToDelete))
	debug("Starting deletion process for %d keys", len(keysToDelete))

	// Define a progress callback for showing batch progress
	var progressCallback func(completed, total int)

	// Only create callback in verbose mode
	if options.Verbose {
		progressCallback = func(completed, total int) {
			percent := float64(completed) / float64(total) * 100
			verbose("Progress: %d/%d keys deleted (%.1f%%)", completed, total, percent)
			debug("Batch deletion progress: %d/%d (%.1f%%)", completed, total, percent)
		}
	}

	// Delete the collected keys
	if options.Concurrency > 0 {
		// Use concurrent deletion for better performance
		verbose("Using concurrent deletion with %d workers", options.Concurrency)
		debug("Initializing concurrent deletion with %d workers, batch size %d", options.Concurrency, options.BatchSize)
		successCount, errs := DeleteMultipleValuesConcurrently(s.client, accountID, namespaceID, keysToDelete, options.BatchSize, options.Concurrency, progressCallback)
		if len(errs) > 0 {
			return successCount, errs[0] // Return the first error encountered
		}
		return successCount, nil
	} else {
		// Fall back to sequential deletion
		verbose("Using sequential deletion")
		debug("Initializing sequential deletion with batch size %d", options.BatchSize)
		err := DeleteMultipleValuesInBatches(s.client, accountID, namespaceID, keysToDelete, options.BatchSize, progressCallback)
		if err != nil {
			return 0, err
		}
		return len(keysToDelete), nil
	}
}

// bulkDeleteWithAdvancedFiltering handles complex delete operations with filtering
func (s *CloudflareKVService) bulkDeleteWithAdvancedFiltering(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
	// Define debug functions that respect verbosity flags
	verbose := func(format string, args ...interface{}) {
		// Print verbose information in verbose mode
		if options.Verbose {
			fmt.Printf("[VERBOSE] "+format+"\n", args...)
		}
	}
	
	debug := func(format string, args ...interface{}) {
		// Only print debug information in debug mode
		if options.Debug {
			fmt.Printf("[DEBUG] "+format+"\n", args...)
		}
	}

	// Define a progress callback for showing batch progress in verbose mode
	var progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)

	// Only create callback in verbose mode
	if options.Verbose {
		progressCallback = func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
			// Show detailed progress information
			if total > 0 {
				fetchPercent := float64(keysFetched) / float64(total) * 100
				procPercent := float64(keysProcessed) / float64(total) * 100
				debug("Progress: %d/%d keys fetched (%.1f%%), %d/%d processed (%.1f%%), %d matched, %d deleted",
					keysFetched, total, fetchPercent, keysProcessed, total, procPercent, keysMatched, keysDeleted)
			} else {
				debug("Progress: %d keys fetched, %d processed, %d matched, %d deleted",
					keysFetched, keysProcessed, keysMatched, keysDeleted)
			}
		}
	}

	// This will use the appropriate purge function based on the options
	if options.SearchValue != "" {
		verbose("Using smart purge by value '%s'", options.SearchValue)
		debug("Starting smart purge operation with search value '%s'", options.SearchValue)
		// Use smart purge by value
		return SmartPurgeByValue(s.client, accountID, namespaceID, options.SearchValue,
			options.BatchSize, options.Concurrency, options.DryRun, progressCallback)
	} else if options.TagField != "" {
		verbose("Using tag-based purge with field '%s', value '%s'", options.TagField, options.TagValue)
		debug("Starting tag-based purge with metadata field '%s', value '%s'", options.TagField, options.TagValue)
		// Use tag-based purge
		return PurgeByMetadataOnly(s.client, accountID, namespaceID, options.TagField, options.TagValue,
			options.BatchSize, options.Concurrency, options.DryRun, progressCallback)
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
