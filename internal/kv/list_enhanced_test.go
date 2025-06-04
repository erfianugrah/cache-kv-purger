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

// TestListKeysEnhanced tests the enhanced list functionality
func TestListKeysEnhanced(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if metadata is requested
		includeMetadata := r.URL.Query().Get("include") == "metadata"
		
		// Mock response
		response := KeyValuesResponse{
			Success: true,
			Result: []KeyValuePair{
				{
					Key:   "key1",
					Value: "",
					Metadata: func() *KeyValueMetadata {
						if includeMetadata {
							return &KeyValueMetadata{
								"status": "active",
								"type":   "test",
							}
						}
						return nil
					}(),
				},
				{
					Key:   "key2",
					Value: "",
					Metadata: func() *KeyValueMetadata {
						if includeMetadata {
							return &KeyValueMetadata{
								"status": "inactive",
								"type":   "test",
							}
						}
						return nil
					}(),
				},
			},
			ResultInfo: struct {
				Cursor string `json:"cursor"`
				Count  int    `json:"count"`
			}{
				Count: 2,
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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
	
	t.Run("WithoutMetadata", func(t *testing.T) {
		options := &EnhancedListOptions{
			Limit:           10,
			IncludeMetadata: false,
		}
		
		result, err := ListKeysEnhanced(client, "test-account", "test-namespace", options)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(result.Keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(result.Keys))
		}
		
		// Metadata should be nil when not requested
		for _, key := range result.Keys {
			if key.Metadata != nil {
				t.Errorf("Expected nil metadata for key %s", key.Key)
			}
		}
	})
	
	t.Run("WithMetadata", func(t *testing.T) {
		options := &EnhancedListOptions{
			Limit:           10,
			IncludeMetadata: true,
		}
		
		result, err := ListKeysEnhanced(client, "test-account", "test-namespace", options)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(result.Keys) != 2 {
			t.Errorf("Expected 2 keys, got %d", len(result.Keys))
		}
		
		// Metadata should be present when requested
		for _, key := range result.Keys {
			if key.Metadata == nil {
				t.Errorf("Expected metadata for key %s", key.Key)
			}
		}
	})
}

// TestStreamListKeysEnhanced tests streaming functionality
func TestStreamListKeysEnhanced(t *testing.T) {
	var requestCount int32
	
	// Create mock server with pagination
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		
		cursor := r.URL.Query().Get("cursor")
		
		// Simulate pagination
		var response KeyValuesResponse
		if cursor == "" {
			// First page
			response = KeyValuesResponse{
				Success: true,
				Result: []KeyValuePair{
					{Key: "key1"},
					{Key: "key2"},
				},
				ResultInfo: struct {
					Cursor string `json:"cursor"`
					Count  int    `json:"count"`
				}{
					Cursor: "next-page",
					Count:  2,
				},
			}
		} else {
			// Second page
			response = KeyValuesResponse{
				Success: true,
				Result: []KeyValuePair{
					{Key: "key3"},
					{Key: "key4"},
				},
				ResultInfo: struct {
					Cursor string `json:"cursor"`
					Count  int    `json:"count"`
				}{
					Cursor: "", // No more pages
					Count:  2,
				},
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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
	
	t.Run("SequentialStreaming", func(t *testing.T) {
		atomic.StoreInt32(&requestCount, 0)
		
		var allKeys []KeyValuePair
		options := &EnhancedListOptions{
			Limit:         10,
			ParallelPages: 1,
			StreamCallback: func(keys []KeyValuePair) error {
				allKeys = append(allKeys, keys...)
				return nil
			},
		}
		
		err := StreamListKeysEnhanced(client, "test-account", "test-namespace", options)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(allKeys) != 4 {
			t.Errorf("Expected 4 keys total, got %d", len(allKeys))
		}
		
		if atomic.LoadInt32(&requestCount) != 2 {
			t.Errorf("Expected 2 requests, got %d", atomic.LoadInt32(&requestCount))
		}
	})
}

// TestBulkGetMetadata tests parallel metadata fetching
func TestBulkGetMetadata(t *testing.T) {
	var metadataRequests int32
	
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&metadataRequests, 1)
		
		// Simulate delay
		time.Sleep(10 * time.Millisecond)
		
		// Mock metadata response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`"test-value"`))
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
	
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	
	start := time.Now()
	results, err := BulkGetMetadata(client, "test-account", "test-namespace", keys)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatal(err)
	}
	
	if len(results) != len(keys) {
		t.Errorf("Expected %d results, got %d", len(keys), len(results))
	}
	
	// With parallel fetching, should be much faster than serial
	serialTime := time.Duration(len(keys)) * 10 * time.Millisecond
	if duration > serialTime {
		t.Errorf("Bulk fetch took too long: %v (expected < %v)", duration, serialTime)
	}
	
	t.Logf("Fetched %d metadata entries in %v (%.2fx speedup)", 
		len(keys), duration, float64(serialTime)/float64(duration))
}

// BenchmarkListComparison compares regular vs enhanced list performance
func BenchmarkListComparison(b *testing.B) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate response with many keys
		var keys []KeyValuePair
		for i := 0; i < 100; i++ {
			keys = append(keys, KeyValuePair{
				Key: fmt.Sprintf("key%d", i),
			})
		}
		
		response := KeyValuesResponse{
			Success: true,
			Result:  keys,
			ResultInfo: struct {
				Cursor string `json:"cursor"`
				Count  int    `json:"count"`
			}{
				Count: len(keys),
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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
	
	b.Run("RegularList", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ListKeys(client, "test-account", "test-namespace")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("EnhancedList", func(b *testing.B) {
		options := &EnhancedListOptions{
			Limit: 100,
		}
		
		for i := 0; i < b.N; i++ {
			_, err := ListKeysEnhanced(client, "test-account", "test-namespace", options)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("EnhancedListWithMetadata", func(b *testing.B) {
		options := &EnhancedListOptions{
			Limit:           100,
			IncludeMetadata: true,
		}
		
		for i := 0; i < b.N; i++ {
			_, err := ListKeysEnhanced(client, "test-account", "test-namespace", options)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}