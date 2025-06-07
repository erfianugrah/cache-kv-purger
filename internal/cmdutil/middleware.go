package cmdutil

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
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

		// Check verbosity settings as well - this ensures all commands using this middleware
		// will respect the verbosity flags even if they don't use WithVerbose specifically
		verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")
		verboseFlag, _ := cmd.Flags().GetBool("verbose")

		// Set verbose environment flag for commands to check
		if verboseFlag || verbosityStr == "verbose" || verbosityStr == "debug" {
			if cfg != nil {
				cfg.SetValue("verbose", "true")
			}
		}

		if verbosityStr == "debug" {
			if cfg != nil {
				cfg.SetValue("debug", "true")
			}
		}

		return fn(cmd, args, cfg, client)
	}
}

// WithVerbose adds a verbose flag extractor to simplify checking verbose mode
// This original version is kept for backward compatibility
func WithVerbose(fn func(*cobra.Command, []string, bool, bool) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Check global verbosity flag (from root command)
		verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")

		// Check command-specific verbose flag
		verboseFlag, _ := cmd.Flags().GetBool("verbose")

		// Determine verbose and debug status - either flag can enable verbose mode
		verbose := verboseFlag || verbosityStr == "verbose" || verbosityStr == "debug"
		debug := verbosityStr == "debug"

		return fn(cmd, args, verbose, debug)
	}
}

// WithVerbosity wraps a command function with a standardized Verbosity object
func WithVerbosity(fn func(*cobra.Command, []string, *common.Verbosity) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Check global verbosity flag (from root command)
		verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")

		// Check command-specific verbose flag
		verboseFlag, _ := cmd.Flags().GetBool("verbose")

		// Parse verbosity level
		level := common.ParseVerbosityLevel(verbosityStr)

		// Override with verbose flag if set
		if verboseFlag && level < common.VerbosityVerbose {
			level = common.VerbosityVerbose
		}

		// Create the verbosity object
		verbosity := common.NewVerbosity(level)

		return fn(cmd, args, verbosity)
	}
}

// WithClientAndVerbosity wraps a command function to provide both an API client and Verbosity
func WithClientAndVerbosity(fn func(*cobra.Command, []string, *api.Client, *common.Verbosity) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Create API client
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Check global verbosity flag (from root command)
		verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")

		// Check command-specific verbose flag
		verboseFlag, _ := cmd.Flags().GetBool("verbose")

		// Parse verbosity level
		level := common.ParseVerbosityLevel(verbosityStr)

		// Override with verbose flag if set
		if verboseFlag && level < common.VerbosityVerbose {
			level = common.VerbosityVerbose
		}

		// Create the verbosity object
		verbosity := common.NewVerbosity(level)

		return fn(cmd, args, client, verbosity)
	}
}

// WithConfigClientAndVerbosity wraps a command function to provide config, client and verbosity
func WithConfigClientAndVerbosity(fn func(*cobra.Command, []string, *config.Config, *api.Client, *common.Verbosity) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := config.LoadFromFile("")
		if err != nil {
			// Create a default config if not found
			cfg = config.New()
		}

		// Create API client
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		// Check global verbosity flag (from root command)
		verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")

		// Check command-specific verbose flag
		verboseFlag, _ := cmd.Flags().GetBool("verbose")

		// Parse verbosity level
		level := common.ParseVerbosityLevel(verbosityStr)

		// Override with verbose flag if set
		if verboseFlag && level < common.VerbosityVerbose {
			level = common.VerbosityVerbose
		}

		// Create the verbosity object
		verbosity := common.NewVerbosity(level)

		// Set verbosity in config for backward compatibility
		if verbosity.IsVerbose() {
			cfg.SetValue("verbose", "true")
		}

		if verbosity.IsDebug() {
			cfg.SetValue("debug", "true")
		}

		return fn(cmd, args, cfg, client, verbosity)
	}
}
