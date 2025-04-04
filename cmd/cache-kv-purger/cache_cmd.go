package main

import (
	"github.com/spf13/cobra"
)

// CacheCmd is the command for cache operations
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage Cloudflare cache",
	Long:  `Perform operations on the Cloudflare cache, such as purging files, tags, and more.`,
}

// PurgeCmd is the command for purging cache
var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Purge Cloudflare cache",
	Long:  `Purge cached content from Cloudflare's edge servers.`,
}

// purgeFlagsVars stores the variables for the purge command flags
var purgeFlagsVars struct {
	zoneID               string
	zones                []string // Support for multiple zones with --zones (repeated flag)
	zonesCSV             []string // Support for multiple zones with --zone-list (comma separated)
	purgeEverything      bool
	files                []string
	tags                 []string
	hosts                []string
	prefixes             []string
	cacheConcurrency     int // Concurrency for cache operations
	multiZoneConcurrency int // Concurrency for multi-zone operations
}

func init() {
	// Add purge command to cache command
	cacheCmd.AddCommand(purgeCmd)

	// Add purge subcommands to purge command
	purgeCmd.AddCommand(createPurgeEverythingCmd())
	purgeCmd.AddCommand(createPurgeFilesCmd())
	purgeCmd.AddCommand(createPurgeTagsCmd())
	purgeCmd.AddCommand(createPurgePrefixesCmd())
	purgeCmd.AddCommand(createPurgeHostsCmd())

	// Add cache command to root command
	rootCmd.AddCommand(cacheCmd)

	// Add global flags to purge command
	purgeCmd.PersistentFlags().StringVar(&purgeFlagsVars.zoneID, "zone", "", "Zone ID or name to purge content from")
	purgeCmd.PersistentFlags().StringArrayVar(&purgeFlagsVars.zones, "zones", []string{}, "Zone IDs or names to purge content from (can be specified multiple times)")
	purgeCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	purgeCmd.PersistentFlags().Bool("all-zones", false, "Purge content from all zones in the account")
	purgeCmd.PersistentFlags().String("zone-list", "", "Comma-delimited list of zone IDs or names to purge content from")
	purgeCmd.PersistentFlags().IntVar(&purgeFlagsVars.cacheConcurrency, "concurrency", 0, "Number of concurrent cache operations (default 10, max 20)")
	purgeCmd.PersistentFlags().IntVar(&purgeFlagsVars.multiZoneConcurrency, "zone-concurrency", 0, "Number of zones to process concurrently (default 3)")
	purgeCmd.PersistentFlags().Bool("dry-run", false, "Show what would be purged without actually purging")
}
