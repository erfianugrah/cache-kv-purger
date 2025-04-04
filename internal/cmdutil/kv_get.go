package cmdutil

import (
	"fmt"
	"os"
	"strings"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"

	"github.com/spf13/cobra"
)

// NewKVGetCommand creates a new get command for KV
func NewKVGetCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID   string
		namespaceID string
		namespace   string
		key         string
		bulk        bool
		keys        string
		prefix      string
		pattern     string
		searchValue string
		tagField    string
		tagValue    string
		metadata    bool
		outputFile  string
		outputJSON  bool
		batchSize   int
		concurrency int
	}

	// Create command
	return NewCommand("get", "Get values for keys in a namespace", `
Get values for one or more keys from a KV namespace.

When used with --key, gets a single key value.
When used with --bulk, gets multiple key values based on filters.
`).WithExample(`  # Get a single key
  cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --key mykey

  # Get a key with metadata
  cache-kv-purger kv get --namespace "My Namespace" --key mykey --metadata

  # Get multiple keys
  cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --bulk --keys "key1,key2,key3"

  # Get keys with prefix
  cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --bulk --prefix "product-" --metadata
`).WithStringFlag(
		"account-id", "", "Cloudflare account ID", &opts.accountID,
	).WithStringFlag(
		"namespace-id", "", "Namespace ID", &opts.namespaceID,
	).WithStringFlag(
		"namespace", "", "Namespace name (alternative to namespace-id)", &opts.namespace,
	).WithStringFlag(
		"key", "", "Key to get (required unless bulk operation)", &opts.key,
	).WithBoolFlag(
		"bulk", false, "Get multiple values based on filters", &opts.bulk,
	).WithStringFlag(
		"keys", "", "Comma-separated list of keys or @file.txt", &opts.keys,
	).WithStringFlag(
		"prefix", "", "Get keys with prefix (for bulk)", &opts.prefix,
	).WithStringFlag(
		"pattern", "", "Get keys matching regex pattern (for bulk)", &opts.pattern,
	).WithStringFlag(
		"search", "", "Get keys containing this value (for bulk)", &opts.searchValue,
	).WithStringFlag(
		"tag-field", "", "Get keys with this metadata field (for bulk)", &opts.tagField,
	).WithStringFlag(
		"tag-value", "", "Get keys with this metadata field/value (for bulk)", &opts.tagValue,
	).WithBoolFlag(
		"metadata", false, "Include metadata with values", &opts.metadata,
	).WithStringFlag(
		"file", "", "Write output to file instead of stdout", &opts.outputFile,
	).WithBoolFlag(
		"json", false, "Output as JSON", &opts.outputJSON,
	).WithIntFlag(
		"batch-size", 0, "Batch size for bulk operations", &opts.batchSize,
	).WithIntFlag(
		"concurrency", 0, "Concurrency for bulk operations", &opts.concurrency,
	).WithRunE(
		WithConfigAndClient(func(cmd *cobra.Command, args []string, cfg *config.Config, client *api.Client) error {
			// Resolve account ID
			accountID, err := common.ValidateAccountID(cmd, cfg, opts.accountID)
			if err != nil {
				return err
			}

			// Create KV service
			service := kv.NewKVService(client)

			// Handle namespace ID resolution if namespace name is provided
			if opts.namespace != "" && opts.namespaceID == "" {
				nsID, err := service.ResolveNamespaceID(cmd.Context(), accountID, opts.namespace)
				if err != nil {
					return fmt.Errorf("failed to resolve namespace: %w", err)
				}
				opts.namespaceID = nsID
			}

			// Validate that we have a namespace ID
			if opts.namespaceID == "" {
				return fmt.Errorf("namespace-id or namespace is required")
			}

			// Validate operation mode
			if !opts.bulk && opts.key == "" {
				return fmt.Errorf("either --key or --bulk is required")
			}

			// If bulk mode, validate we have something to fetch
			if opts.bulk && opts.keys == "" && opts.prefix == "" && opts.pattern == "" &&
				opts.searchValue == "" && opts.tagField == "" {
				return fmt.Errorf("bulk mode requires at least one filter (--keys, --prefix, --pattern, --search, or --tag-field)")
			}

			// Single key mode
			if !opts.bulk {
				key, err := service.Get(cmd.Context(), accountID, opts.namespaceID, opts.key, kv.ServiceGetOptions{
					IncludeMetadata: opts.metadata,
				})
				if err != nil {
					return fmt.Errorf("failed to get key: %w", err)
				}

				// Handle output
				if opts.outputJSON {
					return outputResult(key, opts.outputFile, true)
				}

				// If we're writing to a file, just write the raw value
				if opts.outputFile != "" {
					return os.WriteFile(opts.outputFile, []byte(key.Value), 0644)
				}

				// Otherwise, use formatted output with the common formatter
				data := make(map[string]string)
				data["Key"] = key.Key

				if key.Expiration > 0 {
					data["Expiration"] = fmt.Sprintf("%d", key.Expiration)
				}

				// Add metadata if available
				if key.Metadata != nil {
					metaStr := ""
					for k, v := range *key.Metadata {
						if metaStr != "" {
							metaStr += ", "
						}
						metaStr += fmt.Sprintf("%s: %v", k, v)
					}
					data["Metadata"] = metaStr
				}

				// Add the value
				if len(key.Value) > 200 {
					data["Value"] = fmt.Sprintf("(length: %d chars)\n%s", len(key.Value), key.Value)
				} else {
					data["Value"] = key.Value
				}

				common.FormatKeyValueTable(data)
				return nil
			}

			// Bulk mode - parse keys if provided
			var keys []string
			if opts.keys != "" {
				if strings.HasPrefix(opts.keys, "@") {
					// Load keys from file
					keysFile := strings.TrimPrefix(opts.keys, "@")
					keysData, err := os.ReadFile(keysFile)
					if err != nil {
						return fmt.Errorf("failed to read keys file: %w", err)
					}
					keys = strings.Split(strings.TrimSpace(string(keysData)), "\n")
				} else {
					// Parse comma-separated list
					keys = strings.Split(opts.keys, ",")
				}
			}

			// Prepare bulk get options
			bulkGetOptions := kv.BulkGetOptions{
				IncludeMetadata: opts.metadata,
				BatchSize:       opts.batchSize,
				Concurrency:     opts.concurrency,
				Prefix:          opts.prefix,
				Pattern:         opts.pattern,
			}

			// If we have search criteria, use search instead of bulk get
			var result []kv.KeyValuePair
			if opts.searchValue != "" || opts.tagField != "" {
				searchOptions := kv.SearchOptions{
					SearchValue:     opts.searchValue,
					TagField:        opts.tagField,
					TagValue:        opts.tagValue,
					IncludeMetadata: opts.metadata,
					BatchSize:       opts.batchSize,
					Concurrency:     opts.concurrency,
				}

				matchingKeys, err := service.Search(cmd.Context(), accountID, opts.namespaceID, searchOptions)
				if err != nil {
					return fmt.Errorf("search failed: %w", err)
				}

				// Now get the values for these keys
				result, err = service.BulkGet(cmd.Context(), accountID, opts.namespaceID,
					extractKeys(matchingKeys), bulkGetOptions)
				if err != nil {
					return fmt.Errorf("failed to get values for search results: %w", err)
				}
			} else if len(keys) > 0 {
				// Get by explicit keys
				result, err = service.BulkGet(cmd.Context(), accountID, opts.namespaceID, keys, bulkGetOptions)
				if err != nil {
					return fmt.Errorf("failed to get keys: %w", err)
				}
			} else if opts.prefix != "" || opts.pattern != "" {
				// Get by prefix or pattern
				// First list keys matching criteria
				listOptions := kv.ListOptions{
					Prefix:        opts.prefix,
					Pattern:       opts.pattern,
					IncludeValues: false,
				}

				listResult, err := service.List(cmd.Context(), accountID, opts.namespaceID, listOptions)
				if err != nil {
					return fmt.Errorf("failed to list keys: %w", err)
				}

				// Now get the values for these keys
				result, err = service.BulkGet(cmd.Context(), accountID, opts.namespaceID,
					extractKeys(listResult.Keys), bulkGetOptions)
				if err != nil {
					return fmt.Errorf("failed to get values for matching keys: %w", err)
				}
			}

			// Output results
			if opts.outputJSON {
				return outputResult(result, opts.outputFile, true)
			}

			// If we're writing to a file, format as JSONL
			if opts.outputFile != "" {
				var output strings.Builder
				for _, kv := range result {
					output.WriteString(fmt.Sprintf("%s\t%s\n", kv.Key, kv.Value))
				}
				return os.WriteFile(opts.outputFile, []byte(output.String()), 0644)
			}

			// Enhanced formatted output
			fmt.Printf("Retrieved %d keys:\n\n", len(result))

			for i, kv := range result {
				// Create a formatted key-value map for this entry
				data := make(map[string]string)
				data["Key"] = kv.Key

				if kv.Expiration > 0 {
					data["Expiration"] = fmt.Sprintf("%d", kv.Expiration)
				}

				// Format metadata nicely if available
				if kv.Metadata != nil {
					metaStr := ""
					for k, v := range *kv.Metadata {
						if metaStr != "" {
							metaStr += ", "
						}
						metaStr += fmt.Sprintf("%s: %v", k, v)
					}
					data["Metadata"] = metaStr
				}

				// Handle value (potentially truncate if very long)
				valueDisplay := kv.Value
				if len(valueDisplay) > 500 {
					// For very long values, truncate with indication
					valuePreview := valueDisplay[:250] + "\n...\n" + valueDisplay[len(valueDisplay)-250:]
					data["Value"] = fmt.Sprintf("(length: %d chars)\n%s", len(valueDisplay), valuePreview)
				} else {
					data["Value"] = valueDisplay
				}

				// Use the common formatter
				common.FormatKeyValueTable(data)

				// Add separator between items if not the last one
				if i < len(result)-1 {
					fmt.Println()
				}
			}

			return nil
		}),
	)
}

// Helper function to extract keys from a slice of KeyValuePair
func extractKeys(pairs []kv.KeyValuePair) []string {
	keys := make([]string, len(pairs))
	for i, pair := range pairs {
		keys[i] = pair.Key
	}
	return keys
}

// Helper function to output results to stdout or file
func outputResult(data interface{}, filePath string, asJSON bool) error {
	jsonData, err := common.ToJSON(data)
	if err != nil {
		return fmt.Errorf("failed to convert to JSON: %w", err)
	}

	if filePath != "" {
		return os.WriteFile(filePath, jsonData, 0644)
	}

	fmt.Println(string(jsonData))
	return nil
}
