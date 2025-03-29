package common

import (
	"cache-kv-purger/internal/config"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// ValidateAccountID ensures a valid account ID is available
func ValidateAccountID(cmd *cobra.Command, cfg *config.Config) (string, error) {
	// First try to get from flag
	accountID, _ := cmd.Flags().GetString("account-id")
	
	// If not in flag, try from config
	if accountID == "" && cfg != nil {
		accountID = cfg.GetAccountID()
	}
	
	// If still not found, return error
	if accountID == "" {
		return "", fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
	}
	
	return accountID, nil
}

// ValidateNamespaceID ensures a valid namespace ID is available
func ValidateNamespaceID(cmd *cobra.Command, cfg *config.Config, client interface{}, accountID string) (string, error) {
	// First try to get from flag
	nsID, _ := cmd.Flags().GetString("namespace-id")
	
	// If not in flag, try title
	if nsID == "" {
		title, _ := cmd.Flags().GetString("title")
		if title != "" {
			// We need to resolve the title to namespace ID
			apiClient, ok := client.(interface {
				FindNamespaceByTitle(accountID, title string) (interface{}, error)
			})
			if !ok {
				return "", fmt.Errorf("invalid client for namespace resolution")
			}
			
			ns, err := apiClient.FindNamespaceByTitle(accountID, title)
			if err != nil {
				return "", fmt.Errorf("failed to find namespace by title: %w", err)
			}
			
			// Extract the ID based on the returned type
			switch v := ns.(type) {
			case map[string]interface{}:
				if id, ok := v["id"].(string); ok {
					nsID = id
				}
			case interface{ GetID() string }:
				nsID = v.GetID()
			default:
				return "", fmt.Errorf("unknown namespace type returned")
			}
		}
	}
	
	// If still not found, return error
	if nsID == "" {
		return "", fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
	}
	
	return nsID, nil
}

// ValidateZoneID ensures a valid zone ID or name is available
// Returns the resolved zone ID
func ValidateZoneID(cmd *cobra.Command, cfg *config.Config, client interface{}, accountID string) (string, error) {
	// First try to get from flag
	zoneID, _ := cmd.Flags().GetString("zone")
	
	// If not in flag, try from config
	if zoneID == "" && cfg != nil {
		zoneID = cfg.GetZoneID()
	}
	
	// If still not found, return error
	if zoneID == "" {
		return "", fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
	}
	
	// If the client supports zone resolution, make sure we have the ID not the name
	if resolver, ok := client.(interface {
		ResolveZoneIdentifier(accountID, zone string) (string, error)
	}); ok {
		resolvedID, err := resolver.ResolveZoneIdentifier(accountID, zoneID)
		if err != nil {
			return "", fmt.Errorf("failed to resolve zone: %w", err)
		}
		return resolvedID, nil
	}
	
	// If no resolver available, just return the zone ID/name as is
	return zoneID, nil
}

// ResolveZoneIdentifiers resolves multiple zone identifiers from various sources
func ResolveZoneIdentifiers(cmd *cobra.Command, client interface{}, accountID string) ([]string, error) {
	// Flag to indicate if --all-zones was used
	allZones, _ := cmd.Flags().GetBool("all-zones")

	// If --all-zones is set, fetch all zones for the account
	if allZones {
		// Make sure we have an account ID
		if accountID == "" {
			return nil, fmt.Errorf("account ID is required for --all-zones, set it with CLOUDFLARE_ACCOUNT_ID or in config")
		}

		// Check if the client has ListZones method
		lister, ok := client.(interface {
			ListZones(accountID string) (interface{}, error)
		})
		if !ok {
			return nil, fmt.Errorf("client does not support listing zones")
		}

		// Fetch all zones
		result, err := lister.ListZones(accountID)
		if err != nil {
			return nil, fmt.Errorf("failed to list zones: %w", err)
		}

		// Extract zone IDs based on the returned type
		var zoneIDs []string
		switch v := result.(type) {
		case map[string]interface{}:
			if results, ok := v["result"].([]interface{}); ok {
				for _, zone := range results {
					if zoneMap, ok := zone.(map[string]interface{}); ok {
						if id, ok := zoneMap["id"].(string); ok {
							zoneIDs = append(zoneIDs, id)
						}
					}
				}
			}
		case interface{ GetZoneIDs() []string }:
			zoneIDs = v.GetZoneIDs()
		default:
			return nil, fmt.Errorf("unknown zone list result type")
		}

		if len(zoneIDs) == 0 {
			return nil, fmt.Errorf("no zones found for the account")
		}

		return zoneIDs, nil
	}

	// Check for individual zone flags
	zonesFromFlags, _ := cmd.Flags().GetStringSlice("zones")
	if len(zonesFromFlags) > 0 {
		resolver, ok := client.(interface {
			ResolveZoneIdentifier(accountID, zone string) (string, error)
		})
		if !ok {
			return nil, fmt.Errorf("client does not support zone resolution")
		}

		resolvedZones := make([]string, 0, len(zonesFromFlags))
		for _, zone := range zonesFromFlags {
			resolved, err := resolver.ResolveZoneIdentifier(accountID, zone)
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
		
		resolver, ok := client.(interface {
			ResolveZoneIdentifier(accountID, zone string) (string, error)
		})
		if !ok {
			return nil, fmt.Errorf("client does not support zone resolution")
		}

		resolvedZones := make([]string, 0, len(zoneItems))
		for _, zone := range zoneItems {
			// Trim whitespace
			zone = strings.TrimSpace(zone)
			if zone == "" {
				continue
			}
			resolved, err := resolver.ResolveZoneIdentifier(accountID, zone)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve zone %s: %w", zone, err)
			}
			resolvedZones = append(resolvedZones, resolved)
		}
		if len(resolvedZones) > 0 {
			return resolvedZones, nil
		}
	}

	// Fall back to single zone ID
	zoneID, err := ValidateZoneID(cmd, nil, client, accountID)
	if err != nil {
		return nil, err
	}

	return []string{zoneID}, nil
}