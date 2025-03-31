package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/zones"
)

// getZoneInfo gets information about a zone
func getZoneInfo(client *api.Client, zoneID string) (*zones.ZoneDetailsResponse, error) {
	return zones.GetZoneDetails(client, zoneID)
}