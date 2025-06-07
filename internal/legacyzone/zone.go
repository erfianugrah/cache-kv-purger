package legacyzone

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/zones"
)

// ResolveZoneIdentifier resolves a zone name or ID to a zone ID
// DEPRECATED: Use zones.ResolveZoneIdentifier() instead
// This function is maintained for backwards compatibility and will be removed in future versions
func ResolveZoneIdentifier(client *api.Client, zoneIdentifier string) (string, error) {
	return zones.LegacyResolveZoneIdentifier(client, zoneIdentifier)
}
