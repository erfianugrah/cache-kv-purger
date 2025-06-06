package kv

import (
	"context"
	"fmt"

	"cache-kv-purger/internal/api"
)

// StreamingListOptions configures streaming behavior
type StreamingListOptions struct {
	// Buffer size for the key channel
	BufferSize int
	// Include progress updates
	Progress bool
	// Progress callback
	ProgressCallback func(fetched int)
}

// StreamKeys returns a channel that streams keys as they're fetched
func StreamKeys(ctx context.Context, client *api.Client, accountID, namespaceID string, 
	listOpts *ListKeysOptions, streamOpts *StreamingListOptions) (<-chan KeyValuePair, <-chan error, error) {
	
	// Validate inputs
	if accountID == "" {
		return nil, nil, fmt.Errorf("account ID is required")
	}
	if namespaceID == "" {
		return nil, nil, fmt.Errorf("namespace ID is required")
	}

	// Set defaults
	if listOpts == nil {
		listOpts = &ListKeysOptions{
			Limit: 1000, // Maximum for best performance
		}
	}
	if streamOpts == nil {
		streamOpts = &StreamingListOptions{
			BufferSize: 1000, // Buffer one page
		}
	}

	// Create channels
	keyChan := make(chan KeyValuePair, streamOpts.BufferSize)
	errChan := make(chan error, 1)

	// Start streaming goroutine
	go func() {
		defer close(keyChan)
		defer close(errChan)

		options := *listOpts // Copy to avoid modifying original
		totalFetched := 0

		for {
			// Fetch a page
			result, err := ListKeysWithOptions(client, accountID, namespaceID, &options)
			if err != nil {
				select {
				case errChan <- err:
				default:
					// Error channel is full, skip
				}
				return
			}

			// Stream keys to channel
			for _, key := range result.Keys {
				select {
				case keyChan <- key:
					totalFetched++
					
					// Progress callback
					if streamOpts.Progress && streamOpts.ProgressCallback != nil && totalFetched%1000 == 0 {
						streamOpts.ProgressCallback(totalFetched)
					}
					
				case <-ctx.Done():
					select {
					case errChan <- ctx.Err():
					default:
					}
					return
				}
			}

			// Check if there are more pages
			if !result.HasMore {
				// Final progress update
				if streamOpts.Progress && streamOpts.ProgressCallback != nil {
					streamOpts.ProgressCallback(totalFetched)
				}
				break
			}

			// Update cursor for next page
			options.Cursor = result.Cursor
		}
	}()

	return keyChan, errChan, nil
}

// ProcessKeysStreaming processes keys as they arrive without loading all into memory
func ProcessKeysStreaming(ctx context.Context, client *api.Client, accountID, namespaceID string,
	listOpts *ListKeysOptions, processor func(key KeyValuePair) error) error {
	
	streamOpts := &StreamingListOptions{
		BufferSize: 1000,
		Progress:   true,
		ProgressCallback: func(fetched int) {
			if fetched%5000 == 0 {
				fmt.Printf("Processed %d keys...\n", fetched)
			}
		},
	}

	keyChan, errChan, err := StreamKeys(ctx, client, accountID, namespaceID, listOpts, streamOpts)
	if err != nil {
		return err
	}

	// Process keys as they arrive
	for key := range keyChan {
		if err := processor(key); err != nil {
			return fmt.Errorf("error processing key %s: %w", key.Key, err)
		}
	}

	// Check for any errors
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("streaming error: %w", err)
		}
	default:
	}

	return nil
}