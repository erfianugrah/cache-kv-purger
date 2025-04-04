package common

import (
	"cache-kv-purger/internal/api"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// ResolveZoneIdentifier resolves a zone name or ID to a zone ID
func ResolveZoneIdentifier(client *api.Client, zoneIdentifier string) (string, error) {
	// Check if it's already a zone ID (simple validation for hexadecimal format)
	hexPattern := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if hexPattern.MatchString(zoneIdentifier) {
		return zoneIdentifier, nil
	}

	// If it contains dots, assume it's a domain name and try to look it up
	if strings.Contains(zoneIdentifier, ".") {
		// Get the zone ID from the domain name
		zonesPath := fmt.Sprintf("/zones?name=%s", zoneIdentifier)
		zonesResp, err := client.Request(http.MethodGet, zonesPath, nil, nil)
		if err != nil {
			return "", fmt.Errorf("failed to list zones: %w", err)
		}

		// Parse the response
		var zonesData struct {
			Success bool `json:"success"`
			Result  []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"result"`
		}

		if err := json.Unmarshal(zonesResp, &zonesData); err != nil {
			return "", fmt.Errorf("failed to parse zones response: %w", err)
		}

		if !zonesData.Success || len(zonesData.Result) == 0 {
			return "", fmt.Errorf("no zone found with name: %s", zoneIdentifier)
		}

		// Return the zone ID
		for _, zone := range zonesData.Result {
			if strings.EqualFold(zone.Name, zoneIdentifier) {
				return zone.ID, nil
			}
		}

		// If we get here, we didn't find an exact match
		return "", fmt.Errorf("no exact match for zone name: %s", zoneIdentifier)
	}

	// If not a valid format for ID or domain, return an error
	return "", fmt.Errorf("invalid zone identifier format: %s (must be a valid zone ID or domain name)", zoneIdentifier)
}
