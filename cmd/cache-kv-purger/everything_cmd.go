package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"fmt"
	"github.com/spf13/cobra"
)

// createPurgeEverythingCmd creates a command to purge everything
func createPurgeEverythingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "everything",
		Short: "Purge everything from cache",
		Long:  `Purge all cached files for a zone from Cloudflare's edge servers.`,
		Example: `  # Purge everything from a zone
  cache-kv-purger cache purge everything --zone example.com

  # Purge everything from multiple zones
  cache-kv-purger cache purge everything --zone example.com --zone example.org

  # Purge everything from all zones in an account
  cache-kv-purger cache purge everything --all-zones`,
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

			// Resolve zone identifiers (could be names or IDs)
			resolvedZoneIDs, err := resolveZoneIdentifiers(cmd, client, accountID)
			if err != nil {
				return err
			}

			// Track successes
			successCount := 0
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Purge everything for each zone
			for _, zoneID := range resolvedZoneIDs {
				if verbose {
					// Try to get the zone name for more informative output
					zoneInfo, err := getZoneInfo(client, zoneID)
					zoneName := zoneID
					if err == nil && zoneInfo.Result.Name != "" {
						zoneName = zoneInfo.Result.Name
					}
					fmt.Printf("Purging everything from zone %s...\n", zoneName)
				}

				// Make the API call to purge everything
				resp, err := cache.PurgeEverything(client, zoneID)
				if err != nil {
					fmt.Printf("Error purging zone %s: %s\n", zoneID, err)
					continue
				}

				// Report success
				if verbose {
					fmt.Printf("Successfully purged everything from zone %s. Purge ID: %s\n", zoneID, resp.Result.ID)
				}
				successCount++
			}

			// Final summary
			fmt.Printf("Successfully purged content from %d/%d zones\n", successCount, len(resolvedZoneIDs))
			return nil
		},
	}

	return cmd
}