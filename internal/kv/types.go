package kv

import (
	"cache-kv-purger/internal/api"
)

// KeyValuePair represents a key-value pair in a KV namespace
type KeyValuePair struct {
	Key        string            `json:"name"`
	Value      string            `json:"-"` // Value doesn't come from the API in list operations
	Expiration int64             `json:"expiration,omitempty"`
	Metadata   *KeyValueMetadata `json:"metadata,omitempty"`
}


// KeyValueMetadata represents metadata for a key in a KV namespace
type KeyValueMetadata map[string]interface{}

// KeyName represents a single key name for bulk operations
type KeyName struct {
	Name string `json:"name"`
}

// KeyValuesResponse represents a response containing multiple key-value pairs
type KeyValuesResponse struct {
	Success    bool        `json:"success"`
	Errors     []api.Error `json:"errors,omitempty"`
	Messages   []string    `json:"messages,omitempty"`
	ResultInfo struct {
		Cursor string `json:"cursor"`
		Count  int    `json:"count"`
	} `json:"result_info"`
	Result []KeyValuePair `json:"result"`
}

// WriteOptions represents options for writing a value
type WriteOptions struct {
	Expiration    int64            `json:"expiration,omitempty"`     // Unix timestamp
	ExpirationTTL int64            `json:"expiration_ttl,omitempty"` // TTL in seconds
	Metadata      KeyValueMetadata `json:"metadata,omitempty"`
}

// GetOptions represents options for reading a value
type GetOptions struct {
	IncludeMetadata bool // Whether to include metadata in the response
}

// ListKeysOptions represents options for listing keys
type ListKeysOptions struct {
	Limit  int    `json:"limit,omitempty"`  // Maximum number of keys to return (max 1000)
	Cursor string `json:"cursor,omitempty"` // Cursor for pagination
	Prefix string `json:"prefix,omitempty"` // Filter keys by prefix
}

// ListKeysResult represents the result of a list keys operation, with pagination info
type ListKeysResult struct {
	Keys       []KeyValuePair `json:"keys"`
	Cursor     string         `json:"cursor"`
	HasMore    bool           `json:"has_more"`
	TotalCount int            `json:"total_count"`
}

// KeyValueResponse represents a response for a single key-value operation
type KeyValueResponse struct {
	api.APIResponse
	Result *KeyValuePair `json:"result,omitempty"`
}

// BulkWriteItem represents an item for bulk writes
type BulkWriteItem struct {
	Key           string                 `json:"key"`
	Value         string                 `json:"value"`
	Expiration    int64                  `json:"expiration,omitempty"`
	ExpirationTTL int64                  `json:"expiration_ttl,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// BulkWriteResult represents the result of a bulk write operation
type BulkWriteResult struct {
	Success  bool        `json:"success"`
	Errors   []api.Error `json:"errors,omitempty"`
	Messages []string    `json:"messages,omitempty"`
	Result   struct {
		SuccessCount int `json:"success_count"`
		ErrorCount   int `json:"error_count"`
		Errors       []struct {
			Key   string `json:"key"`
			Error string `json:"error"`
		} `json:"errors,omitempty"`
	} `json:"result"`
}
