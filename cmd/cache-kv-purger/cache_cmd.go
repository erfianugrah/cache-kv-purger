package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// CacheCmd is the command for cache operations
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage Cloudflare cache",
	Long:  `Perform operations on the Cloudflare cache, such as purging files, tags, and more.`,
}

// PurgeCmd is the command for purging cache
var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge Cloudflare cache",
	Long:  `Purge cached content from Cloudflare's edge servers.`,
}

// purgeFlagsVars stores the variables for the purge command flags
var purgeFlagsVars struct {
	zoneID               string
	zones                []string // Support for multiple zones with --zones (repeated flag)
	zonesCSV             []string // Support for multiple zones with --zone-list (comma separated)
	purgeEverything      bool
	files                []string
	tags                 []string
	hosts                []string
	prefixes             []string
	cacheConcurrency     int // Concurrency for cache operations
	multiZoneConcurrency int // Concurrency for multi-zone operations
}

// createPurgeEverythingCmd creates a command to purge everything
func createPurgeEverythingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "everything",
		Short: "Purge everything from cache",
		Long:  `Purge all cached files for a zone from Cloudflare's edge servers.`,
		Example: `  # Purge everything from a zone
  cache-kv-purger cache purge everything --zone example.com

  # Purge everything from multiple zones
  cache-kv-purger cache purge everything --zone example.com --zone example.org

  # Purge everything from all zones in an account
  cache-kv-purger cache purge everything --all-zones`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Resolve zone identifiers (could be names or IDs)
			resolvedZoneIDs, err := resolveZoneIdentifiers(cmd, client, accountID)
			if err != nil {
				return err
			}

			// Track successes
			successCount := 0
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Purge everything for each zone
			for _, zoneID := range resolvedZoneIDs {
				if verbose {
					// Try to get the zone name for more informative output
					zoneInfo, err := getZoneInfo(client, zoneID)
					zoneName := zoneID
					if err == nil && zoneInfo.Result.Name != "" {
						zoneName = zoneInfo.Result.Name
					}
					fmt.Printf("Purging everything from zone %s...\n", zoneName)
				}

				// Make the API call to purge everything
				resp, err := cache.PurgeEverything(client, zoneID)
				if err != nil {
					fmt.Printf("Error purging zone %s: %s\n", zoneID, err)
					continue
				}

				// Report success
				if verbose {
					fmt.Printf("Successfully purged everything from zone %s. Purge ID: %s\n", zoneID, resp.Result.ID)
				}
				successCount++
			}

			// Final summary
			fmt.Printf("Successfully purged content from %d/%d zones\n", successCount, len(resolvedZoneIDs))
			return nil
		},
	}

	return cmd
}

// createPurgeFilesCmd creates a command to purge specific files
func createPurgeFilesCmd() *cobra.Command {
	// Define local variables for this command's flags
	var commaDelimitedFiles string
	var filesListPath string
	var autoZoneDetect bool

	cmd := &cobra.Command{
		Use:   "files",
		Short: "Purge specific files from cache",
		Long:  `Purge specific files from Cloudflare's edge servers.`,
		Example: `  # Purge a single file
  cache-kv-purger cache purge files --zone example.com --file https://example.com/image.jpg

  # Purge multiple files using individual flags
  cache-kv-purger cache purge files --zone example.com --file https://example.com/image.jpg --file https://example.com/script.js

  # Purge multiple files using comma-delimited list
  cache-kv-purger cache purge files --zone example.com --files "https://example.com/image.jpg,https://example.com/script.js"

  # Purge files from a text file (one URL per line)
  cache-kv-purger cache purge files --zone example.com --files-list urls.txt
  
  # Auto-detect zones based on URLs (no need to specify zone)
  cache-kv-purger cache purge files --file https://example.com/image.jpg --file https://example2.com/script.js`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get verbose flag once at the beginning
			verbose, _ := cmd.Flags().GetBool("verbose")

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

			// Collect all files from various input methods
			allFiles := make([]string, 0)

			// Add files from individual --file flags
			allFiles = append(allFiles, purgeFlagsVars.files...)

			// Add files from comma-delimited string if provided
			if commaDelimitedFiles != "" {
				// Split by comma and process each file
				for _, file := range strings.Split(commaDelimitedFiles, ",") {
					// Trim whitespace
					file = strings.TrimSpace(file)
					if file != "" {
						allFiles = append(allFiles, file)
					}
				}

				if verbose {
					fmt.Printf("Added %d files from comma-delimited list\n", len(strings.Split(commaDelimitedFiles, ",")))
				}
			}

			// Add files from list file if specified
			if filesListPath != "" {
				// Read file
				data, err := os.ReadFile(filesListPath)
				if err != nil {
					return fmt.Errorf("failed to read files list: %w", err)
				}

				// Process file as text format (one URL per line)
				lines := strings.Split(string(data), "\n")
				for _, line := range lines {
					// Trim whitespace and skip empty lines or comments
					file := strings.TrimSpace(line)
					if file != "" && !strings.HasPrefix(file, "#") {
						allFiles = append(allFiles, file)
					}
				}

				if verbose {
					fmt.Printf("Extracted %d files from %s\n", len(allFiles)-len(purgeFlagsVars.files), filesListPath)
				}
			}

			// Check if we have any files
			if len(allFiles) == 0 {
				return fmt.Errorf("at least one file is required, specify with --file, --files, or --files-list")
			}

			// Verify all files are valid URLs
			for i, file := range allFiles {
				if !strings.HasPrefix(file, "http://") && !strings.HasPrefix(file, "https://") {
					// Auto-add https:// if missing
					allFiles[i] = "https://" + file
				}

				// Validate URL format
				_, err := url.Parse(allFiles[i])
				if err != nil {
					return fmt.Errorf("invalid URL format for file #%d (%s): %w", i+1, allFiles[i], err)
				}
			}

			// If no specific zone is provided, try auto-detection
			if len(purgeFlagsVars.zones) == 0 && purgeFlagsVars.zoneID == "" && cmd.Flags().Lookup("zone").Value.String() == "" {
				// No zone specified, so try to auto-detect zones from URLs
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

				return handleAutoZoneDetectionForFiles(client, accountID, allFiles, cmd, cacheConcurrency, multiZoneConcurrency)
			}

			// Resolve zone identifiers (could be names or IDs)
			resolvedZoneIDs, err := resolveZoneIdentifiers(cmd, client, accountID)
			if err != nil {
				return err
			}

			// Check if we're purging files across multiple zones
			if len(resolvedZoneIDs) > 1 {
				// Group files by zone
				return handleMultiZoneFilePurge(client, resolvedZoneIDs, allFiles, cmd)
			}

			// Single zone case
			zoneID := resolvedZoneIDs[0]

			// Check which files actually belong to this zone
			validFiles := make([]string, 0)
			for _, file := range allFiles {
				// Parse URL just to validate format
				_, err := url.Parse(file)
				if err != nil {
					fmt.Printf("Warning: Skipping invalid URL: %s\n", file)
					continue
				}

				// Add file if no hostname check needed or if it belongs to this zone
				validFiles = append(validFiles, file)
			}

			// Check if we have any valid files
			if len(validFiles) == 0 {
				return fmt.Errorf("no valid files found for zone %s", zoneID)
			}

			// Track successes
			successCount := 0

			// Purge files for each zone
			for _, zoneID := range resolvedZoneIDs {
				if verbose {
					fmt.Printf("Purging %d files for zone %s...\n", len(validFiles), zoneID)
					for i, file := range validFiles {
						fmt.Printf("  %d. %s\n", i+1, file)
					}
				}

				// Make the API call to purge files
				resp, err := cache.PurgeFiles(client, zoneID, validFiles)
				if err != nil {
					fmt.Printf("Error purging files for zone %s: %s\n", zoneID, err)
					continue
				}

				// Report success
				if verbose {
					fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(validFiles), zoneID, resp.Result.ID)
				}
				successCount++
			}

			// Final summary
			fmt.Printf("Successfully purged %d files\n", len(validFiles))

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.files, "file", []string{}, "File URL to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedFiles, "files", "", "Comma-delimited list of file URLs to purge (e.g., \"https://example.com/image.jpg,https://example.com/script.js\")")
	cmd.Flags().StringVar(&filesListPath, "files-list", "", "Path to a text file containing file URLs to purge (one URL per line)")
	cmd.Flags().BoolVar(&autoZoneDetect, "auto-zone", false, "Auto-detect zones based on file URLs")

	return cmd
}

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

  # Purge tags with batch control (max 30 tags per API call)
  cache-kv-purger cache purge tags --zone example.com --tags "tag1,tag2,tag3,tag4" --batch-size 2

  # Purge tags from a file (CSV, JSON, or text with one tag per line)
  cache-kv-purger cache purge tags --zone example.com --tags-file tags.csv 
  
  # Dry run (show what would be purged, but don't actually purge)
  cache-kv-purger cache purge tags --zone example.com --tags-file tags.csv --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get verbose flag once at the beginning
			verbose, _ := cmd.Flags().GetBool("verbose")

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
				// Determine file extension
				fileExt := strings.ToLower(filepath.Ext(tagsFile))

				switch fileExt {
				case ".json":
					// Read JSON file
					data, err := os.ReadFile(tagsFile)
					if err != nil {
						return fmt.Errorf("failed to read tags file: %w", err)
					}

					// Parse JSON data
					var tags []string
					err = json.Unmarshal(data, &tags)
					if err != nil {
						return fmt.Errorf("failed to parse JSON tags file: %w", err)
					}

					// Add tags
					for _, tag := range tags {
						tag = strings.TrimSpace(tag)
						if tag != "" {
							allTags = append(allTags, tag)
						}
					}

					if verbose {
						fmt.Printf("Added %d tags from JSON file\n", len(tags))
					}

				case ".csv":
					// Read CSV file
					file, err := os.Open(tagsFile)
					if err != nil {
						return fmt.Errorf("failed to open CSV file: %w", err)
					}
					defer file.Close()

					// Create CSV reader
					reader := csv.NewReader(file)
					records, err := reader.ReadAll()
					if err != nil {
						return fmt.Errorf("failed to parse CSV file: %w", err)
					}

					// Process each record
					tagCount := 0
					for _, record := range records {
						for _, field := range record {
							tag := strings.TrimSpace(field)
							if tag != "" && !strings.HasPrefix(tag, "#") {
								allTags = append(allTags, tag)
								tagCount++
							}
						}
					}

					if verbose {
						fmt.Printf("Added %d tags from CSV file\n", tagCount)
					}

				default:
					// Treat as text file (one tag per line)
					data, err := os.ReadFile(tagsFile)
					if err != nil {
						return fmt.Errorf("failed to read tags file: %w", err)
					}

					// Process each line
					lines := strings.Split(string(data), "\n")
					tagCount := 0
					for _, line := range lines {
						tag := strings.TrimSpace(line)
						if tag != "" && !strings.HasPrefix(tag, "#") {
							allTags = append(allTags, tag)
							tagCount++
						}
					}

					if verbose {
						fmt.Printf("Added %d tags from text file\n", tagCount)
					}
				}
			}

			// Remove duplicate tags
			allTags = removeDuplicates(allTags)

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
				batchSize = 30 // Cloudflare's default limit is 30
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
					fmt.Printf("DRY RUN: Would purge %d tags\n", len(allTags))
					return nil
				}

				resp, err := cache.PurgeTags(client, resolvedZoneID, allTags)
				if err != nil {
					return fmt.Errorf("failed to purge tags: %w", err)
				}

				fmt.Printf("Successfully purged content with %d tags. Purge ID: %s\n", len(allTags), resp.Result.ID)
				return nil
			}

			// For larger numbers, use batching with concurrency
			// Get concurrency settings for batch processing  
			concurrency := purgeFlagsVars.cacheConcurrency
			if concurrency <= 0 && cfg != nil {
				concurrency = cfg.GetCacheConcurrency()
			}

			// Cap concurrency to reasonable limits
			if concurrency <= 0 {
				concurrency = 10 // Default
			} else if concurrency > 20 {
				concurrency = 20 // Max
			}

			// Split tags into batches (for preview in dry run mode)
			batches := splitIntoBatches(allTags, batchSize)

			if verbose {
				fmt.Printf("Preparing to purge %d tags in %d batches using %d concurrent workers\n", 
					len(allTags), len(batches), concurrency)
			}

			// Dry run mode
			if dryRun {
				fmt.Printf("DRY RUN: Would purge %d tags in %d batches (batch size: %d)\n", len(allTags), len(batches), batchSize)
				for i, batch := range batches {
					fmt.Printf("Batch %d: %d tags\n", i+1, len(batch))
					if verbose {
						for j, tag := range batch {
							fmt.Printf("  %d. %s\n", j+1, tag)
						}
					}
				}
				return nil
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
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.tags, "tag", []string{}, "Cache tag to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedTags, "tags", "", "Comma-delimited list of cache tags to purge (e.g., \"tag1,tag2,tag3\")")
	cmd.Flags().StringVar(&tagsFile, "tags-file", "", "Path to a file containing cache tags to purge (CSV, JSON, or text with one tag per line)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 30, "Maximum number of tags to purge in each batch (default 30)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be purged without actually purging")

	return cmd
}

// createPurgePrefixesCmd creates a command to purge cache by URL prefix
func createPurgePrefixesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prefixes",
		Short: "Purge cached content by URL prefix",
		Long:  `Purge cached content from Cloudflare's edge servers based on URL prefixes.`,
		Example: `  # Purge a single prefix
  cache-kv-purger cache purge prefixes --zone example.com --prefix https://example.com/blog/

  # Purge multiple prefixes
  cache-kv-purger cache purge prefixes --zone example.com --prefix https://example.com/blog/ --prefix https://example.com/products/`,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// Verify prefix flags
			if len(purgeFlagsVars.prefixes) == 0 {
				return fmt.Errorf("at least one prefix is required, specify with --prefix")
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

			// Purge prefixes
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Purging content with %d prefixes for zone %s...\n", len(purgeFlagsVars.prefixes), resolvedZoneID)
				for i, prefix := range purgeFlagsVars.prefixes {
					fmt.Printf("  %d. %s\n", i+1, prefix)
				}
			}

			resp, err := cache.PurgePrefixes(client, resolvedZoneID, purgeFlagsVars.prefixes)
			if err != nil {
				return fmt.Errorf("failed to purge prefixes: %w", err)
			}

			fmt.Printf("Successfully purged content with %d prefixes. Purge ID: %s\n", len(purgeFlagsVars.prefixes), resp.Result.ID)
			return nil
		},
	}

	return cmd
}

// createPurgeHostsCmd creates a command to purge cache by hostname
func createPurgeHostsCmd() *cobra.Command {
	// Define local variables for this command's flags
	var commaDelimitedHosts string
	var hostsFile string
	var autoZoneDetect bool

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
  
  # Control concurrency
  cache-kv-purger cache purge hosts --zone example.com --hosts "..." --concurrency 15 --zone-concurrency 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Declare verbose once at the beginning
			verbose, _ := cmd.Flags().GetBool("verbose")

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

			// Purge hosts
			if verbose {
				fmt.Printf("Purging content for %d hosts in zone %s...\n", len(allHosts), resolvedZoneID)
				for i, host := range allHosts {
					fmt.Printf("  %d. %s\n", i+1, host)
				}
			}

			resp, err := cache.PurgeHosts(client, resolvedZoneID, allHosts)
			if err != nil {
				return fmt.Errorf("failed to purge hosts: %w", err)
			}

			fmt.Printf("Successfully purged content for %d hosts. Purge ID: %s\n", len(allHosts), resp.Result.ID)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.hosts, "host", []string{}, "Hostname to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedHosts, "hosts", "", "Comma-delimited list of hostnames to purge (e.g., \"host1.com,host2.com,host3.com\")")
	cmd.Flags().StringVar(&hostsFile, "hosts-file", "", "Path to a text file containing hostnames to purge (one host per line)")
	cmd.Flags().BoolVar(&autoZoneDetect, "auto-zone", false, "Auto-detect zones based on hostnames")

	return cmd
}

// getZoneInfo gets information about a zone
func getZoneInfo(client *api.Client, zoneID string) (*zones.ZoneDetailsResponse, error) {
	return zones.GetZoneDetails(client, zoneID)
}

// resolveZoneIdentifiers resolves zone identifiers from various sources
func resolveZoneIdentifiers(cmd *cobra.Command, client *api.Client, accountID string) ([]string, error) {
	// Flag to indicate if --all-zones was used
	allZones, _ := cmd.Flags().GetBool("all-zones")

	// If --all-zones is set, fetch all zones for the account
	if allZones {
		// Make sure we have an account ID
		if accountID == "" {
			return nil, fmt.Errorf("account ID is required for --all-zones, set it with CLOUDFLARE_ACCOUNT_ID or in config")
		}

		// Fetch all zones
		zoneList, err := zones.ListZones(client, accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to list zones: %w", err)
		}

		if len(zoneList.Result) == 0 {
			return nil, fmt.Errorf("no zones found for the account")
		}

		// Extract zone IDs
		zoneIDs := make([]string, 0, len(zoneList.Result))
		for _, zone := range zoneList.Result {
			zoneIDs = append(zoneIDs, zone.ID)
		}

		return zoneIDs, nil
	}

	// Check for individual zone flags
	zonesFromFlags := purgeFlagsVars.zones
	if len(zonesFromFlags) > 0 {
		resolvedZones := make([]string, 0, len(zonesFromFlags))
		for _, zone := range zonesFromFlags {
			resolved, err := zones.ResolveZoneIdentifier(client, accountID, zone)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve zone %s: %w", zone, err)
			}
			resolvedZones = append(resolvedZones, resolved)
		}
		return resolvedZones, nil
	}

	// Check for zone list flag
	zoneList, _ := cmd.Flags().GetString("zone-list")
	if zoneList != "" {
		// Split by comma
		zoneItems := strings.Split(zoneList, ",")
		resolvedZones := make([]string, 0, len(zoneItems))
		for _, zone := range zoneItems {
			// Trim whitespace
			zone = strings.TrimSpace(zone)
			if zone == "" {
				continue
			}
			resolved, err := zones.ResolveZoneIdentifier(client, accountID, zone)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve zone %s: %w", zone, err)
			}
			resolvedZones = append(resolvedZones, resolved)
		}
		if len(resolvedZones) > 0 {
			return resolvedZones, nil
		}
	}

	// Check for single zone flag
	zoneID := purgeFlagsVars.zoneID
	if zoneID == "" {
		// Try to get from the global flag
		zoneID, _ = cmd.Flags().GetString("zone")
	}

	if zoneID == "" {
		// Try to get from config or environment variable
		cfg, err := config.LoadFromFile("")
		if err == nil {
			zoneID = cfg.GetZoneID()
		}
	}

	if zoneID == "" {
		return nil, fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
	}

	// Resolve zone (could be name or ID)
	resolvedZoneID, err := zones.ResolveZoneIdentifier(client, accountID, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve zone: %w", err)
	}

	return []string{resolvedZoneID}, nil
}

// handleAutoZoneDetectionForFiles handles auto-detection of zones from file URLs
func handleAutoZoneDetectionForFiles(client *api.Client, accountID string, files []string, cmd *cobra.Command,
	cacheConcurrency, multiZoneConcurrency int) error {
	// Group files by hostname
	filesByHost := make(map[string][]string)
	for _, file := range files {
		u, err := url.Parse(file)
		if err != nil {
			fmt.Printf("Warning: Skipping invalid URL: %s\n", file)
			continue
		}

		host := u.Hostname()
		if host == "" {
			fmt.Printf("Warning: Skipping URL without hostname: %s\n", file)
			continue
		}

		filesByHost[host] = append(filesByHost[host], file)
	}

	// Extract unique hostnames
	hosts := make([]string, 0, len(filesByHost))
	for host := range filesByHost {
		hosts = append(hosts, host)
	}

	return handleHostZoneDetection(client, accountID, hosts, filesByHost, cmd, cacheConcurrency, multiZoneConcurrency)
}

// handleAutoZoneDetectionForHosts handles auto-detection of zones from hostnames
func handleAutoZoneDetectionForHosts(client *api.Client, accountID string, hosts []string, cmd *cobra.Command,
	cacheConcurrency, multiZoneConcurrency int) error {
	// Create a map where hosts map to themselves (we don't have files here)
	hostMap := make(map[string][]string)
	for _, host := range hosts {
		hostMap[host] = []string{host}
	}

	return handleHostZoneDetection(client, accountID, hosts, hostMap, cmd, cacheConcurrency, multiZoneConcurrency)
}

// handleHostZoneDetection handles detection of zones from a list of hosts
func handleHostZoneDetection(client *api.Client, accountID string, hosts []string, itemsByHost map[string][]string,
	cmd *cobra.Command, cacheConcurrency, multiZoneConcurrency int) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	if verbose {
		fmt.Printf("Auto-detecting zones for %d hosts...\n", len(hosts))
	}

	// Get possible zones for each host
	hostZones := make(map[string]string)
	unknownHosts := make([]string, 0)

	// If we have an account ID, try to get all zones for the account
	var zoneList *zones.ZoneListResponse
	var err error
	if accountID != "" {
		zoneList, err = zones.ListZones(client, accountID)
		if err != nil {
			return fmt.Errorf("failed to list zones: %w", err)
		}

		// Create a map of zone name to zone ID
		zoneMap := make(map[string]string)
		for _, zone := range zoneList.Result {
			zoneMap[zone.Name] = zone.ID
		}

		// Try to match each host to a zone
		for _, host := range hosts {
			found := false
			// Try to find the longest matching zone for this host
			longestMatch := ""
			longestMatchID := ""

			for zoneName, zoneID := range zoneMap {
				// Check if the host ends with the zone name (with a dot before or exact match)
				if host == zoneName || strings.HasSuffix(host, "."+zoneName) {
					// This is a matching zone, but we want the longest match
					if len(zoneName) > len(longestMatch) {
						longestMatch = zoneName
						longestMatchID = zoneID
						found = true
					}
				}
			}

			if found {
				hostZones[host] = longestMatchID
				if verbose {
					fmt.Printf("  Matched host %s to zone %s\n", host, longestMatch)
				}
			} else {
				unknownHosts = append(unknownHosts, host)
				if verbose {
					fmt.Printf("  No matching zone found for host %s\n", host)
				}
			}
		}
	} else {
		// Without an account ID, we can't auto-detect zones
		return fmt.Errorf("account ID is required for auto-detection, set it with CLOUDFLARE_ACCOUNT_ID or in config")
	}

	if len(unknownHosts) > 0 {
		// Some hosts couldn't be mapped to zones
		return fmt.Errorf("%d hosts couldn't be mapped to zones: %v", len(unknownHosts), unknownHosts)
	}

	// Group hosts by zone
	hostsByZone := make(map[string][]string)
	for host, zoneID := range hostZones {
		hostsByZone[zoneID] = append(hostsByZone[zoneID], host)
	}

	// Group items (files or hosts) by zone
	itemsByZone := make(map[string][]string)
	for zone, hostsInZone := range hostsByZone {
		for _, host := range hostsInZone {
			itemsByZone[zone] = append(itemsByZone[zone], itemsByHost[host]...)
		}
	}

	// Process each zone
	successCount := 0
	totalItems := 0
	for zoneID, items := range itemsByZone {
		// Remove duplicates
		items = removeDuplicates(items)
		totalItems += len(items)

		// Get zone info for display
		zoneInfo, err := getZoneInfo(client, zoneID)
		zoneName := zoneID
		if err == nil && zoneInfo.Result.Name != "" {
			zoneName = zoneInfo.Result.Name
		}

		if verbose {
			fmt.Printf("Purging %d items for zone %s (%s)...\n", len(items), zoneName, zoneID)
			for i, item := range items {
				fmt.Printf("  %d. %s\n", i+1, item)
			}
		} else {
			fmt.Printf("Purging %d items for zone %s...\n", len(items), zoneName)
		}

		// Determine what type of item we're purging (files or hosts)
		if strings.HasPrefix(items[0], "http") {
			// These are files
			resp, err := cache.PurgeFiles(client, zoneID, items)
			if err != nil {
				fmt.Printf("Error purging files for zone %s: %s\n", zoneName, err)
				continue
			}
			fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
		} else {
			// These are hosts
			resp, err := cache.PurgeHosts(client, zoneID, items)
			if err != nil {
				fmt.Printf("Error purging hosts for zone %s: %s\n", zoneName, err)
				continue
			}
			fmt.Printf("Successfully purged %d hosts from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
		}

		successCount++
	}

	// Final summary
	fmt.Printf("Successfully purged %d items across %d/%d zones\n", totalItems, successCount, len(hostsByZone))
	return nil
}

// handleMultiZoneFilePurge handles purging files across multiple zones
func handleMultiZoneFilePurge(client *api.Client, zoneIDs []string, files []string, cmd *cobra.Command) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Create a map of zone ID to zone name for display
	zoneNames := make(map[string]string)
	for _, zoneID := range zoneIDs {
		zoneInfo, err := getZoneInfo(client, zoneID)
		if err == nil && zoneInfo.Result.Name != "" {
			zoneNames[zoneID] = zoneInfo.Result.Name
		} else {
			zoneNames[zoneID] = zoneID // Fallback to ID if name not available
		}
	}

	if verbose {
		fmt.Printf("Purging files across %d zones\n", len(zoneIDs))
		for i, zoneID := range zoneIDs {
			fmt.Printf("  %d. %s (%s)\n", i+1, zoneNames[zoneID], zoneID)
		}
	}

	// Group files by zone
	filesByZone := make(map[string][]string)

	// Files without hostnames (or with unknown hostnames) go to all zones
	filesForAllZones := make([]string, 0)

	for _, file := range files {
		// Parse URL to extract hostname
		u, err := url.Parse(file)
		if err != nil {
			fmt.Printf("Warning: Skipping invalid URL: %s\n", file)
			continue
		}

		hostname := u.Hostname()
		if hostname == "" {
			// No hostname, add to all zones
			filesForAllZones = append(filesForAllZones, file)
			continue
		}

		// Try to find matching zone
		matchFound := false
		for _, zoneID := range zoneIDs {
			zoneName := zoneNames[zoneID]
			// Check if hostname matches the zone (exact match or subdomain)
			if hostname == zoneName || strings.HasSuffix(hostname, "."+zoneName) {
				filesByZone[zoneID] = append(filesByZone[zoneID], file)
				matchFound = true
				break
			}
		}

		if !matchFound {
			// No matching zone found, add to all zones
			filesForAllZones = append(filesForAllZones, file)
		}
	}

	// Add files without specific zones to all zones
	if len(filesForAllZones) > 0 {
		if verbose {
			fmt.Printf("Found %d files without specific zone matching, adding to all zones\n", len(filesForAllZones))
		}
		for _, zoneID := range zoneIDs {
			filesByZone[zoneID] = append(filesByZone[zoneID], filesForAllZones...)
		}
	}

	// Process each zone
	successCount := 0
	for _, zoneID := range zoneIDs {
		zoneFiles := filesByZone[zoneID]
		if len(zoneFiles) == 0 {
			if verbose {
				fmt.Printf("No files to purge for zone %s\n", zoneNames[zoneID])
			}
			continue
		}

		if verbose {
			fmt.Printf("Purging %d files for zone %s...\n", len(zoneFiles), zoneNames[zoneID])
			for i, file := range zoneFiles {
				fmt.Printf("  %d. %s\n", i+1, file)
			}
		} else {
			fmt.Printf("Purging %d files for zone %s...\n", len(zoneFiles), zoneNames[zoneID])
		}

		// Make the API call to purge files
		resp, err := cache.PurgeFiles(client, zoneID, zoneFiles)
		if err != nil {
			fmt.Printf("Error purging files for zone %s: %s\n", zoneNames[zoneID], err)
			continue
		}

		fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(zoneFiles), zoneNames[zoneID], resp.Result.ID)
		successCount++
	}

	// Final summary
	fmt.Printf("Successfully purged files from %d/%d zones\n", successCount, len(zoneIDs))
	return nil
}

// splitIntoBatches splits a slice into batches of the specified size
func splitIntoBatches(items []string, batchSize int) [][]string {
	// Calculate number of batches
	numBatches := (len(items) + batchSize - 1) / batchSize

	// Create batches
	batches := make([][]string, 0, numBatches)
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// removeDuplicates removes duplicate strings from a slice while preserving order
func removeDuplicates(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(items))

	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

func init() {
	// Add purge command to cache command
	cacheCmd.AddCommand(purgeCmd)

	// Add purge subcommands to purge command
	purgeCmd.AddCommand(createPurgeEverythingCmd())
	purgeCmd.AddCommand(createPurgeFilesCmd())
	purgeCmd.AddCommand(createPurgeTagsCmd())
	purgeCmd.AddCommand(createPurgePrefixesCmd())
	purgeCmd.AddCommand(createPurgeHostsCmd())

	// Add cache command to root command
	rootCmd.AddCommand(cacheCmd)

	// Add global flags to purge command
	purgeCmd.PersistentFlags().StringVar(&purgeFlagsVars.zoneID, "zone", "", "Zone ID or name to purge content from")
	purgeCmd.PersistentFlags().StringArrayVar(&purgeFlagsVars.zones, "zones", []string{}, "Zone IDs or names to purge content from (can be specified multiple times)")
	purgeCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	purgeCmd.PersistentFlags().Bool("all-zones", false, "Purge content from all zones in the account")
	purgeCmd.PersistentFlags().String("zone-list", "", "Comma-delimited list of zone IDs or names to purge content from")
	purgeCmd.PersistentFlags().IntVar(&purgeFlagsVars.cacheConcurrency, "concurrency", 0, "Number of concurrent cache operations (default 10, max 20)")
	purgeCmd.PersistentFlags().IntVar(&purgeFlagsVars.multiZoneConcurrency, "zone-concurrency", 0, "Number of zones to process concurrently (default 3)")
}