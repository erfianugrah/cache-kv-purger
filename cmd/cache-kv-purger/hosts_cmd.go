package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

// createPurgeHostsCmd creates a command to purge cache by hostname
func createPurgeHostsCmd() *cobra.Command {
	// Define local variables for this command's flags
	var commaDelimitedHosts string
	var hostsFile string
	var autoZoneDetect bool
	var batchSize int
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "Purge cached content by hostname",
		Long:  `Purge cached content from Cloudflare's edge servers based on hostname.`,
		Example: `  # Purge a single host
  cache-kv-purger cache purge hosts --zone example.com --host images.example.com

  # Purge multiple hosts using individual flags
  cache-kv-purger cache purge hosts --zone example.com --host images.example.com --host api.example.com

  # Purge multiple hosts using comma-delimited list
  cache-kv-purger cache purge hosts --zone example.com --hosts "images.example.com,api.example.com,cdn.example.com"

  # Purge hosts from a text file (one host per line)
  cache-kv-purger cache purge hosts --zone example.com --hosts-file hosts.txt
  
  # Auto-detect zones based on hostnames (no need to specify zone)
  cache-kv-purger cache purge hosts --host images.example.com --host api.example2.com
  
  # Control batch size and concurrency
  cache-kv-purger cache purge hosts --zone example.com --hosts-file hosts.txt --batch-size 20 --concurrency 15 --zone-concurrency 5
  
  # Dry run (show what would be purged, but don't actually purge)
  cache-kv-purger cache purge hosts --zone example.com --hosts-file hosts.txt --dry-run`,
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

			// Collect all hosts from various input methods
			allHosts := make([]string, 0)

			// Add hosts from individual --host flags
			allHosts = append(allHosts, purgeFlagsVars.hosts...)

			// Add hosts from comma-delimited string if provided
			if commaDelimitedHosts != "" {
				// Split by comma and process each host
				for _, host := range strings.Split(commaDelimitedHosts, ",") {
					// Trim whitespace
					host = strings.TrimSpace(host)
					if host != "" {
						allHosts = append(allHosts, host)
					}
				}

				if verbose {
					fmt.Printf("Added %d hosts from comma-delimited list\n", len(strings.Split(commaDelimitedHosts, ",")))
				}
			}

			// Add hosts from file if specified
			if hostsFile != "" {
				// Read file
				data, err := os.ReadFile(hostsFile)
				if err != nil {
					return fmt.Errorf("failed to read hosts file: %w", err)
				}

				// Process file as text format (one host per line)
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					// Trim whitespace and skip empty lines or comments
					host := strings.TrimSpace(line)
					if host != "" && !strings.HasPrefix(host, "#") {
						allHosts = append(allHosts, host)
					}
				}

				if verbose {
					fmt.Printf("Extracted %d hosts from %s\n", len(allHosts)-len(purgeFlagsVars.hosts), hostsFile)
				}
			}

			// Remove duplicate hosts
			allHosts = removeDuplicates(allHosts)

			// Check if we have any hosts
			if len(allHosts) == 0 {
				return fmt.Errorf("at least one host is required, specify with --host, --hosts, or --hosts-file")
			}

			// Get concurrency settings
			cacheConcurrency := purgeFlagsVars.cacheConcurrency
			multiZoneConcurrency := purgeFlagsVars.multiZoneConcurrency

			// If not set from command line, get from config
			if cacheConcurrency <= 0 && cfg != nil {
				cacheConcurrency = cfg.GetCacheConcurrency()
			}

			if multiZoneConcurrency <= 0 && cfg != nil {
				multiZoneConcurrency = cfg.GetMultiZoneConcurrency()
			}

			// If no specific zone is provided, try auto-detection
			if len(purgeFlagsVars.zones) == 0 && purgeFlagsVars.zoneID == "" && cmd.Flags().Lookup("zone").Value.String() == "" {
				// No zone specified, so try to auto-detect zones from hosts
				// Pass concurrency settings to the handler
				return handleAutoZoneDetectionForHosts(client, accountID, allHosts, cmd, cacheConcurrency, multiZoneConcurrency)
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

			// If only a few hosts, don't bother with batching
			if len(allHosts) <= batchSize {
				// Simple case - small number of hosts, just do it directly
				if verbose {
					fmt.Printf("Purging content for %d hosts in zone %s...\n", len(allHosts), resolvedZoneID)
					for i, host := range allHosts {
						fmt.Printf("  %d. %s\n", i+1, host)
					}
				}

				// Dry run mode
				if dryRun {
					fmt.Printf("DRY RUN: Would purge %d hosts\n", len(allHosts))
					return nil
				}

				resp, err := cache.PurgeHosts(client, resolvedZoneID, allHosts)
				if err != nil {
					return fmt.Errorf("failed to purge hosts: %w", err)
				}

				fmt.Printf("Successfully purged content for %d hosts. Purge ID: %s\n", len(allHosts), resp.Result.ID)
				return nil
			}

			// For larger numbers, use batching with concurrency
			// Cap concurrency for Enterprise tier
			if cacheConcurrency <= 0 {
				cacheConcurrency = 50 // Enterprise tier default
			} else if cacheConcurrency > 50 {
				cacheConcurrency = 50 // Enterprise tier allows 50 requests per second
			}

			// Split hosts into batches (for preview in dry run mode)
			batches := splitIntoBatches(allHosts, batchSize)

			if verbose {
				fmt.Printf("Preparing to purge %d hosts in %d batches using %d concurrent workers\n",
					len(allHosts), len(batches), cacheConcurrency)
			}

			// Dry run mode
			if dryRun {
				fmt.Printf("DRY RUN: Would purge %d hosts in %d batches (batch size: %d)\n", len(allHosts), len(batches), batchSize)
				for i, batch := range batches {
					fmt.Printf("Batch %d: %d hosts\n", i+1, len(batch))
					if verbose {
						for j, host := range batch {
							fmt.Printf("  %d. %s\n", j+1, host)
						}
					}
				}
				return nil
			}

			// Create progress function
			progressFn := func(completed, total, successful int) {
				if verbose {
					fmt.Printf("Progress: processed %d/%d batches, %d hosts purged\n",
						completed, total, successful)
				} else {
					fmt.Printf("Processing batch %d/%d: %d hosts purged so far...  \r",
						completed, total, successful)
				}
			}

			// Process hosts with concurrent batching
			successful, errors := cache.PurgeHostsInBatches(client, resolvedZoneID, allHosts, progressFn, cacheConcurrency)

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
			fmt.Printf("Completed: Successfully purged %d hosts\n", len(successful))
			return nil
		}),
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.hosts, "host", []string{}, "Hostname to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedHosts, "hosts", "", "Comma-delimited list of hostnames to purge (e.g., \"host1.com,host2.com,host3.com\")")
	cmd.Flags().StringVar(&hostsFile, "hosts-file", "", "Path to a text file containing hostnames to purge (one host per line)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Maximum number of hosts to purge in each batch (API limit: 100 items per request)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be purged without actually purging")
	cmd.Flags().BoolVar(&autoZoneDetect, "auto-zone", false, "Auto-detect zones based on hostnames")

	return cmd
}
