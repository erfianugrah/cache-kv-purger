package kv

import (
	"cache-kv-purger/internal/api"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// mockServer creates a test server for KV operations
func mockServer(t *testing.T) (*httptest.Server, *api.Client) {
	// Keep track of requests
	var requestCount int32
	
	// Create a simple mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		
		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		// Return mock responses based on the endpoint
		switch {
		case r.URL.Path == "/client/v4/accounts/test-account/storage/kv/namespaces/test-namespace/keys":
			// Mock response for list keys
			w.Write([]byte(`{
				"success": true,
				"errors": [],
				"messages": [],
				"result": [
					{"name": "key1", "metadata": {"cache-tag": "test-tag"}},
					{"name": "key2", "metadata": {"cache-tag": "different-tag"}},
					{"name": "key3", "metadata": {"other-field": "value"}}
				]
			}`))
		case r.URL.Path == "/client/v4/accounts/test-account/storage/kv/namespaces/test-namespace/bulk":
			// Mock response for bulk operations
			w.Write([]byte(`{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"success_count": 3
				}
			}`))
		default:
			// Return metadata for specific keys
			w.Write([]byte(`{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"cache-tag": "test-tag"
				}
			}`))
		}
	}))
	
	// Create client with the mock server
	client := api.NewClient(server.URL, "test-token")
	
	return server, client
}

// TestStreamingPurgeByTagFixed verifies that race conditions are fixed
func TestStreamingPurgeByTagFixed(t *testing.T) {
	server, client := mockServer(t)
	defer server.Close()
	
	// Test with concurrent progress updates
	var (
		progressCallCount int32
		progressMutex     sync.Mutex
		lastFetched       int
		lastProcessed     int
		lastDeleted       int
	)
	
	// Create a progress callback that simulates concurrent access
	progressCallback := func(keysFetched, keysProcessed, keysDeleted, total int) {
		atomic.AddInt32(&progressCallCount, 1)
		
		// We use a mutex here to verify thread safety in updates
		progressMutex.Lock()
		defer progressMutex.Unlock()
		
		// Verify that progress never goes backward
		if keysFetched < lastFetched {
			t.Errorf("Race condition: keysFetched went backward: %d < %d", keysFetched, lastFetched)
		}
		if keysProcessed < lastProcessed {
			t.Errorf("Race condition: keysProcessed went backward: %d < %d", keysProcessed, lastProcessed)
		}
		if keysDeleted < lastDeleted {
			t.Errorf("Race condition: keysDeleted went backward: %d < %d", keysDeleted, lastDeleted)
		}
		
		lastFetched = keysFetched
		lastProcessed = keysProcessed
		lastDeleted = keysDeleted
	}
	
	// Call the fixed function
	count, err := StreamingPurgeByTagFixed(client, "test-account", "test-namespace", "cache-tag", "test-tag", 1, 10, false, progressCallback)
	
	if err != nil {
		t.Fatalf("StreamingPurgeByTagFixed failed: %v", err)
	}
	
	// Verify results
	if count == 0 {
		t.Errorf("Expected deleted count > 0, got %d", count)
	}
	
	// Verify progress was reported
	if atomic.LoadInt32(&progressCallCount) == 0 {
		t.Error("Progress callback was never called")
	}
}

// TestPurgeByMetadataOnlyFixed verifies that race conditions are fixed
func TestPurgeByMetadataOnlyFixed(t *testing.T) {
	server, client := mockServer(t)
	defer server.Close()
	
	// Test with concurrent progress updates
	var (
		progressCallCount int32
		progressMutex     sync.Mutex
		lastFetched       int
		lastProcessed     int
		lastMatched       int
		lastDeleted       int
	)
	
	// Create a progress callback that simulates concurrent access
	progressCallback := func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
		atomic.AddInt32(&progressCallCount, 1)
		
		// We use a mutex here to verify thread safety in updates
		progressMutex.Lock()
		defer progressMutex.Unlock()
		
		// Verify that progress never goes backward
		if keysFetched < lastFetched {
			t.Errorf("Race condition: keysFetched went backward: %d < %d", keysFetched, lastFetched)
		}
		if keysProcessed < lastProcessed {
			t.Errorf("Race condition: keysProcessed went backward: %d < %d", keysProcessed, lastProcessed)
		}
		if keysMatched < lastMatched {
			t.Errorf("Race condition: keysMatched went backward: %d < %d", keysMatched, lastMatched)
		}
		if keysDeleted < lastDeleted {
			t.Errorf("Race condition: keysDeleted went backward: %d < %d", keysDeleted, lastDeleted)
		}
		
		lastFetched = keysFetched
		lastProcessed = keysProcessed
		lastMatched = keysMatched
		lastDeleted = keysDeleted
	}
	
	// Call the fixed function
	count, err := PurgeByMetadataOnlyFixed(client, "test-account", "test-namespace", "cache-tag", "test-tag", 1, 10, false, progressCallback)
	
	if err != nil {
		t.Fatalf("PurgeByMetadataOnlyFixed failed: %v", err)
	}
	
	// Verify results
	if count == 0 {
		t.Errorf("Expected deleted count > 0, got %d", count)
	}
	
	// Verify progress was reported
	if atomic.LoadInt32(&progressCallCount) == 0 {
		t.Error("Progress callback was never called")
	}
}

// TestServiceBulkDeleteFixed tests that the service layer fixes work correctly
func TestServiceBulkDeleteFixed(t *testing.T) {
	server, client := mockServer(t)
	defer server.Close()
	
	// Create service
	service := NewKVService(client)
	
	// Test custom service method
	cloudflareService, ok := service.(*CloudflareKVService)
	if !ok {
		t.Fatal("Could not cast to CloudflareKVService")
	}
	
	// Create options
	options := BulkDeleteOptions{
		BatchSize:   10,
		Concurrency: 5,
		TagField:    "cache-tag",
		TagValue:    "test-tag",
		Verbose:     true,
		Debug:       true,
	}
	
	// Call the fixed method
	count, err := cloudflareService.BulkDeleteFixed(nil, "test-account", "test-namespace", nil, options)
	
	if err != nil {
		t.Fatalf("BulkDeleteFixed failed: %v", err)
	}
	
	// Verify results
	if count == 0 {
		t.Errorf("Expected deleted count > 0, got %d", count)
	}
}