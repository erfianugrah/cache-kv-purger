package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"fmt"
	"github.com/spf13/cobra"
	"os"
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
	zoneID          string
	zones           []string // Support for multiple zones with --zones (repeated flag)
	zonesCSV        []string // Support for multiple zones with --zone-list (comma separated)
	purgeEverything bool
	files           []string
	tags            []string
	hosts           []string
	prefixes        []string
}

// createPurgeEverythingCmd creates a command to purge everything
func createPurgeEverythingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "everything",
		Short: "Purge everything from cache",
		Long:  `Purge all cached files for a zone from Cloudflare's edge servers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Get account ID for resolving zone names
			accountID := ""
			cfg, err := config.LoadFromFile("")
			if err == nil {
				accountID = cfg.GetAccountID()
			}

			// Get zone IDs
			rawZones := purgeFlagsVars.zones

			// Add zones from comma-separated list as well
			if len(purgeFlagsVars.zonesCSV) > 0 {
				rawZones = append(rawZones, purgeFlagsVars.zonesCSV...)
			}
			resolvedZoneIDs := make([]string, 0)

			// If multiple zones specified, resolve each one (could be name or ID)
			if len(rawZones) > 0 {
				for _, rawZone := range rawZones {
					// Resolve each zone identifier
					zoneID, err := zones.ResolveZoneIdentifier(client, accountID, rawZone)
					if err != nil {
						fmt.Printf("Warning: %v\n", err)
						continue
					}
					resolvedZoneIDs = append(resolvedZoneIDs, zoneID)
				}

				if len(resolvedZoneIDs) == 0 {
					return fmt.Errorf("failed to resolve any valid zones from the provided identifiers")
				}
			} else {
				// No zones specified with --zones, try single zone flag
				zoneID := purgeFlagsVars.zoneID
				if zoneID == "" {
					// Try to get from the global flag
					zoneID, _ = cmd.Flags().GetString("zone")
				}

				if zoneID == "" {
					// Try to get from config or environment variable
					if cfg != nil {
						zoneID = cfg.GetZoneID()
					}
				}

				if zoneID == "" {
					return fmt.Errorf("zone ID is required, specify it with --zone flag, --zones flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
				}

				// Resolve zone (could be name or ID)
				resolvedZoneID, err := zones.ResolveZoneIdentifier(client, accountID, zoneID)
				if err != nil {
					return fmt.Errorf("failed to resolve zone: %w", err)
				}

				resolvedZoneIDs = []string{resolvedZoneID}
			}

			// Track successes
			successCount := 0
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Purge everything for each zone
			for _, zoneID := range resolvedZoneIDs {
				if verbose {
					// Try to get the zone name for more informative output
					zoneInfo, err := getZoneInfo(client, zoneID)
					if err == nil && zoneInfo != nil {
						fmt.Printf("Purging all cached content for zone %s (%s)...\n", zoneInfo.Name, zoneID)
					} else {
						fmt.Printf("Purging all cached content for zone %s...\n", zoneID)
					}
				}

				resp, err := cache.PurgeEverything(client, zoneID)
				if err != nil {
					fmt.Printf("Failed to purge cache for zone %s: %v\n", zoneID, err)
					continue
				}

				successCount++
				if verbose || len(resolvedZoneIDs) > 1 {
					// Try to get the zone name for more informative output
					zoneInfo, err := getZoneInfo(client, zoneID)
					if err == nil && zoneInfo != nil {
						fmt.Printf("Successfully purged all cache for zone %s (%s). Purge ID: %s\n",
							zoneInfo.Name, zoneID, resp.Result.ID)
					} else {
						fmt.Printf("Successfully purged all cache for zone %s. Purge ID: %s\n",
							zoneID, resp.Result.ID)
					}
				}
			}

			// Final summary if multiple zones were processed
			if len(resolvedZoneIDs) > 1 {
				fmt.Printf("Purge summary: Successfully purged cache from %d out of %d zones\n",
					successCount, len(resolvedZoneIDs))
			} else if successCount > 0 && !verbose {
				fmt.Printf("Successfully purged all cache.\n")
			}

			return nil
		},
	}

	return cmd
}

// createPurgeByOptions creates a command for purging with specific options
func createPurgeFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Purge specific files from cache",
		Long:  `Purge specific files from Cloudflare's edge servers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if files were provided
			if len(purgeFlagsVars.files) == 0 {
				return fmt.Errorf("at least one file URL is required, specify with --file")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Get account ID for resolving zone names
			accountID := ""
			cfg, err := config.LoadFromFile("")
			if err == nil {
				accountID = cfg.GetAccountID()
			}

			// Get zone IDs
			rawZones := purgeFlagsVars.zones
			resolvedZoneIDs := make([]string, 0)

			// If multiple zones specified, resolve each one (could be name or ID)
			if len(rawZones) > 0 {
				for _, rawZone := range rawZones {
					// Resolve each zone identifier
					zoneID, err := zones.ResolveZoneIdentifier(client, accountID, rawZone)
					if err != nil {
						fmt.Printf("Warning: %v\n", err)
						continue
					}
					resolvedZoneIDs = append(resolvedZoneIDs, zoneID)
				}

				if len(resolvedZoneIDs) == 0 {
					return fmt.Errorf("failed to resolve any valid zones from the provided identifiers")
				}
			} else {
				// No zones specified with --zones, try single zone flag
				zoneID := purgeFlagsVars.zoneID
				if zoneID == "" {
					// Try to get from the global flag
					zoneID, _ = cmd.Flags().GetString("zone")
				}

				if zoneID == "" {
					// Try to get from config or environment variable
					if cfg != nil {
						zoneID = cfg.GetZoneID()
					}
				}

				if zoneID == "" {
					return fmt.Errorf("zone ID is required, specify it with --zone flag, --zones flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
				}

				// Resolve zone (could be name or ID)
				resolvedZoneID, err := zones.ResolveZoneIdentifier(client, accountID, zoneID)
				if err != nil {
					return fmt.Errorf("failed to resolve zone: %w", err)
				}

				resolvedZoneIDs = []string{resolvedZoneID}
			}

			// Track successes
			successCount := 0
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Purge files for each zone
			for _, zoneID := range resolvedZoneIDs {
				if verbose {
					// Try to get the zone name for more informative output
					zoneInfo, err := getZoneInfo(client, zoneID)
					if err == nil && zoneInfo != nil {
						fmt.Printf("Purging %d files for zone %s (%s)...\n",
							len(purgeFlagsVars.files), zoneInfo.Name, zoneID)
					} else {
						fmt.Printf("Purging %d files for zone %s...\n",
							len(purgeFlagsVars.files), zoneID)
					}

					for i, file := range purgeFlagsVars.files {
						fmt.Printf("  %d. %s\n", i+1, file)
					}
				}

				resp, err := cache.PurgeFiles(client, zoneID, purgeFlagsVars.files)
				if err != nil {
					fmt.Printf("Failed to purge files for zone %s: %v\n", zoneID, err)
					continue
				}

				successCount++
				if verbose || len(resolvedZoneIDs) > 1 {
					// Try to get the zone name for more informative output
					zoneInfo, err := getZoneInfo(client, zoneID)
					if err == nil && zoneInfo != nil {
						fmt.Printf("Successfully purged %d files from zone %s (%s). Purge ID: %s\n",
							len(purgeFlagsVars.files), zoneInfo.Name, zoneID, resp.Result.ID)
					} else {
						fmt.Printf("Successfully purged %d files from zone %s. Purge ID: %s\n",
							len(purgeFlagsVars.files), zoneID, resp.Result.ID)
					}
				}
			}

			// Final summary if multiple zones were processed
			if len(resolvedZoneIDs) > 1 {
				fmt.Printf("Purge summary: Successfully purged files from %d out of %d zones\n",
					successCount, len(resolvedZoneIDs))
			} else if successCount > 0 && !verbose {
				fmt.Printf("Successfully purged %d files.\n", len(purgeFlagsVars.files))
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.files, "file", []string{}, "URL of a file to purge (can be specified multiple times)")
	if err := cmd.MarkFlagRequired("file"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'file' flag as required: %v\n", err)
	}

	return cmd
}

func createPurgeTagsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Purge cached content by cache tags",
		Long:  `Purge cached content from Cloudflare's edge servers based on Cache-Tag header values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the zone ID from flag, config, or environment variable
			zoneID := purgeFlagsVars.zoneID
			if zoneID == "" {
				// Try to get from the global flag
				zoneID, _ = cmd.Flags().GetString("zone")
			}

			if zoneID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					zoneID = cfg.GetZoneID()
				}
			}

			if zoneID == "" {
				return fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
			}

			// Check if tags were provided
			if len(purgeFlagsVars.tags) == 0 {
				return fmt.Errorf("at least one tag is required, specify with --tag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Purge tags
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Purging content with %d tags for zone %s...\n", len(purgeFlagsVars.tags), zoneID)
				for i, tag := range purgeFlagsVars.tags {
					fmt.Printf("  %d. %s\n", i+1, tag)
				}
			}

			resp, err := cache.PurgeTags(client, zoneID, purgeFlagsVars.tags)
			if err != nil {
				return fmt.Errorf("failed to purge tags: %w", err)
			}

			fmt.Printf("Successfully purged content with %d tags. Purge ID: %s\n", len(purgeFlagsVars.tags), resp.Result.ID)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.tags, "tag", []string{}, "Cache tag to purge (can be specified multiple times)")
	if err := cmd.MarkFlagRequired("tag"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'tag' flag as required: %v\n", err)
	}

	return cmd
}

func createPurgeHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "Purge cached content by hostname",
		Long:  `Purge cached content from Cloudflare's edge servers based on hostname.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the zone ID from flag, config, or environment variable
			zoneID := purgeFlagsVars.zoneID
			if zoneID == "" {
				// Try to get from the global flag
				zoneID, _ = cmd.Flags().GetString("zone")
			}

			if zoneID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					zoneID = cfg.GetZoneID()
				}
			}

			if zoneID == "" {
				return fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
			}

			// Check if hosts were provided
			if len(purgeFlagsVars.hosts) == 0 {
				return fmt.Errorf("at least one host is required, specify with --host")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Purge hosts
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Purging content for %d hosts in zone %s...\n", len(purgeFlagsVars.hosts), zoneID)
				for i, host := range purgeFlagsVars.hosts {
					fmt.Printf("  %d. %s\n", i+1, host)
				}
			}

			resp, err := cache.PurgeHosts(client, zoneID, purgeFlagsVars.hosts)
			if err != nil {
				return fmt.Errorf("failed to purge hosts: %w", err)
			}

			fmt.Printf("Successfully purged content for %d hosts. Purge ID: %s\n", len(purgeFlagsVars.hosts), resp.Result.ID)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.hosts, "host", []string{}, "Hostname to purge (can be specified multiple times)")
	if err := cmd.MarkFlagRequired("host"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'host' flag as required: %v\n", err)
	}

	return cmd
}

func createPurgePrefixesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prefixes",
		Short: "Purge cached content by URL prefix",
		Long:  `Purge cached content from Cloudflare's edge servers based on URL prefix.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the zone ID from flag, config, or environment variable
			zoneID := purgeFlagsVars.zoneID
			if zoneID == "" {
				// Try to get from the global flag
				zoneID, _ = cmd.Flags().GetString("zone")
			}

			if zoneID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					zoneID = cfg.GetZoneID()
				}
			}

			if zoneID == "" {
				return fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
			}

			// Check if prefixes were provided
			if len(purgeFlagsVars.prefixes) == 0 {
				return fmt.Errorf("at least one prefix is required, specify with --prefix")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Purge prefixes
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Purging content with %d prefixes for zone %s...\n", len(purgeFlagsVars.prefixes), zoneID)
				for i, prefix := range purgeFlagsVars.prefixes {
					fmt.Printf("  %d. %s\n", i+1, prefix)
				}
			}

			resp, err := cache.PurgePrefixes(client, zoneID, purgeFlagsVars.prefixes)
			if err != nil {
				return fmt.Errorf("failed to purge prefixes: %w", err)
			}

			fmt.Printf("Successfully purged content with %d prefixes. Purge ID: %s\n", len(purgeFlagsVars.prefixes), resp.Result.ID)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.prefixes, "prefix", []string{}, "URL prefix to purge (can be specified multiple times)")
	if err := cmd.MarkFlagRequired("prefix"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'prefix' flag as required: %v\n", err)
	}

	return cmd
}

func createPurgeCustomCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custom",
		Short: "Purge cached content with custom options",
		Long:  `Purge cached content from Cloudflare's edge servers with custom purge options.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the zone ID from flag, config, or environment variable
			zoneID := purgeFlagsVars.zoneID
			if zoneID == "" {
				// Try to get from the global flag
				zoneID, _ = cmd.Flags().GetString("zone")
			}

			if zoneID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					zoneID = cfg.GetZoneID()
				}
			}

			if zoneID == "" {
				return fmt.Errorf("zone ID is required, specify it with --zone flag, CLOUDFLARE_ZONE_ID environment variable, or set a default zone in config")
			}

			// Check if at least one option was provided
			if !purgeFlagsVars.purgeEverything &&
				len(purgeFlagsVars.files) == 0 &&
				len(purgeFlagsVars.tags) == 0 &&
				len(purgeFlagsVars.hosts) == 0 &&
				len(purgeFlagsVars.prefixes) == 0 {
				return fmt.Errorf("at least one purge parameter (--everything, --file, --tag, --host, --prefix) must be specified")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Create purge options
			options := cache.PurgeOptions{
				PurgeEverything: purgeFlagsVars.purgeEverything,
				Files:           purgeFlagsVars.files,
				Tags:            purgeFlagsVars.tags,
				Hosts:           purgeFlagsVars.hosts,
				Prefixes:        purgeFlagsVars.prefixes,
			}

			// Purge with custom options
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Purging cache for zone %s with custom options:\n", zoneID)
				if options.PurgeEverything {
					fmt.Println("  - Purging everything")
				}
				if len(options.Files) > 0 {
					fmt.Println("  - Files:")
					for i, file := range options.Files {
						fmt.Printf("    %d. %s\n", i+1, file)
					}
				}
				if len(options.Tags) > 0 {
					fmt.Println("  - Tags:")
					for i, tag := range options.Tags {
						fmt.Printf("    %d. %s\n", i+1, tag)
					}
				}
				if len(options.Hosts) > 0 {
					fmt.Println("  - Hosts:")
					for i, host := range options.Hosts {
						fmt.Printf("    %d. %s\n", i+1, host)
					}
				}
				if len(options.Prefixes) > 0 {
					fmt.Println("  - Prefixes:")
					for i, prefix := range options.Prefixes {
						fmt.Printf("    %d. %s\n", i+1, prefix)
					}
				}
			}

			resp, err := cache.PurgeCache(client, zoneID, options)
			if err != nil {
				return fmt.Errorf("failed to purge cache: %w", err)
			}

			fmt.Printf("Successfully purged cache. Purge ID: %s\n", resp.Result.ID)
			return nil
		},
	}

	cmd.Flags().BoolVar(&purgeFlagsVars.purgeEverything, "everything", false, "Purge everything")
	cmd.Flags().StringArrayVar(&purgeFlagsVars.files, "file", []string{}, "URL of a file to purge (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&purgeFlagsVars.tags, "tag", []string{}, "Cache tag to purge (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&purgeFlagsVars.hosts, "host", []string{}, "Hostname to purge (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&purgeFlagsVars.prefixes, "prefix", []string{}, "URL prefix to purge (can be specified multiple times)")

	return cmd
}

// getZoneInfo retrieves zone information for a given zone ID
func getZoneInfo(client *api.Client, zoneID string) (*api.Zone, error) {
	// Use zones list to get all zones, then find the matching one
	zonesList, err := zones.ListZones(client, "")
	if err != nil {
		return nil, err
	}

	// Find the zone with the matching ID
	for _, zone := range zonesList {
		if zone.ID == zoneID {
			return &zone, nil
		}
	}

	return nil, fmt.Errorf("zone not found with ID: %s", zoneID)
}

func init() {
	rootCmd.AddCommand(cacheCmd)
	cacheCmd.AddCommand(purgeCmd)

	// Add zone ID flags to purge command, but keep the global flag as well
	purgeCmd.PersistentFlags().StringVar(&purgeFlagsVars.zoneID, "zone-id", "", "Cloudflare Zone ID or domain name")
	purgeCmd.PersistentFlags().StringArrayVar(&purgeFlagsVars.zones, "zones", []string{}, "Multiple Cloudflare Zone IDs or domain names (can be specified multiple times)")
	purgeCmd.PersistentFlags().StringSliceVar(&purgeFlagsVars.zonesCSV, "zone-list", []string{}, "Comma-separated list of Cloudflare Zone IDs or domain names")

	// Add purge subcommands
	purgeCmd.AddCommand(createPurgeEverythingCmd())
	purgeCmd.AddCommand(createPurgeFilesCmd())
	purgeCmd.AddCommand(createPurgeTagsCmd())
	purgeCmd.AddCommand(createPurgeHostsCmd())
	purgeCmd.AddCommand(createPurgePrefixesCmd())
	purgeCmd.AddCommand(createPurgeCustomCmd())
}

