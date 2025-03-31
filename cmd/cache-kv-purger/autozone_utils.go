package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

// handleHostZoneDetection handles detection of zones from a list of hosts
func handleHostZoneDetection(client *api.Client, accountID string, hosts []string, itemsByHost map[string][]string,
	cmd *cobra.Command, cacheConcurrency, multiZoneConcurrency int) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	
	if batchSize <= 0 {
		batchSize = 30 // Default batch size if not specified
	}

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

	// Process each zone
	successCount := 0
	totalItems := 0
	
	// For dry-run, just show what would be purged
	if dryRun {
		fmt.Printf("DRY RUN: Would purge items across %d zones\n", len(itemsByZone))
		for zoneID, items := range itemsByZone {
			// Remove duplicates
			items = removeDuplicates(items)
			
			// Get zone info for display
			zoneInfo, err := getZoneInfo(client, zoneID)
			zoneName := zoneID
			if err == nil && zoneInfo.Result.Name != "" {
				zoneName = zoneInfo.Result.Name
			}
			
			// Determine what type of item we're purging (files or hosts)
			itemType := "hosts"
			if len(items) > 0 && strings.HasPrefix(items[0], "http") {
				itemType = "files"
			}
			
			// Calculate batches for display
			batches := splitIntoBatches(items, batchSize)
			
			fmt.Printf("Zone: %s - would purge %d %s in %d batches using %d concurrent workers\n", 
				zoneName, len(items), itemType, len(batches), cacheConcurrency)
			
			if verbose {
				for i, batch := range batches {
					fmt.Printf("  Batch %d: %d items\n", i+1, len(batch))
					for j, item := range batch {
						if j < 5 { // List first 5 items to avoid overwhelming output
							fmt.Printf("    %d. %s\n", j+1, item)
						} else if j == 5 {
							fmt.Printf("    ... and %d more items\n", len(batch)-5)
							break
						}
					}
				}
			}
			
			totalItems += len(items)
		}
		
		fmt.Printf("DRY RUN SUMMARY: Would purge %d total items across %d zones\n", totalItems, len(itemsByZone))
		return nil
	}
	
	// Process all zones with actual purging
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

		// Determine what type of item we're purging (files or hosts)
		if len(items) > 0 && strings.HasPrefix(items[0], "http") {
			// These are files
			if verbose {
				fmt.Printf("Purging %d files for zone %s...\n", len(items), zoneName)
			}
			
			// For large number of files, use batching
			if len(items) > batchSize {
				// For specialized function for files with headers when implemented in the future:
				// progressFn := func(completed, total, successful int) {
				//     if verbose {
				//         fmt.Printf("Progress for zone %s: processed %d/%d batches, %d files purged\n", 
				//             zoneName, completed, total, successful)
				//     } else {
				//         fmt.Printf("Zone %s: processing batch %d/%d: %d files purged so far...  \r", 
				//             zoneName, completed, total, successful)
				//     }
				// }
				// successful, errors := PurgeFilesInBatches(client, zoneID, items, progressFn, cacheConcurrency)
				
				// For now, use standard API as batch function for files isn't implemented yet
				resp, err := cache.PurgeFiles(client, zoneID, items)
				if err != nil {
					fmt.Printf("Error purging files for zone %s: %s\n", zoneName, err)
					continue
				}
				fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
				successCount++
			} else {
				// Small number of files, just use single API call
				resp, err := cache.PurgeFiles(client, zoneID, items)
				if err != nil {
					fmt.Printf("Error purging files for zone %s: %s\n", zoneName, err)
					continue
				}
				fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
				successCount++
			}
		} else {
			// These are hosts
			if verbose {
				fmt.Printf("Purging %d hosts for zone %s...\n", len(items), zoneName)
			}
			
			// For large number of hosts, use batching with concurrency
			if len(items) > batchSize {
				// Create progress function
				progressFn := func(completed, total, successful int) {
					if verbose {
						fmt.Printf("Progress for zone %s: processed %d/%d batches, %d hosts purged\n", 
							zoneName, completed, total, successful)
					} else {
						fmt.Printf("Zone %s: processing batch %d/%d: %d hosts purged so far...  \r", 
							zoneName, completed, total, successful)
					}
				}
				
				// Process hosts with concurrent batching
				successful, errors := cache.PurgeHostsInBatches(client, zoneID, items, progressFn, cacheConcurrency)
				
				// Print a newline to clear the progress line
				if !verbose {
					fmt.Println()
				}
				
				// Report errors if any
				if len(errors) > 0 {
					fmt.Printf("Encountered %d errors during purging for zone %s:\n", len(errors), zoneName)
					for i, err := range errors {
						if i < 3 { // Show at most 3 errors to avoid flooding the console
							fmt.Printf("  - %s\n", err)
						} else {
							fmt.Printf("  - ... and %d more errors\n", len(errors)-3)
							break
						}
					}
					continue
				}
				
				fmt.Printf("Successfully purged %d hosts from zone %s\n", len(successful), zoneName)
				successCount++
			} else {
				// Small number of hosts, just use single API call
				resp, err := cache.PurgeHosts(client, zoneID, items)
				if err != nil {
					fmt.Printf("Error purging hosts for zone %s: %s\n", zoneName, err)
					continue
				}
				fmt.Printf("Successfully purged %d hosts from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
				successCount++
			}
		}
	}

	// Final summary
	fmt.Printf("Successfully purged %d items across %d/%d zones\n", totalItems, successCount, len(hostsByZone))
	return nil
}