package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"net/url"
	"strings"
)

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
