package main

import (
	"fmt"
	"os"

	"cache-kv-purger/internal/config"
	"github.com/spf13/cobra"
)

// configCmd is the command for managing global configuration
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure global settings",
	Long:  `Set default zone, account ID, and other global settings.`,
}

// configDefaultsCmd is the command for setting default values
var configDefaultsCmd = &cobra.Command{
	Use:   "set-defaults",
	Short: "Set default values",
	Long:  `Set default values for zone ID, account ID, and API endpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load existing config
		cfg, err := config.LoadFromFile("")
		if err != nil {
			cfg = config.New()
		}

		// Get values from flags
		zoneID, _ := cmd.Flags().GetString("zone")
		accountID, _ := cmd.Flags().GetString("account-id")
		apiEndpoint, _ := cmd.Flags().GetString("api-endpoint")

		// Update config
		changed := false
		if zoneID != "" {
			cfg.DefaultZone = zoneID
			changed = true
		}
		if accountID != "" {
			cfg.AccountID = accountID
			changed = true
		}
		if apiEndpoint != "" {
			cfg.APIEndpoint = apiEndpoint
			changed = true
		}

		// Save config if changed
		if changed {
			if err := cfg.SaveToFile(""); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			fmt.Println("Successfully updated configuration.")
		} else {
			fmt.Println("No configuration changes were made.")
		}

		return nil
	},
}

// configShowCmd is the command for showing current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration values.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load existing config
		cfg, err := config.LoadFromFile("")
		if err != nil {
			cfg = config.New()
		}

		// Display config
		fmt.Println("Current configuration:")
		fmt.Printf("  API Endpoint: %s\n", cfg.APIEndpoint)

		// Zone ID (may come from env var, config, or neither)
		zoneID := cfg.GetZoneID()
		if zoneID != "" {
			if os.Getenv(config.EnvZoneID) != "" {
				fmt.Printf("  Default Zone ID: %s (from environment variable)\n", zoneID)
			} else {
				fmt.Printf("  Default Zone ID: %s\n", zoneID)
			}
		} else {
			fmt.Printf("  Default Zone ID: (not set)\n")
		}

		// Account ID (may come from env var, config, or neither)
		accountID := cfg.GetAccountID()
		if accountID != "" {
			if os.Getenv(config.EnvAccountID) != "" {
				fmt.Printf("  Default Account ID: %s (from environment variable)\n", accountID)
			} else {
				fmt.Printf("  Default Account ID: %s\n", accountID)
			}
		} else {
			fmt.Printf("  Default Account ID: (not set)\n")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configDefaultsCmd)
	configCmd.AddCommand(configShowCmd)

	// Add flags to set-defaults command
	configDefaultsCmd.Flags().String("zone", "", "Default zone ID")
	configDefaultsCmd.Flags().String("account-id", "", "Default account ID")
	configDefaultsCmd.Flags().String("api-endpoint", "", "API endpoint URL")
}
