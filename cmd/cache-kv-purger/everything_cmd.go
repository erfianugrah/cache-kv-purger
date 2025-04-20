package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/cmdutil"
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
		RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
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

			// Get zone concurrency limit
			zoneConcurrency := purgeFlagsVars.multiZoneConcurrency
			if zoneConcurrency <= 0 {
				zoneConcurrency = 3 // Default
			} else if zoneConcurrency > 10 {
				zoneConcurrency = 10 // Maximum to avoid overwhelming API
			}

			if debug {
				fmt.Printf("Using zone concurrency of %d\n", zoneConcurrency)
			}

			// Create a result channel for completed zones
			type zoneResult struct {
				zoneID     string
				zoneName   string
				successful bool
				purgeID    string
				err        error
			}

			resultChan := make(chan zoneResult, len(resolvedZoneIDs))

			// Use a semaphore to limit concurrent zone processing
			sem := make(chan struct{}, zoneConcurrency)

			// Process all zones concurrently with semaphore control
			for _, zoneID := range resolvedZoneIDs {
				// Acquire semaphore slot
				sem <- struct{}{}

				// Launch a goroutine to process this zone
				go func(zID string) {
					defer func() { <-sem }() // Release semaphore when done

					// Get zone name for reporting
					zoneName := zID
					if verbose {
						zoneInfo, err := getZoneInfo(client, zID)
						if err == nil && zoneInfo.Result.Name != "" {
							zoneName = zoneInfo.Result.Name
						}
						fmt.Printf("Purging everything from zone %s...\n", zoneName)
					}

					// Make the API call to purge everything
					resp, err := cache.PurgeEverything(client, zID)
					if err != nil {
						resultChan <- zoneResult{
							zoneID:     zID,
							zoneName:   zoneName,
							successful: false,
							err:        err,
						}
						return
					}

					// Return success
					resultChan <- zoneResult{
						zoneID:     zID,
						zoneName:   zoneName,
						successful: true,
						purgeID:    resp.Result.ID,
					}
				}(zoneID)
			}

			// Collect results from all zones
			for i := 0; i < len(resolvedZoneIDs); i++ {
				result := <-resultChan

				if result.err != nil {
					fmt.Printf("Error purging zone %s: %s\n", result.zoneID, result.err)
				} else {
					if verbose {
						fmt.Printf("Successfully purged everything from zone %s. Purge ID: %s\n", result.zoneName, result.purgeID)
					}
					successCount++
				}
			}

			// Final summary
			fmt.Printf("Successfully purged content from %d/%d zones\n", successCount, len(resolvedZoneIDs))
			return nil
		}),
	}

	return cmd
}