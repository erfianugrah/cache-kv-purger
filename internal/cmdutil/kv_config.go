package cmdutil

import (
	"fmt"

	"cache-kv-purger/internal/config"

	"github.com/spf13/cobra"
)

// NewKVConfigCommand creates a new config command for KV
func NewKVConfigCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID     string
		show          bool
	}

	// Create command
	return NewCommand("config", "Configure KV settings", `
Configure default settings for KV operations.

When used with --account-id, sets the default account ID for KV operations.
When used with --show, displays the current configuration.
`).WithExample(`  # Set default account ID
  cache-kv-purger kv config --account-id YOUR_ACCOUNT_ID

  # Show current configuration
  cache-kv-purger kv config --show
`).WithStringFlag(
		"account-id", "", "Set default account ID for KV operations", &opts.accountID,
	).WithBoolFlag(
		"show", false, "Show current configuration", &opts.show,
	).WithRunE(
		func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := config.LoadFromFile("")
			if err != nil {
				cfg = config.New()
			}

			// If showing config, display it
			if opts.show {
				fmt.Println("KV Configuration:")
				fmt.Printf("Default Account ID: %s\n", cfg.AccountID)
				return nil
			}

			// If setting account ID
			if opts.accountID != "" {
				cfg.AccountID = opts.accountID
				if err := cfg.SaveToFile(""); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}
				fmt.Printf("Default account ID set to: %s\n", opts.accountID)
				return nil
			}

			// If no operation specified, show help
			return cmd.Help()
		},
	)
}