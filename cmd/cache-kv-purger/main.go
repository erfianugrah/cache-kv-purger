package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cache-kv-purger",
	Short: "CLI tool for managing Cloudflare cache purging and KV operations",
	Long: `A command-line interface tool for managing Cloudflare cache purging and KV store operations.
This tool uses Cloudflare's API to perform various operations related to cache management
and KV store manipulation.`,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringP("zone", "z", "", "Cloudflare Zone ID or domain name (required for most commands)")
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}