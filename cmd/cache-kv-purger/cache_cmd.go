package main

import (
	"bytes"
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cache"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/zones"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
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

// extractTagsFromJSON extracts tag strings from JSON data
func extractTagsFromJSON(data []byte, jsonField string, verbose bool) ([]string, error) {
	var tags []string

	// Try to unmarshal as array of strings first
	err := json.Unmarshal(data, &tags)
	if err == nil {
		// Successfully parsed as string array
		return tags, nil
	}

	// Try to unmarshal as array of objects
	var objArray []map[string]interface{}
	err = json.Unmarshal(data, &objArray)
	if err == nil {
		// Successfully parsed as object array
		// If no field specified and objects contain a "tag" or "name" field, use that
		field := jsonField
		if field == "" {
			// Try to autodetect field
			if len(objArray) > 0 {
				// Check common field names
				commonFields := []string{"tag", "name", "id", "key"}
				for _, f := range commonFields {
					if _, ok := objArray[0][f]; ok {
						field = f
						if verbose {
							fmt.Printf("Auto-detected JSON field '%s' for tags\n", field)
						}
						break
					}
				}

				if field == "" {
					// List available fields
					var fields []string
					for k := range objArray[0] {
						fields = append(fields, k)
					}
					return nil, fmt.Errorf("JSON contains objects but no field specified. Available fields: %s", strings.Join(fields, ", "))
				}
			}
		}

		// Extract values from the specified field
		for _, obj := range objArray {
			if val, ok := obj[field]; ok {
				// Convert to string
				switch v := val.(type) {
				case string:
					tags = append(tags, v)
				case float64:
					tags = append(tags, fmt.Sprintf("%g", v))
				case int:
					tags = append(tags, fmt.Sprintf("%d", v))
				case bool:
					tags = append(tags, fmt.Sprintf("%t", v))
				default:
					// Skip complex objects
					continue
				}
			}
		}
		return tags, nil
	}

	// Try to unmarshal as a single object with array field
	var obj map[string]interface{}
	err = json.Unmarshal(data, &obj)
	if err == nil {
		// Successfully parsed as object
		// Try to find an array field
		for k, v := range obj {
			// If field is specified, only use that field
			if jsonField != "" && k != jsonField {
				continue
			}

			// Check if value is an array
			if arr, ok := v.([]interface{}); ok {
				for _, item := range arr {
					// Convert array items to strings
					switch i := item.(type) {
					case string:
						tags = append(tags, i)
					case float64:
						tags = append(tags, fmt.Sprintf("%g", i))
					case int:
						tags = append(tags, fmt.Sprintf("%d", i))
					case map[string]interface{}:
						// For objects in the array, try to extract by field name
						if jsonField != "" {
							if tagVal, ok := i[jsonField].(string); ok {
								tags = append(tags, tagVal)
							}
						}
					}
				}
			}
		}

		if len(tags) > 0 {
			return tags, nil
		}
	}

	return nil, fmt.Errorf("could not parse JSON data as array of strings, array of objects, or object with array field")
}

// extractTagsFromCSV extracts tags from CSV data
func extractTagsFromCSV(data []byte, columnName string, verbose bool) ([]string, error) {
	// Use CSV reader for parsing
	reader := csv.NewReader(bytes.NewReader(data))

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	// Get headers from first row
	headers := records[0]

	// Determine column index to use
	colIndex := 0 // Default to first column

	if columnName != "" {
		// Find specified column
		found := false
		for i, header := range headers {
			if strings.EqualFold(strings.TrimSpace(header), strings.TrimSpace(columnName)) {
				colIndex = i
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("column '%s' not found in CSV. Available columns: %s",
				columnName, strings.Join(headers, ", "))
		}
	}

	if verbose {
		fmt.Printf("Using CSV column '%s' (index %d) for tags\n", headers[colIndex], colIndex)
	}

	// Extract values from the specified column
	var tags []string
	for i, record := range records {
		// Skip header row
		if i == 0 {
			continue
		}

		// Skip rows without enough columns
		if len(record) <= colIndex {
			continue
		}

		// Get tag value and trim whitespace
		tag := strings.TrimSpace(record[colIndex])
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

// createPurgeTagsBatchCmd creates a command for purging tags in batches
func createPurgeTagsBatchCmd() *cobra.Command {
	// Create variables for additional flags specific to this command
	var tagsFile string
	var csvColumn string
	var jsonField string
	var commaDelimitedTags string

	cmd := &cobra.Command{
		Use:   "tags-batch",
		Short: "Purge cached content by cache tags in batches",
		Long:  `Purge cached content from Cloudflare's edge servers based on Cache-Tag header values, handling batches of up to 30 tags per API call automatically.`,
		Example: `  # Purge a large number of tags using comma-delimited list (easier than many --tag flags)
  cache-kv-purger cache purge tags-batch --zone example.com --tags "product-tag-1,product-tag-2,product-tag-3,product-tag-4"
  
  # Purge tags using individual --tag flags
  cache-kv-purger cache purge tags-batch --zone example.com --tag product-tag-1 --tag product-tag-2 --tag product-tag-3
  
  # Purge tags from a text file (one tag per line)
  cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.txt
  
  # Purge tags from a CSV file (specify which column contains tags)
  cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.csv --csv-column "tag_name"
  
  # Purge tags from a JSON file (array of strings)
  cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.json
  
  # Purge tags from a JSON file (array of objects, specify which field contains the tag)
  cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.json --json-field "name"
  
  # Purge across multiple zones
  cache-kv-purger cache purge tags-batch --zones example.com --zones example.org --tag product-tag-1 --tag product-tag-2`,
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

			// Prepare final list of tags
			allTags := make([]string, 0)

			// Add tags from individual --tag flags
			allTags = append(allTags, purgeFlagsVars.tags...)

			// Add tags from comma-delimited string if provided
			if commaDelimitedTags != "" {
				// Split by comma and process each tag
				for _, tag := range strings.Split(commaDelimitedTags, ",") {
					// Trim whitespace
					tag = strings.TrimSpace(tag)
					if tag != "" {
						allTags = append(allTags, tag)
					}
				}

				verbose, _ := cmd.Flags().GetBool("verbose")
				if verbose {
					fmt.Printf("Added %d tags from comma-delimited list\n", len(strings.Split(commaDelimitedTags, ",")))
				}
			}

			// Add tags from file if specified
			if tagsFile != "" {
				// Read file
				data, err := os.ReadFile(tagsFile)
				if err != nil {
					return fmt.Errorf("failed to read tags file: %w", err)
				}

				// Determine file format based on extension
				fileExt := strings.ToLower(filepath.Ext(tagsFile))

				verbose, _ := cmd.Flags().GetBool("verbose")

				switch fileExt {
				case ".json":
					// Process JSON format
					tagsFromJSON, err := extractTagsFromJSON(data, jsonField, verbose)
					if err != nil {
						return fmt.Errorf("failed to parse JSON file: %w", err)
					}
					allTags = append(allTags, tagsFromJSON...)

				case ".csv":
					// Process CSV format
					tagsFromCSV, err := extractTagsFromCSV(data, csvColumn, verbose)
					if err != nil {
						return fmt.Errorf("failed to parse CSV file: %w", err)
					}
					allTags = append(allTags, tagsFromCSV...)

				default:
					// Default to text format (one tag per line)
					fileLines := strings.Split(string(data), "\n")
					for _, line := range fileLines {
						// Trim whitespace and skip empty lines or comments
						tag := strings.TrimSpace(line)
						if tag != "" && !strings.HasPrefix(tag, "#") {
							allTags = append(allTags, tag)
						}
					}
				}

				if verbose {
					fmt.Printf("Extracted %d tags from %s\n", len(allTags), tagsFile)
				}
			}

			// Check if we have any tags
			if len(allTags) == 0 {
				return fmt.Errorf("at least one tag is required, specify with --tag, --tags, or --tags-file")
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Prepared to purge %d unique tags\n", len(allTags))
			}

			// Get zone IDs
			rawZones := purgeFlagsVars.zones

			// Add zones from comma-separated list as well
			if len(purgeFlagsVars.zonesCSV) > 0 {
				rawZones = append(rawZones, purgeFlagsVars.zonesCSV...)
			}

			// Process either multiple zones or a single zone
			if len(rawZones) > 0 {
				// Process multiple zones
				resolvedZoneIDs := make([]string, 0)

				for _, rawZone := range rawZones {
					// Resolve each zone identifier
					zoneID, err := zones.ResolveZoneIdentifier(client, accountID, rawZone)
					if err != nil {
						fmt.Printf("Warning: %v\n", err)
						continue
					}
					resolvedZoneIDs = append(resolvedZoneIDs, zoneID)

					if verbose {
						// Try to get the zone name for more informative output
						zoneInfo, err := getZoneInfo(client, zoneID)
						if err == nil && zoneInfo != nil {
							fmt.Printf("Resolved zone '%s' to ID: %s\n", rawZone, zoneID)
						}
					}
				}

				if len(resolvedZoneIDs) == 0 {
					return fmt.Errorf("failed to resolve any valid zones from the provided identifiers")
				}

				// Create progress callback
				progressCallback := func(zoneIndex, totalZones, batchesDone, totalBatches, successfulCount int) {
					if verbose {
						fmt.Printf("Progress: Zone %d/%d - Batch %d/%d complete\n",
							zoneIndex, totalZones, batchesDone, totalBatches)
					}
				}

				// Purge tags across all zones
				fmt.Printf("Purging %d tags across %d zones in batches of up to 30 tags per request...\n",
					len(allTags), len(resolvedZoneIDs))

				successByZone, errorsByZone := cache.PurgeTagsAcrossZonesInBatches(
					client, resolvedZoneIDs, allTags, progressCallback)

				// Report results
				totalSuccess := 0
				totalErrors := 0

				fmt.Println("\nPurge Summary:")
				for _, zoneID := range resolvedZoneIDs {
					zoneInfo, _ := getZoneInfo(client, zoneID)
					zoneName := zoneID
					if zoneInfo != nil {
						zoneName = fmt.Sprintf("%s (%s)", zoneInfo.Name, zoneID)
					}

					successTags := successByZone[zoneID]
					errors := errorsByZone[zoneID]

					if len(successTags) > 0 {
						fmt.Printf("  ✓ Zone %s: Successfully purged %d/%d tags\n",
							zoneName, len(successTags), len(allTags))
						totalSuccess++
					}

					if len(errors) > 0 {
						fmt.Printf("  ✗ Zone %s: Failed with %d errors\n", zoneName, len(errors))
						if verbose {
							for i, err := range errors {
								fmt.Printf("    Error %d: %v\n", i+1, err)
							}
						}
						totalErrors++
					}
				}

				// Final summary
				fmt.Printf("\nFinal Results: %d zones succeeded, %d zones had errors\n",
					totalSuccess, totalErrors)

				if totalErrors > 0 && !verbose {
					fmt.Println("Use --verbose flag to see detailed error messages")
				}

				if totalErrors > 0 {
					return fmt.Errorf("failed to purge tags on some zones")
				}

			} else {
				// Process single zone
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

				// Create progress callback
				progressCallback := func(batchCompleted, batchTotal, successfulCount int) {
					if verbose {
						fmt.Printf("Progress: Batch %d/%d complete (%d tags purged)\n",
							batchCompleted, batchTotal, successfulCount)
					}
				}

				// Get zone info for display
				zoneInfo, _ := getZoneInfo(client, resolvedZoneID)
				displayZone := resolvedZoneID
				if zoneInfo != nil {
					displayZone = fmt.Sprintf("%s (%s)", zoneInfo.Name, resolvedZoneID)
				}

				// Purge tags in batches
				fmt.Printf("Purging %d tags for zone %s in batches of up to 30 tags per request...\n",
					len(allTags), displayZone)

				successful, errors := cache.PurgeTagsInBatches(client, resolvedZoneID, allTags, progressCallback)

				// Report results
				fmt.Printf("\nSuccessfully purged %d of %d tags\n", len(successful), len(allTags))

				if len(errors) > 0 {
					fmt.Printf("Encountered %d errors during purge\n", len(errors))
					if verbose {
						for i, err := range errors {
							fmt.Printf("Error %d: %v\n", i+1, err)
						}
					} else {
						fmt.Println("Use --verbose flag to see detailed error messages")
					}
					return fmt.Errorf("failed to purge all tags")
				}
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&purgeFlagsVars.tags, "tag", []string{}, "Cache tag to purge (can be specified multiple times)")
	cmd.Flags().StringVar(&commaDelimitedTags, "tags", "", "Comma-delimited list of cache tags to purge (e.g., \"tag1,tag2,tag3\")")
	cmd.Flags().StringVar(&tagsFile, "tags-file", "", "File containing tags to purge (txt, csv, or json format)")
	cmd.Flags().StringVar(&csvColumn, "csv-column", "", "Column name containing tags in CSV file (default: first column)")
	cmd.Flags().StringVar(&jsonField, "json-field", "", "Field name containing tag in JSON objects (if JSON contains objects)")

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
	purgeCmd.AddCommand(createPurgeTagsBatchCmd()) // Add our new command here
	purgeCmd.AddCommand(createPurgeHostsCmd())
	purgeCmd.AddCommand(createPurgePrefixesCmd())
	purgeCmd.AddCommand(createPurgeCustomCmd())
}
