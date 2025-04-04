package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
)

// createPurgeFilesCmd creates a new command for purging specific files from cache
func createPurgeFilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "files",
		Short: "Purge specific files from cache",
		Long: `Purge specific files from Cloudflare's cache.

Files should be provided as full URLs or relative paths. If a file doesn't start with http:// or https://, 
https:// will be automatically added.`,
		Example: `  # Purge a single file
  cache-kv-purger cache purge files --zone example.com --file https://example.com/css/styles.css

  # Purge multiple files
  cache-kv-purger cache purge files --zone example.com --file image1.jpg --file image2.jpg

  # Purge many files from a list
  cache-kv-purger cache purge files --zone example.com --files-list myfiles.txt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flags
			var opts struct {
				files     []string
				filesList string
				zoneID    string
				zones     []string
				dryRun    bool
				verbose   bool
			}

			// Extract flags once at the beginning
			opts.files = purgeFlagsVars.files
			opts.filesList = cmd.Flag("files-list").Value.String()
			opts.zoneID = purgeFlagsVars.zoneID
			opts.zones = purgeFlagsVars.zones
			opts.dryRun, _ = cmd.Flags().GetBool("dry-run")
			opts.verbose, _ = cmd.Flags().GetBool("verbose")

			// Load config
			cfg, err := config.LoadFromFile("")
			if err != nil {
				// Just use defaults if config fails to load
				cfg = config.New()
			}

			// Get API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Collect all files to purge
			var allFiles []string

			// Add files from command line flags
			allFiles = append(allFiles, opts.files...)

			// Add files from file list if provided
			if opts.filesList != "" {
				filesListPath := opts.filesList
				// Read the file
				fileData, err := os.ReadFile(filesListPath)
				if err != nil {
					return fmt.Errorf("failed to read files list: %w", err)
				}

				// Split the file content by lines
				lines := strings.Split(string(fileData), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" && !strings.HasPrefix(line, "#") {
						allFiles = append(allFiles, line)
					}
				}

				if opts.verbose {
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
			}

			// Process one zone at a time
			zoneID := opts.zoneID
			if zoneID == "" {
				// Try to get from the global flag
				zoneID, _ = cmd.Root().Flags().GetString("zone")
			}

			// If zone still not set, check if there are zones in the flag vars, config, or environment
			if zoneID == "" && len(opts.zones) > 0 {
				zoneID = opts.zones[0] // Just use the first one for now
			}

			if zoneID == "" && cfg != nil {
				zoneID = cfg.GetZoneID()
			}

			if zoneID == "" {
				return fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
			}

			// Get account ID for zone resolver
			accountID := ""
			if cfg != nil {
				accountID = cfg.GetAccountID()
			}

			// Resolve zone (could be name or ID)
			zoneID, err = zones.ResolveZoneIdentifier(client, accountID, zoneID)
			if err != nil {
				return fmt.Errorf("failed to resolve zone: %w", err)
			}

			// Now purge the files
			validFiles := allFiles

			// Handle dry run mode
			if opts.dryRun {
				fmt.Printf("DRY RUN: Would purge %d files from zone %s\n", len(validFiles), zoneID)
				if opts.verbose {
					for i, file := range validFiles {
						fmt.Printf("  %d. %s\n", i+1, file)
					}
				}
				return nil
			}

			// Track successes
			successCount := 0

			// Purge files for each zone
			if opts.verbose {
				fmt.Printf("Purging %d files for zone %s...\n", len(validFiles), zoneID)
				for i, file := range validFiles {
					fmt.Printf("  %d. %s\n", i+1, file)
				}
			}

			// Make the API call to purge files
			resp, err := cache.PurgeFiles(client, zoneID, validFiles)
			if err != nil {
				fmt.Printf("Error purging files for zone %s: %s\n", zoneID, err)
				return fmt.Errorf("failed to purge files: %w", err)
			}

			// Report success with formatted table
			data := make(map[string]string)
			data["Operation"] = "Purge Files"
			data["Zone"] = zoneID
			data["Files Purged"] = fmt.Sprintf("%d", len(validFiles))
			data["Purge ID"] = resp.Result.ID
			data["Status"] = "Success"

			common.FormatKeyValueTable(data)
			successCount++

			return nil
		},
	}
}
