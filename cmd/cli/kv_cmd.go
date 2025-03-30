package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"
	"time"
)

// kvCmd is the command for KV operations
var kvCmd = &cobra.Command{
	Use:                   "kv",
	Short:                 "Manage Cloudflare Workers KV storage",
	Long:                  `Perform operations on Cloudflare Workers KV namespaces and key-value pairs.`,
	DisableFlagsInUseLine: false,
	TraverseChildren:      true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("\nUsage:")
		fmt.Printf("  %s [command]\n", cmd.CommandPath())

		fmt.Println("\nAvailable Commands:")
		for _, subcmd := range cmd.Commands() {
			if subcmd.Hidden {
				continue
			}
			fmt.Printf("  %-15s %s\n", subcmd.Name(), subcmd.Short)
		}

		fmt.Println("\nFlags:")
		fmt.Print(cmd.LocalFlags().FlagUsages())

		fmt.Println("\nGlobal Flags:")
		fmt.Print(cmd.InheritedFlags().FlagUsages())

		fmt.Printf("\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	},
}

// kvNamespaceCmd is the command for KV namespace operations
var kvNamespaceCmd = &cobra.Command{
	Use:                   "namespace",
	Aliases:               []string{"ns"},
	Short:                 "Manage KV namespaces",
	Long:                  `Create, list, modify, and delete KV namespaces.`,
	DisableFlagsInUseLine: true,
	TraverseChildren:      true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("\nUsage:")
		fmt.Printf("  %s [command]\n", cmd.CommandPath())

		fmt.Println("\nAvailable Commands:")
		for _, subcmd := range cmd.Commands() {
			if subcmd.Hidden {
				continue
			}
			fmt.Printf("  %-15s %s\n", subcmd.Name(), subcmd.Short)
		}

		fmt.Println("\nFlags:")
		fmt.Print(cmd.LocalFlags().FlagUsages())

		fmt.Println("\nGlobal Flags:")
		fmt.Print(cmd.InheritedFlags().FlagUsages())

		fmt.Printf("\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	},
}

// kvValuesCmd is the command for KV key-value operations
var kvValuesCmd = &cobra.Command{
	Use:                   "values",
	Short:                 "Manage KV keys and values",
	Long:                  `Read, write, and delete key-value pairs in a KV namespace.`,
	DisableFlagsInUseLine: true,
	TraverseChildren:      true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(cmd.Long)
		fmt.Println("\nUsage:")
		fmt.Printf("  %s [command]\n", cmd.CommandPath())

		fmt.Println("\nAvailable Commands:")
		for _, subcmd := range cmd.Commands() {
			if subcmd.Hidden {
				continue
			}
			fmt.Printf("  %-15s %s\n", subcmd.Name(), subcmd.Short)
		}

		fmt.Println("\nFlags:")
		fmt.Print(cmd.LocalFlags().FlagUsages())

		fmt.Println("\nGlobal Flags:")
		fmt.Print(cmd.InheritedFlags().FlagUsages())

		fmt.Printf("\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	},
}

// kvFlagsVars stores the variables for the KV command flags
var kvFlagsVars struct {
	accountID   string
	namespaceID string
	title       string
	key         string
	value       string
	file        string
	expiration  int64
	prefix      string
}

func createNamespaceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List KV namespaces",
		Long:  `List all KV namespaces in an account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// List namespaces
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Listing KV namespaces for account %s...\n", accountID)
			}

			namespaces, err := kv.ListNamespaces(client, accountID)
			if err != nil {
				return fmt.Errorf("failed to list namespaces: %w", err)
			}

			// Output result
			if len(namespaces) == 0 {
				fmt.Println("No KV namespaces found")
				return nil
			}

			fmt.Printf("Found %d KV namespaces:\n", len(namespaces))
			for i, ns := range namespaces {
				fmt.Printf("%d. %s (ID: %s)\n", i+1, ns.Title, ns.ID)
			}

			return nil
		},
	}

	return cmd
}

// createNamespaceCreateCmd creates a command to create a KV namespace
func createNamespaceCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new KV namespace",
		Long:  `Create a new KV namespace in an account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get title
			if kvFlagsVars.title == "" {
				return fmt.Errorf("title is required, specify it with --title flag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Create namespace
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Creating KV namespace '%s' for account %s...\n", kvFlagsVars.title, accountID)
			}

			namespace, err := kv.CreateNamespace(client, accountID, kvFlagsVars.title)
			if err != nil {
				return fmt.Errorf("failed to create namespace: %w", err)
			}

			// Output result
			fmt.Printf("Successfully created KV namespace '%s' with ID: %s\n", namespace.Title, namespace.ID)

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title for the new namespace")
	if err := cmd.MarkFlagRequired("title"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'title' flag as required: %v\n", err)
	}
	return cmd
}

// createNamespaceDeleteCmd creates a command to delete a KV namespace
func createNamespaceDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a KV namespace",
		Long:  `Delete a KV namespace from an account.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Delete namespace
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Deleting KV namespace '%s' from account %s...\n", namespaceID, accountID)
			}

			err = kv.DeleteNamespace(client, accountID, namespaceID)
			if err != nil {
				return fmt.Errorf("failed to delete namespace: %w", err)
			}

			// Output result
			fmt.Printf("Successfully deleted KV namespace with ID: %s\n", namespaceID)

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace to delete")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace to delete (alternative to namespace-id)")

	return cmd
}

func createNamespaceRenameCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename",
		Short: "Rename a KV namespace",
		Long:  `Change the title of a KV namespace.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				return fmt.Errorf("namespace ID is required, specify with --namespace-id flag")
			}

			// Get new title
			if kvFlagsVars.title == "" {
				return fmt.Errorf("new title is required, specify it with --title flag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Rename namespace
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Renaming KV namespace '%s' to '%s'...\n", namespaceID, kvFlagsVars.title)
			}

			namespace, err := kv.RenameNamespace(client, accountID, namespaceID, kvFlagsVars.title)
			if err != nil {
				return fmt.Errorf("failed to rename namespace: %w", err)
			}

			// Output result
			fmt.Printf("Successfully renamed KV namespace to '%s'\n", namespace.Title)

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace to rename")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "New title for the namespace")
	if err := cmd.MarkFlagRequired("namespace-id"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'namespace-id' flag as required: %v\n", err)
	}
	if err := cmd.MarkFlagRequired("title"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'title' flag as required: %v\n", err)
	}

	return cmd
}

// createNamespaceBulkDeleteCmd creates a command to delete multiple KV namespaces
func createNamespaceBulkDeleteCmd() *cobra.Command {
	// Define variables for flags
	var (
		namespacePattern string
		namespaceIDs     []string
	)

	cmd := &cobra.Command{
		Use:   "bulk-delete",
		Short: "Delete multiple KV namespaces",
		Long:  `Delete multiple KV namespaces from an account, by ID or pattern matching.`,
		Example: `  # Delete multiple namespaces by their IDs
  cache-kv-purger kv namespace bulk-delete --account-id YOUR_ACCOUNT_ID --namespace-ids id1,id2,id3

  # Delete all namespaces matching a pattern
  cache-kv-purger kv namespace bulk-delete --account-id YOUR_ACCOUNT_ID --pattern "test-*"

  # Dry-run to preview which namespaces would be deleted
  cache-kv-purger kv namespace bulk-delete --account-id YOUR_ACCOUNT_ID --pattern "dev-*" --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// Get list of namespaces to delete
			var namespacesToDelete []kv.Namespace

			// Case 1: Delete by pattern
			if namespacePattern != "" {
				if verbose {
					fmt.Printf("Finding namespaces matching pattern '%s'...\n", namespacePattern)
				}

				matchingNamespaces, err := kv.FindNamespacesByPattern(client, accountID, namespacePattern)
				if err != nil {
					return fmt.Errorf("failed to find namespaces by pattern: %w", err)
				}

				if len(matchingNamespaces) == 0 {
					fmt.Printf("No namespaces found matching pattern '%s'\n", namespacePattern)
					return nil
				}

				namespacesToDelete = matchingNamespaces
			} else if len(namespaceIDs) > 0 {
				// Case 2: Delete by IDs provided
				if verbose {
					fmt.Printf("Preparing to delete %d namespaces by ID...\n", len(namespaceIDs))
				}

				// Verify each namespace exists
				for _, nsID := range namespaceIDs {
					ns, err := kv.GetNamespace(client, accountID, nsID)
					if err != nil {
						fmt.Printf("Warning: namespace with ID '%s' not found or cannot be accessed: %v\n", nsID, err)
						continue
					}

					namespacesToDelete = append(namespacesToDelete, *ns)
				}

				if len(namespacesToDelete) == 0 {
					fmt.Println("No valid namespaces found from provided IDs")
					return nil
				}
			} else {
				return fmt.Errorf("either namespace IDs or a pattern must be provided")
			}

			// Display namespaces to be deleted
			fmt.Printf("Found %d namespaces to delete:\n", len(namespacesToDelete))
			for i, ns := range namespacesToDelete {
				fmt.Printf("%d. %s (ID: %s)\n", i+1, ns.Title, ns.ID)
			}

			// If dry run, exit here
			if dryRun {
				fmt.Println("\nDRY RUN: No namespaces were actually deleted")
				return nil
			}

			// Confirm deletion if not forced
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				fmt.Print("\nAre you sure you want to delete these namespaces? This action cannot be undone. [y/N]: ")
				var confirm string
				if _, err := fmt.Scanln(&confirm); err != nil {
					// Handle EOF or unexpected input errors gracefully
					// For example, if user just presses Enter without input
					confirm = "" // Default to empty to ensure it's not "y" or "Y"
				}
				if confirm != "y" && confirm != "Y" {
					fmt.Println("Operation cancelled")
					return nil
				}
			}

			// Extract namespace IDs
			deleteIDs := make([]string, len(namespacesToDelete))
			for i, ns := range namespacesToDelete {
				deleteIDs[i] = ns.ID
			}

			// Delete namespaces with progress feedback
			progressCallback := func(completed, total, success, failed int) {
				if verbose {
					fmt.Printf("Progress: %d/%d completed (%d successful, %d failed)\n",
						completed, total, success, failed)
				}
			}

			successIDs, errors := kv.DeleteMultipleNamespacesWithProgress(client, accountID, deleteIDs, progressCallback)

			// Report results
			if len(successIDs) > 0 {
				fmt.Printf("Successfully deleted %d/%d namespaces\n", len(successIDs), len(namespacesToDelete))
			}

			if len(errors) > 0 {
				fmt.Printf("Failed to delete %d namespaces:\n", len(errors))
				for i, err := range errors {
					fmt.Printf("%d. %v\n", i+1, err)
				}
				return fmt.Errorf("some namespace deletions failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&namespacePattern, "pattern", "", "Pattern to match namespace titles (regex syntax)")
	cmd.Flags().StringSliceVar(&namespaceIDs, "namespace-ids", nil, "Comma-separated list of namespace IDs to delete")
	cmd.Flags().Bool("dry-run", false, "Show namespaces that would be deleted without actually deleting them")
	cmd.Flags().Bool("force", false, "Skip confirmation prompt")

	return cmd
}

func createValuesListCmd() *cobra.Command {
	var prefix string
	var pageSize int
	var pageNumber int
	var limit int
	
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List keys in a KV namespace",
		Long:  `List all keys in a KV namespace.`,
		Example: `  # List first page of keys
  cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7
  
  # List all keys (handle pagination automatically)
  cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --all
  
  # Filter keys by prefix
  cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --prefix "product-" --all
  
  # List keys with pagination control
  cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --page-size 1000 --page 2`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Check if we should list all keys
			listAll, _ := cmd.Flags().GetBool("all")

			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Listing keys in KV namespace '%s'...\n", namespaceID)
				if prefix != "" {
					fmt.Printf("Filtering by prefix: %s\n", prefix)
				}
				if pageSize > 0 {
					fmt.Printf("Page size: %d\n", pageSize)
				}
				if pageNumber > 0 {
					fmt.Printf("Page number: %d\n", pageNumber)
				}
				if limit > 0 {
					fmt.Printf("Limit: %d\n", limit)
				}
			}

			var keys []kv.KeyValuePair

			if listAll {
				// Create progress callback
				progressCallback := func(fetched, total int) {
					if verbose {
						if total > 0 {
							fmt.Printf("Retrieved %d/%d keys so far...\n", fetched, total)
						} else {
							fmt.Printf("Retrieved %d keys so far...\n", fetched)
						}
					}
				}

				// Set up options for ListAllKeys
				options := &kv.ListKeysOptions{}
				if prefix != "" {
					options.Prefix = prefix
				}
				if pageSize > 0 {
					options.Limit = pageSize
				}

				// Use ListAllKeys to automatically handle pagination
				allKeys, err := kv.ListAllKeys(client, accountID, namespaceID, progressCallback)
				if err != nil {
					return fmt.Errorf("failed to list keys: %w", err)
				}
				
				// Apply limit if specified
				if limit > 0 && len(allKeys) > limit {
					keys = allKeys[:limit]
				} else {
					keys = allKeys
				}
			} else {
				// Set up options for specific pagination
				options := &kv.ListKeysOptions{}
				
				if prefix != "" {
					options.Prefix = prefix
				}
				
				// Calculate effective page size
				effectivePageSize := 1000 // Default
				if pageSize > 0 {
					effectivePageSize = pageSize
				}
				options.Limit = effectivePageSize
				
				// Handle pagination
				if pageNumber > 1 {
					// We need to fetch pages sequentially to get to the desired page
					var cursor string
					for i := 1; i < pageNumber; i++ {
						tempOptions := &kv.ListKeysOptions{
							Limit:  effectivePageSize,
							Cursor: cursor,
							Prefix: prefix,
						}
						result, err := kv.ListKeysWithOptions(client, accountID, namespaceID, tempOptions)
						if err != nil {
							return fmt.Errorf("failed to navigate to page %d: %w", pageNumber, err)
						}
						
						if !result.HasMore {
							return fmt.Errorf("page %d does not exist (only %d pages available)", pageNumber, i)
						}
						
						cursor = result.Cursor
					}
					
					// Now fetch the actual page
					options.Cursor = cursor
				}
				
				// Apply limit if specified and different from page size
				if limit > 0 && limit < options.Limit {
					options.Limit = limit
				}
				
				// Fetch the specified page
				result, err := kv.ListKeysWithOptions(client, accountID, namespaceID, options)
				if err != nil {
					return fmt.Errorf("failed to list keys: %w", err)
				}
				
				keys = result.Keys
				
				// Mention pagination info
				if result.HasMore {
					fmt.Printf("Note: More keys are available. Use --page %d --page-size %d to see the next page, or use --all to list all keys.\n", 
						pageNumber+1, effectivePageSize)
				}
			}

			// Output result
			if len(keys) == 0 {
				fmt.Println("No keys found in the namespace")
				return nil
			}

			fmt.Printf("Found %d keys:\n", len(keys))
			for i, key := range keys {
				expirationStr := ""
				if key.Expiration > 0 {
					expirationTime := time.Unix(key.Expiration, 0)
					expirationStr = fmt.Sprintf(" (expires: %s)", expirationTime.Format(time.RFC3339))
				}
				fmt.Printf("%d. %s%s\n", i+1, key.Key, expirationStr)
			}

			if !listAll && pageNumber <= 1 && pageSize == 0 && limit == 0 {
				fmt.Println("\nNote: Only showing first page of results. Use --all flag to list all keys.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().Bool("all", false, "List all keys (automatically handle pagination)")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Filter keys by prefix")
	cmd.Flags().IntVar(&pageSize, "page-size", 0, "Number of keys per page (max 1000)")
	cmd.Flags().IntVar(&pageNumber, "page", 0, "Page number to fetch (starting from 1)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of keys to return")

	return cmd
}

// createValuesGetCmd creates a command to get a value from a KV namespace
func createValuesGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a value from a KV namespace",
		Long:  `Get a value for a key from a KV namespace.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Get key
			if kvFlagsVars.key == "" {
				return fmt.Errorf("key is required, specify with --key flag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Get value
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Getting value for key '%s' from KV namespace '%s'...\n", kvFlagsVars.key, namespaceID)
			}

			value, err := kv.GetValue(client, accountID, namespaceID, kvFlagsVars.key)
			if err != nil {
				return fmt.Errorf("failed to get value: %w", err)
			}

			// Output result (without newline to preserve value formatting)
			fmt.Print(value)

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.key, "key", "", "Key to get value for")
	if err := cmd.MarkFlagRequired("key"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'key' flag as required: %v\n", err)
	}
	return cmd
}

// createValuesPutCmd creates a command to put a value in a KV namespace
func createValuesPutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "put",
		Short: "Put a value in a KV namespace",
		Long:  `Write a value for a key to a KV namespace.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Get key
			if kvFlagsVars.key == "" {
				return fmt.Errorf("key is required, specify with --key flag")
			}

			// Get value
			value := kvFlagsVars.value
			if value == "" && kvFlagsVars.file != "" {
				// Read value from file
				data, err := os.ReadFile(kvFlagsVars.file)
				if err != nil {
					return fmt.Errorf("failed to read value from file: %w", err)
				}
				value = string(data)
			}

			if value == "" {
				return fmt.Errorf("value is required, specify with --value flag or --file flag")
			}

			// Create options
			var options *kv.WriteOptions
			if kvFlagsVars.expiration > 0 {
				options = &kv.WriteOptions{
					Expiration: kvFlagsVars.expiration,
				}
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Put value
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Writing value for key '%s' to KV namespace '%s'...\n", kvFlagsVars.key, namespaceID)

				if options != nil && options.Expiration > 0 {
					expirationTime := time.Unix(options.Expiration, 0)
					fmt.Printf("Value will expire at: %s\n", expirationTime.Format(time.RFC3339))
				}
			}

			err = kv.WriteValue(client, accountID, namespaceID, kvFlagsVars.key, value, options)
			if err != nil {
				return fmt.Errorf("failed to write value: %w", err)
			}

			// Output result
			fmt.Printf("Successfully wrote value for key '%s'\n", kvFlagsVars.key)

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.key, "key", "", "Key to write value for")
	cmd.Flags().StringVar(&kvFlagsVars.value, "value", "", "Value to write")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "File to read value from (alternative to --value)")
	cmd.Flags().Int64Var(&kvFlagsVars.expiration, "expiration", 0, "Expiration time (Unix timestamp)")
	if err := cmd.MarkFlagRequired("key"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'key' flag as required: %v\n", err)
	}

	return cmd
}

// createValuesDeleteCmd creates a command to delete a value from a KV namespace
func createValuesDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a value from a KV namespace",
		Long:  `Delete a value for a key from a KV namespace.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Get key
			if kvFlagsVars.key == "" {
				return fmt.Errorf("key is required, specify with --key flag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Delete value
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Deleting value for key '%s' from KV namespace '%s'...\n", kvFlagsVars.key, namespaceID)
			}

			err = kv.DeleteValue(client, accountID, namespaceID, kvFlagsVars.key)
			if err != nil {
				return fmt.Errorf("failed to delete value: %w", err)
			}

			// Output result
			fmt.Printf("Successfully deleted value for key '%s'\n", kvFlagsVars.key)

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.key, "key", "", "Key to delete value for")
	if err := cmd.MarkFlagRequired("key"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'key' flag as required: %v\n", err)
	}
	return cmd
}

// createKVConfigCmd is the command for managing KV configuration
func createKVConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configure KV settings",
		Long:  `Set default account ID for KV operations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load existing config
			cfg, err := config.LoadFromFile("")
			if err != nil {
				cfg = config.New()
			}

			// Update config with account ID
			if kvFlagsVars.accountID != "" {
				cfg.AccountID = kvFlagsVars.accountID

				// Save updated config
				if err := cfg.SaveToFile(""); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				fmt.Printf("Successfully set default account ID to '%s'\n", kvFlagsVars.accountID)
			} else {
				// Just display the current settings
				fmt.Printf("Current configuration:\n")

				// Account ID (may come from env var, config, or neither)
				accountID := cfg.GetAccountID()
				if accountID != "" {
					if os.Getenv(config.EnvAccountID) != "" {
						fmt.Printf("  Account ID: %s (from environment variable)\n", accountID)
					} else {
						fmt.Printf("  Account ID: %s\n", accountID)
					}
				} else {
					fmt.Printf("  Account ID: (not set)\n")
				}

				// Zone ID (may come from env var, config, or neither)
				zoneID := cfg.GetZoneID()
				if zoneID != "" {
					if os.Getenv(config.EnvZoneID) != "" {
						fmt.Printf("  Default Zone ID: %s (from environment variable)\n", zoneID)
					} else {
						fmt.Printf("  Default Zone ID: %s\n", zoneID)
					}
				} else {
					fmt.Printf("  Default Zone ID: (not set)\n")
				}

				fmt.Printf("  API Endpoint: %s\n", cfg.APIEndpoint)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.accountID, "account-id", "", "Default account ID to use for KV operations")

	return cmd
}

// createGetKeyWithMetadataCmd creates a command to get a key including its metadata
func createGetKeyWithMetadataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "get-with-metadata",
		Short:                 "Get a key including its metadata",
		Long:                  `Get a key's value and metadata from a KV namespace.`,
		DisableFlagsInUseLine: false,
		Example: `  # Get a key with metadata using namespace ID
  cache-kv-purger kv get-with-metadata --namespace-id your-namespace-id --key mykey`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Get key
			if kvFlagsVars.key == "" {
				return fmt.Errorf("key is required, specify with --key flag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Get key with metadata
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Getting key '%s' with metadata from namespace '%s'...\n", kvFlagsVars.key, namespaceID)
			}

			kvPair, err := kv.GetKeyWithMetadata(client, accountID, namespaceID, kvFlagsVars.key)
			if err != nil {
				return fmt.Errorf("failed to get key with metadata: %w", err)
			}

			// Output result
			fmt.Printf("Key: %s\n", kvPair.Key)
			fmt.Printf("Value: %s\n", kvPair.Value)

			if kvPair.Expiration > 0 {
				expirationTime := time.Unix(kvPair.Expiration, 0)
				fmt.Printf("Expiration: %s (%d)\n", expirationTime.Format(time.RFC3339), kvPair.Expiration)
			}

			if kvPair.Metadata != nil && len(*kvPair.Metadata) > 0 {
				fmt.Println("Metadata:")
				metadataJSON, err := json.MarshalIndent(kvPair.Metadata, "  ", "  ")
				if err != nil {
					fmt.Printf("  Error formatting metadata: %v\n", err)
				} else {
					fmt.Printf("  %s\n", string(metadataJSON))
				}
			} else {
				fmt.Println("No metadata found")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.key, "key", "", "Key to get")
	if err := cmd.MarkFlagRequired("key"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'key' flag as required: %v\n", err)
	}

	return cmd
}

// createKeyExistsCmd creates a command to check if a key exists in a KV namespace
func createKeyExistsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "exists",
		Short:                 "Check if a key exists in a KV namespace",
		Long:                  `Check if a key exists in a KV namespace without retrieving its value.`,
		DisableFlagsInUseLine: false,
		Example: `  # Check if a key exists using namespace ID
  cache-kv-purger kv exists --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 364f2f5c31f442709ef4df47c148e76e --key mykey
  
  # Check if a key exists using namespace title
  cache-kv-purger kv exists --account-id 01a7362d577a6c3019a474fd6f485823 --title "My KV Namespace" --key mykey`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID from flag, config, or environment variable
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				// Try to get from config or environment variable
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}

			if accountID == "" {
				return fmt.Errorf("account ID is required, specify it with --account-id flag, CLOUDFLARE_ACCOUNT_ID environment variable, or set a default account in config")
			}

			// Get namespace ID
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				// If title is provided, try to find namespace by title
				if kvFlagsVars.title != "" {
					// Create API client
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}

					// Find namespace by title
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}

					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required, specify with --namespace-id or --title flag")
				}
			}

			// Get key
			if kvFlagsVars.key == "" {
				return fmt.Errorf("key is required, specify with --key flag")
			}

			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}

			// Check if key exists
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Checking if key '%s' exists in namespace '%s'...\n", kvFlagsVars.key, namespaceID)
			}

			exists, err := kv.KeyExists(client, accountID, namespaceID, kvFlagsVars.key)
			if err != nil {
				return fmt.Errorf("failed to check if key exists: %w", err)
			}

			// Output result
			if exists {
				// Success is 0 exit code
				fmt.Printf("Key '%s' exists in the namespace\n", kvFlagsVars.key)
			} else {
				// For CLI tools, non-existence is often a non-zero exit code
				fmt.Printf("Key '%s' does not exist in the namespace\n", kvFlagsVars.key)
				// Return an error to set a non-zero exit code
				cmd.SilenceErrors = true // Don't print the error message
				return fmt.Errorf("key not found")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.key, "key", "", "Key to check")
	if err := cmd.MarkFlagRequired("key"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to mark 'key' flag as required: %v\n", err)
	}

	return cmd
}

// createUnifiedSearchCmd creates a unified command that can both find and purge keys
func createUnifiedSearchCmd() *cobra.Command {
	// Define flags for the unified command
	var searchValue string
	var tagField string
	var tagValue string
	var showMetadata bool
	var purgeMatches bool
	var dryRun bool
	var outputJSON bool
	var chunkSize int
	var concurrency int
	// We don't need a local verbose var since we use the global one
	
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for keys with powerful filtering, with option to purge",
			Long:  `Search for keys using powerful filtering options with the ability to also purge matching keys.

The search command supports two different search modes:

1. Smart search (--value):
   - Performs a deep recursive search through all metadata
   - Finds the value anywhere in the metadata structure (fields, arrays, nested objects)
   - Case-insensitive matching
   - Best for general-purpose searching when you do not know the exact field structure

2. Field-specific search (--tag-field and --tag-value):
   - Searches for a specific field with a specific value
   - More precise when you know the exact field you want to match
   - Must specify both the field name and the expected value

Smart search is more powerful for finding content anywhere in the metadata,
while field-specific search is better for precise field matching.

For detailed progress information during search and purge operations, 
use the global --verbose flag. This will show:
- Search progress with matched keys count
- Batch processing details during purging
- API operation details and timing information`,
			Example: `  # Smart search for keys containing value anywhere in metadata (RECOMMENDED)
  cache-kv-purger kv search --namespace-id YOUR_NAMESPACE_ID --value "your-search-term" --metadata
  
  # Field-specific search for keys with a specific tag field and value
  cache-kv-purger kv search --namespace-id YOUR_NAMESPACE_ID --tag-field "cache-tag" --tag-value "your-tag-value" --metadata
  
  # Smart search and purge keys (dry run first)
  cache-kv-purger kv search --namespace-id YOUR_NAMESPACE_ID --value "your-search-term" --purge --dry-run
  
  # Smart search and purge keys for real
  cache-kv-purger kv search --namespace-id YOUR_NAMESPACE_ID --value "your-search-term" --purge`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get common parameters
			accountID := kvFlagsVars.accountID
			namespaceID := kvFlagsVars.namespaceID
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Validate inputs
			if accountID == "" {
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
				if accountID == "" {
					return fmt.Errorf("account ID is required")
				}
			}
			
			if namespaceID == "" {
				if kvFlagsVars.title != "" {
					client, err := api.NewClient()
					if err != nil {
						return err
					}
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return err
					}
					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID is required")
				}
			}
			
			// Require at least one search criterion
			if searchValue == "" && tagField == "" {
				return fmt.Errorf("at least one search criterion is required, specify --value or --tag-field")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return err
			}
			
			// Find matching keys
			var matchedKeys []kv.KeyValuePair
			
			// If both criteria provided, use smart search first then filter by tag
			if searchValue != "" && tagField != "" {
				if verbose {
					fmt.Printf("Searching for keys with value '%s' and tag field '%s'='%s'...\n", 
						searchValue, tagField, tagValue)
				}
				
				// Do smart search first
				searchMatches, err := kv.SmartFindKeysWithValue(
					client, 
					accountID, 
					namespaceID, 
					searchValue,
					chunkSize,
					concurrency, 
					func(keysFetched, keysProcessed, keysMatched, total int) {
						if verbose {
							fmt.Printf("Smart search progress: %d/%d processed, %d matched\n", 
								keysProcessed, total, keysMatched)
						}
					},
				)
				
				if err != nil {
					return err
				}
				
				// Filter by tag field
				var filtered []kv.KeyValuePair
				for _, key := range searchMatches {
					if key.Metadata != nil {
						if fieldValue, ok := (*key.Metadata)[tagField]; ok {
							matched := false
							
							// String field
							if fieldStr, isString := fieldValue.(string); isString {
								if tagValue == "" || fieldStr == tagValue {
									matched = true
								}
							}
							
							// Array field
							if fieldArray, isArray := fieldValue.([]interface{}); isArray {
								if tagValue == "" {
									if len(fieldArray) > 0 {
										matched = true
									}
								} else {
									for _, item := range fieldArray {
										if itemStr, ok := item.(string); ok && itemStr == tagValue {
											matched = true
											break
										}
									}
								}
							}
							
							if matched {
								filtered = append(filtered, key)
							}
						}
					}
				}
				
				matchedKeys = filtered
			} else if searchValue != "" {
				// Just smart search
				if verbose {
					fmt.Printf("Searching for keys with value '%s'...\n", searchValue)
				}
				
				matchedKeys, err = kv.SmartFindKeysWithValue(
					client, 
					accountID, 
					namespaceID, 
					searchValue,
					chunkSize,
					concurrency, 
					func(keysFetched, keysProcessed, keysMatched, total int) {
						if verbose {
							fmt.Printf("Smart search progress: %d/%d processed, %d matched\n", 
								keysProcessed, total, keysMatched)
						}
					},
				)
				
				if err != nil {
					return err
				}
			} else if tagField != "" {
				// Just tag field search
				if verbose {
					fmt.Printf("Searching for keys with tag field '%s'='%s'...\n", tagField, tagValue)
				}
				
				matchedKeys, err = kv.StreamingFilterKeysByMetadata(
					client,
					accountID,
					namespaceID,
					tagField,
					tagValue,
					chunkSize,
					concurrency,
					func(keysFetched, keysProcessed, keysMatched, total int) {
						if verbose {
							fmt.Printf("Tag filter progress: %d/%d processed, %d matched\n", 
								keysProcessed, total, keysMatched)
						}
					},
				)
				
				if err != nil {
					return err
				}
			}
			
			// Check if we found any matches
			if len(matchedKeys) == 0 {
				fmt.Println("No keys found matching the search criteria")
				return nil
			}
			
			// Output results or purge
			if purgeMatches {
				// Output what we found first
				fmt.Printf("Found %d keys matching search criteria\n", len(matchedKeys))
				
				// Show sample of keys if not verbose
				if !verbose {
					displayCount := 5
					if len(matchedKeys) < displayCount {
						displayCount = len(matchedKeys)
					}
					
					for i := 0; i < displayCount; i++ {
						fmt.Printf("  %d. %s\n", i+1, matchedKeys[i].Key)
					}
					
					if len(matchedKeys) > displayCount {
						fmt.Printf("  ... and %d more keys\n", len(matchedKeys)-displayCount)
					}
				} else {
					// Show all in verbose mode
					for i, key := range matchedKeys {
						fmt.Printf("  %d. %s\n", i+1, key.Key)
					}
				}
				
				// If dry run, stop here
				if dryRun {
					fmt.Printf("\nDRY RUN: Would delete %d keys\n", len(matchedKeys))
					return nil
				}
				
				// Confirm deletion
				fmt.Printf("\nAre you sure you want to delete these %d keys? This cannot be undone. [y/N]: ", len(matchedKeys))
				var confirm string
				if _, err := fmt.Scanln(&confirm); err != nil || (confirm != "y" && confirm != "Y") {
					fmt.Println("Operation cancelled")
					return nil
				}
				
				// Extract key names for deletion
				keyNames := make([]string, len(matchedKeys))
				for i, key := range matchedKeys {
					keyNames[i] = key.Key
				}
				
				// Delete in batches
				batchSize := 1000 // Cloudflare API limit
				totalDeleted := 0
				
				for i := 0; i < len(keyNames); i += batchSize {
					end := i + batchSize
					if end > len(keyNames) {
						end = len(keyNames)
					}
					
					batch := keyNames[i:end]
					if err := kv.DeleteMultipleValues(client, accountID, namespaceID, batch); err != nil {
						return fmt.Errorf("error deleting batch: %w", err)
					}
					
					totalDeleted += len(batch)
					
					if verbose {
						fmt.Printf("Deleted batch %d: %d/%d keys deleted\n", (i/batchSize)+1, totalDeleted, len(keyNames))
					}
				}
				
				fmt.Printf("Successfully deleted %d keys\n", totalDeleted)
			} else {
				// Just output results
				if outputJSON {
					// JSON output
					jsonData, err := json.MarshalIndent(matchedKeys, "", "  ")
					if err != nil {
						return fmt.Errorf("failed to format as JSON: %w", err)
					}
					fmt.Println(string(jsonData))
				} else {
					// Human readable output
					fmt.Printf("Found %d keys matching search criteria:\n", len(matchedKeys))
					
					for i, key := range matchedKeys {
						fmt.Printf("%d. %s", i+1, key.Key)
						
						if key.Expiration > 0 {
							expirationTime := time.Unix(key.Expiration, 0)
							fmt.Printf(" (expires: %s)", expirationTime.Format(time.RFC3339))
						}
						
						fmt.Println()
						
						if showMetadata && key.Metadata != nil && len(*key.Metadata) > 0 {
							fmt.Printf("   Metadata:\n")
							for field, value := range *key.Metadata {
								fmt.Printf("     %s: %v\n", field, value)
							}
						}
						
						// Add separator between keys
						if i < len(matchedKeys)-1 {
							fmt.Println()
						}
					}
				}
			}
			
			return nil
		},
	}
	
	// Add flags
	cmd.Flags().StringVar(&searchValue, "value", "", "Value to search for anywhere in metadata")
	cmd.Flags().StringVar(&tagField, "tag-field", "", "Metadata field name to filter by")
	cmd.Flags().StringVar(&tagValue, "tag-value", "", "Value to match in the tag field")
	cmd.Flags().BoolVarP(&showMetadata, "metadata", "m", false, "Show metadata for matching keys")
	cmd.Flags().BoolVar(&purgeMatches, "purge", false, "Delete matching keys")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without actually deleting (for --purge)")
	cmd.Flags().BoolVar(&outputJSON, "json", false, "Output results as JSON")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 100, "Size of chunks to process")
	cmd.Flags().IntVar(&concurrency, "concurrency", 10, "Number of concurrent operations")
	// Note: We removed this local verbose flag in favor of the global one
	// cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output for detailed progress")
	
	return cmd
}

// addMissingValueValidation adds a special validation for commands to ensure flags
// have values when specified and displays better error messages
func addMissingValueValidation(cmd *cobra.Command) {
	// Store the original functions if they exist
	originalRunE := cmd.RunE
	originalRun := cmd.Run
	
	// Replace with our validation
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Check if --help is present anywhere in the arguments
		// This ensures help takes precedence over validation errors
		for _, arg := range os.Args {
			if arg == "--help" || arg == "-h" {
				cmd.Help()
				return nil
			}
		}
		
		// Check for help flag before validation
		if help, _ := cmd.Flags().GetBool("help"); help {
			cmd.Help()
			return nil
		}
		
		// Check for flags with missing values
		var missingValues []string
		cmd.Flags().Visit(func(f *pflag.Flag) {
			// Only check string flags
			if f.Value.Type() == "string" {
				// If the flag's value starts with a dash and it's the actual value
				// (not just an empty string), it might be an argument
				if f.Value.String() != "" && f.Value.String()[0] == '-' {
					missingValues = append(missingValues, f.Name)
				}
			}
		})
		
		if len(missingValues) > 0 {
			// Check if this is a help request again to avoid showing errors
			for _, flag := range missingValues {
				if flag == "help" || flag == "h" {
					cmd.Help()
					return nil
				}
			}
			return fmt.Errorf("missing values for flags: %v", missingValues)
		}
		
		// Run the original function
		if originalRunE != nil {
			return originalRunE(cmd, args)
		} else if originalRun != nil {
			// If the command used Run instead of RunE, call it and return nil
			originalRun(cmd, args)
		}
		return nil
	}
	
	// Clear the original Run function to avoid duplication
	if cmd.Run != nil {
		cmd.Run = nil
	}
	
	// Recursively add to all subcommands
	for _, subCmd := range cmd.Commands() {
		addMissingValueValidation(subCmd)
	}
}

func init() {
	rootCmd.AddCommand(kvCmd)

	// Add common flags to kv command
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.accountID, "account-id", "", "Cloudflare Account ID")
	
	// Add validation for missing values to all KV commands
	addMissingValueValidation(kvCmd)
	
	// Note: Legacy KV commands have been removed.
	// We now only use the consolidated verb-based commands registered in kv_consolidated_cmd.go
	// The search command registration is also moved to kv_consolidated_cmd.go

	// Add direct flags to kvCmd for common use cases
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.file, "file", "", "Output or input file path")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.key, "key", "", "Key name")
}
