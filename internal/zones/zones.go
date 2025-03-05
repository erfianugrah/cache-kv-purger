package zones

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cache-kv-purger/internal/api"
)

// ListZones retrieves all zones for an account
func ListZones(client *api.Client, accountID string) ([]api.Zone, error) {
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
	
	// Parse the response
	var zonesResp api.ZonesResponse
	if err := json.Unmarshal(respBody, &zonesResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}
	
	if !zonesResp.Success {
		errorStr := "API reported failure"
		if len(zonesResp.Errors) > 0 {
			errorStr = zonesResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to list zones: %s", errorStr)
	}
	
	return zonesResp.Result, nil
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
	
	// Parse the response
	var zonesResp api.ZonesResponse
	if err := json.Unmarshal(respBody, &zonesResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}
	
	if !zonesResp.Success {
		errorStr := "API reported failure"
		if len(zonesResp.Errors) > 0 {
			errorStr = zonesResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to find zone: %s", errorStr)
	}
	
	if len(zonesResp.Result) == 0 {
		return nil, fmt.Errorf("no zone found with name '%s'", name)
	}
	
	return &zonesResp.Result[0], nil
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
		zones, err := ListZones(client, accountID)
		if err != nil {
			return "", fmt.Errorf("failed to list zones: %w", err)
		}
		
		// Look for a parent domain of the specified name
		domainParts := strings.Split(identifier, ".")
		for i := 1; i < len(domainParts); i++ {
			// Try each possible parent domain
			parentDomain := strings.Join(domainParts[i:], ".")
			
			// Check if this is a valid zone
			for _, zone := range zones {
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