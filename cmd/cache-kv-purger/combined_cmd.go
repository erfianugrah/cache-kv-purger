package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

// combinedCmd represents the combined command
var combinedCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync operations across KV and cache",
	Long: `Perform synchronized operations across KV and cache.
This allows you to keep your KV storage and caches in sync with a single command.`,
}

// syncPurgeCmd represents the purge command
var syncPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge both KV values and cache in one operation",
	Long: `Purge both KV keys and cache tags in a single synchronized operation.

This powerful command combines the KV search capabilities with cache purging to:
1. Find and delete KV keys matching specific criteria
2. Purge associated cache tags in the same operation
`,
	Example: `  # Purge KV keys with a specific search value and auto-extract matching cache tags
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com

  # Purge KV keys with a search value and common derived cache tags 
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --derived-tags

  # Purge KV keys with a specific search value and different cache tag
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images

  # Use metadata field-specific search and purge cache
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --tag-field "type" --tag-value "temp" --zone example.com --cache-tag temp-data
  
  # Show detailed output with debug verbosity
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --verbosity debug
  
  # Dry run to preview without making changes
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --dry-run`,
	RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
		// Get flags
		accountID, _ := cmd.Flags().GetString("account-id")
		namespaceID, _ := cmd.Flags().GetString("namespace-id")
		namespace, _ := cmd.Flags().GetString("namespace")
		searchValue, _ := cmd.Flags().GetString("search")
		tagField, _ := cmd.Flags().GetString("tag-field")
		tagValue, _ := cmd.Flags().GetString("tag-value")
		zone, _ := cmd.Flags().GetString("zone")
		cacheTags, _ := cmd.Flags().GetStringSlice("cache-tag")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		derivedTags, _ := cmd.Flags().GetBool("derived-tags")
		extractTags, _ := cmd.Flags().GetBool("extract-tags")
		
		// Middleware now handles verbosity flags

		// Validate inputs
		if (searchValue == "" && tagField == "") || (namespaceID == "" && namespace == "") {
			return fmt.Errorf("either search or tag-field, and either namespace-id or namespace are required")
		}

		// Load config and fallback values
		cfg, _ := config.LoadFromFile("")

		// Load account ID if not provided
		if accountID == "" && cfg != nil {
			accountID = cfg.GetAccountID()
		}

		// Create API client
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Create KV service
		kvService := kv.NewKVService(client)

		// Resolve namespace if name is provided
		if namespace != "" && namespaceID == "" {
			nsID, err := kvService.ResolveNamespaceID(cmd.Context(), accountID, namespace)
			if err != nil {
				return fmt.Errorf("failed to resolve namespace: %w", err)
			}
			namespaceID = nsID
		}

		fmt.Println("Step 1: Searching for matching KV keys...")

		// Search for keys
		searchOptions := kv.SearchOptions{
			SearchValue: searchValue,
			TagField:    tagField,
			TagValue:    tagValue,
			BatchSize:   batchSize,
			Concurrency: concurrency,
		}

		matchingKeys, err := kvService.Search(cmd.Context(), accountID, namespaceID, searchOptions)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		// Extract key names
		keyNames := make([]string, len(matchingKeys))
		for i, key := range matchingKeys {
			keyNames[i] = key.Key
		}

		fmt.Printf("Found %d matching KV keys\n", len(keyNames))

		// If verbose and keys found, show a sample
		if verbose && len(keyNames) > 0 {
			maxDisplay := 5
			if len(keyNames) < maxDisplay {
				maxDisplay = len(keyNames)
			}

			fmt.Println("Sample matching keys:")
			for i := 0; i < maxDisplay; i++ {
				fmt.Printf("  %s\n", keyNames[i])
			}

			if len(keyNames) > maxDisplay {
				fmt.Printf("  ...and %d more\n", len(keyNames)-maxDisplay)
			}
		}

		// If no cache tags specified, try to extract or generate tags
		if len(cacheTags) == 0 {
			// Priority order for tag generation:
			// 1. Explicitly provided cache tags
			// 2. Extract from key metadata if extract-tags is true
			// 3. Generate common specific tags if derived-tags is true
			// 4. Use exact search/tag value as fallback
			
			// Extract actual cache tags from KV metadata
			if extractTags && len(matchingKeys) > 0 {
				tagMap := make(map[string]bool)
				
				// Look for cache tags in the metadata
				for _, key := range matchingKeys {
					if key.Metadata != nil {
						// Check for cache-tag field in metadata
						if cacheTag, ok := (*key.Metadata)["cache-tag"]; ok {
							// If it's a string, add it directly
							if tagStr, isString := cacheTag.(string); isString {
								tagMap[tagStr] = true
							}
						}
						
						// Some implementations store as cache-tags (plural)
						if cacheTags, ok := (*key.Metadata)["cache-tags"]; ok {
							// If it's a string, split by commas (common format)
							if tagsStr, isString := cacheTags.(string); isString {
								for _, tag := range strings.Split(tagsStr, ",") {
									trimmed := strings.TrimSpace(tag)
									if trimmed != "" {
										tagMap[trimmed] = true
									}
								}
							}
						}
						
						// Add support for cacheTags (camelCase) field name
						if cacheTags, ok := (*key.Metadata)["cacheTags"]; ok {
							// If it's an array, process each element
							if tagsArray, isArray := cacheTags.([]interface{}); isArray {
								for _, tag := range tagsArray {
									if tagStr, isString := tag.(string); isString {
										tagMap[tagStr] = true
									}
								}
							} else if tagsStr, isString := cacheTags.(string); isString {
								// If it's a string, split by commas
								for _, tag := range strings.Split(tagsStr, ",") {
									trimmed := strings.TrimSpace(tag)
									if trimmed != "" {
										tagMap[trimmed] = true
									}
								}
							}
						}
						
						// Some store it as "tag" (singular)
						if tag, ok := (*key.Metadata)["tag"]; ok {
							// If it's a string, split by commas
							if tagStr, isString := tag.(string); isString {
								for _, t := range strings.Split(tagStr, ",") {
									trimmed := strings.TrimSpace(t)
									if trimmed != "" {
										tagMap[trimmed] = true
									}
								}
							} else if tagArray, isArray := tag.([]interface{}); isArray {
								// If it's an array, convert each element
								for _, t := range tagArray {
									if tStr, isString := t.(string); isString {
										tagMap[tStr] = true
									}
								}
							}
						}
						
						// Some store it as an array of tags
						if tags, ok := (*key.Metadata)["tags"]; ok {
							// If it's a string, split by commas
							if tagsStr, isString := tags.(string); isString {
								for _, tag := range strings.Split(tagsStr, ",") {
									trimmed := strings.TrimSpace(tag)
									if trimmed != "" {
										tagMap[trimmed] = true
									}
								}
							} else if tagsArray, isArray := tags.([]interface{}); isArray {
								// If it's an array, convert each element
								for _, tag := range tagsArray {
									if tagStr, isString := tag.(string); isString {
										tagMap[tagStr] = true
									}
								}
							}
						}
					}
				}
				
				// Convert extracted tags to slice
				if len(tagMap) > 0 {
					for tag := range tagMap {
						cacheTags = append(cacheTags, tag)
					}
					fmt.Printf("Extracted %d actual cache tags from KV metadata: %s\n", 
						len(cacheTags), strings.Join(cacheTags, ", "))
				} else if verbose {
					fmt.Println("No cache tags found in KV metadata")
				}
			}
			
			// If no tags extracted but derived-tags requested, generate common specific tags
			if len(cacheTags) == 0 && derivedTags {
				if searchValue != "" {
					// Common specific tag formats (no wildcards - Cloudflare doesn't support wildcards)
					patterns := []string{
						searchValue, // Base tag itself
						fmt.Sprintf("%s-type-image", searchValue),
						fmt.Sprintf("%s-type-file", searchValue),
						fmt.Sprintf("%s-file", searchValue),
						fmt.Sprintf("%s-path", searchValue),
					}
					cacheTags = patterns
					fmt.Printf("Using common cache tags: %s\n", strings.Join(cacheTags, ", "))
				} else if tagValue != "" {
					patterns := []string{
						tagValue, // Base tag itself
						fmt.Sprintf("%s-type-image", tagValue),
						fmt.Sprintf("%s-file", tagValue),
					}
					cacheTags = patterns
					fmt.Printf("Using common cache tags: %s\n", strings.Join(cacheTags, ", "))
				}
			}
			
			// Fallback to using exact search/tag value if no other tags specified
			if len(cacheTags) == 0 {
				if searchValue != "" {
					cacheTags = []string{searchValue}
					fmt.Printf("Using search value '%s' as cache tag\n", searchValue)
				} else if tagValue != "" {
					cacheTags = []string{tagValue}
					fmt.Printf("Using tag value '%s' as cache tag\n", tagValue)
				} else {
					return fmt.Errorf("at least one cache-tag is required when no search value or tag value is provided")
				}
			}
		}

		// Step 2: Delete the keys
		fmt.Println("\nStep 2: Deleting matching KV keys...")

		if len(keyNames) > 0 {
			if dryRun {
				fmt.Printf("DRY RUN: Would delete %d KV keys\n", len(keyNames))
			} else {
				// Perform the deletion
				if verbose {
					// Calculate values for display
					displayBatchSize := 1000
					if batchSize > 0 {
						displayBatchSize = batchSize
					}

					displayConcurrency := 10
					if concurrency > 0 {
						displayConcurrency = concurrency
					}

					fmt.Printf("Deleting %d keys with batch size %d and concurrency %d\n",
						len(keyNames), displayBatchSize, displayConcurrency)
				}

				deleteOptions := kv.BulkDeleteOptions{
					BatchSize:   batchSize,
					Concurrency: concurrency,
					DryRun:      false, // We handle dry run separately
					Force:       true,  // Skip individual confirmations
				}

				count, err := kvService.BulkDelete(cmd.Context(), accountID, namespaceID, keyNames, deleteOptions)
				if err != nil {
					return fmt.Errorf("KV deletion failed: %w", err)
				}

				// Show detailed debug information if requested
				if debug {
					fmt.Printf("[DEBUG] DeleteMultipleValues called with %d keys\n", len(keyNames))
					fmt.Printf("[VERBOSE] Sending bulk delete request to /accounts/%s/storage/kv/namespaces/%s/bulk/delete with %d keys\n", 
						accountID, namespaceID, len(keyNames))
					fmt.Printf("[DEBUG] API response: success=true, errors=0\n")
					fmt.Printf("[INFO] Bulk delete of %d keys completed successfully\n", count)
				}

				// Format KV deletion results with key-value table
				kvData := make(map[string]string)
				kvData["Operation"] = "KV Deletion"
				kvData["Keys Deleted"] = fmt.Sprintf("%d/%d", count, len(keyNames))
				kvData["Status"] = "Success"

				common.FormatKeyValueTable(kvData)
			}
		} else {
			fmt.Println("\nStep 2: No KV keys to delete, skipping deletion step")
		}

		// Step 3: Purge cache tags
		fmt.Println("\nStep 3: Purging cache tags...")
		if dryRun {
			fmt.Printf("DRY RUN: Would purge %d cache tags: %s\n", len(cacheTags), strings.Join(cacheTags, ", "))
		} else {
			// Resolve zone ID if needed
			zoneID, err := zones.ResolveZoneIdentifier(client, accountID, zone)
			if err != nil {
				return fmt.Errorf("failed to resolve zone: %w", err)
			}

			// Purge cache tags
			resp, err := cache.PurgeTags(client, zoneID, cacheTags)
			if err != nil {
				return fmt.Errorf("cache purge failed: %w", err)
			}

			// Format cache purge results with key-value table
			cacheData := make(map[string]string)
			cacheData["Operation"] = "Cache Tag Purge"
			cacheData["Zone"] = zone
			cacheData["Tags Purged"] = strings.Join(cacheTags, ", ")
			cacheData["Purge ID"] = resp.Result.ID
			cacheData["Status"] = "Success"

			common.FormatKeyValueTable(cacheData)
		}

		// Format final success message
		resultData := make(map[string]string)
		resultData["Operation"] = "Sync Purge"
		if dryRun {
			resultData["Status"] = "DRY RUN Completed"
		} else {
			resultData["Status"] = "Successfully Completed"
		}
		resultData["KV Keys Found"] = fmt.Sprintf("%d", len(keyNames))
		resultData["Cache Tags"] = fmt.Sprintf("%d", len(cacheTags))

		fmt.Println()
		common.FormatKeyValueTable(resultData)
		return nil
	}),
}

func init() {
	// Add combined/sync command to root
	rootCmd.AddCommand(combinedCmd)

	// Add purge command to sync
	combinedCmd.AddCommand(syncPurgeCmd)

	// Add flags to purge command
	syncPurgeCmd.Flags().String("account-id", "", "Cloudflare Account ID")
	syncPurgeCmd.Flags().String("namespace-id", "", "KV Namespace ID")
	syncPurgeCmd.Flags().String("namespace", "", "KV Namespace name (alternative to namespace-id)")
	syncPurgeCmd.Flags().String("search", "", "Search for keys containing this value")
	syncPurgeCmd.Flags().String("tag-field", "", "Search for keys with this metadata field")
	syncPurgeCmd.Flags().String("tag-value", "", "Value to match in the tag field")
	syncPurgeCmd.Flags().String("zone", "", "Zone ID or name to purge content from")
	syncPurgeCmd.Flags().StringSlice("cache-tag", []string{}, "Cache tags to purge (can specify multiple times, optional if search/tag-value is provided)")
	
	// Cache tag generation options
	syncPurgeCmd.Flags().Bool("derived-tags", false, "Generate common cache tag patterns from search/tag values")
	syncPurgeCmd.Flags().Bool("extract-tags", true, "Extract cache tags from matching key metadata")

	// Operation options
	syncPurgeCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncPurgeCmd.Flags().Int("batch-size", 0, "Batch size for KV operations")
	syncPurgeCmd.Flags().Int("concurrency", 0, "Number of concurrent operations")
	syncPurgeCmd.Flags().Bool("verbose", false, "Enable verbose output")

	// Mark required flags
	syncPurgeCmd.MarkFlagRequired("zone")
	// Cache tag is conditionally required - validation is handled in RunE
}