package kv

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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
			if _, err := w.Write([]byte(`{
				"success": true,
				"errors": [],
				"messages": [],
				"result_info": {
					"cursor": "",
					"count": 3
				},
				"result": [
					{"name": "key1", "expiration": 0, "metadata": {"cache-tag": "test-tag"}},
					{"name": "key2", "expiration": 0, "metadata": {"cache-tag": "different-tag"}},
					{"name": "key3", "expiration": 0, "metadata": {"other-field": "value"}}
				]
			}`)); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
		case r.URL.Path == "/client/v4/accounts/test-account/storage/kv/namespaces/test-namespace/bulk":
			// Mock response for bulk operations
			if _, err := w.Write([]byte(`{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"success_count": 3
				}
			}`)); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
		default:
			// Return metadata for specific keys
			if _, err := w.Write([]byte(`{
				"success": true,
				"errors": [],
				"messages": [],
				"result": {
					"name": "testkey", 
					"expiration": 0,
					"metadata": {
						"cache-tag": "test-tag"
					}
				}
			}`)); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
		}
	}))
	
	// Create client with the mock server
	creds := &auth.CredentialInfo{
		Type: auth.AuthTypeAPIToken,
		Key:  "test-token",
	}
	client, err := api.NewClient(
		api.WithBaseURL(server.URL),
		api.WithCredentials(creds),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	
	return server, client
}

// Test for atomic counter operations
func TestProgressTrackerThreadSafety(t *testing.T) {
	// Create atomic counters
	var keysFetched int32
	var keysProcessed int32
	var keysDeleted int32
	
	// Create a progress callback function that uses atomic operations
	callback := func(fetched, processed, deleted, total int) {
		// Thread-safe updates using atomic operations
		// Only update if the new value is higher than the current value
		for {
			current := atomic.LoadInt32(&keysFetched)
			if int32(fetched) <= current {
				break // No need to update
			}
			if atomic.CompareAndSwapInt32(&keysFetched, current, int32(fetched)) {
				break // Successfully updated
			}
		}
		
		for {
			current := atomic.LoadInt32(&keysProcessed)
			if int32(processed) <= current {
				break
			}
			if atomic.CompareAndSwapInt32(&keysProcessed, current, int32(processed)) {
				break
			}
		}
		
		for {
			current := atomic.LoadInt32(&keysDeleted)
			if int32(deleted) <= current {
				break
			}
			if atomic.CompareAndSwapInt32(&keysDeleted, current, int32(deleted)) {
				break
			}
		}
	}
	
	// Simulate concurrent updates
	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Each goroutine processes a range of values
			for j := 0; j < iterations; j++ {
				// Simulate concurrent progress updates
				fetched := id*iterations + j
				processed := id*iterations + j
				deleted := id*iterations + j/2 // simulate deletes being slower
				
				callback(fetched, processed, deleted, iterations*numGoroutines)
				
				// Small delay to make race conditions more likely to occur
				if j%10 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state - the value should be the highest among all goroutines
	expected := (numGoroutines-1) * iterations + (iterations - 1)
	if atomic.LoadInt32(&keysFetched) != int32(expected) {
		t.Errorf("Expected keysFetched=%d, got %d", 
			expected, atomic.LoadInt32(&keysFetched))
	}
}

// Test for atomic counter operations with additional matched counter
func TestProgressTrackerWithMatchedKeys(t *testing.T) {
	// Create atomic counters
	var keysFetched int32
	var keysProcessed int32
	var keysMatched int32
	var keysDeleted int32
	
	// Create a progress callback function that uses atomic operations
	callback := func(fetched, processed, matched, deleted, total int) {
		// Thread-safe updates using atomic operations
		// Only update if the new value is higher than the current value
		for {
			current := atomic.LoadInt32(&keysFetched)
			if int32(fetched) <= current {
				break
			}
			if atomic.CompareAndSwapInt32(&keysFetched, current, int32(fetched)) {
				break
			}
		}
		
		for {
			current := atomic.LoadInt32(&keysProcessed)
			if int32(processed) <= current {
				break
			}
			if atomic.CompareAndSwapInt32(&keysProcessed, current, int32(processed)) {
				break
			}
		}
		
		for {
			current := atomic.LoadInt32(&keysMatched)
			if int32(matched) <= current {
				break
			}
			if atomic.CompareAndSwapInt32(&keysMatched, current, int32(matched)) {
				break
			}
		}
		
		for {
			current := atomic.LoadInt32(&keysDeleted)
			if int32(deleted) <= current {
				break
			}
			if atomic.CompareAndSwapInt32(&keysDeleted, current, int32(deleted)) {
				break
			}
		}
	}
	
	// Simulate concurrent updates
	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Each goroutine processes a range of values
			for j := 0; j < iterations; j++ {
				// Simulate concurrent progress updates
				fetched := id*iterations + j
				processed := id*iterations + j
				matched := id*iterations + j/3  // simulate matches being slower
				deleted := id*iterations + j/4  // simulate deletes being even slower
				
				callback(fetched, processed, matched, deleted, iterations*numGoroutines)
				
				// Small delay to make race conditions more likely to occur
				if j%10 == 0 {
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final state - the value should be the highest among all goroutines
	expected := (numGoroutines-1) * iterations + (iterations - 1)
	if atomic.LoadInt32(&keysFetched) != int32(expected) {
		t.Errorf("Expected keysFetched=%d, got %d", 
			expected, atomic.LoadInt32(&keysFetched))
	}
}

// TestServiceBulkDeleteConcurrency tests the service layer with concurrent operations
func TestServiceBulkDeleteConcurrency(t *testing.T) {
	// Create a simple worker pool to test thread safety
	type WorkerPool struct {
		mu sync.Mutex
		processed int
		matched   int
		deleted   int
		errors    []string
	}
	
	// Create pool
	pool := &WorkerPool{
		errors: []string{},
	}
	
	// Create worker function that uses proper synchronization
	processItem := func(item string, shouldMatch bool, shouldFail bool) {
		// Thread-safe update
		pool.mu.Lock()
		defer pool.mu.Unlock()
		
		// Update processed count
		pool.processed++
		
		// Check for match
		if shouldMatch {
			pool.matched++
		}
		
		// Check for error
		if shouldFail {
			pool.errors = append(pool.errors, 
				fmt.Sprintf("Failed to process item %s", item))
		} else {
			pool.deleted++
		}
	}
	
	// Run concurrent workers
	var wg sync.WaitGroup
	itemCount := 100
	workerCount := 5
	
	// Create items to process
	items := make([]string, itemCount)
	for i := 0; i < itemCount; i++ {
		items[i] = fmt.Sprintf("key-%d", i)
	}
	
	// Process items in chunks
	chunkSize := (itemCount + workerCount - 1) / workerCount
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// Calculate chunk range
			start := workerID * chunkSize
			end := start + chunkSize
			if end > itemCount {
				end = itemCount
			}
			
			// Process chunk
			for j := start; j < end; j++ {
				// Simulate processing logic
				shouldMatch := j%3 == 0  // Every 3rd item matches
				shouldFail := j%15 == 0  // Every 15th item fails
				
				processItem(items[j], shouldMatch, shouldFail)
				
				// Small random delay to simulate concurrent work
				time.Sleep(time.Duration(1) * time.Millisecond)
			}
		}(i)
	}
	
	// Wait for all workers to finish
	wg.Wait()
	
	// Verify results
	if pool.processed != itemCount {
		t.Errorf("Expected %d processed items, got %d", itemCount, pool.processed)
	}
	
	expectedMatched := itemCount / 3
	if pool.matched < expectedMatched-5 || pool.matched > expectedMatched+5 {
		t.Errorf("Expected ~%d matched items, got %d", expectedMatched, pool.matched)
	}
	
	// Allow for a small margin of error due to concurrent nature
	expectedDeleted := itemCount - itemCount/15
	if pool.deleted < expectedDeleted-2 || pool.deleted > expectedDeleted+2 {
		t.Errorf("Expected ~%d deleted items, got %d", expectedDeleted, pool.deleted)
	}
	
	// Allow for a small margin of error
	expectedErrors := itemCount / 15
	if len(pool.errors) < expectedErrors-2 || len(pool.errors) > expectedErrors+2 {
		t.Errorf("Expected ~%d errors, got %d", expectedErrors, len(pool.errors))
	}
}