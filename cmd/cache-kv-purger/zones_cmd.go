package main

import (
	"fmt"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"github.com/spf13/cobra"
)

// zonesCmd is the command for zone operations
var zonesCmd = &cobra.Command{
	Use:   "zones",
	Short: "Manage Cloudflare zones",
	Long:  `List and select Cloudflare zones for use with other commands.`,
}

// zonesListCmd is the command for listing zones
var zonesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all zones",
	Long:  `List all zones available for your account.`,
	RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
		// Get account ID from flag, config, or environment variable
		accountID, _ := cmd.Flags().GetString("account-id")
		if accountID == "" {
			// Try to get from config or environment variable
			cfg, err := config.LoadFromFile("")
			if err == nil {
				accountID = cfg.GetAccountID()
			}
		}

		// Create API client
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// List zones
		if verbose {
			if accountID != "" {
				fmt.Printf("Listing zones for account %s...\n", accountID)
			} else {
				fmt.Println("Listing all zones...")
			}
		}

		zones, err := zones.ListZones(client, accountID)
		if err != nil {
			return fmt.Errorf("failed to list zones: %w", err)
		}

		// Output result
		if len(zones.Result) == 0 {
			fmt.Println("No zones found")
			return nil
		}

		fmt.Printf("Found %d zones:\n", len(zones.Result))
		for i, zone := range zones.Result {
			fmt.Printf("%d. %s (ID: %s, Status: %s)\n", i+1, zone.Name, zone.ID, zone.Status)
		}

		return nil
	}),
}

// zonesGetCmd is the command for getting a zone by name
var zonesGetCmd = &cobra.Command{
	Use:   "get [domain]",
	Short: "Get a zone by domain name",
	Long:  `Get a zone's details by its domain name.`,
	Args:  cobra.ExactArgs(1),
	RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
		// Get domain name from arguments
		domainName := args[0]

		// Get account ID from flag, config, or environment variable
		accountID, _ := cmd.Flags().GetString("account-id")
		if accountID == "" {
			// Try to get from config or environment variable
			cfg, err := config.LoadFromFile("")
			if err == nil {
				accountID = cfg.GetAccountID()
			}
		}

		// Create API client
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Get zone
		if verbose {
			fmt.Printf("Looking up zone for domain '%s'...\n", domainName)
		}

		zone, err := zones.GetZoneByName(client, accountID, domainName)
		if err != nil {
			return fmt.Errorf("failed to get zone: %w", err)
		}

		// Output result
		fmt.Printf("Zone Information:\n")
		fmt.Printf("  Name: %s\n", zone.Name)
		fmt.Printf("  ID: %s\n", zone.ID)
		fmt.Printf("  Status: %s\n", zone.Status)
		fmt.Printf("  Type: %s\n", zone.Type)

		if len(zone.NameServers) > 0 {
			fmt.Printf("  Nameservers:\n")
			for i, ns := range zone.NameServers {
				fmt.Printf("    %d. %s\n", i+1, ns)
			}
		}

		// Suggest how to use this zone ID
		fmt.Printf("\nTo use this zone for commands:\n")
		fmt.Printf("  - Set it as default: cache-kv-purger config set-defaults --zone %s\n", zone.ID)
		fmt.Printf("  - Use it in a command: cache-kv-purger cache purge everything --zone %s\n", zone.ID)
		fmt.Printf("  - Set environment variable: export CLOUDFLARE_ZONE_ID=%s\n", zone.ID)

		return nil
	}),
}

// zonesConfigCmd is the command for setting a default zone
var zonesConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure default zone",
	Long:  `Set a default zone to use for cache operations.`,
	RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
		// Load existing config
		cfg, err := config.LoadFromFile("")
		if err != nil {
			cfg = config.New()
		}

		// Get zone identifier
		zoneIdentifier, _ := cmd.Flags().GetString("zone-id")
		if zoneIdentifier == "" {
			zoneIdentifier, _ = cmd.Flags().GetString("zone")
		}

		// Update config
		if zoneIdentifier != "" {
			// Check if this is a domain name that needs to be resolved
			if len(zoneIdentifier) != 32 || !isHexString(zoneIdentifier) {
				// Create API client
				client, err := api.NewClient()
				if err != nil {
					return fmt.Errorf("failed to create API client: %w", err)
				}

				// Try to get account ID
				accountID := cfg.GetAccountID()

				// Try to resolve the zone
				resolvedZoneID, err := zones.ResolveZoneIdentifier(client, accountID, zoneIdentifier)
				if err != nil {
					return fmt.Errorf("failed to resolve zone '%s': %w", zoneIdentifier, err)
				}

				// Get the zone info to show the domain name
				zoneInfo, err := zones.GetZoneDetails(client, resolvedZoneID)
				if err == nil && zoneInfo != nil {
					fmt.Printf("Resolved zone name '%s' to ID: %s\n", zoneIdentifier, resolvedZoneID)
				}

				zoneIdentifier = resolvedZoneID
			}

			cfg.DefaultZone = zoneIdentifier

			// Save updated config
			if err := cfg.SaveToFile(""); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Successfully set default zone ID to '%s'\n", zoneIdentifier)
		} else {
			// Just display the current settings
			zoneID := cfg.GetZoneID()

			if zoneID != "" {
				// Try to get the zone name for more informative output
				client, err := api.NewClient()
				if err == nil {
					zoneInfo, err := zones.GetZoneDetails(client, zoneID)
					if err == nil && zoneInfo != nil {
						fmt.Printf("Current default zone: %s (%s)\n", zoneInfo.Result.Name, zoneID)
						return nil
					}
				}

				fmt.Printf("Current default zone: %s\n", zoneID)
			} else {
				fmt.Printf("No default zone configured.\n")
			}
		}

		return nil
	}),
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

func init() {
	rootCmd.AddCommand(zonesCmd)
	zonesCmd.AddCommand(zonesListCmd)
	zonesCmd.AddCommand(zonesGetCmd)
	zonesCmd.AddCommand(zonesConfigCmd)

	// Add flags
	zonesListCmd.Flags().String("account-id", "", "Account ID to list zones for")
	zonesGetCmd.Flags().String("account-id", "", "Account ID to search within")
	zonesConfigCmd.Flags().String("zone-id", "", "Zone ID or domain name to set as default")
	zonesConfigCmd.Flags().String("zone", "", "Zone ID or domain name to set as default (alias for zone-id)")
}
