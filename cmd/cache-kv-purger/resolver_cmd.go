package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)


// handleAutoZoneDetectionForHosts handles auto-detection of zones from hostnames
func handleAutoZoneDetectionForHosts(client *api.Client, accountID string, hosts []string, cmd *cobra.Command,
	cacheConcurrency, multiZoneConcurrency int) error {
	verbose, _ := cmd.Flags().GetBool("verbose")

	if verbose {
		fmt.Printf("Auto-detecting zones for %d hosts...\n", len(hosts))
	}

	// Create a map where hosts map to themselves (we don't have files here)
	hostMap := make(map[string][]string)
	for _, host := range hosts {
		hostMap[host] = []string{host}
	}

	// Detect zones for hosts
	hostZones, unknownHosts, err := zones.DetectZonesFromHosts(client, accountID, hosts)
	if err != nil {
		return fmt.Errorf("failed to detect zones: %w", err)
	}

	if len(unknownHosts) > 0 {
		return fmt.Errorf("%d hosts couldn't be mapped to zones: %v", len(unknownHosts), unknownHosts)
	}

	// Group hosts by zone
	itemsByZone := zones.GroupItemsByZone(hostZones, hostMap)

	// Now handle processing with the results
	return handleItemsForZones(client, itemsByZone, cmd, cacheConcurrency, multiZoneConcurrency, "hosts")
}

// handleItemsForZones handles processing items (files or hosts) by zone
func handleItemsForZones(client *api.Client, itemsByZone map[string][]string, cmd *cobra.Command,
	cacheConcurrency, multiZoneConcurrency int, itemType string) error {

	verbose, _ := cmd.Flags().GetBool("verbose")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	batchSize, _ := cmd.Flags().GetInt("batch-size")

	if batchSize <= 0 {
		batchSize = 30 // Default batch size if not specified
	}

	// Cap concurrency to reasonable limits
	if cacheConcurrency <= 0 {
		cacheConcurrency = 10 // Default
	} else if cacheConcurrency > 20 {
		cacheConcurrency = 20 // Max
	}

	// Validate multi-zone concurrency
	if multiZoneConcurrency <= 0 {
		multiZoneConcurrency = 3 // Default
	} else if multiZoneConcurrency > 5 {
		multiZoneConcurrency = 5 // Max to avoid overwhelming API
	}

	// Define the handler function for processing items in each zone
	handler := func(zoneID string, zoneName string, items []string) (bool, error) {
		// Process items based on type (files or hosts)
		switch itemType {
		case "files":
			if verbose {
				fmt.Printf("Purging %d files for zone %s...\n", len(items), zoneName)
			}

			// Make the API call to purge files
			resp, err := cache.PurgeFiles(client, zoneID, items)
			if err != nil {
				return false, fmt.Errorf("failed to purge files: %w", err)
			}

			fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
			return true, nil

		case "hosts":
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
					errMsg := fmt.Sprintf("Encountered %d errors during purging for zone %s:\n", len(errors), zoneName)
					for i, err := range errors {
						if i < 3 { // Show at most 3 errors
							errMsg += fmt.Sprintf("  - %s\n", err)
						} else {
							errMsg += fmt.Sprintf("  - ... and %d more errors\n", len(errors)-3)
							break
						}
					}
					return false, fmt.Errorf(errMsg)
				}

				fmt.Printf("Successfully purged %d hosts from zone %s\n", len(successful), zoneName)
				return true, nil
			} else {
				// Small number of hosts, just use single API call
				resp, err := cache.PurgeHosts(client, zoneID, items)
				if err != nil {
					return false, fmt.Errorf("failed to purge hosts: %w", err)
				}
				fmt.Printf("Successfully purged %d hosts from zone %s. Purge ID: %s\n", len(items), zoneName, resp.Result.ID)
				return true, nil
			}
		default:
			return false, fmt.Errorf("unknown item type: %s", itemType)
		}
	}

	// Use ProcessMultiZoneItems for concurrent processing
	totalItems, successCount, err := zones.ProcessMultiZoneItems(
		client, 
		itemsByZone, 
		handler, 
		verbose, 
		dryRun, 
		multiZoneConcurrency,
	)
	
	if err != nil {
		return fmt.Errorf("failed to process zones: %w", err)
	}

	// The summary is already printed by ProcessMultiZoneItems
	_ = totalItems
	_ = successCount

	return nil
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
		return zones.ResolveZoneIdentifiers(client, accountID, zonesFromFlags)
	}

	// Check for zone list flag
	zoneList, _ := cmd.Flags().GetString("zone-list")
	if zoneList != "" {
		// Split by comma
		zoneItems := strings.Split(zoneList, ",")

		// Filter out empty items
		var filteredItems []string
		for _, zone := range zoneItems {
			// Trim whitespace
			zone = strings.TrimSpace(zone)
			if zone != "" {
				filteredItems = append(filteredItems, zone)
			}
		}

		if len(filteredItems) > 0 {
			return zones.ResolveZoneIdentifiers(client, accountID, filteredItems)
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

	// Resolve a single zone
	resolvedZoneID, err := zones.ResolveZoneIdentifier(client, accountID, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve zone: %w", err)
	}

	return []string{resolvedZoneID}, nil
}
