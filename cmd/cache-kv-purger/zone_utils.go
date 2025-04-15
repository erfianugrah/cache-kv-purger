package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/zones"
)

// DEPRECATED: Use zones.GetZoneDetails directly
// This is kept for backward compatibility but will be removed in future versions
func getZoneInfo(client *api.Client, zoneID string) (*zones.ZoneDetailsResponse, error) {
	return zones.GetZoneDetails(client, zoneID)
}
