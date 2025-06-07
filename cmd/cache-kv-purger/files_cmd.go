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
	// Initialize flags
	var filesList string
	var files []string
	var batchSize int
	var concurrency int

	cmd := &cobra.Command{
		Use:   "files",
		Short: "Purge specific files from cache",
		Long: `Purge specific files from Cloudflare's cache.

Files must be provided as full URLs including the protocol (http:// or https://). 
The Cloudflare API requires complete URLs for cache purging.`,
		Example: `  # Purge a single file
  cache-kv-purger cache purge files --zone example.com --file https://example.com/css/styles.css

  # Purge multiple files
  cache-kv-purger cache purge files --zone example.com --file https://example.com/image1.jpg --file https://example.com/image2.jpg

  # Purge many files from a list
  cache-kv-purger cache purge files --zone example.com --files-list myfiles.txt
  
  # Purge many files with batch processing
  cache-kv-purger cache purge files --zone example.com --files-list myfiles.txt --batch-size 500 --concurrency 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get flags
			var opts struct {
				files       []string
				filesList   string
				zoneID      string
				zones       []string
				dryRun      bool
				verbose     bool
				batchSize   int
				concurrency int
			}

			// Extract flags once at the beginning
			opts.files = files         // Use local variable
			opts.filesList = filesList // Use local variable
			opts.zoneID = purgeFlagsVars.zoneID
			opts.zones = purgeFlagsVars.zones
			opts.dryRun, _ = cmd.Flags().GetBool("dry-run")

			// Handle verbosity settings - check both --verbose flag and --verbosity global flag
			verboseFlag, _ := cmd.Flags().GetBool("verbose")
			verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")
			opts.verbose = verboseFlag || verbosityStr == "verbose" || verbosityStr == "debug"

			opts.batchSize = batchSize
			opts.concurrency = concurrency

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
			allFiles = common.RemoveDuplicates(allFiles)

			// Verify all files are valid URLs
			// Check if all files have a valid URL scheme
			// Cloudflare API requires full URLs with protocol
			for _, file := range allFiles {
				if !strings.HasPrefix(file, "http://") && !strings.HasPrefix(file, "https://") {
					return fmt.Errorf("file URLs must include http:// or https:// prefix: %s", file)
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

			// Purge files for each zone
			if opts.verbose {
				fmt.Printf("Purging %d files for zone %s...\n", len(validFiles), zoneID)
				for i, file := range validFiles {
					fmt.Printf("  %d. %s\n", i+1, file)
				}
			}

			// Check if we need to use batch processing
			useBatchProcessing := len(validFiles) > 100 || opts.batchSize > 0 || opts.concurrency > 0

			if !useBatchProcessing {
				// For small numbers of files, use the direct API call
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
			} else {
				// For large numbers of files, use batch processing
				if opts.verbose {
					fmt.Printf("Using batch processing with batch size %d and concurrency %d\n", opts.batchSize, opts.concurrency)
				}

				// Create batch processor
				processor := common.NewBatchProcessor().
					WithBatchSize(opts.batchSize).
					WithConcurrency(opts.concurrency).
					WithProgressCallback(func(completed, total, successful int) {
						if opts.verbose {
							fmt.Printf("Progress: %d/%d batches completed, %d files purged\n",
								completed, total, successful)
						}
					})

				// Process in batches
				successful, errors := processor.ProcessStrings(validFiles, func(batch []string) ([]string, error) {
					_, err := cache.PurgeFiles(client, zoneID, batch)
					if err != nil {
						return nil, err
					}
					return batch, nil
				})

				// Report errors if any
				if len(errors) > 0 {
					for _, err := range errors {
						fmt.Printf("Error during batch processing: %s\n", err)
					}
				}

				// Report success
				data := make(map[string]string)
				data["Operation"] = "Purge Files (Batch)"
				data["Zone"] = zoneID
				data["Files Purged"] = fmt.Sprintf("%d", len(successful))
				data["Batches"] = fmt.Sprintf("%d", (len(validFiles)+opts.batchSize-1)/opts.batchSize)
				data["Failed Batches"] = fmt.Sprintf("%d", len(errors))
				data["Status"] = "Complete"

				common.FormatKeyValueTable(data)
			}

			return nil
		},
	}

	// Add command flags
	cmd.Flags().StringArrayVar(&files, "file", []string{}, "URL or path of a file to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&filesList, "files-list", "", "Path to a file containing a list of files to purge (one URL per line)")
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Maximum number of files to purge in a single API request (max 500)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 10, "Maximum number of concurrent API requests (1-50)")

	// No need to update global variables - we use local variables directly

	return cmd
}
