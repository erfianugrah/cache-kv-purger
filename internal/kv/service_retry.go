package kv

import (
	"context"
	"fmt"
	"time"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// RetryableKVService wraps KVService with automatic retry logic
type RetryableKVService struct {
	service     KVService
	retryConfig *common.RetryConfig
}

// NewRetryableKVService creates a new KV service with retry capabilities
func NewRetryableKVService(service KVService, config *common.RetryConfig) KVService {
	if config == nil {
		config = &common.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 500 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			Jitter:       0.1,
		}
	}

	return &RetryableKVService{
		service:     service,
		retryConfig: config,
	}
}

// List implements KVService.List with retry
func (r *RetryableKVService) List(ctx context.Context, accountID, namespaceID string, options ListOptions) (*ListKeysResult, error) {
	var result *ListKeysResult
	
	err := common.RetryOperation(ctx, "kv_list", func() error {
		var err error
		result, err = r.service.List(ctx, accountID, namespaceID, options)
		return err
	})
	
	return result, err
}

// ListAll implements KVService.ListAll with retry
func (r *RetryableKVService) ListAll(ctx context.Context, accountID, namespaceID string, options ListOptions) ([]KeyValuePair, error) {
	var result []KeyValuePair
	
	err := common.RetryOperation(ctx, "kv_list_all", func() error {
		var err error
		result, err = r.service.ListAll(ctx, accountID, namespaceID, options)
		return err
	})
	
	return result, err
}

// Get implements KVService.Get with retry
func (r *RetryableKVService) Get(ctx context.Context, accountID, namespaceID, key string, options ServiceGetOptions) (*KeyValuePair, error) {
	var result *KeyValuePair
	
	err := common.RetryOperation(ctx, "kv_get", func() error {
		var err error
		result, err = r.service.Get(ctx, accountID, namespaceID, key, options)
		return err
	})
	
	return result, err
}

// Put implements KVService.Put with retry
func (r *RetryableKVService) Put(ctx context.Context, accountID, namespaceID, key, value string, options WriteOptions) error {
	return common.RetryOperation(ctx, "kv_put", func() error {
		return r.service.Put(ctx, accountID, namespaceID, key, value, options)
	})
}

// Delete implements KVService.Delete with retry
func (r *RetryableKVService) Delete(ctx context.Context, accountID, namespaceID, key string) error {
	return common.RetryOperation(ctx, "kv_delete", func() error {
		return r.service.Delete(ctx, accountID, namespaceID, key)
	})
}

// Exists implements KVService.Exists with retry
func (r *RetryableKVService) Exists(ctx context.Context, accountID, namespaceID, key string) (bool, error) {
	var result bool
	
	err := common.RetryOperation(ctx, "kv_exists", func() error {
		var err error
		result, err = r.service.Exists(ctx, accountID, namespaceID, key)
		return err
	})
	
	return result, err
}

// BulkGet implements KVService.BulkGet with retry
func (r *RetryableKVService) BulkGet(ctx context.Context, accountID, namespaceID string, keys []string, options BulkGetOptions) ([]KeyValuePair, error) {
	var result []KeyValuePair
	
	err := common.RetryOperation(ctx, "kv_bulk_get", func() error {
		var err error
		result, err = r.service.BulkGet(ctx, accountID, namespaceID, keys, options)
		return err
	})
	
	return result, err
}

// BulkPut implements KVService.BulkPut with retry
func (r *RetryableKVService) BulkPut(ctx context.Context, accountID, namespaceID string, items []BulkWriteItem, options BulkWriteOptions) (int, error) {
	var result int
	
	err := common.RetryOperation(ctx, "kv_bulk_put", func() error {
		var err error
		result, err = r.service.BulkPut(ctx, accountID, namespaceID, items, options)
		return err
	})
	
	return result, err
}

// BulkDelete implements KVService.BulkDelete with retry
func (r *RetryableKVService) BulkDelete(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
	var result int
	
	err := common.RetryOperation(ctx, "kv_bulk_delete", func() error {
		var err error
		result, err = r.service.BulkDelete(ctx, accountID, namespaceID, keys, options)
		return err
	})
	
	return result, err
}

// Search implements KVService.Search with retry
func (r *RetryableKVService) Search(ctx context.Context, accountID, namespaceID string, options SearchOptions) ([]KeyValuePair, error) {
	var result []KeyValuePair
	
	err := common.RetryOperation(ctx, "kv_search", func() error {
		var err error
		result, err = r.service.Search(ctx, accountID, namespaceID, options)
		return err
	})
	
	return result, err
}

// ListNamespaces implements KVService.ListNamespaces with retry
func (r *RetryableKVService) ListNamespaces(ctx context.Context, accountID string) ([]Namespace, error) {
	var result []Namespace
	
	err := common.RetryOperation(ctx, "kv_list_namespaces", func() error {
		var err error
		result, err = r.service.ListNamespaces(ctx, accountID)
		return err
	})
	
	return result, err
}

// CreateNamespace implements KVService.CreateNamespace with retry
func (r *RetryableKVService) CreateNamespace(ctx context.Context, accountID, title string) (*Namespace, error) {
	var result *Namespace
	
	err := common.RetryOperation(ctx, "kv_create_namespace", func() error {
		var err error
		result, err = r.service.CreateNamespace(ctx, accountID, title)
		return err
	})
	
	return result, err
}

// RenameNamespace implements KVService.RenameNamespace with retry
func (r *RetryableKVService) RenameNamespace(ctx context.Context, accountID, namespaceID, newTitle string) (*Namespace, error) {
	var result *Namespace
	
	err := common.RetryOperation(ctx, "kv_rename_namespace", func() error {
		var err error
		result, err = r.service.RenameNamespace(ctx, accountID, namespaceID, newTitle)
		return err
	})
	
	return result, err
}

// DeleteNamespace implements KVService.DeleteNamespace with retry
func (r *RetryableKVService) DeleteNamespace(ctx context.Context, accountID, namespaceID string) error {
	return common.RetryOperation(ctx, "kv_delete_namespace", func() error {
		return r.service.DeleteNamespace(ctx, accountID, namespaceID)
	})
}

// FindNamespaceByTitle implements KVService.FindNamespaceByTitle with retry
func (r *RetryableKVService) FindNamespaceByTitle(ctx context.Context, accountID, title string) (*Namespace, error) {
	var result *Namespace
	
	err := common.RetryOperation(ctx, "kv_find_namespace", func() error {
		var err error
		result, err = r.service.FindNamespaceByTitle(ctx, accountID, title)
		return err
	})
	
	return result, err
}

// FindNamespacesByPattern implements KVService.FindNamespacesByPattern with retry
func (r *RetryableKVService) FindNamespacesByPattern(ctx context.Context, accountID, pattern string) ([]Namespace, error) {
	var result []Namespace
	
	err := common.RetryOperation(ctx, "kv_find_namespaces_pattern", func() error {
		var err error
		result, err = r.service.FindNamespacesByPattern(ctx, accountID, pattern)
		return err
	})
	
	return result, err
}

// ResolveNamespaceID implements KVService.ResolveNamespaceID with retry
func (r *RetryableKVService) ResolveNamespaceID(ctx context.Context, accountID, nameOrID string) (string, error) {
	var result string
	
	err := common.RetryOperation(ctx, "kv_resolve_namespace", func() error {
		var err error
		result, err = r.service.ResolveNamespaceID(ctx, accountID, nameOrID)
		return err
	})
	
	return result, err
}

// NewKVServiceWithRetry creates a new KV service with retry capabilities
func NewKVServiceWithRetry(client *api.Client, retryConfig *common.RetryConfig) KVService {
	baseService := NewKVService(client)
	return NewRetryableKVService(baseService, retryConfig)
}

// DemoRetryBehavior demonstrates retry behavior
func DemoRetryBehavior(client *api.Client, accountID, namespaceID string) {
	fmt.Println("🔄 Retry Mechanism Demo")
	fmt.Println("======================")
	
	// Create service with custom retry config
	retryConfig := &common.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.2,
	}
	
	service := NewKVServiceWithRetry(client, retryConfig)
	
	// Test various operations
	ctx := context.Background()
	
	// Test listing with potential retry
	fmt.Println("\n1️⃣ Testing List Operation with Retry")
	start := time.Now()
	
	result, err := service.List(ctx, accountID, namespaceID, ListOptions{Limit: 10})
	if err != nil {
		fmt.Printf("❌ Error after retries: %v\n", err)
	} else {
		fmt.Printf("✅ Success: Found %d keys in %v\n", len(result.Keys), time.Since(start))
	}
	
	// Test get with retry
	if len(result.Keys) > 0 {
		fmt.Println("\n2️⃣ Testing Get Operation with Retry")
		start = time.Now()
		
		key := result.Keys[0].Key
		kvp, err := service.Get(ctx, accountID, namespaceID, key, ServiceGetOptions{})
		if err != nil {
			fmt.Printf("❌ Error after retries: %v\n", err)
		} else {
			fmt.Printf("✅ Success: Retrieved key '%s' in %v\n", kvp.Key, time.Since(start))
		}
	}
	
	fmt.Println("\n💡 Retry Benefits:")
	fmt.Println("- Automatic retry on transient failures")
	fmt.Println("- Exponential backoff prevents overwhelming the API")
	fmt.Println("- Jitter prevents thundering herd")
	fmt.Println("- Circuit breaker prevents cascading failures")
}