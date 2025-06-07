package zones

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
	// Validate and set concurrency limits
	if concurrency <= 0 {
		concurrency = 3 // Default
	} else if concurrency > 5 {
		concurrency = 5 // Cap at 5 to avoid overwhelming API
	}

	// Track progress
	var totalItems int
	for _, items := range itemsByZone {
		totalItems += len(items)
	}

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
		}

		fmt.Printf("DRY RUN SUMMARY: Would process %d total items across %d zones (concurrency: %d)\n", 
			totalItems, len(itemsByZone), concurrency)
		return totalItems, len(itemsByZone), nil
	}

	// Process zones concurrently
	type zoneResult struct {
		zoneID   string
		zoneName string
		success  bool
		err      error
		itemCount int
	}

	// Create work items
	type zoneWork struct {
		zoneID string
		items  []string
		index  int
		total  int
	}

	work := make([]zoneWork, 0, len(itemsByZone))
	index := 0
	for zoneID, items := range itemsByZone {
		work = append(work, zoneWork{
			zoneID: zoneID, 
			items: items,
			index: index + 1,
			total: len(itemsByZone),
		})
		index++
	}

	// Create channels
	workChan := make(chan zoneWork, len(work))
	resultChan := make(chan zoneResult, len(work))

	// Progress tracking
	var processedZones int32
	startTime := time.Now()

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for w := range workChan {
				// Progress update
				current := atomic.AddInt32(&processedZones, 1)
				if verbose {
					fmt.Printf("[Worker %d] Processing zone %d/%d...\n", workerID+1, current, w.total)
				}

				// Get zone info for display
				zoneInfo, err := GetZoneDetails(client, w.zoneID)
				zoneName := w.zoneID
				if err == nil && zoneInfo.Result.Name != "" {
					zoneName = zoneInfo.Result.Name
				}

				// Process items for this zone
				startZone := time.Now()
				success, err := handler(w.zoneID, zoneName, w.items)
				duration := time.Since(startZone)
				
				if verbose && err == nil {
					fmt.Printf("[Worker %d] Zone %s processed in %v\n", workerID+1, zoneName, duration)
				}
				
				resultChan <- zoneResult{
					zoneID:   w.zoneID,
					zoneName: zoneName,
					success:  success,
					err:      err,
					itemCount: len(w.items),
				}
			}
		}(i)
	}

	// Send work to workers
	for _, w := range work {
		workChan <- w
	}
	close(workChan)

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	successCount := 0
	var errors []error
	processedItems := 0
	
	for result := range resultChan {
		if result.err != nil {
			errors = append(errors, fmt.Errorf("zone %s: %w", result.zoneName, result.err))
			fmt.Printf("❌ Error processing zone %s: %s\n", result.zoneName, result.err)
		} else if result.success {
			successCount++
			processedItems += result.itemCount
			if !verbose {
				fmt.Printf("✅ Zone %s: %d items processed\n", result.zoneName, result.itemCount)
			}
		}
	}

	// Final summary
	totalDuration := time.Since(startTime)
	fmt.Printf("\n🏁 Completed in %v: %d items across %d/%d zones (concurrency: %d)\n", 
		totalDuration, processedItems, successCount, len(itemsByZone), concurrency)
	
	if len(errors) > 0 {
		fmt.Printf("⚠️  %d zones had errors\n", len(errors))
	}
	
	return processedItems, successCount, nil
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
