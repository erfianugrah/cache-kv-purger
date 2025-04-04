package cmdutil

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/config"
	"fmt"
	"github.com/spf13/cobra"
)

// WithConfig wraps a command function to provide a config
func WithConfig(fn func(*cobra.Command, []string, *config.Config) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadFromFile("")
		if err != nil {
			// Create a default config if not found
			cfg = config.New()
		}

		return fn(cmd, args, cfg)
	}
}

// WithClient wraps a command function to provide an API client
func WithClient(fn func(*cobra.Command, []string, *api.Client) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		return fn(cmd, args, client)
	}
}

// WithConfigAndClient wraps a command function to provide both config and client
func WithConfigAndClient(fn func(*cobra.Command, []string, *config.Config, *api.Client) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadFromFile("")
		if err != nil {
			// Create a default config if not found
			cfg = config.New()
		}

		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		return fn(cmd, args, cfg, client)
	}
}

// WithVerbose adds a verbose flag extractor to simplify checking verbose mode
func WithVerbose(fn func(*cobra.Command, []string, bool) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")
		return fn(cmd, args, verbose)
	}
}
