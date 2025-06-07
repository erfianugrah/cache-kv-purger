package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"

	"github.com/spf13/cobra"
)

// NewKVDeleteCommand creates a new delete command for KV
func NewKVDeleteCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID       string
		namespaceID     string
		namespace       string
		key             string
		namespaceItself bool
		bulk            bool
		keys            string
		keysFile        string
		prefix          string
		pattern         string
		searchValue     string
		tagField        string
		tagValue        string
		allKeys         bool
		dryRun          bool
		force           bool
		batchSize       int
		concurrency     int
	}

	// Create command
	return NewCommand("delete", "Delete keys or namespaces", `
Delete one or more keys from a KV namespace, or delete a namespace itself.

When used with --key, deletes a single key.
When used with --namespace-itself, deletes the namespace itself.
When used with --bulk, deletes multiple keys based on filters.
`).WithExample(`  # Delete a single key
  cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --key mykey

  # Delete the namespace itself
  cache-kv-purger kv delete --namespace "My Namespace" --namespace-itself

  # Delete all keys with a prefix (with dry run)
  cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --prefix "temp-" --dry-run
  
  # Delete all keys in the namespace
  cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --all-keys

  # Delete keys by metadata (with confirmation)
  cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --tag-field "status" --tag-value "archived"

  # Smart search and delete (powerful recursive metadata search)
  cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --search "product-tag"
`).WithStringFlag(
		"account-id", "", "Cloudflare account ID", &opts.accountID,
	).WithStringFlag(
		"namespace-id", "", "Namespace ID", &opts.namespaceID,
	).WithStringFlag(
		"namespace", "", "Namespace name (alternative to namespace-id)", &opts.namespace,
	).WithStringFlag(
		"key", "", "Key to delete (required unless bulk deletion or namespace deletion)", &opts.key,
	).WithBoolFlag(
		"namespace-itself", false, "Delete the namespace itself (not keys)", &opts.namespaceItself,
	).WithBoolFlag(
		"bulk", false, "Delete multiple keys based on filters", &opts.bulk,
	).WithStringFlag(
		"keys", "", "Comma-separated list of keys", &opts.keys,
	).WithStringFlag(
		"keys-file", "", "File containing keys (one per line)", &opts.keysFile,
	).WithStringFlag(
		"prefix", "", "Delete keys with prefix", &opts.prefix,
	).WithStringFlag(
		"pattern", "", "Delete keys matching regex pattern", &opts.pattern,
	).WithStringFlag(
		"search", "", "Delete keys containing this value (deep recursive search in metadata)", &opts.searchValue,
	).WithStringFlag(
		"tag-field", "", "Delete keys with this metadata field", &opts.tagField,
	).WithStringFlag(
		"tag-value", "", "Delete keys with this metadata field/value", &opts.tagValue,
	).WithBoolFlag(
		"all-keys", false, "Delete all keys in the namespace", &opts.allKeys,
	).WithBoolFlag(
		"dry-run", false, "Show what would be deleted without deleting", &opts.dryRun,
	).WithBoolFlag(
		"force", false, "Skip confirmation prompt", &opts.force,
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

			// If we're deleting the namespace itself, that's a separate operation
			if opts.namespaceItself {
				// Get namespace info for confirmation
				namespaces, err := service.ListNamespaces(cmd.Context(), accountID)
				if err != nil {
					return fmt.Errorf("failed to list namespaces: %w", err)
				}

				var nsTitle string
				for _, ns := range namespaces {
					if ns.ID == opts.namespaceID {
						nsTitle = ns.Title
						break
					}
				}

				if nsTitle == "" {
					return fmt.Errorf("namespace with ID %s not found", opts.namespaceID)
				}

				// Confirm deletion unless --force is used
				if !opts.force {
					fmt.Printf("You are about to delete the namespace '%s' (%s) and ALL of its keys. This action cannot be undone.\n", nsTitle, opts.namespaceID)
					fmt.Print("Are you sure? (y/N): ")

					reader := bufio.NewReader(os.Stdin)
					confirmation, _ := reader.ReadString('\n')
					confirmation = strings.TrimSpace(strings.ToLower(confirmation))

					if confirmation != "y" && confirmation != "yes" {
						fmt.Println("Deletion cancelled.")
						return nil
					}
				}

				if opts.dryRun {
					fmt.Printf("DRY RUN: Would delete namespace '%s' (%s)\n", nsTitle, opts.namespaceID)
					return nil
				}

				// Delete the namespace
				err = service.DeleteNamespace(cmd.Context(), accountID, opts.namespaceID)
				if err != nil {
					return fmt.Errorf("failed to delete namespace: %w", err)
				}

				fmt.Printf("Successfully deleted namespace '%s' (%s)\n", nsTitle, opts.namespaceID)
				return nil
			}

			// Otherwise, we're deleting keys, either single or bulk
			if !opts.bulk {
				// Single key mode validation
				if opts.key == "" {
					return fmt.Errorf("key is required for single key operations")
				}

				// Confirm deletion unless --force is used
				if !opts.force {
					fmt.Printf("You are about to delete the key '%s'. This action cannot be undone.\n", opts.key)
					fmt.Print("Are you sure? (y/N): ")

					reader := bufio.NewReader(os.Stdin)
					confirmation, _ := reader.ReadString('\n')
					confirmation = strings.TrimSpace(strings.ToLower(confirmation))

					if confirmation != "y" && confirmation != "yes" {
						fmt.Println("Deletion cancelled.")
						return nil
					}
				}

				if opts.dryRun {
					fmt.Printf("DRY RUN: Would delete key '%s'\n", opts.key)
					return nil
				}

				// Delete the key
				err := service.Delete(cmd.Context(), accountID, opts.namespaceID, opts.key)
				if err != nil {
					return fmt.Errorf("failed to delete key: %w", err)
				}

				fmt.Printf("Successfully deleted key '%s'\n", opts.key)
				return nil
			}

			// Bulk mode - get keys to delete
			var keys []string

			// If explicit keys are provided
			if opts.keys != "" {
				keys = strings.Split(opts.keys, ",")
			} else if opts.keysFile != "" {
				// Read from file
				fileData, err := os.ReadFile(opts.keysFile)
				if err != nil {
					return fmt.Errorf("failed to read keys file: %w", err)
				}
				lines := strings.Split(string(fileData), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						keys = append(keys, line)
					}
				}
			}

			// Check if we have filtering criteria without explicit keys
			// Note: An empty prefix means match all keys when explicitly provided
			prefixSpecified := opts.prefix != "" || cmd.Flags().Changed("prefix")
			hasFilteringCriteria := prefixSpecified || opts.pattern != "" || opts.tagField != "" || opts.tagValue != "" || opts.searchValue != "" || opts.allKeys

			// Check for the enhanced "deep search" capability
			if opts.searchValue != "" && opts.tagField == "" {
				// This is a deep recursive metadata search (similar to the old search command)
				// This is more powerful when you don't know the exact metadata structure

				// Use the service.Search directly
				searchOptions := kv.SearchOptions{
					SearchValue:     opts.searchValue,
					IncludeMetadata: true,
					BatchSize:       opts.batchSize,
					Concurrency:     opts.concurrency,
				}

				// Find matching keys first
				matchingKeys, err := service.Search(cmd.Context(), accountID, opts.namespaceID, searchOptions)
				if err != nil {
					return fmt.Errorf("search operation failed: %w", err)
				}

				if len(matchingKeys) == 0 {
					fmt.Println("No keys found matching the search criteria.")
					return nil
				}

				// Extract key names only for deletion
				keyNames := make([]string, len(matchingKeys))
				for i, key := range matchingKeys {
					keyNames[i] = key.Key
				}

				// Confirm deletion unless --force is used
				if !opts.force {
					fmt.Printf("Found %d keys matching '%s'.\n", len(keyNames), opts.searchValue)
					fmt.Println("Sample matched keys:")

					// Show the first few keys as samples
					sampleSize := 5
					if len(keyNames) < sampleSize {
						sampleSize = len(keyNames)
					}

					for i := 0; i < sampleSize; i++ {
						fmt.Printf("  - %s\n", keyNames[i])
					}

					if len(keyNames) > sampleSize {
						fmt.Printf("  - ... and %d more\n", len(keyNames)-sampleSize)
					}

					fmt.Print("\nAre you sure you want to delete these keys? This action cannot be undone. [y/N]: ")

					reader := bufio.NewReader(os.Stdin)
					confirmation, _ := reader.ReadString('\n')
					confirmation = strings.TrimSpace(strings.ToLower(confirmation))

					if confirmation != "y" && confirmation != "yes" {
						fmt.Println("Deletion cancelled.")
						return nil
					}
				}

				if opts.dryRun {
					fmt.Printf("DRY RUN: Would delete %d keys matching '%s'.\n", len(keyNames), opts.searchValue)
					return nil
				}

				// Delete the keys
				// Get verbosity flags
				verbosityStr, _ := cmd.Flags().GetString("verbosity")
				verbose := false
				debug := false

				switch verbosityStr {
				case "quiet":
					// No output
				case "verbose":
					verbose = true
				case "debug":
					verbose = true
					debug = true
				default:
					// Normal mode
				}

				deleteOptions := kv.BulkDeleteOptions{
					BatchSize:   opts.batchSize,
					Concurrency: opts.concurrency,
					DryRun:      false, // We handle dry run above
					Force:       true,  // We already confirmed
					Verbose:     verbose,
					Debug:       debug,
				}

				count, err := service.BulkDelete(cmd.Context(), accountID, opts.namespaceID, keyNames, deleteOptions)
				if err != nil {
					return fmt.Errorf("bulk delete operation failed: %w", err)
				}

				fmt.Printf("Successfully deleted %d/%d keys matching '%s'\n", count, len(keyNames), opts.searchValue)
				return nil
			}

			// Regular bulk delete with options
			// prefixSpecified was already defined above, reuse it

			// Get verbosity flags
			verbosityStr, _ := cmd.Flags().GetString("verbosity")
			verbose := false
			debug := false

			switch verbosityStr {
			case "quiet":
				// No output
			case "verbose":
				verbose = true
			case "debug":
				verbose = true
				debug = true
			default:
				// Normal mode
			}

			bulkDeleteOptions := kv.BulkDeleteOptions{
				BatchSize:       opts.batchSize,
				Concurrency:     opts.concurrency,
				DryRun:          opts.dryRun,
				Force:           opts.force,
				Verbose:         verbose,
				Debug:           debug,
				Prefix:          opts.prefix,
				PrefixSpecified: prefixSpecified,
				AllKeys:         opts.allKeys,
				Pattern:         opts.pattern,
				TagField:        opts.tagField,
				TagValue:        opts.tagValue,
				SearchValue:     opts.searchValue, // This is less powerful than the deep search above
			}

			// If we have filtering criteria but no explicit keys
			if len(keys) == 0 && hasFilteringCriteria {
				// We'll let the service handle finding matching keys
				count, err := service.BulkDelete(cmd.Context(), accountID, opts.namespaceID, nil, bulkDeleteOptions)
				if err != nil {
					return fmt.Errorf("bulk delete operation failed: %w", err)
				}

				if opts.dryRun {
					fmt.Printf("DRY RUN: Would delete %d keys\n", count)
				} else {
					fmt.Printf("Successfully deleted %d keys\n", count)
				}
				return nil
			}

			// If we have explicit keys
			if len(keys) > 0 {
				// Confirm deletion unless --force is used
				if !opts.force {
					fmt.Printf("You are about to delete %d keys. This action cannot be undone.\n", len(keys))
					fmt.Print("Are you sure? (y/N): ")

					reader := bufio.NewReader(os.Stdin)
					confirmation, _ := reader.ReadString('\n')
					confirmation = strings.TrimSpace(strings.ToLower(confirmation))

					if confirmation != "y" && confirmation != "yes" {
						fmt.Println("Deletion cancelled.")
						return nil
					}
				}

				if opts.dryRun {
					fmt.Printf("DRY RUN: Would delete %d keys\n", len(keys))
					return nil
				}

				// Delete the keys
				count, err := service.BulkDelete(cmd.Context(), accountID, opts.namespaceID, keys, bulkDeleteOptions)
				if err != nil {
					return fmt.Errorf("bulk delete operation failed: %w", err)
				}

				fmt.Printf("Successfully deleted %d/%d keys\n", count, len(keys))
				return nil
			}

			return fmt.Errorf("no keys specified for bulk deletion. Use --key, --keys, --keys-file, --prefix, --pattern, or --search")
		}),
	)
}
