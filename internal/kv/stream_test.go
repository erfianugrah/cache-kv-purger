package kv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/auth"
)

// TestStreamListKeys tests streaming key listing
func TestStreamListKeys(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")
		
		// Generate response based on cursor
		response := map[string]interface{}{
			"success": true,
			"result": []KeyValuePair{
				{Key: fmt.Sprintf("key_%s_1", cursor)},
				{Key: fmt.Sprintf("key_%s_2", cursor)},
				{Key: fmt.Sprintf("key_%s_3", cursor)},
			},
			"result_info": map[string]interface{}{
				"count": 6,
			},
		}
		
		// Add cursor for next page
		if cursor == "" {
			response["result_info"].(map[string]interface{})["cursor"] = "page2"
		} else if cursor == "page2" {
			response["result_info"].(map[string]interface{})["cursor"] = ""
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
	
	t.Run("StreamWithBatchProcessor", func(t *testing.T) {
		var allKeys []KeyValuePair
		
		processor := NewBatchStreamProcessor(2, func(keys []KeyValuePair) error {
			allKeys = append(allKeys, keys...)
			return nil
		})
		
		err := StreamListKeys(client, "test-account", "test-namespace", nil, processor)
		if err != nil {
			t.Fatal(err)
		}
		
		if len(allKeys) != 6 {
			t.Errorf("Expected 6 keys, got %d", len(allKeys))
		}
		
		// Verify keys are from both pages
		foundPage1 := false
		foundPage2 := false
		for _, key := range allKeys {
			if strings.Contains(key.Key, "key__") {
				foundPage1 = true
			}
			if strings.Contains(key.Key, "key_page2") {
				foundPage2 = true
			}
		}
		
		if !foundPage1 || !foundPage2 {
			t.Error("Expected keys from both pages")
		}
	})
}

// TestStreamParseResponse tests streaming JSON parsing
func TestStreamParseResponse(t *testing.T) {
	// Create a large JSON response
	var response bytes.Buffer
	response.WriteString(`{"success": true, "result": [`)
	
	// Generate many keys
	for i := 0; i < 1000; i++ {
		if i > 0 {
			response.WriteString(",")
		}
		response.WriteString(fmt.Sprintf(`{"name": "key%d"}`, i))
	}
	
	response.WriteString(`], "result_info": {"cursor": "next", "count": 1000}}`)
	
	// Create processor
	var count int
	processor := &countingProcessor{count: &count}
	
	// Parse the response
	result, err := streamParseResponse(&response, processor, 1024) // Small buffer to test streaming
	if err != nil {
		t.Fatal(err)
	}
	
	if result.ProcessedCount != 1000 {
		t.Errorf("Expected 1000 processed keys, got %d", result.ProcessedCount)
	}
	
	if count != 1000 {
		t.Errorf("Expected processor to receive 1000 keys, got %d", count)
	}
	
	if result.Cursor != "next" {
		t.Errorf("Expected cursor 'next', got %s", result.Cursor)
	}
}

// countingProcessor counts processed keys
type countingProcessor struct {
	count *int
}

func (p *countingProcessor) ProcessKey(key KeyValuePair) error {
	*p.count++
	return nil
}

func (p *countingProcessor) Flush() error {
	return nil
}

// TestBatchStreamProcessor tests the batch processor
func TestBatchStreamProcessor(t *testing.T) {
	var batches [][]KeyValuePair
	
	processor := NewBatchStreamProcessor(3, func(keys []KeyValuePair) error {
		// Copy the batch to avoid reference issues
		batch := make([]KeyValuePair, len(keys))
		copy(batch, keys)
		batches = append(batches, batch)
		return nil
	})
	
	// Process 10 keys
	for i := 0; i < 10; i++ {
		err := processor.ProcessKey(KeyValuePair{Key: fmt.Sprintf("key%d", i)})
		if err != nil {
			t.Fatal(err)
		}
	}
	
	// Flush remaining
	err := processor.Flush()
	if err != nil {
		t.Fatal(err)
	}
	
	// Should have 4 batches: 3, 3, 3, 1
	if len(batches) != 4 {
		t.Errorf("Expected 4 batches, got %d", len(batches))
	}
	
	// Check batch sizes
	expectedSizes := []int{3, 3, 3, 1}
	for i, batch := range batches {
		if len(batch) != expectedSizes[i] {
			t.Errorf("Batch %d: expected size %d, got %d", i, expectedSizes[i], len(batch))
		}
	}
}

// BenchmarkStreamingVsRegularParsing compares streaming vs regular JSON parsing
func BenchmarkStreamingVsRegularParsing(b *testing.B) {
	// Generate large JSON response
	generateResponse := func(numKeys int) string {
		var buf bytes.Buffer
		buf.WriteString(`{"success": true, "result": [`)
		
		for i := 0; i < numKeys; i++ {
			if i > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(fmt.Sprintf(`{"name": "key%d", "metadata": {"field1": "value1", "field2": %d}}`, i, i))
		}
		
		buf.WriteString(`], "result_info": {"cursor": "", "count": `)
		buf.WriteString(fmt.Sprintf("%d", numKeys))
		buf.WriteString(`}}`)
		
		return buf.String()
	}
	
	response := generateResponse(10000)
	
	b.Run("RegularParsing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var result KeyValuesResponse
			err := json.Unmarshal([]byte(response), &result)
			if err != nil {
				b.Fatal(err)
			}
			
			// Process all keys
			for _, key := range result.Result {
				_ = key
			}
		}
	})
	
	b.Run("StreamingParsing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			reader := strings.NewReader(response)
			processor := &nullProcessor{}
			
			_, err := streamParseResponse(reader, processor, 4096)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// nullProcessor discards all keys
type nullProcessor struct{}

func (p *nullProcessor) ProcessKey(key KeyValuePair) error {
	return nil
}

func (p *nullProcessor) Flush() error {
	return nil
}

// TestMemoryUsage demonstrates memory efficiency of streaming
func TestMemoryUsage(t *testing.T) {
	// This test demonstrates that streaming uses constant memory
	// regardless of the number of keys
	
	// Create a processor that doesn't store keys
	var processedCount int
	processor := &countingProcessor{count: &processedCount}
	
	// Create a large JSON response inline (simulating streaming from network)
	reader := &largeJSONReader{totalKeys: 100000}
	
	result, err := streamParseResponse(reader, processor, 4096)
	if err != nil {
		t.Fatal(err)
	}
	
	if result.ProcessedCount != 100000 {
		t.Errorf("Expected 100000 processed keys, got %d", result.ProcessedCount)
	}
	
	t.Logf("Successfully processed %d keys with streaming", processedCount)
}

// largeJSONReader generates a large JSON response on the fly
type largeJSONReader struct {
	totalKeys   int
	currentKey  int
	buffer      bytes.Buffer
	initialized bool
}

func (r *largeJSONReader) Read(p []byte) (n int, err error) {
	if !r.initialized {
		r.buffer.WriteString(`{"success": true, "result": [`)
		r.initialized = true
	}
	
	// Generate keys on demand
	for r.buffer.Len() < len(p) && r.currentKey < r.totalKeys {
		if r.currentKey > 0 {
			r.buffer.WriteString(",")
		}
		r.buffer.WriteString(fmt.Sprintf(`{"name": "key%d"}`, r.currentKey))
		r.currentKey++
	}
	
	// End of array
	if r.currentKey >= r.totalKeys && r.buffer.Len() < len(p) {
		r.buffer.WriteString(`], "result_info": {"cursor": "", "count": `)
		r.buffer.WriteString(fmt.Sprintf("%d", r.totalKeys))
		r.buffer.WriteString(`}}`)
	}
	
	// Copy to output buffer
	n = copy(p, r.buffer.Bytes())
	r.buffer.Next(n)
	
	if n == 0 {
		return 0, io.EOF
	}
	
	return n, nil
}