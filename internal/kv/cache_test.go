package kv

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
)

// TestLRUCache tests basic LRU cache functionality
func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(3, 1024, 1*time.Minute)
	
	// Test basic set/get
	cache.Set("key1", "value1", 10)
	cache.Set("key2", "value2", 10)
	cache.Set("key3", "value3", 10)
	
	val, found := cache.Get("key1")
	if !found || val != "value1" {
		t.Errorf("Expected to find key1 with value1")
	}
	
	// Test eviction by count
	cache.Set("key4", "value4", 10)
	
	// key2 should be evicted (LRU)
	_, found = cache.Get("key2")
	if found {
		t.Errorf("Expected key2 to be evicted")
	}
	
	// key1 should still exist (was accessed recently)
	_, found = cache.Get("key1")
	if !found {
		t.Errorf("Expected key1 to still exist")
	}
	
	// Test stats
	stats := cache.Stats()
	if stats.Entries != 3 {
		t.Errorf("Expected 3 entries, got %d", stats.Entries)
	}
}

// TestCacheExpiration tests TTL functionality
func TestCacheExpiration(t *testing.T) {
	cache := NewLRUCache(10, 1024, 100*time.Millisecond)
	
	cache.Set("key1", "value1", 10)
	
	// Should exist immediately
	_, found := cache.Get("key1")
	if !found {
		t.Error("Expected key1 to exist")
	}
	
	// Wait for expiration
	time.Sleep(150 * time.Millisecond)
	
	// Should be expired
	_, found = cache.Get("key1")
	if found {
		t.Error("Expected key1 to be expired")
	}
}

// TestCacheConcurrency tests thread safety
func TestCacheConcurrency(t *testing.T) {
	cache := NewLRUCache(1000, 10*1024*1024, 1*time.Minute)
	
	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 1000
	
	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Set(key, j, 10)
			}
		}(i)
	}
	
	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Get(key)
			}
		}(i)
	}
	
	wg.Wait()
	
	stats := cache.Stats()
	t.Logf("Cache stats after concurrent operations: %+v", stats)
}

// TestMetadataCache tests metadata-specific caching
func TestMetadataCache(t *testing.T) {
	cache := NewMetadataCache(100, 10, 1*time.Minute)
	
	metadata := &KeyValueMetadata{
		"field1": "value1",
		"field2": 42,
		"field3": map[string]interface{}{
			"nested": "value",
		},
	}
	
	cache.SetMetadata("key1", metadata)
	
	retrieved, found := cache.GetMetadata("key1")
	if !found {
		t.Error("Expected to find metadata")
	}
	
	if (*retrieved)["field1"] != "value1" {
		t.Error("Metadata mismatch")
	}
	
	stats := cache.Stats()
	if stats.Entries != 1 {
		t.Errorf("Expected 1 entry, got %d", stats.Entries)
	}
}

// TestCachedGetKeyMetadata tests cached metadata fetching
func TestCachedGetKeyMetadata(t *testing.T) {
	var apiCalls int32
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		
		// Handle different endpoints
		if strings.Contains(r.URL.Path, "/metadata/") {
			// Return metadata response
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success": true, "result": {"field1": "value1"}}`))
		} else {
			// Return value response
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("test-value"))
		}
	}))
	defer server.Close()
	
	// Create client
	client, err := api.NewClient(
		api.WithBaseURL(server.URL),
		api.WithCredentials(&auth.CredentialInfo{
			Type:  auth.AuthTypeAPIToken,
			Key:   "test-token",
			Email: "test@example.com",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	
	// Clear cache before test
	GetGlobalMetadataCache().Clear()
	
	// First call should hit API (GetKeyWithMetadata makes 2 API calls: value + metadata)
	metadata1, err := CachedGetKeyMetadata(client, "account1", "namespace1", "key1")
	if err != nil {
		t.Fatal(err)
	}
	
	firstCallCount := atomic.LoadInt32(&apiCalls)
	if firstCallCount != 2 {
		t.Errorf("Expected 2 API calls for GetKeyWithMetadata, got %d", firstCallCount)
	}
	
	// Second call should hit cache
	metadata2, err := CachedGetKeyMetadata(client, "account1", "namespace1", "key1")
	if err != nil {
		t.Fatal(err)
	}
	
	if atomic.LoadInt32(&apiCalls) != firstCallCount {
		t.Errorf("Expected no new API calls (cache hit), got %d", atomic.LoadInt32(&apiCalls)-firstCallCount)
	}
	
	// Metadata should be the same
	if metadata1 != metadata2 {
		t.Error("Expected same metadata from cache")
	}
	
	// Different key should hit API again
	_, err = CachedGetKeyMetadata(client, "account1", "namespace1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	
	newCallCount := atomic.LoadInt32(&apiCalls) - firstCallCount
	if newCallCount != 2 {
		t.Errorf("Expected 2 new API calls, got %d", newCallCount)
	}
}

// TestCachedBulkGetMetadata tests bulk metadata fetching with cache
func TestCachedBulkGetMetadata(t *testing.T) {
	var apiCalls int32
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		
		// Handle different endpoints
		if strings.Contains(r.URL.Path, "/metadata/") {
			// Return metadata response
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success": true, "result": {"field1": "value1"}}`))
		} else {
			// Return value response
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("test-value"))
		}
	}))
	defer server.Close()
	
	// Create client
	client, err := api.NewClient(
		api.WithBaseURL(server.URL),
		api.WithCredentials(&auth.CredentialInfo{
			Type:  auth.AuthTypeAPIToken,
			Key:   "test-token",
			Email: "test@example.com",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	
	// Clear cache before test
	GetGlobalMetadataCache().Clear()
	
	// First bulk request
	keys := []string{"key1", "key2", "key3"}
	results1, err := CachedBulkGetMetadata(client, "account1", "namespace1", keys)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(results1) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results1))
	}
	
	initialCalls := atomic.LoadInt32(&apiCalls)
	
	// Second request with overlapping keys should use cache
	keys2 := []string{"key1", "key2", "key4"} // key1 and key2 should be cached
	results2, err := CachedBulkGetMetadata(client, "account1", "namespace1", keys2)
	if err != nil {
		t.Fatal(err)
	}
	
	if len(results2) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results2))
	}
	
	// Should only make API calls for key4 (2 calls: value + metadata)
	// key1 and key2 should be served from cache
	newCalls := atomic.LoadInt32(&apiCalls) - initialCalls
	if newCalls != 2 {
		t.Errorf("Expected 2 new API calls for key4, got %d", newCalls)
	}
}

// BenchmarkCachePerformance benchmarks cache vs no-cache performance
func BenchmarkCachePerformance(b *testing.B) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond) // Simulate API latency
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("test-value"))
	}))
	defer server.Close()
	
	// Create client
	client, err := api.NewClient(
		api.WithBaseURL(server.URL),
		api.WithCredentials(&auth.CredentialInfo{
			Type:  auth.AuthTypeAPIToken,
			Key:   "test-token",
			Email: "test@example.com",
		}),
	)
	if err != nil {
		b.Fatal(err)
	}
	
	// Pre-populate cache
	GetGlobalMetadataCache().Clear()
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		CachedGetKeyMetadata(client, "account1", "namespace1", key)
	}
	
	b.Run("WithCache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key%d", i%100)
			CachedGetKeyMetadata(client, "account1", "namespace1", key)
		}
	})
	
	b.Run("WithoutCache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key%d", i%100)
			GetKeyWithMetadata(client, "account1", "namespace1", key)
		}
	})
}