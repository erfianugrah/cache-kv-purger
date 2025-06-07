package kv

import (
	"context"
	"fmt"
)

// Updated bulkDeleteWithAdvancedFiltering handles complex delete operations with filtering
// This version uses the fixed implementations to avoid race conditions
func (s *CloudflareKVService) bulkDeleteWithAdvancedFilteringFixed(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
	// Define debug functions that respect verbosity flags
	verbose := func(format string, args ...interface{}) {
		// Print verbose information in verbose mode
		if options.Verbose {
			fmt.Printf("[VERBOSE] "+format+"\n", args...)
		}
	}

	debug := func(format string, args ...interface{}) {
		// Only print debug information in debug mode
		if options.Debug {
			fmt.Printf("[DEBUG] "+format+"\n", args...)
		}
	}

	// Define a progress callback for showing batch progress in verbose mode
	var progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)

	// Only create callback in verbose mode
	if options.Verbose {
		progressCallback = func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
			// Show detailed progress information
			if total > 0 {
				fetchPercent := float64(keysFetched) / float64(total) * 100
				procPercent := float64(keysProcessed) / float64(total) * 100
				debug("Progress: %d/%d keys fetched (%.1f%%), %d/%d processed (%.1f%%), %d matched, %d deleted",
					keysFetched, total, fetchPercent, keysProcessed, total, procPercent, keysMatched, keysDeleted)
			} else {
				debug("Progress: %d keys fetched, %d processed, %d matched, %d deleted",
					keysFetched, keysProcessed, keysMatched, keysDeleted)
			}
		}
	}

	// This will use the appropriate purge function based on the options
	if options.SearchValue != "" {
		verbose("Using smart purge by value '%s'", options.SearchValue)
		debug("Starting smart purge operation with search value '%s'", options.SearchValue)
		// Use smart purge by value
		return SmartPurgeByValue(s.client, accountID, namespaceID, options.SearchValue,
			options.BatchSize, options.Concurrency, options.DryRun, progressCallback)
	} else if options.TagField != "" {
		verbose("Using tag-based purge with field '%s', value '%s'", options.TagField, options.TagValue)
		debug("Starting tag-based purge with metadata field '%s', value '%s'", options.TagField, options.TagValue)
		// Use tag-based purge with fixed implementation
		return PurgeByMetadataOnlyFixed(s.client, accountID, namespaceID, options.TagField, options.TagValue,
			options.BatchSize, options.Concurrency, options.DryRun, progressCallback)
	}

	// Shouldn't reach here but just in case
	return 0, fmt.Errorf("invalid advanced filtering options")
}

// BulkDeleteFixed deletes multiple values in bulk with race condition fixes
func (s *CloudflareKVService) BulkDeleteFixed(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
	// Define debug functions that respect verbosity flags
	verbose := func(format string, args ...interface{}) {
		// Print verbose information in verbose mode
		if options.Verbose {
			fmt.Printf("[VERBOSE] "+format+"\n", args...)
		}
	}

	debug := func(format string, args ...interface{}) {
		// Only print debug information in debug mode
		if options.Debug {
			fmt.Printf("[DEBUG] "+format+"\n", args...)
		}
	}
	// Handle filtering first to get an accurate count for dry run
	var keysToDelete []string

	// If keys are provided, use them directly
	if len(keys) > 0 {
		keysToDelete = keys
		debug("Using provided keys: %d keys", len(keysToDelete))
	} else {
		// Otherwise check for filtering criteria:
		// 1. Explicit --all-keys flag
		// 2. Non-empty prefix filtering
		// 3. Empty prefix specified with --prefix ""
		// 4. Pattern-based filtering
		shouldListAllKeys := options.AllKeys || options.Prefix != "" || options.PrefixSpecified || options.Pattern != ""

		if shouldListAllKeys {
			debug("Finding keys with criteria: prefix='%s', pattern='%s', allKeys=%v",
				options.Prefix, options.Pattern, options.AllKeys)

			// Use existing pagination-aware function to list keys
			listOptions := &ListKeysOptions{
				Prefix: options.Prefix,
				// Pattern is handled separately, not directly in the listing API
			}

			allKeys, err := ListAllKeysWithOptions(s.client, accountID, namespaceID, listOptions, nil)
			if err != nil {
				return 0, fmt.Errorf("failed to list keys: %w", err)
			}

			verbose("Found %d keys matching criteria", len(allKeys))
			debug("Matched keys count: %d, proceeding with deletion", len(allKeys))

			// Extract key names
			keysToDelete = make([]string, len(allKeys))
			for i, key := range allKeys {
				keysToDelete[i] = key.Key
			}
		} else {
			verbose("No keys or filtering criteria provided")
			debug("Empty criteria, no keys to process")
		}
	}

	// If we have tag-based filtering or search, use the appropriate functions
	if options.TagField != "" || options.SearchValue != "" {
		verbose("Using advanced filtering with tag field '%s' or search value '%s'",
			options.TagField, options.SearchValue)
		debug("Starting advanced filtering process with field='%s', value='%s'",
			options.TagField, options.SearchValue)

		// If dry run, simulate count for advanced filtering
		if options.DryRun {
			verbose("Dry run, would process %d keys with advanced filtering", len(keysToDelete))
			debug("Dry run mode, skipping actual deletion for %d keys", len(keysToDelete))
			return len(keysToDelete), nil
		}

		return s.bulkDeleteWithAdvancedFilteringFixed(ctx, accountID, namespaceID, keys, options)
	}

	// If dry run, return the count without deleting
	if options.DryRun {
		verbose("Dry run, would delete %d keys", len(keysToDelete))
		debug("Dry run mode active, skipping actual deletion")
		return len(keysToDelete), nil
	}

	// If we have no keys to delete after all filtering, just return 0
	if len(keysToDelete) == 0 {
		verbose("No keys to delete after filtering")
		debug("Filter result: 0 keys matched criteria")
		return 0, nil
	}

	verbose("Deleting %d keys", len(keysToDelete))
	debug("Starting deletion process for %d keys", len(keysToDelete))

	// Define a progress callback for showing batch progress
	var progressCallback func(completed, total int)

	// Only create callback in verbose mode
	if options.Verbose {
		progressCallback = func(completed, total int) {
			percent := float64(completed) / float64(total) * 100
			verbose("Progress: %d/%d keys deleted (%.1f%%)", completed, total, percent)
			debug("Batch deletion progress: %d/%d (%.1f%%)", completed, total, percent)
		}
	}

	// Delete the collected keys
	if options.Concurrency > 0 {
		// Use concurrent deletion for better performance
		verbose("Using concurrent deletion with %d workers", options.Concurrency)
		debug("Initializing concurrent deletion with %d workers, batch size %d", options.Concurrency, options.BatchSize)
		successCount, errs := DeleteMultipleValuesConcurrently(s.client, accountID, namespaceID, keysToDelete, options.BatchSize, options.Concurrency, progressCallback)
		if len(errs) > 0 {
			return successCount, errs[0] // Return the first error encountered
		}
		return successCount, nil
	} else {
		// Fall back to sequential deletion
		verbose("Using sequential deletion")
		debug("Initializing sequential deletion with batch size %d", options.BatchSize)
		err := DeleteMultipleValuesInBatches(s.client, accountID, namespaceID, keysToDelete, options.BatchSize, progressCallback)
		if err != nil {
			return 0, err
		}
		return len(keysToDelete), nil
	}
}
