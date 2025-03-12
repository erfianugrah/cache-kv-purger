package zones

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"cache-kv-purger/internal/api"
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
	
	// Parse the response
	var zonesResp ZoneListResponse
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
	
	return &zonesResp, nil
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
	var zonesResp ZoneListResponse
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

	var detailsResp ZoneDetailsResponse
	if err := json.Unmarshal(respBody, &detailsResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !detailsResp.Success {
		errorStr := "API reported failure"
		if len(detailsResp.Errors) > 0 {
			errorStr = detailsResp.Errors[0].Message
		}
		return nil, fmt.Errorf("failed to get zone details: %s", errorStr)
	}

	return &detailsResp, nil
}