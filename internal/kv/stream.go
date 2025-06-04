package kv

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	
	"cache-kv-purger/internal/api"
)

// StreamingOptions provides options for streaming operations
type StreamingOptions struct {
	Prefix          string
	BatchSize       int
	IncludeMetadata bool
	BufferSize      int
	Context         context.Context
}

// StreamProcessor processes keys as they arrive from the API
type StreamProcessor interface {
	ProcessKey(key KeyValuePair) error
	Flush() error
}

// BatchStreamProcessor batches keys before processing
type BatchStreamProcessor struct {
	batchSize int
	buffer    []KeyValuePair
	callback  func([]KeyValuePair) error
}

// NewBatchStreamProcessor creates a new batch stream processor
func NewBatchStreamProcessor(batchSize int, callback func([]KeyValuePair) error) *BatchStreamProcessor {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BatchStreamProcessor{
		batchSize: batchSize,
		buffer:    make([]KeyValuePair, 0, batchSize),
		callback:  callback,
	}
}

// ProcessKey adds a key to the batch
func (p *BatchStreamProcessor) ProcessKey(key KeyValuePair) error {
	p.buffer = append(p.buffer, key)
	
	if len(p.buffer) >= p.batchSize {
		return p.Flush()
	}
	
	return nil
}

// Flush processes any remaining keys in the buffer
func (p *BatchStreamProcessor) Flush() error {
	if len(p.buffer) == 0 {
		return nil
	}
	
	err := p.callback(p.buffer)
	p.buffer = p.buffer[:0]
	return err
}

// StreamListKeys streams keys from the API without loading all into memory
func StreamListKeys(client *api.Client, accountID, namespaceID string, options *StreamingOptions, processor StreamProcessor) error {
	if options == nil {
		options = &StreamingOptions{
			BatchSize:  1000,
			BufferSize: 65536, // 64KB
		}
	}
	
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Build the API path
	path := fmt.Sprintf("/accounts/%s/storage/kv/namespaces/%s/keys", accountID, namespaceID)
	
	cursor := ""
	totalProcessed := 0
	
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Build query parameters
		queryParams := url.Values{}
		queryParams.Set("limit", fmt.Sprintf("%d", options.BatchSize))
		if cursor != "" {
			queryParams.Set("cursor", cursor)
		}
		if options.Prefix != "" {
			queryParams.Set("prefix", options.Prefix)
		}
		if options.IncludeMetadata {
			queryParams.Set("include", "metadata")
		}
		
		// Get raw response for streaming
		resp, err := getRawResponse(client, http.MethodGet, path, queryParams)
		if err != nil {
			return fmt.Errorf("failed to get response: %w", err)
		}
		
		// Stream parse the response
		result, err := streamParseResponse(resp.Body, processor, options.BufferSize)
		resp.Body.Close()
		
		if err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		
		totalProcessed += result.ProcessedCount
		
		// Check if there are more pages
		if result.Cursor == "" || !result.HasMore {
			break
		}
		
		cursor = result.Cursor
	}
	
	// Flush any remaining data
	if err := processor.Flush(); err != nil {
		return fmt.Errorf("failed to flush processor: %w", err)
	}
	
	return nil
}

// streamParseResult holds the result of streaming parse
type streamParseResult struct {
	ProcessedCount int
	Cursor         string
	HasMore        bool
}

// streamParseResponse parses a JSON response stream
func streamParseResponse(body io.Reader, processor StreamProcessor, bufferSize int) (*streamParseResult, error) {
	if bufferSize <= 0 {
		bufferSize = 65536 // 64KB default
	}
	
	reader := bufio.NewReaderSize(body, bufferSize)
	decoder := json.NewDecoder(reader)
	
	result := &streamParseResult{}
	
	// Read opening brace
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('{') {
		return nil, fmt.Errorf("expected object start, got %v", token)
	}
	
	// Parse the response object
	for decoder.More() {
		// Read field name
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		
		fieldName, ok := token.(string)
		if !ok {
			continue
		}
		
		switch fieldName {
		case "success":
			var success bool
			if err := decoder.Decode(&success); err != nil {
				return nil, err
			}
			if !success {
				return nil, fmt.Errorf("API returned success=false")
			}
			
		case "result":
			// Stream parse the result array
			count, err := streamParseKeyArray(decoder, processor)
			if err != nil {
				return nil, fmt.Errorf("failed to parse result array: %w", err)
			}
			result.ProcessedCount = count
			
		case "result_info":
			var info struct {
				Cursor string `json:"cursor"`
				Count  int    `json:"count"`
			}
			if err := decoder.Decode(&info); err != nil {
				return nil, err
			}
			result.Cursor = info.Cursor
			result.HasMore = info.Cursor != ""
			
		case "errors":
			var errors []interface{}
			if err := decoder.Decode(&errors); err != nil {
				return nil, err
			}
			if len(errors) > 0 {
				return nil, fmt.Errorf("API returned errors: %v", errors)
			}
			
		default:
			// Skip unknown fields
			var ignore json.RawMessage
			decoder.Decode(&ignore)
		}
	}
	
	return result, nil
}

// streamParseKeyArray parses an array of keys from the decoder
func streamParseKeyArray(decoder *json.Decoder, processor StreamProcessor) (int, error) {
	// Read array start
	token, err := decoder.Token()
	if err != nil {
		return 0, err
	}
	if token != json.Delim('[') {
		return 0, fmt.Errorf("expected array start, got %v", token)
	}
	
	count := 0
	
	// Parse each key in the array
	for decoder.More() {
		var key KeyValuePair
		if err := decoder.Decode(&key); err != nil {
			return count, fmt.Errorf("failed to decode key: %w", err)
		}
		
		if err := processor.ProcessKey(key); err != nil {
			return count, fmt.Errorf("processor error: %w", err)
		}
		
		count++
	}
	
	// Read array end
	token, err = decoder.Token()
	if err != nil {
		return count, err
	}
	if token != json.Delim(']') {
		return count, fmt.Errorf("expected array end, got %v", token)
	}
	
	return count, nil
}

// getRawResponse gets a raw HTTP response from the API
func getRawResponse(client *api.Client, method, path string, query url.Values) (*http.Response, error) {
	// For now, use the regular request and wrap response
	// This is less efficient but maintains compatibility
	respBody, err := client.Request(method, path, query, nil)
	if err != nil {
		return nil, err
	}
	
	// Create a fake response with the body
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(respBody))),
	}
	
	return resp, nil
}

// StreamExportKeys exports keys using streaming to handle large datasets
func StreamExportKeys(client *api.Client, accountID, namespaceID string, options *StreamingOptions) error {
	if options == nil {
		options = &StreamingOptions{}
	}
	
	// Create a processor that fetches values and outputs JSON
	exporter := &streamingExporter{
		client:      client,
		accountID:   accountID,
		namespaceID: namespaceID,
		first:       true,
	}
	
	// Start JSON array
	fmt.Print("[")
	
	err := StreamListKeys(client, accountID, namespaceID, options, exporter)
	
	// End JSON array
	fmt.Print("\n]")
	
	return err
}

// streamingExporter exports keys as JSON while streaming
type streamingExporter struct {
	client      *api.Client
	accountID   string
	namespaceID string
	first       bool
}

func (e *streamingExporter) ProcessKey(key KeyValuePair) error {
	// Get the value
	value, err := GetValue(e.client, e.accountID, e.namespaceID, key.Key)
	if err != nil {
		return err
	}
	
	// Create export object
	export := map[string]interface{}{
		"key":   key.Key,
		"value": value,
	}
	
	if key.Metadata != nil {
		export["metadata"] = key.Metadata
	}
	
	if key.Expiration > 0 {
		export["expiration"] = key.Expiration
	}
	
	// Output JSON
	if !e.first {
		fmt.Print(",")
	}
	fmt.Print("\n  ")
	
	jsonBytes, err := json.Marshal(export)
	if err != nil {
		return err
	}
	
	fmt.Print(string(jsonBytes))
	e.first = false
	
	return nil
}

func (e *streamingExporter) Flush() error {
	return nil
}