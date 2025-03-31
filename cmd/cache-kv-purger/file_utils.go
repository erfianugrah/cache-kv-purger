package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"strings"
)

// handleMultiZoneFilePurge handles purging files across multiple zones
func handleMultiZoneFilePurge(client *api.Client, zoneIDs []string, files []string, cmd *cobra.Command) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	
	if batchSize <= 0 {
		batchSize = 30 // Default batch size if not specified
	}
	
	// Get concurrency settings
	cacheConcurrency := purgeFlagsVars.cacheConcurrency
	multiZoneConcurrency := purgeFlagsVars.multiZoneConcurrency
	
	// Get config for default concurrency values if not set
	cfg, _ := config.LoadFromFile("")
	if cacheConcurrency <= 0 && cfg != nil {
		cacheConcurrency = cfg.GetCacheConcurrency()
	}
	
	if multiZoneConcurrency <= 0 && cfg != nil {
		multiZoneConcurrency = cfg.GetMultiZoneConcurrency()
	}
	
	// Cap concurrency to reasonable limits
	if cacheConcurrency <= 0 {
		cacheConcurrency = 10 // Default
	} else if cacheConcurrency > 20 {
		cacheConcurrency = 20 // Max
	}
	
	if multiZoneConcurrency <= 0 {
		multiZoneConcurrency = 3 // Default
	} else if multiZoneConcurrency > 5 {
		multiZoneConcurrency = 5 // Max to avoid overwhelming API
	}

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

	// For dry-run, just show what would be purged
	if dryRun {
		fmt.Printf("DRY RUN: Would purge files across %d zones\n", len(filesByZone))
		totalFiles := 0
		
		for zoneID, zoneFiles := range filesByZone {
			if len(zoneFiles) == 0 {
				if verbose {
					fmt.Printf("Zone %s: No files to purge\n", zoneNames[zoneID])
				}
				continue
			}
			
			// Remove duplicates
			zoneFiles = removeDuplicates(zoneFiles)
			totalFiles += len(zoneFiles)
			
			// Calculate batches for display
			batches := splitIntoBatches(zoneFiles, batchSize)
			
			fmt.Printf("Zone %s: Would purge %d files in %d batches using %d concurrent workers\n", 
				zoneNames[zoneID], len(zoneFiles), len(batches), cacheConcurrency)
			
			if verbose {
				for i, batch := range batches {
					fmt.Printf("  Batch %d: %d files\n", i+1, len(batch))
					for j, file := range batch {
						if j < 5 { // List first 5 files to avoid overwhelming output
							fmt.Printf("    %d. %s\n", j+1, file)
						} else if j == 5 {
							fmt.Printf("    ... and %d more files\n", len(batch)-5)
							break
						}
					}
				}
			}
		}
		
		fmt.Printf("DRY RUN SUMMARY: Would purge %d total files across %d zones\n", totalFiles, len(filesByZone))
		return nil
	}

	// Process each zone
	successCount := 0
	totalFiles := 0
	
	// Use a simple sequential approach for now since we don't have a specialized PurgeFilesInBatches function yet
	// Future enhancement: Process zones concurrently with batched file operations
	for _, zoneID := range zoneIDs {
		zoneFiles := filesByZone[zoneID]
		if len(zoneFiles) == 0 {
			if verbose {
				fmt.Printf("No files to purge for zone %s\n", zoneNames[zoneID])
			}
			continue
		}

		// Remove duplicates
		zoneFiles = removeDuplicates(zoneFiles)
		totalFiles += len(zoneFiles)

		if verbose {
			fmt.Printf("Purging %d files for zone %s...\n", len(zoneFiles), zoneNames[zoneID])
			for i, file := range zoneFiles {
				fmt.Printf("  %d. %s\n", i+1, file)
			}
		} else {
			fmt.Printf("Purging %d files for zone %s...\n", len(zoneFiles), zoneNames[zoneID])
		}

		// For large number of files, handle in batches (when we have a specialized function)
		// For now, use the standard API call
		// Future enhancement:
		// if len(zoneFiles) > batchSize {
		//     // Process with batching...
		//     successful, errors := cache.PurgeFilesInBatches(client, zoneID, zoneFiles, progressFn, cacheConcurrency)
		// }
		
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
	fmt.Printf("Successfully purged %d files from %d/%d zones\n", totalFiles, successCount, len(zoneIDs))
	return nil
}