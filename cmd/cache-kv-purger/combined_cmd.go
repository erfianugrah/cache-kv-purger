package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"
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
	Example: `  # Purge KV keys with a specific search value and related cache tags 
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images

  # Use metadata field-specific search and purge cache
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --tag-field "type" --tag-value "temp" --zone example.com --cache-tag temp-data
  
  # Dry run to preview without making changes
  cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images --dry-run`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Validate inputs
		if (searchValue == "" && tagField == "") || (namespaceID == "" && namespace == "") {
			return fmt.Errorf("either search or tag-field, and either namespace-id or namespace are required")
		}

		if len(cacheTags) == 0 {
			return fmt.Errorf("at least one cache-tag is required")
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
		verbose, _ := cmd.Flags().GetBool("verbose")
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
			zoneID, err := common.ResolveZoneIdentifier(client, zone)
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
	},
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
	syncPurgeCmd.Flags().StringSlice("cache-tag", []string{}, "Cache tags to purge (can specify multiple times)")

	// Operation options
	syncPurgeCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncPurgeCmd.Flags().Int("batch-size", 0, "Batch size for KV operations")
	syncPurgeCmd.Flags().Int("concurrency", 0, "Number of concurrent operations")

	// Mark required flags
	syncPurgeCmd.MarkFlagRequired("zone")
	syncPurgeCmd.MarkFlagRequired("cache-tag")
}
