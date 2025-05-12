package zones

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
)

// ZoneDetailsResponse represents the response from a zone details request
type ZoneDetailsResponse struct {
	api.APIResponse
	Result api.Zone `json:"result"`
}

// ZoneListResponse is an alias for ZonesResponse for better naming consistency
type ZoneListResponse = api.ZonesResponse

// ListZones retrieves all zones for an account
func ListZones(client *api.Client, accountID string) (*ZoneListResponse, error) {
	// Build query params
	query := url.Values{}
	if accountID != "" {
		query.Add("account.id", accountID)
	}

	// Make the request
	path := "/zones"
	respBody, err := client.Request(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}

	// Parse the response using our generic parser
	var zones []api.Zone
	resp, err := api.ParsePaginatedResponse(respBody, &zones)
	if err != nil {
		return nil, fmt.Errorf("failed to list zones: %w", err)
	}

	// Build the response object
	zonesResp := &ZoneListResponse{
		PaginatedResponse: *resp,
		Result:            zones,
	}

	return zonesResp, nil
}

// GetZoneByName finds a zone by its domain name
func GetZoneByName(client *api.Client, accountID, name string) (*api.Zone, error) {
	// Build query params
	query := url.Values{}
	if accountID != "" {
		query.Add("account.id", accountID)
	}
	query.Add("name", name)

	// Make the request
	path := "/zones"
	respBody, err := client.Request(http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}

	// Parse the response using our generic parser
	var zones []api.Zone
	_, err = api.ParsePaginatedResponse(respBody, &zones)
	if err != nil {
		return nil, fmt.Errorf("failed to find zone: %w", err)
	}

	if len(zones) == 0 {
		return nil, fmt.Errorf("no zone found with name '%s'", name)
	}

	return &zones[0], nil
}

// ResolveZoneIdentifier takes a string that could be either:
// - A zone ID (32-character hexadecimal string)
// - A domain name
// And returns the corresponding zone ID
func ResolveZoneIdentifier(client *api.Client, accountID, identifier string) (string, error) {
	// Check if it's already a zone ID (32-character hexadecimal)
	if len(identifier) == 32 && isHexString(identifier) {
		return identifier, nil
	}

	// Try to resolve as domain name
	zone, err := GetZoneByName(client, accountID, identifier)
	if err != nil {
		// Try to handle subdomains by finding parent domain
		zonesResp, err := ListZones(client, accountID)
		if err != nil {
			return "", fmt.Errorf("failed to list zones: %w", err)
		}

		// Look for a parent domain of the specified name
		domainParts := strings.Split(identifier, ".")
		for i := 1; i < len(domainParts); i++ {
			// Try each possible parent domain
			parentDomain := strings.Join(domainParts[i:], ".")

			// Check if this is a valid zone
			for _, zone := range zonesResp.Result {
				if zone.Name == parentDomain {
					return zone.ID, nil
				}
			}
		}

		return "", fmt.Errorf("failed to resolve '%s' as a zone: %w", identifier, err)
	}

	return zone.ID, nil
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GetZoneDetails retrieves details for a specific zone by its ID
func GetZoneDetails(client *api.Client, zoneID string) (*ZoneDetailsResponse, error) {
	path := fmt.Sprintf("/zones/%s", zoneID)
	respBody, err := client.Request(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}

	var zone api.Zone
	resp, err := api.ParseAPIResponse(respBody, &zone)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone details: %w", err)
	}

	// Build the response object
	detailsResp := &ZoneDetailsResponse{
		APIResponse: *resp,
		Result:      zone,
	}

	return detailsResp, nil
}

// DetectZonesFromHosts attempts to find the appropriate zone for each hostname
// Returns:
// - A map of host to zone ID for hosts that could be matched to zones
// - A slice of hosts that couldn't be matched
func DetectZonesFromHosts(client *api.Client, accountID string, hosts []string) (map[string]string, []string, error) {
	// Create maps to store results
	hostZones := make(map[string]string)
	unknownHosts := make([]string, 0)

	// If we don't have an account ID, we can't auto-detect zones
	if accountID == "" {
		return nil, hosts, fmt.Errorf("account ID is required for auto-detection")
	}

	// Get all zones for the account
	zoneList, err := ListZones(client, accountID)
	if err != nil {
		return nil, hosts, fmt.Errorf("failed to list zones: %w", err)
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
		} else {
			unknownHosts = append(unknownHosts, host)
		}
	}

	return hostZones, unknownHosts, nil
}

// GroupItemsByZone groups items by zone based on hostname mapping
// itemsByHost is a map of hostname to the items (e.g., URLs) associated with that host
// hostZones is a map of hostname to zone ID
func GroupItemsByZone(hostZones map[string]string, itemsByHost map[string][]string) map[string][]string {
	// First group hosts by zone
	hostsByZone := make(map[string][]string)
	for host, zoneID := range hostZones {
		hostsByZone[zoneID] = append(hostsByZone[zoneID], host)
	}

	// Then group items by zone based on their host
	itemsByZone := make(map[string][]string)
	for zone, hostsInZone := range hostsByZone {
		for _, host := range hostsInZone {
			itemsByZone[zone] = append(itemsByZone[zone], itemsByHost[host]...)
		}
	}

	// Remove duplicates in each zone's items
	for zone, items := range itemsByZone {
		itemsByZone[zone] = common.RemoveDuplicates(items)
	}

	return itemsByZone
}

// ProcessMultiZoneItems processes items grouped by zone using a handler function
// handler is a function that processes items for a specific zone
// verbose enables verbose output
// dryRun only shows what would be processed without actual processing
// concurrency specifies how many zones to process concurrently
func ProcessMultiZoneItems(
	client *api.Client,
	itemsByZone map[string][]string,
	handler func(zoneID string, zoneName string, items []string) (bool, error),
	verbose bool,
	dryRun bool,
	concurrency int,
) (int, int, error) {
	// Cap concurrency to reasonable limits
	if concurrency <= 0 {
		concurrency = 3 // Default
	} else if concurrency > 5 {
		concurrency = 5 // Max to avoid overwhelming API
	}

	// Track progress
	successCount := 0
	totalItems := 0

	// For dry-run, just show what would be processed
	if dryRun {
		fmt.Printf("DRY RUN: Would process items across %d zones\n", len(itemsByZone))
		for zoneID, items := range itemsByZone {
			// Get zone info for display
			zoneInfo, err := GetZoneDetails(client, zoneID)
			zoneName := zoneID
			if err == nil && zoneInfo.Result.Name != "" {
				zoneName = zoneInfo.Result.Name
			}

			fmt.Printf("Zone: %s - would process %d items\n", zoneName, len(items))

			if verbose {
				// Show sample of items
				for i, item := range items {
					if i < 5 { // List first 5 items to avoid overwhelming output
						fmt.Printf("  %d. %s\n", i+1, item)
					} else if i == 5 {
						fmt.Printf("  ... and %d more items\n", len(items)-5)
						break
					}
				}
			}

			totalItems += len(items)
		}

		fmt.Printf("DRY RUN SUMMARY: Would process %d total items across %d zones\n", totalItems, len(itemsByZone))
		return totalItems, len(itemsByZone), nil
	}

	// Process all zones with actual operation
	for zoneID, items := range itemsByZone {
		totalItems += len(items)

		// Get zone info for display
		zoneInfo, err := GetZoneDetails(client, zoneID)
		zoneName := zoneID
		if err == nil && zoneInfo.Result.Name != "" {
			zoneName = zoneInfo.Result.Name
		}

		// Process items for this zone
		success, err := handler(zoneID, zoneName, items)
		if err != nil {
			fmt.Printf("Error processing items for zone %s: %s\n", zoneName, err)
			continue
		}

		if success {
			successCount++
		}
	}

	// Final summary
	fmt.Printf("Successfully processed %d items across %d/%d zones\n", totalItems, successCount, len(itemsByZone))
	return totalItems, successCount, nil
}

// ResolveZoneIdentifiers resolves zone identifiers from a list of zone names or IDs
func ResolveZoneIdentifiers(client *api.Client, accountID string, zones []string) ([]string, error) {
	if len(zones) == 0 {
		return nil, fmt.Errorf("at least one zone identifier is required")
	}

	resolvedZones := make([]string, 0, len(zones))
	for _, zone := range zones {
		resolved, err := ResolveZoneIdentifier(client, accountID, zone)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve zone %s: %w", zone, err)
		}
		resolvedZones = append(resolvedZones, resolved)
	}

	return resolvedZones, nil
}
