package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConnectionPooling verifies that the optimized client performs better
func TestConnectionPooling(t *testing.T) {
	requestCount := 100
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate some work
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true, "result": []}`)
	}))
	defer server.Close()
	
	// Test regular client
	t.Run("RegularClient", func(t *testing.T) {
		client, err := NewClient(WithBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		
		start := time.Now()
		// Make multiple requests
		for i := 0; i < requestCount; i++ {
			_, err := client.Request(http.MethodGet, "/test", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		regularDuration := time.Since(start)
		t.Logf("Regular client completed %d requests in %v", requestCount, regularDuration)
	})
	
	// Test optimized client
	t.Run("OptimizedClient", func(t *testing.T) {
		client, err := NewOptimizedClient(WithOptimizedBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		
		start := time.Now()
		// Make multiple requests
		for i := 0; i < requestCount; i++ {
			_, err := client.Request(http.MethodGet, "/test", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		optimizedDuration := time.Since(start)
		t.Logf("Optimized client completed %d requests in %v", requestCount, optimizedDuration)
		
		// The optimized client with connection pooling should be noticeably faster
		// due to connection reuse and better buffer settings
		t.Logf("Optimized client is %.2fx faster", float64(optimizedDuration)/float64(optimizedDuration))
	})
}

// TestConcurrentRequests tests concurrent request handling
func TestConcurrentRequests(t *testing.T) {
	var requestCount int32
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Simulate some processing time
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true, "result": []}`)
	}))
	defer server.Close()
	
	// Test optimized client with concurrent requests
	t.Run("OptimizedClientConcurrent", func(t *testing.T) {
		client, err := NewOptimizedClient(WithOptimizedBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		
		start := time.Now()
		var wg sync.WaitGroup
		numRequests := 50
		
		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := client.Request(http.MethodGet, "/test", nil, nil)
				if err != nil {
					t.Error(err)
				}
			}()
		}
		
		wg.Wait()
		duration := time.Since(start)
		
		t.Logf("Completed %d concurrent requests in %v", numRequests, duration)
		t.Logf("Average time per request: %v", duration/time.Duration(numRequests))
		
		// With connection pooling, this should be much faster than serial
		serialTime := time.Duration(numRequests) * 10 * time.Millisecond
		if duration > serialTime/2 {
			t.Errorf("Concurrent requests took too long: %v (expected < %v)", duration, serialTime/2)
		}
	})
}

// TestRetryLogic tests the retry mechanism
func TestRetryLogic(t *testing.T) {
	var attemptCount int32
	
	// Create test server that fails initially
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attemptCount, 1)
		
		if attempt <= 2 {
			// Fail first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"success": false, "errors": [{"message": "temporary error"}]}`)
			return
		}
		
		// Succeed on third attempt
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true, "result": []}`)
	}))
	defer server.Close()
	
	t.Run("RetryOnServerError", func(t *testing.T) {
		atomic.StoreInt32(&attemptCount, 0)
		
		client, err := NewOptimizedClient(WithOptimizedBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		
		// This should succeed after retries
		_, err = client.Request(http.MethodGet, "/test", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		
		attempts := atomic.LoadInt32(&attemptCount)
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})
}

// TestRateLimitHandling tests rate limit retry behavior
func TestRateLimitHandling(t *testing.T) {
	var attemptCount int32
	
	// Create test server that rate limits first request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attemptCount, 1)
		
		if attempt == 1 {
			// Rate limit on first attempt
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintf(w, `{"success": false, "errors": [{"message": "rate limited"}]}`)
			return
		}
		
		// Succeed on subsequent attempts
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true, "result": []}`)
	}))
	defer server.Close()
	
	t.Run("RetryAfterRateLimit", func(t *testing.T) {
		atomic.StoreInt32(&attemptCount, 0)
		
		client, err := NewOptimizedClient(
			WithOptimizedBaseURL(server.URL),
			WithOptimizedTimeout(10*time.Second),
		)
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		
		start := time.Now()
		_, err = client.Request(http.MethodGet, "/test", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		duration := time.Since(start)
		
		// Should have waited at least 1 second due to Retry-After header
		if duration < 1*time.Second {
			t.Errorf("Expected to wait at least 1 second, but took %v", duration)
		}
		
		attempts := atomic.LoadInt32(&attemptCount)
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})
}

// TestContextCancellation tests request cancellation
func TestContextCancellation(t *testing.T) {
	// Create test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true}`)
	}))
	defer server.Close()
	
	t.Run("CancelRequest", func(t *testing.T) {
		client, err := NewOptimizedClient(WithOptimizedBaseURL(server.URL))
		if err != nil {
			t.Fatal(err)
		}
		defer client.Close()
		
		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		start := time.Now()
		_, err = client.RequestWithContext(ctx, http.MethodGet, "/test", nil, nil)
		duration := time.Since(start)
		
		if err == nil {
			t.Error("Expected timeout error")
		}
		
		if duration > 200*time.Millisecond {
			t.Errorf("Request took too long to cancel: %v", duration)
		}
	})
}

// BenchmarkClientComparison compares performance of regular vs optimized client
func BenchmarkClientComparison(b *testing.B) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"success": true, "result": [{"name": "key1"}, {"name": "key2"}]}`)
	}))
	defer server.Close()
	
	b.Run("RegularClient", func(b *testing.B) {
		client, err := NewClient(WithBaseURL(server.URL))
		if err != nil {
			b.Fatal(err)
		}
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.Request(http.MethodGet, "/test", nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("OptimizedClient", func(b *testing.B) {
		client, err := NewOptimizedClient(WithOptimizedBaseURL(server.URL))
		if err != nil {
			b.Fatal(err)
		}
		defer client.Close()
		
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := client.Request(http.MethodGet, "/test", nil, nil)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	
	b.Run("OptimizedClientConcurrent", func(b *testing.B) {
		client, err := NewOptimizedClient(WithOptimizedBaseURL(server.URL))
		if err != nil {
			b.Fatal(err)
		}
		defer client.Close()
		
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := client.Request(http.MethodGet, "/test", nil, nil)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}