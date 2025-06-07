package cmdutil

import (
	"context"
	"fmt"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"

	"github.com/spf13/cobra"
)

// NewKVListCommand creates a new list command for KV
func NewKVListCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID   string
		namespaceID string
		namespace   string
		key         string
		prefix      string
		pattern     string
		limit       int
		cursor      string
		metadata    bool
		values      bool
		searchValue string
		tagField    string
		tagValue    string
		batchSize   int
		concurrency int
		outputJSON  bool
		verbose     bool
		debug       bool
		all         bool
	}

	// Create command
	return NewCommand("list", "List namespaces or keys in a namespace", `
List KV namespaces or keys in a namespace.

When used without --namespace-id or --namespace, lists all namespaces in the account.
When used with --namespace-id or --namespace, lists keys in the specified namespace.
`).WithExample(`  # List all namespaces
  cache-kv-purger kv list --account-id YOUR_ACCOUNT_ID

  # List keys in a namespace by ID
  cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID

  # List keys in a namespace by name
  cache-kv-purger kv list --namespace "My Namespace"

  # List keys with a prefix
  cache-kv-purger kv list --namespace "My Namespace" --prefix "product-"

  # List keys with metadata
  cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --metadata

  # Search for keys containing a value (deep recursive search in metadata)
  cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-image"
  
  # Search for keys with specific metadata field
  cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --tag-field "status" --tag-value "archived"
`).WithStringFlag(
		"account-id", "", "Cloudflare account ID", &opts.accountID,
	).WithStringFlag(
		"namespace-id", "", "Namespace ID to list keys from", &opts.namespaceID,
	).WithStringFlag(
		"namespace", "", "Namespace name (alternative to namespace-id)", &opts.namespace,
	).WithStringFlag(
		"key", "", "Get details about a specific key", &opts.key,
	).WithStringFlag(
		"prefix", "", "Filter keys by prefix", &opts.prefix,
	).WithStringFlag(
		"pattern", "", "Filter keys by regex pattern", &opts.pattern,
	).WithIntFlag(
		"limit", 0, "Maximum number of items to return", &opts.limit,
	).WithStringFlag(
		"cursor", "", "Pagination cursor", &opts.cursor,
	).WithBoolFlag(
		"metadata", false, "Include metadata with keys", &opts.metadata,
	).WithBoolFlag(
		"values", false, "Include values with keys (slower for large result sets)", &opts.values,
	).WithStringFlag(
		"search", "", "Search for keys containing this value (deep recursive search in metadata)", &opts.searchValue,
	).WithStringFlag(
		"tag-field", "", "Metadata field to filter by", &opts.tagField,
	).WithStringFlag(
		"tag-value", "", "Value to match in the tag field", &opts.tagValue,
	).WithIntFlag(
		"batch-size", 0, "Batch size for bulk operations", &opts.batchSize,
	).WithIntFlag(
		"concurrency", 0, "Number of concurrent operations", &opts.concurrency,
	).WithBoolFlag(
		"json", false, "Output as JSON", &opts.outputJSON,
	).WithBoolFlag(
		"verbose", false, "Enable verbose output", &opts.verbose,
	).WithBoolFlag(
		"debug", false, "Enable debug output", &opts.debug,
	).WithBoolFlag(
		"all", false, "Fetch all keys (automatically handle pagination)", &opts.all,
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

			// If namespace ID is not provided, list namespaces
			if opts.namespaceID == "" {
				// Create a context with verbosity flags
				verboseCtx := context.WithValue(cmd.Context(), common.VerboseKey, opts.verbose)
				ctx := context.WithValue(verboseCtx, common.DebugKey, opts.debug)

				// List namespaces with the enhanced context
				namespaces, err := service.ListNamespaces(ctx, accountID)
				if err != nil {
					return fmt.Errorf("failed to list namespaces: %w", err)
				}

				// Display results
				if opts.outputJSON {
					return common.OutputJSON(namespaces)
				}

				// Table format
				headers := []string{"ID", "Title"}
				rows := make([][]string, len(namespaces))
				for i, ns := range namespaces {
					rows[i] = []string{ns.ID, ns.Title}
				}
				fmt.Printf("Namespaces (%d):\n", len(namespaces))
				common.FormatTable(headers, rows)
				return nil
			}

			// If a specific key is requested, get that key
			if opts.key != "" {
				key, err := service.Get(cmd.Context(), accountID, opts.namespaceID, opts.key, kv.ServiceGetOptions{
					IncludeMetadata: opts.metadata,
				})
				if err != nil {
					return fmt.Errorf("failed to get key: %w", err)
				}

				// Display result
				if opts.outputJSON {
					return common.OutputJSON(key)
				}

				// Simple format using key-value table
				data := make(map[string]string)
				data["Key"] = key.Key
				if key.Expiration > 0 {
					data["Expiration"] = fmt.Sprintf("%d", key.Expiration)
				}

				// Show metadata if available
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

				// Show value if requested
				if opts.values {
					// Truncate long values for display
					valueDisplay := key.Value
					if len(valueDisplay) > 100 {
						valueDisplay = valueDisplay[:97] + "..."
					}
					data["Value"] = valueDisplay
				}

				common.FormatKeyValueTable(data)
				return nil
			}

			// List keys in the namespace
			listOptions := kv.ListOptions{
				Limit:           opts.limit,
				Cursor:          opts.cursor,
				Prefix:          opts.prefix,
				Pattern:         opts.pattern,
				IncludeMetadata: opts.metadata,
				IncludeValues:   opts.values,
			}

			// If we have search criteria, use search instead of list
			if opts.searchValue != "" || opts.tagField != "" {
				var keys []kv.KeyValuePair
				var err error

				// Check for the enhanced "deep search" capability
				fmt.Println("Searching for keys...")

				searchOptions := kv.SearchOptions{
					SearchValue:     opts.searchValue,
					TagField:        opts.tagField,
					TagValue:        opts.tagValue,
					IncludeMetadata: opts.metadata,
					BatchSize:       opts.batchSize,
					Concurrency:     opts.concurrency,
				}

				// If search value provided without tag field, indicate we're doing a deep recursive search
				if opts.searchValue != "" && opts.tagField == "" {
					fmt.Printf("Performing deep recursive metadata search for '%s'...\n", opts.searchValue)
				} else if opts.tagField != "" {
					if opts.tagValue != "" {
						fmt.Printf("Searching for keys with metadata field '%s' matching '%s'...\n", opts.tagField, opts.tagValue)
					} else {
						fmt.Printf("Searching for keys with metadata field '%s'...\n", opts.tagField)
					}
				}

				keys, err = service.Search(cmd.Context(), accountID, opts.namespaceID, searchOptions)
				if err != nil {
					return fmt.Errorf("search failed: %w", err)
				}

				// Display results
				if opts.outputJSON {
					return common.OutputJSON(keys)
				}

				// Table format
				fmt.Printf("\nFound %d matching keys:\n", len(keys))

				// If we have no results, exit early
				if len(keys) == 0 {
					fmt.Println("No keys match the search criteria.")
					return nil
				}

				// Prepare table data
				var headers []string
				var rows [][]string

				if opts.metadata {
					headers = []string{"Key", "Expiration", "Metadata"}
					rows = make([][]string, len(keys))

					for i, key := range keys {
						expStr := ""
						if key.Expiration > 0 {
							expStr = fmt.Sprintf("%d", key.Expiration)
						}

						metaStr := "<none>"
						if key.Metadata != nil {
							metaStr = fmt.Sprintf("%v", *key.Metadata)
						}

						rows[i] = []string{key.Key, expStr, metaStr}
					}
				} else {
					headers = []string{"Key", "Expiration"}
					rows = make([][]string, len(keys))

					for i, key := range keys {
						expStr := ""
						if key.Expiration > 0 {
							expStr = fmt.Sprintf("%d", key.Expiration)
						}
						rows[i] = []string{key.Key, expStr}
					}
				}

				common.FormatTable(headers, rows)

				// Include note about metadata
				if !opts.metadata && len(keys) > 0 {
					fmt.Println("\nTip: Use --metadata to see metadata for these keys")
				}

				return nil
			}

			// List keys
			var keys []kv.KeyValuePair
			var hasMore bool
			var currentCursor string

			if opts.all {
				keys, err = service.ListAll(cmd.Context(), accountID, opts.namespaceID, listOptions)
				if err != nil {
					return fmt.Errorf("failed to list keys: %w", err)
				}
			} else {
				result, err := service.List(cmd.Context(), accountID, opts.namespaceID, listOptions)
				if err != nil {
					return fmt.Errorf("failed to list keys: %w", err)
				}
				keys = result.Keys
				hasMore = result.Cursor != ""
				currentCursor = result.Cursor
			}

			// Display results
			if opts.outputJSON {
				return common.OutputJSON(keys)
			}

			// Table format
			fmt.Printf("Keys in namespace (%d):\n", len(keys))

			// Prepare table data
			var headers []string
			var rows [][]string

			if opts.metadata {
				headers = []string{"Key", "Expiration", "Metadata"}
				rows = make([][]string, len(keys))

				for i, key := range keys {
					expStr := ""
					if key.Expiration > 0 {
						expStr = fmt.Sprintf("%d", key.Expiration)
					}

					metaStr := "<none>"
					if key.Metadata != nil {
						metaStr = fmt.Sprintf("%v", *key.Metadata)
					}

					rows[i] = []string{key.Key, expStr, metaStr}
				}
			} else {
				headers = []string{"Key", "Expiration"}
				rows = make([][]string, len(keys))

				for i, key := range keys {
					expStr := ""
					if key.Expiration > 0 {
						expStr = fmt.Sprintf("%d", key.Expiration)
					}
					rows[i] = []string{key.Key, expStr}
				}
			}

			common.FormatTable(headers, rows)

			// Include note about metadata if appropriate
			if !opts.metadata && len(keys) > 0 {
				fmt.Println("\nTip: Use --metadata to see metadata information")
			}

			if hasMore && !opts.all {
				fmt.Printf("\nMore keys available. Use --cursor '%s' to see the next page, or use --all to fetch all keys.\n", currentCursor)
			}

			return nil
		}),
	)
}
