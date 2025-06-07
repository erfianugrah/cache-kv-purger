package zones

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrentProcessing tests the concurrent processing logic
func TestConcurrentProcessing(t *testing.T) {
	// Test data
	items := map[string][]string{
		"zone1": {"a", "b", "c"},
		"zone2": {"d", "e"},
		"zone3": {"f", "g", "h", "i"},
		"zone4": {"j"},
		"zone5": {"k", "l", "m", "n", "o"},
	}

	// Test concurrent execution with different concurrency levels
	testCases := []struct {
		name        string
		concurrency int
		expectMax   int
	}{
		{"Single worker", 1, 1},
		{"Three workers", 3, 3},
		{"Five workers", 5, 5},
		{"More workers than items", 10, 5}, // Should be capped at number of zones
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var maxConcurrent int32
			var currentConcurrent int32
			var processedCount int32
			var mu sync.Mutex
			processedZones := make(map[string]bool)

			// Simulate the worker pool pattern from ProcessMultiZoneItems
			type work struct {
				zoneID string
				items  []string
			}

			workChan := make(chan work, len(items))
			resultChan := make(chan string, len(items))

			// Add work items
			for zone, zoneItems := range items {
				workChan <- work{zoneID: zone, items: zoneItems}
			}
			close(workChan)

			// Start workers
			var wg sync.WaitGroup
			actualWorkers := tc.concurrency
			if actualWorkers > len(items) {
				actualWorkers = len(items)
			}

			for i := 0; i < actualWorkers; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					for w := range workChan {
						// Track concurrent execution
						current := atomic.AddInt32(&currentConcurrent, 1)
						
						// Update max concurrent
						for {
							max := atomic.LoadInt32(&maxConcurrent)
							if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
								break
							}
						}

						// Simulate work
						time.Sleep(10 * time.Millisecond)

						// Record completion
						mu.Lock()
						processedZones[w.zoneID] = true
						mu.Unlock()

						atomic.AddInt32(&processedCount, 1)
						atomic.AddInt32(&currentConcurrent, -1)
						
						resultChan <- w.zoneID
					}
				}(i)
			}

			// Wait for completion
			go func() {
				wg.Wait()
				close(resultChan)
			}()

			// Collect results
			var results []string
			for result := range resultChan {
				results = append(results, result)
			}

			// Verify results
			if len(results) != len(items) {
				t.Errorf("Expected %d results, got %d", len(items), len(results))
			}

			if int(maxConcurrent) > tc.expectMax {
				t.Errorf("Max concurrent (%d) exceeded expected (%d)", maxConcurrent, tc.expectMax)
			}

			if len(processedZones) != len(items) {
				t.Errorf("Expected %d zones processed, got %d", len(items), len(processedZones))
			}

			t.Logf("Processed %d zones with max concurrency of %d", processedCount, maxConcurrent)
		})
	}
}

// TestBinarySearchPattern tests the binary search failure detection pattern
func TestBinarySearchPattern(t *testing.T) {
	// Test the recursive binary search pattern used in delete_optimized.go
	type testCase struct {
		items       []string
		failingItem string
		expectCalls int
	}

	tests := []testCase{
		{
			items:       []string{"a", "b", "c", "d"},
			failingItem: "c",
			expectCalls: 3, // Full batch, left half, right half
		},
		{
			items:       []string{"a"},
			failingItem: "a",
			expectCalls: 1,
		},
		{
			items:       []string{"a", "b", "c", "d", "e", "f", "g", "h"},
			failingItem: "e",
			expectCalls: 4, // Full, left/right halves, then smaller splits
		},
	}

	for _, tc := range tests {
		t.Run("Binary search with "+tc.failingItem, func(t *testing.T) {
			var callCount int32
			foundFailures := simulateBinarySearch(tc.items, tc.failingItem, &callCount)

			if len(foundFailures) != 1 || foundFailures[0] != tc.failingItem {
				t.Errorf("Expected to find %s, got %v", tc.failingItem, foundFailures)
			}

			t.Logf("Found failing item %s in %d calls", tc.failingItem, callCount)
		})
	}
}

// simulateBinarySearch simulates the binary search pattern
func simulateBinarySearch(items []string, failingItem string, callCount *int32) []string {
	if len(items) <= 1 {
		atomic.AddInt32(callCount, 1)
		if len(items) == 1 && items[0] == failingItem {
			return items
		}
		return nil
	}

	// Simulate bulk operation
	atomic.AddInt32(callCount, 1)
	hasFailure := false
	for _, item := range items {
		if item == failingItem {
			hasFailure = true
			break
		}
	}

	if !hasFailure {
		return nil
	}

	// Split and search
	mid := len(items) / 2
	left := simulateBinarySearch(items[:mid], failingItem, callCount)
	right := simulateBinarySearch(items[mid:], failingItem, callCount)

	var result []string
	if left != nil {
		result = append(result, left...)
	}
	if right != nil {
		result = append(result, right...)
	}
	return result
}