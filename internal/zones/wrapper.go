package zones

import (
	"cache-kv-purger/internal/api"
	"fmt"
)

// LegacyResolveZoneIdentifier provides backward compatibility with the old method signature
// This should be used by code that hasn't been updated to pass accountID
func LegacyResolveZoneIdentifier(client *api.Client, zoneIdentifier string) (string, error) {
	// Call the current implementation with an empty accountID
	// Note: This will be less efficient than using the accountID version
	// because it may need to try multiple API calls
	return ResolveZoneIdentifier(client, "", zoneIdentifier)
}

// HandleMultiZoneOperation is a common entry point for multi-zone operations
// It handles zone detection, item grouping, and processing
// hosts: list of hostnames
// itemsByHost: mapping of hostnames to items (e.g. files or URLs) to process
// client: API client
// accountID: account ID for zone resolution
// verbose: enable verbose output
// dryRun: only show what would be processed without actual processing
// zoneConcurrency: how many zones to process concurrently
// itemConcurrency: how many items within a zone to process concurrently
// batchSize: size of batches for processing
// handler: function to process items for a specific zone
func HandleMultiZoneOperation(
	hosts []string,
	itemsByHost map[string][]string,
	client *api.Client,
	accountID string,
	verbose bool,
	dryRun bool,
	zoneConcurrency int,
	itemConcurrency int,
	batchSize int,
	handler func(zoneID string, zoneName string, items []string) (bool, error),
) error {
	if accountID == "" {
		return fmt.Errorf("account ID is required for zone detection")
	}

	// Get possible zones for each host
	hostZones, unknownHosts, err := DetectZonesFromHosts(client, accountID, hosts)
	if err != nil {
		return fmt.Errorf("failed to detect zones: %w", err)
	}

	if len(unknownHosts) > 0 {
		// Some hosts couldn't be mapped to zones
		return fmt.Errorf("%d hosts couldn't be mapped to zones: %v", len(unknownHosts), unknownHosts)
	}

	// Group items by zone
	itemsByZone := GroupItemsByZone(hostZones, itemsByHost)

	// Process the items by zone
	_, _, err = ProcessMultiZoneItems(
		client,
		itemsByZone,
		handler,
		verbose,
		dryRun,
		zoneConcurrency,
	)

	return err
}