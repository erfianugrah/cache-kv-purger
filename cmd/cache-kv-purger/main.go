package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command for the CLI application
var rootCmd = &cobra.Command{
	Use:   "cache-kv-purger",
	Short: "CLI tool for managing Cloudflare cache purging and KV operations",
	Long: `A command-line interface tool for managing Cloudflare cache purging and KV store operations.
This tool uses Cloudflare's API to perform various operations related to cache management
and KV store manipulation.`,
}

// main is the entry point for the application
func main() {
	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}