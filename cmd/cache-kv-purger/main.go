package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Version information (set by ldflags during build)
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// rootCmd is the base command for the CLI application
var rootCmd = &cobra.Command{
	Use:   "cache-kv-purger",
	Short: "CLI tool for managing Cloudflare cache purging and KV operations",
	Long: `A command-line interface tool for managing Cloudflare cache purging and KV store operations.
This tool uses Cloudflare's API to perform various operations related to cache management
and KV store manipulation.`,
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cache-kv-purger version %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
	},
}

func init() {
	// Add version command
	rootCmd.AddCommand(versionCmd)

	// Add global flags
	rootCmd.PersistentFlags().String("verbosity", "normal", "Verbosity level: quiet, normal, verbose, debug. Overrides command-specific --verbose flags")
	rootCmd.PersistentFlags().StringP("zone", "z", "", "Cloudflare Zone ID or domain name (required for most commands)")
	rootCmd.PersistentFlags().Bool("version", false, "Print version information")

	// Initialize default rate limits
	initializeRateLimits()

	// Demo commands disabled for release build
}

// initializeRateLimits sets up default rate limits
func initializeRateLimits() {
	// Import required package
	_ = "cache-kv-purger/internal/common"
	// Rate limits are initialized when first used
}

// setupCommandValidation recursively adds help and flag validation to all commands
func setupCommandValidation(cmd *cobra.Command) {
	// Add special handling for help flag (-h/--help)
	// Store original function to create our validator
	original := cmd.PersistentPreRunE
	originalHelp := cmd.HelpFunc()

	// Replace the help function to prioritize it over everything else
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Use the original help function
		originalHelp(cmd, args)
	})

	// Add our validator that prioritizes help
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Check if --help is present anywhere in the arguments
		for _, arg := range os.Args {
			if arg == "--help" || arg == "-h" {
				_ = cmd.Help()
				os.Exit(0)
			}
		}

		// Check for version flag (only for root command)
		if cmd == rootCmd {
			if v, _ := cmd.Flags().GetBool("version"); v {
				fmt.Printf("cache-kv-purger version %s\n", version)
				fmt.Printf("  commit: %s\n", commit)
				fmt.Printf("  built:  %s\n", date)
				os.Exit(0)
			}
		}

		// Continue with original pre-run if it exists
		if original != nil {
			return original(cmd, args)
		}
		return nil
	}

	// Add validation to all command's flags
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		// For string flags that should have values, add validation
		if flag.Value.Type() == "string" {
			// Add better error message for string flags
			flag.Usage += " (requires a value)"
		}
	})

	// Recurse into subcommands
	for _, subCmd := range cmd.Commands() {
		setupCommandValidation(subCmd)
	}
}

// main is the entry point for the application
func main() {
	// Add validation to all commands
	// Import pflag for the validation
	_ = os.Args // Force import of os to avoid issues

	// Apply validation to all commands
	setupCommandValidation(rootCmd)

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		// Skip error output for --help requests
		if err.Error() != "help requested" {
			fmt.Println(err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
