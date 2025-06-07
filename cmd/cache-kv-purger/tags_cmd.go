package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

// createPurgeTagsCmd creates a command to purge cache by tag
func createPurgeTagsCmd() *cobra.Command {
	// Define local variables for this command's flags
	var commaDelimitedTags string
	var tagsFile string
	var batchSize int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Purge cached content by tags",
		Long:  `Purge cached content from Cloudflare's edge servers based on cache tags.`,
		Example: `  # Purge a single tag
  cache-kv-purger cache purge tags --zone example.com --tag video-resizer

  # Purge multiple tags
  cache-kv-purger cache purge tags --zone example.com --tag product-images --tag category-pages

  # Purge tags from a comma-delimited list
  cache-kv-purger cache purge tags --zone example.com --tags "tag1,tag2,tag3,tag4" 

  # Purge tags with batch control (API limit: 100 tags per request)
  cache-kv-purger cache purge tags --zone example.com --tags "tag1,tag2,tag3,tag4" --batch-size 50

  # Purge tags from a file (CSV, JSON, or text with one tag per line)
  cache-kv-purger cache purge tags --zone example.com --tags-file tags.csv 
  
  # Dry run (show what would be purged, but don't actually purge)
  cache-kv-purger cache purge tags --zone example.com --tags-file tags.csv --dry-run`,
		RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
			// Middleware now handles verbose flags

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Get account ID for resolving zone names
			accountID := ""
			cfg, err := config.LoadFromFile("")
			if err == nil {
				accountID = cfg.GetAccountID()
			}

			// Collect all tags from various input methods
			allTags := make([]string, 0)

			// Add tags from individual --tag flags
			allTags = append(allTags, purgeFlagsVars.tags...)

			// Add tags from comma-delimited string if provided
			if commaDelimitedTags != "" {
				// Split by comma and process each tag
				for _, tag := range strings.Split(commaDelimitedTags, ",") {
					// Trim whitespace
					tag = strings.TrimSpace(tag)
					if tag != "" {
						allTags = append(allTags, tag)
					}
				}

				if verbose {
					fmt.Printf("Added %d tags from comma-delimited list\n", len(strings.Split(commaDelimitedTags, ",")))
				}
			}

			// Add tags from file if specified
			if tagsFile != "" {
				// Use the centralized file reading utility
				tagsFromFile, err := common.ReadItemsFromFile(tagsFile, "", nil)
				if err != nil {
					return fmt.Errorf("failed to read tags file: %w", err)
				}

				allTags = append(allTags, tagsFromFile...)

				if verbose {
					fmt.Printf("Added %d tags from file\n", len(tagsFromFile))
				}
			}

			// Remove duplicate tags
			allTags = common.RemoveDuplicates(allTags)

			// Check if we have any tags
			if len(allTags) == 0 {
				return fmt.Errorf("at least one tag is required, specify with --tag, --tags, or --tags-file")
			}

			if verbose {
				fmt.Printf("Prepared to purge %d unique tags\n", len(allTags))
			}

			// Get the zone ID from flag, config, or environment variable
			zoneID := purgeFlagsVars.zoneID
			if zoneID == "" {
				// Try to get from the global flag
				zoneID, _ = cmd.Flags().GetString("zone")
			}

			if zoneID == "" {
				// Try to get from config or environment variable
				if cfg != nil {
					zoneID = cfg.GetZoneID()
				}
			}

			if zoneID == "" {
				return fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
			}

			// Resolve zone (could be name or ID)
			resolvedZoneID, err := zones.ResolveZoneIdentifier(client, accountID, zoneID)
			if err != nil {
				return fmt.Errorf("failed to resolve zone: %w", err)
			}

			// Default batch size if not specified or invalid
			if batchSize <= 0 {
				batchSize = 100 // API has a limit of 100 items per purge request
			}

			// Ensure batch size is at most 100 (API limit)
			if batchSize > 100 {
				if verbose {
					fmt.Println("Warning: Reducing batch size to 100 (Cloudflare API limit)")
				}
				batchSize = 100
			}

			// If only a few tags, don't bother with batching
			if len(allTags) <= batchSize {
				// Simple case - small number of tags, just do it directly
				if verbose {
					fmt.Printf("Purging content with %d tags for zone %s...\n", len(allTags), resolvedZoneID)
					for i, tag := range allTags {
						fmt.Printf("  %d. %s\n", i+1, tag)
					}
				}

				// Dry run mode
				if dryRun {
					dryRunOpts := common.DryRunOptions{
						Enabled:    true,
						Verbose:    verbose,
						ItemType:   "tags",
						ActionVerb: "purge",
						BatchSize:  batchSize,
					}

					// Use the standardized dry run handler
					if !common.HandleDryRun(dryRunOpts, allTags, nil) {
						return nil
					}
				}

				// Confirm before purging, unless force is enabled
				if purgeFlagsVars.force || common.ConfirmBatchOperation(len(allTags), "tags", "purge", purgeFlagsVars.force) {
					resp, err := cache.PurgeTags(client, resolvedZoneID, allTags)
					if err != nil {
						return fmt.Errorf("failed to purge tags: %w", err)
					}

					fmt.Printf("Successfully purged content with %d tags. Purge ID: %s\n", len(allTags), resp.Result.ID)
				} else {
					fmt.Println("Operation cancelled.")
					return nil
				}
				return nil
			}

			// For larger numbers, use batching with concurrency
			// Get concurrency settings for batch processing
			concurrency := purgeFlagsVars.cacheConcurrency
			if concurrency <= 0 && cfg != nil {
				concurrency = cfg.GetCacheConcurrency()
			}

			// Cap concurrency for Enterprise tier
			if concurrency <= 0 {
				concurrency = 50 // Default
			} else if concurrency > 50 {
				concurrency = 50 // Enterprise tier allows 50 requests per second
			}

			// Split tags into batches (for preview in dry run mode)
			batches := common.SplitIntoBatches(allTags, batchSize)

			if verbose {
				fmt.Printf("Preparing to purge %d tags in %d batches using %d concurrent workers\n",
					len(allTags), len(batches), concurrency)
			}

			// Dry run mode
			if dryRun {
				dryRunOpts := common.DryRunOptions{
					Enabled:    true,
					Verbose:    verbose,
					ItemType:   "tags",
					ActionVerb: "purge",
					BatchSize:  batchSize,
				}

				// Use the standardized dry run handler with sample display
				if !common.HandleDryRunWithSample(dryRunOpts, allTags, batches) {
					return nil
				}
			}

			// Create progress function
			progressFn := func(completed, total, successful int) {
				if verbose {
					fmt.Printf("Progress: processed %d/%d batches, %d tags purged\n",
						completed, total, successful)
				} else {
					fmt.Printf("Processing batch %d/%d: %d tags purged so far...  \r",
						completed, total, successful)
				}
			}

			// Confirm the operation unless force is enabled
			if !purgeFlagsVars.force && !common.ConfirmBatchOperation(len(allTags), "tags", "purge", purgeFlagsVars.force) {
				fmt.Println("Operation cancelled.")
				return nil
			}

			// Process tags with concurrent batching
			successful, errors := cache.PurgeTagsInBatches(client, resolvedZoneID, allTags, progressFn, concurrency)

			// Print a newline to clear the progress line
			if !verbose {
				fmt.Println()
			}

			// Report errors if any
			if len(errors) > 0 {
				fmt.Printf("Encountered %d errors during purging:\n", len(errors))
				for i, err := range errors {
					if i < 5 { // Show at most 5 errors to avoid flooding the console
						fmt.Printf("  - %s\n", err)
					} else {
						fmt.Printf("  - ... and %d more errors\n", len(errors)-5)
						break
					}
				}
			}

			// Final summary
			fmt.Printf("Completed: Successfully purged %d tags\n", len(successful))
			return nil
		}),
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.tags, "tag", []string{}, "Cache tag to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedTags, "tags", "", "Comma-delimited list of cache tags to purge (e.g., \"tag1,tag2,tag3\")")
	cmd.Flags().StringVar(&tagsFile, "tags-file", "", "Path to a file containing cache tags to purge (CSV, JSON, or text with one tag per line)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Maximum number of tags to purge in each batch (API limit: 100 tags per request)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be purged without actually purging")
	cmd.Flags().BoolVar(&purgeFlagsVars.force, "force", false, "Skip confirmation prompt")

	return cmd
}
