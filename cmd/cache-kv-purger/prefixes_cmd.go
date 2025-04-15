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
	"os"
	"strings"
)

// createPurgePrefixesCmd creates a command to purge cache by URL prefix
func createPurgePrefixesCmd() *cobra.Command {
	// Define local variables for this command's flags
	var commaDelimitedPrefixes string
	var prefixesFile string
	var batchSize int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "prefixes",
		Short: "Purge cached content by URL prefix",
		Long:  `Purge cached content from Cloudflare's edge servers based on URL prefixes.`,
		Example: `  # Purge a single prefix
  cache-kv-purger cache purge prefixes --zone example.com --prefix https://example.com/blog/

  # Purge multiple prefixes
  cache-kv-purger cache purge prefixes --zone example.com --prefix https://example.com/blog/ --prefix https://example.com/products/
  
  # Purge prefixes from a comma-delimited list
  cache-kv-purger cache purge prefixes --zone example.com --prefixes "https://example.com/blog/,https://example.com/products/" 
  
  # Purge prefixes with batch control (max 30 prefixes per API call)
  cache-kv-purger cache purge prefixes --zone example.com --prefixes-file prefixes.txt --batch-size 10
  
  # Dry run (show what would be purged, but don't actually purge)
  cache-kv-purger cache purge prefixes --zone example.com --prefixes-file prefixes.txt --dry-run`,
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

			// Collect all prefixes from various input methods
			allPrefixes := make([]string, 0)

			// Add prefixes from individual --prefix flags
			allPrefixes = append(allPrefixes, purgeFlagsVars.prefixes...)

			// Add prefixes from comma-delimited string if provided
			if commaDelimitedPrefixes != "" {
				// Split by comma and process each prefix
				for _, prefix := range strings.Split(commaDelimitedPrefixes, ",") {
					// Trim whitespace
					prefix = strings.TrimSpace(prefix)
					if prefix != "" {
						allPrefixes = append(allPrefixes, prefix)
					}
				}

				if verbose {
					fmt.Printf("Added %d prefixes from comma-delimited list\n", len(strings.Split(commaDelimitedPrefixes, ",")))
				}
			}

			// Add prefixes from file if specified
			if prefixesFile != "" {
				// Read file
				data, err := os.ReadFile(prefixesFile)
				if err != nil {
					return fmt.Errorf("failed to read prefixes file: %w", err)
				}

				// Process file as text format (one prefix per line)
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					// Trim whitespace and skip empty lines or comments
					prefix := strings.TrimSpace(line)
					if prefix != "" && !strings.HasPrefix(prefix, "#") {
						allPrefixes = append(allPrefixes, prefix)
					}
				}

				if verbose {
					fmt.Printf("Extracted %d prefixes from %s\n", len(allPrefixes)-len(purgeFlagsVars.prefixes), prefixesFile)
				}
			}

			// Remove duplicate prefixes
			allPrefixes = removeDuplicates(allPrefixes)

			// Verify we have prefixes
			if len(allPrefixes) == 0 {
				return fmt.Errorf("at least one prefix is required, specify with --prefix, --prefixes, or --prefixes-file")
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

			// If only a few prefixes, don't bother with batching
			if len(allPrefixes) <= batchSize {
				// Simple case - small number of prefixes, just do it directly
				if verbose {
					fmt.Printf("Purging content with %d prefixes for zone %s...\n", len(allPrefixes), resolvedZoneID)
					for i, prefix := range allPrefixes {
						fmt.Printf("  %d. %s\n", i+1, prefix)
					}
				}

				// Dry run mode
				if dryRun {
					fmt.Printf("DRY RUN: Would purge %d prefixes\n", len(allPrefixes))
					return nil
				}

				resp, err := cache.PurgePrefixes(client, resolvedZoneID, allPrefixes)
				if err != nil {
					return fmt.Errorf("failed to purge prefixes: %w", err)
				}

				// Format success with key-value table
				data := make(map[string]string)
				data["Operation"] = "Purge Prefixes"
				data["Zone"] = resolvedZoneID
				data["Prefixes Purged"] = fmt.Sprintf("%d", len(allPrefixes))
				data["Purge ID"] = resp.Result.ID
				data["Status"] = "Success"

				common.FormatKeyValueTable(data)
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
				concurrency = 50 // Enterprise tier default
			} else if concurrency > 50 {
				concurrency = 50 // Enterprise tier allows 50 requests per second
			}

			// Split prefixes into batches (for preview in dry run mode)
			batches := splitIntoBatches(allPrefixes, batchSize)

			if verbose {
				fmt.Printf("Preparing to purge %d prefixes in %d batches using %d concurrent workers\n",
					len(allPrefixes), len(batches), concurrency)
			}

			// Dry run mode
			if dryRun {
				fmt.Printf("DRY RUN: Would purge %d prefixes in %d batches (batch size: %d)\n", len(allPrefixes), len(batches), batchSize)
				for i, batch := range batches {
					fmt.Printf("Batch %d: %d prefixes\n", i+1, len(batch))
					if verbose {
						for j, prefix := range batch {
							fmt.Printf("  %d. %s\n", j+1, prefix)
						}
					}
				}
				return nil
			}

			// Create progress function
			progressFn := func(completed, total, successful int) {
				if verbose {
					fmt.Printf("Progress: processed %d/%d batches, %d prefixes purged\n",
						completed, total, successful)
				} else {
					fmt.Printf("Processing batch %d/%d: %d prefixes purged so far...  \r",
						completed, total, successful)
				}
			}

			// Process prefixes with concurrent batching
			successful, errors := cache.PurgePrefixesInBatches(client, resolvedZoneID, allPrefixes, progressFn, concurrency)

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
			fmt.Printf("Completed: Successfully purged %d prefixes\n", len(successful))
			return nil
		}),
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.prefixes, "prefix", []string{}, "URL prefix to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedPrefixes, "prefixes", "", "Comma-delimited list of URL prefixes to purge")
	cmd.Flags().StringVar(&prefixesFile, "prefixes-file", "", "Path to a text file containing URL prefixes to purge (one prefix per line)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Maximum number of prefixes to purge in each batch (API limit: 100 items per request)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be purged without actually purging")

	return cmd
}
