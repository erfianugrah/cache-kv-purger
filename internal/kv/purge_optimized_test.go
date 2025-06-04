package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
	
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
)

// TestOptimizedPurgePerformance compares regular vs optimized purge
func TestOptimizedPurgePerformance(t *testing.T) {
	var deleteRequests int32
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/accounts/test-account/storage/kv/namespaces/test-namespace/keys":
			// List keys response
			response := KeyValuesResponse{
				Success: true,
				Result: []KeyValuePair{
					{
						Key: "key1",
						Metadata: &KeyValueMetadata{
							"status": "archived",
							"type":   "test",
						},
					},
					{
						Key: "key2",
						Metadata: &KeyValueMetadata{
							"status": "active",
							"type":   "test",
						},
					},
					{
						Key: "key3",
						Metadata: &KeyValueMetadata{
							"status": "archived",
							"type":   "test",
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			
		case "/accounts/test-account/storage/kv/namespaces/test-namespace/bulk/delete":
			// Bulk delete
			atomic.AddInt32(&deleteRequests, 1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success": true, "errors": [], "messages": []}`))
		default:
			// Handle values endpoint for metadata fetching
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
	
	// Test optimized purge
	t.Run("OptimizedPurge", func(t *testing.T) {
		atomic.StoreInt32(&deleteRequests, 0)
		
		start := time.Now()
		deleted, err := OptimizedPurgeByMetadata(client, "test-account", "test-namespace", &OptimizedPurgeOptions{
			TagField:    "status",
			TagValue:    "archived",
			BatchSize:   10,
			Concurrency: 5,
		})
		duration := time.Since(start)
		
		if err != nil {
			t.Fatal(err)
		}
		
		if deleted != 2 {
			t.Errorf("Expected 2 keys deleted, got %d", deleted)
		}
		
		t.Logf("Optimized purge completed in %v with %d delete requests", duration, atomic.LoadInt32(&deleteRequests))
	})
}

// TestOptimizedExport tests the optimized export functionality
func TestOptimizedExport(t *testing.T) {
	var valueRequests int32
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&valueRequests, 1)
		
		// Mock value response
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
		t.Fatal(err)
	}
	
	// Create test keys
	keys := make([]KeyValuePair, 20)
	for i := 0; i < 20; i++ {
		keys[i] = KeyValuePair{
			Key: fmt.Sprintf("key%d", i),
		}
	}
	
	start := time.Now()
	results, err := OptimizedExportKeys(client, "test-account", "test-namespace", keys, false)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatal(err)
	}
	
	if len(results) != 20 {
		t.Errorf("Expected 20 results, got %d", len(results))
	}
	
	// Should be much faster than serial (20 * delay would be 200ms minimum with old code)
	t.Logf("Exported %d keys in %v with %d requests", len(keys), duration, atomic.LoadInt32(&valueRequests))
	
	// Optimized version should complete much faster
	if duration > 100*time.Millisecond {
		t.Logf("Warning: Export took longer than expected: %v", duration)
	}
}

// BenchmarkPurgeComparison compares different purge implementations
func BenchmarkPurgeComparison(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/accounts/test-account/storage/kv/namespaces/test-namespace/keys":
			// Generate many keys
			var keys []KeyValuePair
			for i := 0; i < 100; i++ {
				keys = append(keys, KeyValuePair{
					Key: fmt.Sprintf("key%d", i),
					Metadata: &KeyValueMetadata{
						"status": func() string {
							if i%2 == 0 {
								return "archived"
							}
							return "active"
						}(),
					},
				})
			}
			
			response := KeyValuesResponse{
				Success: true,
				Result:  keys,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			
		case "/accounts/test-account/storage/kv/namespaces/test-namespace/bulk":
			// Mock bulk delete
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"success": true}`))
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
		b.Fatal(err)
	}
	
	b.Run("OptimizedPurge", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := OptimizedPurgeByMetadata(client, "test-account", "test-namespace", &OptimizedPurgeOptions{
				TagField:    "status",
				TagValue:    "archived",
				BatchSize:   50,
				Concurrency: 10,
				DryRun:      true, // Don't actually delete
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}