package kv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
	
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
)

// TestParallelListAllKeys tests parallel pagination
func TestParallelListAllKeys(t *testing.T) {
	var requestCount int32
	
	// Create mock server with multiple pages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		
		cursor := r.URL.Query().Get("cursor")
		limit := r.URL.Query().Get("limit")
		
		// Simulate multiple pages
		var response KeyValuesResponse
		response.Success = true
		
		// Parse cursor to determine page number
		pageNum := 1
		if cursor != "" {
			if n, err := strconv.Atoi(cursor); err == nil {
				pageNum = n
			}
		}
		
		// Generate keys for this page
		keysPerPage := 10
		if limit != "" {
			if n, err := strconv.Atoi(limit); err == nil && n < keysPerPage {
				keysPerPage = n
			}
		}
		
		for i := 0; i < keysPerPage; i++ {
			keyNum := (pageNum-1)*keysPerPage + i
			response.Result = append(response.Result, KeyValuePair{
				Key: fmt.Sprintf("key%d", keyNum),
			})
		}
		
		// Set pagination info
		response.ResultInfo.Count = 50 // Total count
		if pageNum < 5 { // 5 pages total
			response.ResultInfo.Cursor = fmt.Sprintf("%d", pageNum+1)
		}
		
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		
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
	
	t.Run("ParallelFetch", func(t *testing.T) {
		atomic.StoreInt32(&requestCount, 0)
		
		start := time.Now()
		keys, err := ParallelListAllKeys(client, "test-account", "test-namespace", &ParallelListOptions{
			BatchSize:        10,
			ParallelRequests: 3,
		})
		duration := time.Since(start)
		
		if err != nil {
			t.Fatal(err)
		}
		
		if len(keys) != 50 {
			t.Errorf("Expected 50 keys, got %d", len(keys))
		}
		
		requests := atomic.LoadInt32(&requestCount)
		t.Logf("Fetched %d keys in %v with %d requests", len(keys), duration, requests)
		
		// With parallel fetching, should be faster than serial (5 pages * 10ms = 50ms minimum)
		if duration > 40*time.Millisecond {
			t.Logf("Parallel fetch completed in %v (expected < 40ms)", duration)
		}
	})
}

// TestStreamParallelListKeys tests streaming with parallel pagination
func TestStreamParallelListKeys(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := KeyValuesResponse{
			Success: true,
			Result: []KeyValuePair{
				{Key: "key1"},
				{Key: "key2"},
				{Key: "key3"},
			},
			ResultInfo: struct {
				Cursor string `json:"cursor"`
				Count  int    `json:"count"`
			}{
				Count: 3,
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
	
	var allKeys []KeyValuePair
	err = StreamParallelListKeys(client, "test-account", "test-namespace", nil, func(keys []KeyValuePair) error {
		allKeys = append(allKeys, keys...)
		return nil
	})
	
	if err != nil {
		t.Fatal(err)
	}
	
	if len(allKeys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(allKeys))
	}
}

// BenchmarkParallelVsSequentialList compares parallel vs sequential listing
func BenchmarkParallelVsSequentialList(b *testing.B) {
	// Create mock server with many pages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		
		pageNum := 1
		if cursor != "" {
			if n, err := strconv.Atoi(cursor); err == nil {
				pageNum = n
			}
		}
		
		// Generate 100 keys per page
		var keys []KeyValuePair
		for i := 0; i < 100; i++ {
			keys = append(keys, KeyValuePair{
				Key: fmt.Sprintf("key%d_%d", pageNum, i),
			})
		}
		
		response := KeyValuesResponse{
			Success: true,
			Result:  keys,
			ResultInfo: struct {
				Cursor string `json:"cursor"`
				Count  int    `json:"count"`
			}{
				Count: 1000, // 10 pages total
			},
		}
		
		if pageNum < 10 {
			response.ResultInfo.Cursor = fmt.Sprintf("%d", pageNum+1)
		}
		
		// Simulate API latency
		time.Sleep(5 * time.Millisecond)
		
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
	
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ListAllKeys(client, "test-account", "test-namespace", nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("Parallel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := ParallelListAllKeys(client, "test-account", "test-namespace", &ParallelListOptions{
				BatchSize:        100,
				ParallelRequests: 5,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}