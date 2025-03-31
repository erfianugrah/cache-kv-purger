package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"os"
	"strings"
)

// createPurgeFilesCmd creates a command to purge specific files
func createPurgeFilesCmd() *cobra.Command {
	// Define local variables for this command's flags
	var commaDelimitedFiles string
	var filesListPath string
	var autoZoneDetect bool
	var dryRun bool
	var batchSize int

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
  cache-kv-purger cache purge files --file https://example.com/image.jpg --file https://example2.com/script.js
  
  # Dry run (show what would be purged, but don't actually purge)
  cache-kv-purger cache purge files --zone example.com --files-list urls.txt --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get verbose flag once at the beginning
			verbose, _ := cmd.Flags().GetBool("verbose")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

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

			// Remove duplicates
			allFiles = removeDuplicates(allFiles)

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

			// Default batch size if not specified
			if batchSize <= 0 {
				batchSize = 30
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

			// Get zone info for display
			zoneInfo, err := getZoneInfo(client, zoneID)
			zoneName := zoneID
			if err == nil && zoneInfo.Result.Name != "" {
				zoneName = zoneInfo.Result.Name
			}

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

			// Dry run mode
			if dryRun {
				fmt.Printf("DRY RUN: Would purge %d files from zone %s\n", len(validFiles), zoneName)
				if verbose {
					for i, file := range validFiles {
						fmt.Printf("  %d. %s\n", i+1, file)
					}
				}
				return nil
			}

			// Track successes
			successCount := 0

			// Purge files for each zone
			if verbose {
				fmt.Printf("Purging %d files for zone %s...\n", len(validFiles), zoneName)
				for i, file := range validFiles {
					fmt.Printf("  %d. %s\n", i+1, file)
				}
			}

			// Make the API call to purge files
			resp, err := cache.PurgeFiles(client, zoneID, validFiles)
			if err != nil {
				fmt.Printf("Error purging files for zone %s: %s\n", zoneName, err)
				return fmt.Errorf("failed to purge files: %w", err)
			}

			// Report success
			fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(validFiles), zoneName, resp.Result.ID)
			successCount++

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.files, "file", []string{}, "File URL to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedFiles, "files", "", "Comma-delimited list of file URLs to purge (e.g., \"https://example.com/image.jpg,https://example.com/script.js\")")
	cmd.Flags().StringVar(&filesListPath, "files-list", "", "Path to a text file containing file URLs to purge (one URL per line)")
	cmd.Flags().BoolVar(&autoZoneDetect, "auto-zone", false, "Auto-detect zones based on file URLs")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be purged without actually purging")
	cmd.Flags().IntVar(&batchSize, "batch-size", 30, "Maximum number of files to purge in each batch (default 30)")

	return cmd
}