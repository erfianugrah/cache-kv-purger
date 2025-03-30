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
		verbose, _ := cmd.Flags().GetBool("verbose")

		// Validate required flags
		if (namespaceID == "" && namespace == "") || (searchValue == "" && tagField == "") {
			return fmt.Errorf("missing required flags: need --namespace-id or --namespace, and either --search or --tag-field")
		}

		if zone == "" || len(cacheTags) == 0 {
			return fmt.Errorf("missing required flags: need --zone and at least one --cache-tag")
		}

		// Load config for resolving account ID etc.
		cfg, err := config.LoadFromFile("")
		if err == nil && accountID == "" {
			accountID = cfg.GetAccountID()
		}

		if accountID == "" {
			return fmt.Errorf("account ID is required via --account-id flag, environment variable, or config file")
		}

		// Create API client
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Create KV service
		kvService := kv.NewKVService(client)

		// Resolve namespace ID if name provided
		if namespace != "" && namespaceID == "" {
			nsID, err := kvService.ResolveNamespaceID(cmd.Context(), accountID, namespace)
			if err != nil {
				return fmt.Errorf("failed to resolve namespace: %w", err)
			}
			namespaceID = nsID
		}

		// First, find matching KV keys
		fmt.Println("Step 1: Finding matching KV keys...")

		searchOptions := kv.SearchOptions{
			SearchValue:     searchValue,
			TagField:        tagField,
			TagValue:        tagValue,
			IncludeMetadata: true,
			BatchSize:       batchSize,
			Concurrency:     concurrency,
		}

		matchingKeys, err := kvService.Search(cmd.Context(), accountID, namespaceID, searchOptions)
		if err != nil {
			return fmt.Errorf("KV search failed: %w", err)
		}

		// Extract just the key names for deletion
		keyNames := make([]string, len(matchingKeys))
		for i, key := range matchingKeys {
			keyNames[i] = key.Key
		}

		// Report on keys found
		fmt.Printf("Found %d matching KV keys\n", len(keyNames))
		if verbose && len(keyNames) > 0 {
			sampleSize := 5
			if len(keyNames) < sampleSize {
				sampleSize = len(keyNames)
			}
			fmt.Println("Sample keys:")
			for i := 0; i < sampleSize; i++ {
				fmt.Printf("  - %s\n", keyNames[i])
			}
			if len(keyNames) > sampleSize {
				fmt.Printf("  - ... and %d more\n", len(keyNames)-sampleSize)
			}
		}

		// If no keys found, skip KV deletion but still purge cache
		kvDeleteNeeded := len(keyNames) > 0

		// Step 2: Delete matching KV keys (if any found and not dry run)
		if kvDeleteNeeded {
			fmt.Println("\nStep 2: Deleting matching KV keys...")
			if dryRun {
				fmt.Printf("DRY RUN: Would delete %d KV keys\n", len(keyNames))
			} else {
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
				fmt.Printf("Successfully deleted %d/%d KV keys\n", count, len(keyNames))
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
			_, err = cache.PurgeTags(client, zoneID, cacheTags)
			if err != nil {
				return fmt.Errorf("cache purge failed: %w", err)
			}
			fmt.Printf("Successfully purged %d cache tags: %s\n", len(cacheTags), strings.Join(cacheTags, ", "))
		}

		fmt.Println("\nSync operation completed successfully!")
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
	syncPurgeCmd.Flags().String("namespace-id", "", "Namespace ID to search in")
	syncPurgeCmd.Flags().String("namespace", "", "Namespace name (alternative to namespace-id)")
	
	// Search options
	syncPurgeCmd.Flags().String("search", "", "Search for keys containing this value (deep recursive metadata search)")
	syncPurgeCmd.Flags().String("tag-field", "", "Metadata field to filter by")
	syncPurgeCmd.Flags().String("tag-value", "", "Value to match in the tag field")
	
	// Cache options
	syncPurgeCmd.Flags().String("zone", "", "Zone ID or domain to purge cache from")
	syncPurgeCmd.Flags().StringSlice("cache-tag", []string{}, "Cache tags to purge (can specify multiple times)")
	
	// Operation options
	syncPurgeCmd.Flags().Bool("dry-run", false, "Show what would be done without making changes")
	syncPurgeCmd.Flags().Int("batch-size", 0, "Batch size for KV operations")
	syncPurgeCmd.Flags().Int("concurrency", 0, "Number of concurrent operations")

	// Mark required flags
	syncPurgeCmd.MarkFlagRequired("zone")
	syncPurgeCmd.MarkFlagRequired("cache-tag")
}