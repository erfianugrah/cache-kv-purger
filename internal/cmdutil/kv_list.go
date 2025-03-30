package cmdutil

import (
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
		accountID     string
		namespaceID   string
		namespace     string
		key           string
		prefix        string
		pattern       string
		limit         int
		cursor        string
		metadata      bool
		values        bool
		searchValue   string
		tagField      string
		tagValue      string
		batchSize     int
		concurrency   int
		outputJSON    bool
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

  # Search for keys containing a value
  cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-image"
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
		"search", "", "Search for keys containing this value", &opts.searchValue,
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
				namespaces, err := service.ListNamespaces(cmd.Context(), accountID)
				if err != nil {
					return fmt.Errorf("failed to list namespaces: %w", err)
				}

				// Display results
				if opts.outputJSON {
					return common.OutputJSON(namespaces)
				}

				// Table format
				fmt.Println("Namespaces:")
				fmt.Println("ID\tTitle")
				fmt.Println("-------------------------------------------------")
				for _, ns := range namespaces {
					fmt.Printf("%s\t%s\n", ns.ID, ns.Title)
				}
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

				// Simple format
				fmt.Printf("Key: %s\n", key.Key)
				if key.Expiration > 0 {
					fmt.Printf("Expiration: %d\n", key.Expiration)
				}
				if key.Metadata != nil {
					fmt.Println("Metadata:")
					for k, v := range *key.Metadata {
						fmt.Printf("  %s: %v\n", k, v)
					}
				}
				if opts.values {
					fmt.Println("Value:")
					fmt.Println(key.Value)
				}
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

				searchOptions := kv.SearchOptions{
					SearchValue:     opts.searchValue,
					TagField:        opts.tagField,
					TagValue:        opts.tagValue,
					IncludeMetadata: opts.metadata,
					BatchSize:       opts.batchSize,
					Concurrency:     opts.concurrency,
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
				fmt.Printf("Found %d matching keys:\n", len(keys))
				fmt.Println("Key\tExpiration")
				fmt.Println("-------------------------------------------------")
				for _, key := range keys {
					expStr := ""
					if key.Expiration > 0 {
						expStr = fmt.Sprintf("%d", key.Expiration)
					}
					fmt.Printf("%s\t%s\n", key.Key, expStr)
				}
				return nil
			}

			// List keys
			result, err := service.List(cmd.Context(), accountID, opts.namespaceID, listOptions)
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
			}

			// Display results
			if opts.outputJSON {
				return common.OutputJSON(result)
			}

			// Table format
			fmt.Printf("Keys in namespace (%d):\n", len(result.Keys))
			fmt.Println("Key\tExpiration")
			fmt.Println("-------------------------------------------------")
			for _, key := range result.Keys {
				expStr := ""
				if key.Expiration > 0 {
					expStr = fmt.Sprintf("%d", key.Expiration)
				}
				fmt.Printf("%s\t%s\n", key.Key, expStr)
			}

			if result.Cursor != "" {
				fmt.Printf("\nMore keys available. Use --cursor '%s' to see the next page.\n", result.Cursor)
			}

			return nil
		}),
	)
}