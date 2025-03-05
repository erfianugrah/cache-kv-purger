package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"
	"github.com/spf13/cobra"
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

// createNamespaceListCmd creates a command to list KV namespaces
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
	cmd.MarkFlagRequired("title")
	
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

// createNamespaceRenameCmd creates a command to rename a KV namespace
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
	cmd.MarkFlagRequired("namespace-id")
	cmd.MarkFlagRequired("title")
	
	return cmd
}

// createValuesListCmd creates a command to list keys in a KV namespace
func createValuesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List keys in a KV namespace",
		Long:  `List all keys in a KV namespace.`,
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
				
				// Use ListAllKeys to automatically handle pagination
				keys, err = kv.ListAllKeys(client, accountID, namespaceID, progressCallback)
			} else {
				// Just get the first page
				keys, err = kv.ListKeys(client, accountID, namespaceID)
			}
			
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
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
			
			if !listAll {
				fmt.Println("\nNote: Only showing first page of results. Use --all flag to list all keys.")
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().Bool("all", false, "List all keys (automatically handle pagination)")
	
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
	cmd.MarkFlagRequired("key")
	
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
	cmd.MarkFlagRequired("key")
	
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
	cmd.MarkFlagRequired("key")
	
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

// createPurgeByTagCmd creates a command to purge KV values by cache-tag
// This is now just a wrapper around the streaming purge command which is more efficient
func createPurgeByTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge-by-tag",
		Short: "Purge KV values by cache-tag (legacy command, use purge-by-tag-streaming instead)",
		Long:  `Find and delete KV values with matching cache-tag field value. This is a legacy command, please use purge-by-tag-streaming for better performance.`,
		Example: `  # Purge all entries with a specific cache-tag value
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id --tag-value "homepage"

  # Purge all entries with any cache-tag value
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id

  # Use a different field name instead of cache-tag
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id --field "category" --tag-value "blog"

  # Preview what would be deleted without actually deleting
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id --tag-value "product" --dry-run`,
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
			
			// Get the tag field name to search for
			field, _ := cmd.Flags().GetString("field")
			if field == "" {
				// Default to "cache-tag" if not specified
				field = "cache-tag"
			}
			
			// Get the tag value to match
			tagValue := kvFlagsVars.value
			
			// Get batch size for deletion
			batchSize, _ := cmd.Flags().GetInt("batch-size")
			if batchSize <= 0 {
				batchSize = 100 // Default batch size for deletes
			}
			
			// Check if this is a dry run
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			
			// Verbose output
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Print a message about the deprecated command
			fmt.Println("NOTE: This command is deprecated. Please use 'purge-by-tag-streaming' for better performance.")
			
			// Progress tracking
			var lastProgressTime time.Time
			var lastKeysFetched, lastKeysProcessed, lastKeysDeleted int
			
			// Create the progress callback
			progressCallback := func(keysFetched, keysProcessed, keysDeleted, total int) {
				if !verbose {
					return
				}
				
				// Only update progress every 0.5 seconds to avoid flooding the terminal
				now := time.Now()
				if lastProgressTime.IsZero() || now.Sub(lastProgressTime) > 500*time.Millisecond {
					if keysFetched > lastKeysFetched {
						fmt.Printf("Progress: Listed %d keys\n", keysFetched)
						lastKeysFetched = keysFetched
					}
					
					if keysProcessed > lastKeysProcessed {
						fmt.Printf("Progress: Processed %d/%d keys (%d%%)\n", 
							keysProcessed, total, keysProcessed*100/total)
						lastKeysProcessed = keysProcessed
					}
					
					if keysDeleted > lastKeysDeleted {
						fmt.Printf("Progress: Deleted %d keys\n", keysDeleted)
						lastKeysDeleted = keysDeleted
					}
					
					lastProgressTime = now
				}
			}
			
			// Use the streaming implementation which is more efficient
			concurrency := 10
			startTime := time.Now()
			count, err := kv.StreamingPurgeByTag(client, accountID, namespaceID, field, tagValue, 
				batchSize, concurrency, dryRun, progressCallback)
			duration := time.Since(startTime)
			
			if err != nil {
				return fmt.Errorf("failed during purge operation: %w", err)
			}
			
			// Output result
			if dryRun {
				// In dry run mode, count is the number of matches found
				if tagValue != "" {
					fmt.Printf("Dry run: Found %d keys with '%s' field value matching '%s'\n", count, field, tagValue)
				} else {
					fmt.Printf("Dry run: Found %d keys with '%s' field\n", count, field)
				}
			} else {
				// In actual run mode, count is the number of keys deleted
				if tagValue != "" {
					fmt.Printf("Successfully deleted %d keys with '%s' field value matching '%s'\n", count, field, tagValue)
				} else {
					fmt.Printf("Successfully deleted %d keys with '%s' field\n", count, field)
				}
			}
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().String("field", "cache-tag", "The JSON field to search for in values")
	cmd.Flags().StringVar(&kvFlagsVars.value, "tag-value", "", "The tag value to match (if empty, all values will be purged)")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	cmd.Flags().Bool("use-bulk-delete", true, "Use bulk delete API instead of individual deletes (faster for large operations)")
	cmd.Flags().Int("batch-size", 100, "Number of items to process in each batch")
	
	// Add a deprecated flag for backward compatibility
	cmd.Flags().Bool("memory-efficient", false, "Deprecated: This flag has no effect, the command now always uses the streaming implementation")
	cmd.Flags().MarkDeprecated("memory-efficient", "This flag is deprecated and has no effect")
	
	return cmd
}

// createBulkUploadBatchCmd creates a command to upload a bulk JSON file to a KV namespace using batch operations
func createBulkUploadBatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-batch",
		Short: "Upload bulk JSON data to a KV namespace (deprecated, use bulk-concurrent instead)",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace. This command is deprecated, please use bulk-concurrent instead.`,
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
			
			// Check for input file
			if kvFlagsVars.file == "" {
				return fmt.Errorf("input file is required, specify with --file flag")
			}
			
			// Print a message about the deprecated command
			fmt.Println("NOTE: This command is deprecated. Please use 'bulk-concurrent' for better performance.")
			
			// Read the input file
			data, err := os.ReadFile(kvFlagsVars.file)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			
			// Parse the JSON
			type KVItem struct {
				Key           string                 `json:"key"`
				Value         interface{}            `json:"value"`
				Metadata      map[string]interface{} `json:"metadata,omitempty"`
				Expiration    int64                  `json:"expiration,omitempty"`
				ExpirationTTL int64                  `json:"expiration_ttl,omitempty"`
			}
			
			var items []KVItem
			if err := json.Unmarshal(data, &items); err != nil {
				return fmt.Errorf("failed to parse JSON input: %w", err)
			}
			
			if len(items) == 0 {
				return fmt.Errorf("no items found in input file")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			// Convert all items to bulk write items
			bulkItems := make([]kv.BulkWriteItem, len(items))
			for i, item := range items {
				// Convert value to JSON string
				valueBytes, err := json.Marshal(item.Value)
				if err != nil {
					fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				// Calculate expiration if needed
				expiration := int64(0)
				if item.Expiration > 0 {
					expiration = item.Expiration
				} else if item.ExpirationTTL > 0 {
					expiration = time.Now().Unix() + item.ExpirationTTL
				}
				
				// Add to bulk items array
				bulkItems[i] = kv.BulkWriteItem{
					Key:        item.Key,
					Value:      string(valueBytes),
					Expiration: expiration,
					Metadata:   item.Metadata,
				}
			}
			
			// Upload using concurrent batches (which is much more efficient)
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Create progress callback
			progressCallback := func(completed, total int) {
				if verbose && total > 0 {
					fmt.Printf("Progress: %d/%d items uploaded (%d%%)\n", 
						completed, total, completed*100/total)
				}
			}
			
			// Use concurrent batch upload with reasonable defaults
			batchSize := 100 // Cloudflare's recommended batch size
			concurrency := 10 // Moderate concurrency
			
			startTime := time.Now()
			successCount, err := kv.WriteMultipleValuesConcurrently(
				client, 
				accountID, 
				namespaceID, 
				bulkItems,
				batchSize,
				concurrency,
				progressCallback,
			)
			duration := time.Since(startTime)
			
			if err != nil {
				fmt.Printf("Warning: some batches failed: %v\n", err)
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
				if successCount > 0 {
					// Calculate throughput
					itemsPerSecond := float64(successCount) / duration.Seconds()
					fmt.Printf("Upload throughput: %.1f items/second\n", itemsPerSecond)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "JSON file to upload")
	cmd.MarkFlagRequired("file")
	
	return cmd
}

// createBulkUploadConcurrentCmd creates a command to upload a bulk JSON file with concurrent operations
func createBulkUploadConcurrentCmd() *cobra.Command {
	var (
		batchSize   int
		concurrency int
	)
	
	cmd := &cobra.Command{
		Use:   "bulk-concurrent",
		Short: "Upload bulk JSON data to a KV namespace using concurrent operations (optimized for high rate limits)",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace using concurrent batch operations for maximum throughput.`,
		Example: `  # Upload data with default settings
  cache-kv-purger kv bulk-concurrent --namespace-id your-namespace-id --file data.json
  
  # Upload with high concurrency for faster throughput (with high rate limits)
  cache-kv-purger kv bulk-concurrent --namespace-id your-namespace-id --file data.json --concurrency 30 --batch-size 100
  
  # Upload with smaller batches but higher concurrency
  cache-kv-purger kv bulk-concurrent --namespace-id your-namespace-id --file data.json --concurrency 50 --batch-size 50`,
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
			
			// Check for input file
			if kvFlagsVars.file == "" {
				return fmt.Errorf("input file is required, specify with --file flag")
			}
			
			// Read the input file
			data, err := os.ReadFile(kvFlagsVars.file)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			
			// Parse the JSON
			type KVItem struct {
				Key           string                 `json:"key"`
				Value         interface{}            `json:"value"`
				Metadata      map[string]interface{} `json:"metadata,omitempty"`
				Expiration    int64                  `json:"expiration,omitempty"`
				ExpirationTTL int64                  `json:"expiration_ttl,omitempty"`
			}
			
			var items []KVItem
			if err := json.Unmarshal(data, &items); err != nil {
				return fmt.Errorf("failed to parse JSON input: %w", err)
			}
			
			if len(items) == 0 {
				return fmt.Errorf("no items found in input file")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			// Get verbosity flag
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			if verbose {
				fmt.Printf("Uploading %d items to KV namespace '%s' using concurrent operations\n", len(items), namespaceID)
				fmt.Printf("Performance settings: concurrency=%d, batch-size=%d\n", concurrency, batchSize)
				fmt.Println("Using concurrent upload mode for maximum throughput")
			}
			
			// Start the timer to measure performance
			startTime := time.Now()
			
			// Convert all items to bulk write items
			bulkItems := make([]kv.BulkWriteItem, len(items))
			for i, item := range items {
				// Convert value to JSON string
				valueBytes, err := json.Marshal(item.Value)
				if err != nil {
					fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				// Calculate expiration if needed
				expiration := int64(0)
				if item.Expiration > 0 {
					expiration = item.Expiration
				} else if item.ExpirationTTL > 0 {
					expiration = time.Now().Unix() + item.ExpirationTTL
				}
				
				// Add to bulk items array
				bulkItems[i] = kv.BulkWriteItem{
					Key:        item.Key,
					Value:      string(valueBytes),
					Expiration: expiration,
					Metadata:   item.Metadata,
				}
			}
			
			// Create progress callback
			progressCallback := func(completed, total int) {
				if !verbose {
					return
				}
				
				// Show progress as percentage 
				fmt.Printf("Progress: %d/%d items uploaded (%d%%)\n", 
					completed, total, completed*100/total)
			}
			
			// Upload all items using our concurrent function
			successCount, err := kv.WriteMultipleValuesConcurrently(
				client, 
				accountID, 
				namespaceID, 
				bulkItems,
				batchSize,
				concurrency,
				progressCallback,
			)
			
			// Calculate elapsed time
			duration := time.Since(startTime)
			
			if err != nil {
				fmt.Printf("Warning: some batches failed: %v\n", err)
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
				if successCount > 0 {
					// Calculate throughput
					itemsPerSecond := float64(successCount) / duration.Seconds()
					fmt.Printf("Upload throughput: %.1f items/second\n", itemsPerSecond)
				}
			}
			
			return nil
		},
	}
	
	// Add flags specific to this command
	cmd.Flags().IntVar(&batchSize, "batch-size", 100, "Number of items to include in each batch (max 10000)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 20, "Number of concurrent upload operations (recommend 10-50)")
	
	// Add standard flags
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "JSON file to upload")
	cmd.MarkFlagRequired("file")
	
	return cmd
}

// createTestDataWithMetadataCmd creates a command to generate test data with metadata
func createTestDataWithMetadataCmd() *cobra.Command {
	var (
		numItems    int
		outputFile  string
		metadataKey string
		tagValues   []string
	)

	cmd := &cobra.Command{
		Use:   "generate-test-data",
		Short: "Generate test data with metadata tags",
		Long:  `Generate a JSON file with test data including metadata tags for testing the metadata purge functionality.`,
		Example: `  # Generate 1000 test items with random metadata tags
  cache-kv-purger kv generate-test-data --count 1000 --output test-data.json --tag-field cache-tag --tag-values product,blog,homepage,api

  # Generate test data with specific tag values
  cache-kv-purger kv generate-test-data --count 500 --output test-metadata.json --tag-field status --tag-values draft,published,archived`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if numItems <= 0 {
				return fmt.Errorf("count must be greater than 0")
			}
			
			if outputFile == "" {
				return fmt.Errorf("output file is required")
			}
			
			if metadataKey == "" {
				return fmt.Errorf("tag field name is required")
			}
			
			if len(tagValues) == 0 {
				return fmt.Errorf("at least one tag value is required")
			}
			
			// Generate test data
			type TestItem struct {
				Key      string                 `json:"key"`
				Value    map[string]interface{} `json:"value"`
				Metadata map[string]interface{} `json:"metadata"`
			}
			
			testData := make([]TestItem, numItems)
			
			for i := 0; i < numItems; i++ {
				// Select a random tag value
				tagValue := tagValues[i%len(tagValues)]
				
				// Generate a UUID-like key to ensure uniqueness
				timeComponent := fmt.Sprintf("%d", time.Now().UnixNano())
				randomComponent := fmt.Sprintf("%d", i)
				key := fmt.Sprintf("test-key-%s-%s", timeComponent, randomComponent)
				
				// Create item
				testData[i] = TestItem{
					Key: key,
					Value: map[string]interface{}{
						"message": fmt.Sprintf("This is test item %d", i+1),
						"index":   i,
						"time":    time.Now().Format(time.RFC3339),
					},
					Metadata: map[string]interface{}{
						metadataKey: tagValue,
						"generated": time.Now().Unix(),
						"test":      true,
					},
				}
			}
			
			// Count tags
			tagCounts := make(map[string]int)
			for _, item := range testData {
				tagValue := item.Metadata[metadataKey].(string)
				tagCounts[tagValue]++
			}
			
			// Convert to JSON
			jsonData, err := json.MarshalIndent(testData, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to convert to JSON: %w", err)
			}
			
			// Write to file
			err = os.WriteFile(outputFile, jsonData, 0644)
			if err != nil {
				return fmt.Errorf("failed to write to file: %w", err)
			}
			
			// Show summary
			fmt.Printf("Generated %d test items with metadata and saved to %s\n", numItems, outputFile)
			fmt.Println("Tag distribution:")
			for tag, count := range tagCounts {
				fmt.Printf("  - %s: %d items (%.1f%%)\n", tag, count, float64(count)/float64(numItems)*100)
			}
			
			fmt.Println("\nYou can now upload this data with:")
			fmt.Printf("  cache-kv-purger kv bulk-batch --namespace-id <your-namespace-id> --file %s\n", outputFile)
			fmt.Println("\nAnd then test purging by metadata with:")
			fmt.Printf("  cache-kv-purger kv purge-by-metadata --namespace-id <your-namespace-id> --field %s --value <tag-value>\n", metadataKey)
			
			return nil
		},
	}
	
	cmd.Flags().IntVar(&numItems, "count", 100, "Number of test items to generate")
	cmd.Flags().StringVar(&outputFile, "output", "test-data.json", "Output file path")
	cmd.Flags().StringVar(&metadataKey, "tag-field", "cache-tag", "Metadata field name to use for tags")
	cmd.Flags().StringSliceVar(&tagValues, "tag-values", []string{"product", "blog", "homepage", "api"}, "Possible tag values (comma-separated)")
	
	return cmd
}

// createSimpleUploadCmd creates a command to upload a bulk JSON file to a KV namespace without expiration
func createSimpleUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "simple-upload",
		Short: "Upload bulk JSON data to a KV namespace (deprecated, use bulk-concurrent instead)",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace. This command is deprecated, please use bulk-concurrent instead.`,
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
			
			// Check for input file
			if kvFlagsVars.file == "" {
				return fmt.Errorf("input file is required, specify with --file flag")
			}
			
			// Print a message about the deprecated command
			fmt.Println("NOTE: This command is deprecated. Please use 'bulk-concurrent' for better performance.")
			
			// Read the input file
			data, err := os.ReadFile(kvFlagsVars.file)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			
			// Parse the JSON
			type KVItem struct {
				Key    string      `json:"key"`
				Value  interface{} `json:"value"`
			}
			
			var items []KVItem
			if err := json.Unmarshal(data, &items); err != nil {
				return fmt.Errorf("failed to parse JSON input: %w", err)
			}
			
			if len(items) == 0 {
				return fmt.Errorf("no items found in input file")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			// Convert to bulk write items for batch processing
			bulkItems := make([]kv.BulkWriteItem, len(items))
			for i, item := range items {
				// Convert value to JSON string
				valueBytes, err := json.Marshal(item.Value)
				if err != nil {
					fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				// Add to bulk items (no expiration)
				bulkItems[i] = kv.BulkWriteItem{
					Key:   item.Key,
					Value: string(valueBytes),
				}
			}
			
			// Upload using concurrent batches (which is much more efficient)
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Create progress callback
			progressCallback := func(completed, total int) {
				if verbose && total > 0 {
					fmt.Printf("Progress: %d/%d items uploaded (%d%%)\n", 
						completed, total, completed*100/total)
				}
			}
			
			// Use concurrent batch upload with reasonable defaults
			batchSize := 100 // Cloudflare's recommended batch size
			concurrency := 10 // Moderate concurrency
			
			startTime := time.Now()
			successCount, err := kv.WriteMultipleValuesConcurrently(
				client, 
				accountID, 
				namespaceID, 
				bulkItems,
				batchSize,
				concurrency,
				progressCallback,
			)
			duration := time.Since(startTime)
			
			if err != nil {
				fmt.Printf("Warning: some batches failed: %v\n", err)
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
				if successCount > 0 {
					// Calculate throughput
					itemsPerSecond := float64(successCount) / duration.Seconds()
					fmt.Printf("Upload throughput: %.1f items/second\n", itemsPerSecond)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "JSON file to upload")
	cmd.MarkFlagRequired("file")
	
	return cmd
}

// createBulkUploadCmd creates a command to upload a bulk JSON file to a KV namespace
func createBulkUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-upload",
		Short: "Upload bulk JSON data to a KV namespace (deprecated, use bulk-concurrent instead)",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace. This command is deprecated, please use bulk-concurrent for better performance.`,
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
			
			// Check for input file
			if kvFlagsVars.file == "" {
				return fmt.Errorf("input file is required, specify with --file flag")
			}
			
			// Print a message about the deprecated command
			fmt.Println("NOTE: This command is deprecated. Please use 'bulk-concurrent' for better performance.")
			
			// Read the input file
			data, err := os.ReadFile(kvFlagsVars.file)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			
			// Parse the JSON
			type KVItem struct {
				Key         string      `json:"key"`
				Value       interface{} `json:"value"`
				Expiration  int64       `json:"expiration,omitempty"`
				ExpirationTTL int64     `json:"expiration_ttl,omitempty"`
				Metadata    map[string]interface{} `json:"metadata,omitempty"`
			}
			
			var items []KVItem
			if err := json.Unmarshal(data, &items); err != nil {
				return fmt.Errorf("failed to parse JSON input: %w", err)
			}
			
			if len(items) == 0 {
				return fmt.Errorf("no items found in input file")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			// Convert to bulk write items for batch processing
			bulkItems := make([]kv.BulkWriteItem, len(items))
			for i, item := range items {
				// Convert value to JSON string
				valueBytes, err := json.Marshal(item.Value)
				if err != nil {
					fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				// Calculate expiration if needed
				expiration := int64(0)
				if item.Expiration > 0 {
					expiration = item.Expiration
				} else if item.ExpirationTTL > 0 {
					expiration = time.Now().Unix() + item.ExpirationTTL
				}
				
				// Add to bulk items
				bulkItems[i] = kv.BulkWriteItem{
					Key:        item.Key,
					Value:      string(valueBytes),
					Expiration: expiration,
					Metadata:   item.Metadata,
				}
			}
			
			// Upload using concurrent batches (which is much more efficient)
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Create progress callback
			progressCallback := func(completed, total int) {
				if verbose && total > 0 {
					fmt.Printf("Progress: %d/%d items uploaded (%d%%)\n", 
						completed, total, completed*100/total)
				}
			}
			
			// Use concurrent batch upload with reasonable defaults
			batchSize := 100 // Cloudflare's recommended batch size
			concurrency := 10 // Moderate concurrency
			
			startTime := time.Now()
			successCount, err := kv.WriteMultipleValuesConcurrently(
				client, 
				accountID, 
				namespaceID, 
				bulkItems,
				batchSize,
				concurrency,
				progressCallback,
			)
			duration := time.Since(startTime)
			
			if err != nil {
				fmt.Printf("Warning: some batches failed: %v\n", err)
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
				if successCount > 0 {
					// Calculate throughput
					itemsPerSecond := float64(successCount) / duration.Seconds()
					fmt.Printf("Upload throughput: %.1f items/second\n", itemsPerSecond)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "JSON file to upload")
	cmd.MarkFlagRequired("file")
	
	return cmd
}

// createExportNamespaceCmd creates a command to export all keys and values from a KV namespace
func createExportNamespaceCmd() *cobra.Command {
	// Define variable for flags
	var includeMetadata bool
	
	cmd := &cobra.Command{
		Use:                   "export",
		Short:                 "Export all keys and values from a KV namespace to a JSON file",
		Long:                  `Export all keys and values from a KV namespace to a JSON file for backup or migration.`,
		DisableFlagsInUseLine: false,
		Example: `  # Export a namespace using namespace ID
  cache-kv-purger kv export --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 364f2f5c31f442709ef4df47c148e76e --file export.json

  # Export a namespace using namespace title, without metadata
  cache-kv-purger kv export --account-id 01a7362d577a6c3019a474fd6f485823 --title "My KV Namespace" --file export.json --include-metadata=false`,
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
			
			// Check for output file
			outputFile := kvFlagsVars.file
			if outputFile == "" {
				return fmt.Errorf("output file is required, specify with --file flag")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			// Get metadata option
			includeMetadata, _ := cmd.Flags().GetBool("include-metadata")
			
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Exporting keys and values from namespace '%s'...\n", namespaceID)
			}
			
			// Create progress callback
			progressCallback := func(fetched, total int) {
				if verbose && total > 0 {
					fmt.Printf("Progress: %d/%d (%d%%)\n", fetched, total, fetched*100/total)
				}
			}
			
			// Export keys and values
			exportedItems, err := kv.ExportKeysAndValuesToJSON(client, accountID, namespaceID, includeMetadata, progressCallback)
			if err != nil {
				return fmt.Errorf("failed to export namespace: %w", err)
			}
			
			// Convert to JSON
			jsonData, err := json.MarshalIndent(exportedItems, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to serialize to JSON: %w", err)
			}
			
			// Write to file
			if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			
			// Output result
			fmt.Printf("Successfully exported %d keys to %s\n", len(exportedItems), outputFile)
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "Output JSON file path")
	cmd.Flags().BoolVar(&includeMetadata, "include-metadata", true, "Include metadata in the export")
	
	// Make the flag directly accessible to the root command for help to work properly
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.file, "file", "", "Output or input file path")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.key, "key", "", "Key name")
	
	cmd.MarkFlagRequired("file")
	
	return cmd
}

// createKeyExistsCmd creates a command to check if a key exists in a KV namespace
func createGetKeyWithMetadataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "get-with-metadata",
		Short:                 "Get a key including its metadata",
		Long:                  `Get a key's value and metadata from a KV namespace.`,
		DisableFlagsInUseLine: false,
		Example:               `  # Get a key with metadata using namespace ID
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
	cmd.MarkFlagRequired("key")
	
	return cmd
}

func createKeyExistsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "exists",
		Short:                 "Check if a key exists in a KV namespace",
		Long:                  `Check if a key exists in a KV namespace without retrieving its value.`,
		DisableFlagsInUseLine: false,
		Example:               `  # Check if a key exists using namespace ID
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
	cmd.MarkFlagRequired("key")
	
	return cmd
}

// createSearchValuesCmd creates a command to search/filter through KV values with regex patterns
func createSearchValuesCmd() *cobra.Command {
	// Define variables for flags
	var (
		keyPattern   string
		valuePattern string
		outputFormat string
		loadAll      bool
	)

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search KV keys and values using regex patterns",
		Long:  `Search and filter KV keys and values using regular expressions, with optional caching for performance.`,
		Example: `  # Search for keys matching a pattern
  cache-kv-purger kv search --namespace-id 364f2f5c31f442709ef4df47c148e76e --key-pattern "user_.*"

  # Search for values containing a pattern
  cache-kv-purger kv search --namespace-id 364f2f5c31f442709ef4df47c148e76e --value-pattern "cache-tag.*maintenance"

  # Search with both key and value patterns
  cache-kv-purger kv search --namespace-id 364f2f5c31f442709ef4df47c148e76e --key-pattern "page_.*" --value-pattern "status\":\"published"

  # Search with pre-loading all values in memory (faster for large searches)
  cache-kv-purger kv search --namespace-id 364f2f5c31f442709ef4df47c148e76e --value-pattern "error" --load-all

  # Output in JSON format
  cache-kv-purger kv search --namespace-id 364f2f5c31f442709ef4df47c148e76e --value-pattern "cache-tag" --format json`,
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
			
			// Validate that at least one pattern is provided
			if keyPattern == "" && valuePattern == "" {
				return fmt.Errorf("at least one pattern is required, specify with --key-pattern or --value-pattern flag")
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Create progress callback
			progressCallback := func(fetched, total int) {
				if verbose {
					if total > 0 {
						fmt.Printf("Progress: %d/%d keys (%d%%)\n", fetched, total, fetched*100/total)
					} else {
						fmt.Printf("Retrieved %d keys so far...\n", fetched)
					}
				}
			}
			
			// List all keys
			if verbose {
				fmt.Printf("Listing keys in namespace '%s'...\n", namespaceID)
			}
			
			keyList, err := kv.ListAllKeys(client, accountID, namespaceID, progressCallback)
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
			}
			
			if len(keyList) == 0 {
				fmt.Println("No keys found in the namespace")
				return nil
			}
			
			if verbose {
				fmt.Printf("Found %d keys, starting search...\n", len(keyList))
			}
			
			// Process the search
			type SearchResult struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}
			
			var results []SearchResult
			
			// Check if we need to preload all values
			if loadAll && valuePattern != "" {
				// Since we're searching values, load all data at once
				if verbose {
					fmt.Println("Pre-loading all values into memory...")
				}
				
				// Export with all values (but without metadata to save memory)
				bulkData, err := kv.ExportKeysAndValuesToJSON(client, accountID, namespaceID, false, progressCallback)
				if err != nil {
					return fmt.Errorf("failed to load values: %w", err)
				}
				
				// Build a map for quick access
				valueMap := make(map[string]string)
				for _, item := range bulkData {
					valueMap[item.Key] = item.Value
				}
				
				// Process all keys
				for _, key := range keyList {
					keyMatches := true
					valueMatches := true
					
					// Check key pattern if provided
					if keyPattern != "" {
						matches, err := matchesPattern(key.Key, keyPattern)
						if err != nil {
							return fmt.Errorf("invalid key pattern: %w", err)
						}
						keyMatches = matches
					}
					
					// If key doesn't match, skip to next
					if !keyMatches {
						continue
					}
					
					// Check value pattern if provided
					if valuePattern != "" {
						// Get the value from our preloaded map
						value, ok := valueMap[key.Key]
						if !ok {
							// This shouldn't happen, but just in case
							fmt.Printf("Warning: value for key '%s' not found in preloaded data\n", key.Key)
							continue
						}
						
						matches, err := matchesPattern(value, valuePattern)
						if err != nil {
							return fmt.Errorf("invalid value pattern: %w", err)
						}
						valueMatches = matches
					}
					
					// If both patterns match, add to results
					if keyMatches && valueMatches {
						results = append(results, SearchResult{
							Key:   key.Key,
							Value: valueMap[key.Key],
						})
					}
				}
			} else {
				// Process each key individually
				for i, key := range keyList {
					// Update progress
					if verbose && i%10 == 0 && len(keyList) > 20 {
						fmt.Printf("Processed %d/%d keys (%d%%)...\n", i, len(keyList), i*100/len(keyList))
					}
					
					keyMatches := true
					
					// Check key pattern if provided
					if keyPattern != "" {
						matches, err := matchesPattern(key.Key, keyPattern)
						if err != nil {
							return fmt.Errorf("invalid key pattern: %w", err)
						}
						
						if !matches {
							continue // Skip to next key
						}
					}
					
					// Get and check value pattern if provided
					var value string
					if valuePattern != "" {
						// Fetch the value from Cloudflare
						val, err := kv.GetValue(client, accountID, namespaceID, key.Key)
						if err != nil {
							fmt.Printf("Warning: failed to get value for key '%s': %v\n", key.Key, err)
							continue
						}
						value = val
						
						// Check if value matches the pattern
						matches, err := matchesPattern(value, valuePattern)
						if err != nil {
							return fmt.Errorf("invalid value pattern: %w", err)
						}
						
						if !matches {
							continue // Skip to next key
						}
					}
					
					// If we get here, both patterns matched or were not provided
					if valuePattern == "" && keyMatches {
						// We didn't fetch the value above, fetch it now for the results
						val, err := kv.GetValue(client, accountID, namespaceID, key.Key)
						if err != nil {
							fmt.Printf("Warning: failed to get value for key '%s': %v\n", key.Key, err)
							continue
						}
						value = val
					}
					
					// Add to results
					results = append(results, SearchResult{
						Key:   key.Key,
						Value: value,
					})
				}
			}
			
			// Display results
			if len(results) == 0 {
				fmt.Println("No matching keys found")
				return nil
			}
			
			switch outputFormat {
			case "json":
				// Output as JSON
				jsonData, err := json.MarshalIndent(results, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to format JSON results: %w", err)
				}
				fmt.Println(string(jsonData))
				
			case "table":
				// Output as a table
				fmt.Printf("Found %d matching keys:\n\n", len(results))
				fmt.Printf("%-40s | %s\n", "KEY", "VALUE")
				fmt.Printf("%s+%s\n", strings.Repeat("-", 40), strings.Repeat("-", 40))
				
				for _, result := range results {
					// Truncate value for display if it's too long
					displayValue := result.Value
					if len(displayValue) > 40 {
						displayValue = displayValue[:37] + "..."
					}
					fmt.Printf("%-40s | %s\n", result.Key, displayValue)
				}
				
			case "keys":
				// Output just the keys
				for _, result := range results {
					fmt.Println(result.Key)
				}
				
			default:
				// Default format: simple
				fmt.Printf("Found %d matching keys:\n\n", len(results))
				for i, result := range results {
					fmt.Printf("%d. Key: %s\n", i+1, result.Key)
					
					// Truncate value if it's too long
					displayValue := result.Value
					if len(displayValue) > 200 {
						displayValue = displayValue[:197] + "..."
					}
					fmt.Printf("   Value: %s\n\n", displayValue)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&keyPattern, "key-pattern", "", "Regular expression pattern to match against keys")
	cmd.Flags().StringVar(&valuePattern, "value-pattern", "", "Regular expression pattern to match against values")
	cmd.Flags().StringVar(&outputFormat, "format", "simple", "Output format: simple, table, json, or keys")
	cmd.Flags().BoolVar(&loadAll, "load-all", false, "Pre-load all values into memory for faster searching (uses more memory)")
	
	return cmd
}

// matchesPattern checks if a string matches a regular expression pattern
func matchesPattern(str, pattern string) (bool, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return regex.MatchString(str), nil
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
				fmt.Scanln(&confirm)
				
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

// createStreamingPurgeByTagCmd creates a command to purge KV values by cache-tag using streaming approach
func createStreamingPurgeByTagCmd() *cobra.Command {
	var (
		tagField    string
		chunkSize   int
		concurrency int
	)

	cmd := &cobra.Command{
		Use:   "purge-by-tag-streaming",
		Short: "Purge KV values by cache-tag using streaming (optimized for large namespaces)",
		Long:  `Find and delete KV values with matching cache-tag field value using a streaming approach that is much more efficient for large namespaces.`,
		Example: `  # Purge all entries with a specific cache-tag value
  cache-kv-purger kv purge-by-tag-streaming --namespace-id your-namespace-id --tag-value "homepage"

  # Purge all entries with any cache-tag value
  cache-kv-purger kv purge-by-tag-streaming --namespace-id your-namespace-id

  # Fine-tune performance with chunk size and concurrency
  cache-kv-purger kv purge-by-tag-streaming --namespace-id your-namespace-id --tag-value "api" --chunk-size 200 --concurrency 20

  # Preview what would be deleted without actually deleting
  cache-kv-purger kv purge-by-tag-streaming --namespace-id your-namespace-id --tag-value "product" --dry-run`,
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
			
			// Get the tag field name to search for
			if tagField == "" {
				// Default to "cache-tag" if not specified
				tagField = "cache-tag"
			}
			
			// Get the tag value to match
			tagValue := kvFlagsVars.value
			
			// Check if this is a dry run
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			
			// Verbose output
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Log initial info
			if verbose {
				if tagValue != "" {
					fmt.Printf("Finding keys with '%s' field value matching '%s' in namespace '%s'...\n", tagField, tagValue, namespaceID)
				} else {
					fmt.Printf("Finding all keys with '%s' field in namespace '%s'...\n", tagField, namespaceID)
				}
				
				if dryRun {
					fmt.Println("Running in dry-run mode. No keys will be deleted.")
				}
				
				fmt.Printf("Performance settings: chunk-size=%d, concurrency=%d\n", chunkSize, concurrency)
			}
			
			// Progress bar variables for display
			var lastProgressTime time.Time
			var lastKeysFetched, lastKeysProcessed, lastKeysDeleted int
			
			// Create the progress callback
			progressCallback := func(keysFetched, keysProcessed, keysDeleted, total int) {
				if !verbose {
					return
				}
				
				// Only update progress every 0.5 seconds to avoid flooding the terminal
				now := time.Now()
				if lastProgressTime.IsZero() || now.Sub(lastProgressTime) > 500*time.Millisecond {
					if keysFetched > lastKeysFetched {
						fmt.Printf("Progress: Listed %d keys\n", keysFetched)
						lastKeysFetched = keysFetched
					}
					
					if keysProcessed > lastKeysProcessed {
						fmt.Printf("Progress: Processed %d/%d keys (%d%%)\n", 
							keysProcessed, total, keysProcessed*100/total)
						lastKeysProcessed = keysProcessed
					}
					
					if keysDeleted > lastKeysDeleted {
						fmt.Printf("Progress: Deleted %d keys\n", keysDeleted)
						lastKeysDeleted = keysDeleted
					}
					
					lastProgressTime = now
				}
			}
			
			// Run the streaming purge
			startTime := time.Now()
			count, err := kv.StreamingPurgeByTag(client, accountID, namespaceID, tagField, tagValue, 
				chunkSize, concurrency, dryRun, progressCallback)
			duration := time.Since(startTime)
			
			if err != nil {
				return fmt.Errorf("failed during purge operation: %w", err)
			}
			
			// Output result
			if dryRun {
				// In dry run mode, count is the number of matches found
				if tagValue != "" {
					fmt.Printf("Dry run: Found %d keys with '%s' field value matching '%s'\n", count, tagField, tagValue)
				} else {
					fmt.Printf("Dry run: Found %d keys with '%s' field\n", count, tagField)
				}
			} else {
				// In actual run mode, count is the number of keys deleted
				if tagValue != "" {
					fmt.Printf("Successfully deleted %d keys with '%s' field value matching '%s'\n", count, tagField, tagValue)
				} else {
					fmt.Printf("Successfully deleted %d keys with '%s' field\n", count, tagField)
				}
			}
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&tagField, "field", "cache-tag", "The JSON field to search for in values")
	cmd.Flags().StringVar(&kvFlagsVars.value, "tag-value", "", "The tag value to match (if empty, all values will be purged)")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 100, "Number of keys to process in each chunk (affects memory usage)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 10, "Number of concurrent workers (affects performance)")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	return cmd
}

// createTestMetadataCmd creates a simplified command to test metadata features
func createTestMetadataCmd() *cobra.Command {
	var (
		metadataField string
		metadataValue string
		limit         int
	)

	cmd := &cobra.Command{
		Use:   "test-metadata",
		Short: "Test metadata functionality on a small set of keys",
		Long:  `Test metadata functionality by fetching and optionally deleting a small set of keys with specific metadata.`,
		Example: `  # Test metadata functionality
  cache-kv-purger kv test-metadata --namespace-id your-namespace-id --field cache-tag --value blog --limit 10 --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get account ID and namespace ID
			accountID := kvFlagsVars.accountID
			if accountID == "" {
				cfg, err := config.LoadFromFile("")
				if err == nil {
					accountID = cfg.GetAccountID()
				}
			}
			
			if accountID == "" {
				return fmt.Errorf("account ID is required")
			}
			
			namespaceID := kvFlagsVars.namespaceID
			if namespaceID == "" {
				if kvFlagsVars.title != "" {
					client, err := api.NewClient()
					if err != nil {
						return fmt.Errorf("failed to create API client: %w", err)
					}
					
					ns, err := kv.FindNamespaceByTitle(client, accountID, kvFlagsVars.title)
					if err != nil {
						return fmt.Errorf("failed to find namespace by title: %w", err)
					}
					
					namespaceID = ns.ID
				} else {
					return fmt.Errorf("namespace ID or title is required")
				}
			}
			
			// Create API client
			client, err := api.NewClient()
			if err != nil {
				return fmt.Errorf("failed to create API client: %w", err)
			}
			
			// Check if we should actually delete
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			
			// Get verbose flag
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// List keys
			fmt.Println("Listing keys...")
			keys, err := kv.ListKeys(client, accountID, namespaceID)
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
			}
			
			fmt.Printf("Found %d keys in namespace\n", len(keys))
			
			// Get a small sample of keys
			sampleSize := limit
			if sampleSize <= 0 {
				sampleSize = 10 // Default to 10 keys
			}
			if sampleSize > len(keys) {
				sampleSize = len(keys)
			}
			
			fmt.Printf("Checking %d keys for metadata field '%s'", sampleSize, metadataField)
			if metadataValue != "" {
				fmt.Printf(" = '%s'", metadataValue)
			}
			fmt.Println()
			
			// Check each key for metadata
			var matchingKeys []string
			
			for i, key := range keys[:sampleSize] {
				if verbose {
					fmt.Printf("Checking key %d/%d: %s\n", i+1, sampleSize, key.Key)
				}
				
				// Get key with metadata using our function
				keyWithMeta, err := kv.GetKeyWithMetadata(client, accountID, namespaceID, key.Key)
				if err != nil {
					fmt.Printf("Error fetching key %s: %v\n", key.Key, err)
					continue
				}
				
				// Check if the key has metadata
				if keyWithMeta.Metadata == nil {
					fmt.Printf("Key '%s' has no metadata\n", key.Key)
					continue
				}
				
				// Print available metadata fields (in verbose mode)
				if verbose {
					fmt.Printf("Key '%s' metadata fields: ", key.Key)
					for field := range *keyWithMeta.Metadata {
						fmt.Printf("%s ", field)
					}
					fmt.Println()
				}
				
				// Check for specific field
				metadataMap := *keyWithMeta.Metadata
				fieldValue, exists := metadataMap[metadataField]
				if !exists {
					if verbose {
						fmt.Printf("Key '%s' does not have metadata field '%s'\n", key.Key, metadataField)
					}
					continue
				}
				
				// Check value if provided
				if metadataValue == "" || fmt.Sprintf("%v", fieldValue) == metadataValue {
					matchingKeys = append(matchingKeys, key.Key)
					fmt.Printf("Match found: key '%s' has %s = %v\n", key.Key, metadataField, fieldValue)
				}
			}
			
			// Show what we found
			fmt.Printf("\nFound %d matching keys with metadata field '%s'", 
				len(matchingKeys), metadataField)
			if metadataValue != "" {
				fmt.Printf(" = '%s'", metadataValue)
			}
			fmt.Println()
			
			// If dry run, we're done
			if dryRun || len(matchingKeys) == 0 {
				return nil
			}
			
			// Delete matching keys
			fmt.Printf("Deleting %d matching keys...\n", len(matchingKeys))
			err = kv.DeleteMultipleValues(client, accountID, namespaceID, matchingKeys)
			if err != nil {
				return fmt.Errorf("failed to delete keys: %w", err)
			}
			fmt.Printf("Successfully deleted %d keys\n", len(matchingKeys))
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&metadataField, "field", "cache-tag", "The metadata field to search for")
	cmd.Flags().StringVar(&metadataValue, "value", "", "The metadata value to match (if empty, any value will match)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Limit the number of keys to process")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	return cmd
}

// createPurgeByMetadataOnlyCmd creates a command to purge KV values by metadata field/value using only metadata
func createPurgeByMetadataOnlyCmd() *cobra.Command {
	var (
		metadataField string
		metadataValue string
		chunkSize     int
		concurrency   int
	)

	cmd := &cobra.Command{
		Use:   "purge-by-metadata-only",
		Short: "Purge KV values by metadata field/value (extremely fast)",
		Long:  `Find and delete KV values with matching metadata field and value using metadata-only approach for maximum performance.`,
		Example: `  # Purge all entries with a specific metadata field value
  cache-kv-purger kv purge-by-metadata-only --namespace-id your-namespace-id --field "cache-tag" --value "blog"

  # Purge all entries with any value for a metadata field
  cache-kv-purger kv purge-by-metadata-only --namespace-id your-namespace-id --field "cache-tag"

  # Fine-tune performance with chunk size and concurrency
  cache-kv-purger kv purge-by-metadata-only --namespace-id your-namespace-id --field "cache-tag" --value "api" --chunk-size 1000 --concurrency 50

  # Preview what would be deleted without actually deleting
  cache-kv-purger kv purge-by-metadata-only --namespace-id your-namespace-id --field "cache-tag" --value "product" --dry-run`,
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
			
			// Check if dry run
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			
			// Verbose output
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Log initial info
			if verbose {
				if metadataValue != "" {
					fmt.Printf("Finding keys with metadata field '%s' value matching '%s' in namespace '%s'...\n", 
						metadataField, metadataValue, namespaceID)
				} else {
					fmt.Printf("Finding all keys with metadata field '%s' in namespace '%s'...\n", 
						metadataField, namespaceID)
				}
				
				if dryRun {
					fmt.Println("Running in dry-run mode. No keys will be deleted.")
				}
				
				fmt.Printf("Performance settings: chunk-size=%d, concurrency=%d (metadata-only mode for maximum speed)\n", 
					chunkSize, concurrency)
			}
			
			// Progress tracking
			var lastProgressTime time.Time
			var lastKeysFetched, lastKeysProcessed, lastKeysMatched, lastKeysDeleted int
			
			// Create progress callback
			progressCallback := func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
				if !verbose {
					return
				}
				
				// Only update progress every 0.5 seconds to avoid flooding terminal
				now := time.Now()
				if lastProgressTime.IsZero() || now.Sub(lastProgressTime) > 500*time.Millisecond {
					if keysFetched > lastKeysFetched {
						fmt.Printf("Progress: Listed %d keys\n", keysFetched)
						lastKeysFetched = keysFetched
					}
					
					if keysProcessed > lastKeysProcessed {
						fmt.Printf("Progress: Processed %d/%d keys (%d%%)\n", 
							keysProcessed, total, keysProcessed*100/total)
						lastKeysProcessed = keysProcessed
					}
					
					if keysMatched > lastKeysMatched {
						fmt.Printf("Progress: Found %d matching keys\n", keysMatched)
						lastKeysMatched = keysMatched
					}
					
					if keysDeleted > lastKeysDeleted {
						fmt.Printf("Progress: Deleted %d keys\n", keysDeleted)
						lastKeysDeleted = keysDeleted
					}
					
					lastProgressTime = now
				}
			}
			
			// Run the purge using metadata-only approach for maximum performance
			startTime := time.Now()
			count, err := kv.PurgeByMetadataOnly(client, accountID, namespaceID, metadataField, metadataValue,
				chunkSize, concurrency, dryRun, progressCallback)
			duration := time.Since(startTime)
			
			if err != nil {
				return fmt.Errorf("failed during purge operation: %w", err)
			}
			
			// Output result
			if dryRun {
				// In dry run mode, count is the number of matches found
				if metadataValue != "" {
					fmt.Printf("Dry run: Found %d keys with metadata field '%s' value matching '%s'\n", 
						count, metadataField, metadataValue)
				} else {
					fmt.Printf("Dry run: Found %d keys with metadata field '%s'\n", 
						count, metadataField)
				}
			} else {
				// In actual run mode, count is the number of keys deleted
				if metadataValue != "" {
					fmt.Printf("Successfully deleted %d keys with metadata field '%s' value matching '%s'\n", 
						count, metadataField, metadataValue)
				} else {
					fmt.Printf("Successfully deleted %d keys with metadata field '%s'\n", 
						count, metadataField)
				}
			}
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&metadataField, "field", "cache-tag", "The metadata field to search for")
	cmd.Flags().StringVar(&metadataValue, "value", "", "The metadata value to match (if empty, any value will match)")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 1000, "Number of keys to process in each chunk (affects memory usage)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1000, "Number of concurrent workers (max 1000, affects performance)")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	return cmd
}

// createPurgeByMetadataUpfrontCmd creates a command to purge KV values by metadata using upfront loading approach
func createPurgeByMetadataUpfrontCmd() *cobra.Command {
	var (
		metadataField string
		metadataValue string
		concurrency   int
	)

	cmd := &cobra.Command{
		Use:   "purge-by-metadata-upfront",
		Short: "Purge KV values by metadata with upfront loading (for high rate limits)",
		Long:  `Find and delete KV values with matching metadata by loading all metadata upfront in memory. Optimized for high API rate limits.`,
		Example: `  # Purge all entries with a specific metadata field value
  cache-kv-purger kv purge-by-metadata-upfront --namespace-id your-namespace-id --field "cache-tag" --value "blog"

  # Purge all entries with any value for a metadata field
  cache-kv-purger kv purge-by-metadata-upfront --namespace-id your-namespace-id --field "cache-tag"

  # Fine-tune performance with concurrency
  cache-kv-purger kv purge-by-metadata-upfront --namespace-id your-namespace-id --field "cache-tag" --concurrency 500

  # Preview what would be deleted without actually deleting
  cache-kv-purger kv purge-by-metadata-upfront --namespace-id your-namespace-id --field "cache-tag" --value "product" --dry-run`,
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
			
			// Check if dry run
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			
			// Verbose output
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Log initial info
			if verbose {
				if metadataValue != "" {
					fmt.Printf("Finding keys with metadata field '%s' value matching '%s' in namespace '%s'...\n", 
						metadataField, metadataValue, namespaceID)
				} else {
					fmt.Printf("Finding all keys with metadata field '%s' in namespace '%s'...\n", 
						metadataField, namespaceID)
				}
				
				if dryRun {
					fmt.Println("Running in dry-run mode. No keys will be deleted.")
				}
				
				fmt.Printf("Performance settings: concurrency=%d (upfront loading mode for maximum throughput)\n", concurrency)
				fmt.Println("This mode loads all metadata upfront before processing - optimal for high API rate limits")
			}
			
			// Progress tracking
			var lastProgressTime time.Time
			var lastKeysFetched, lastKeysProcessed, lastKeysMatched, lastKeysDeleted int
			
			// Create progress callback
			progressCallback := func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
				if !verbose {
					return
				}
				
				// Only update progress every 0.5 seconds to avoid flooding terminal
				now := time.Now()
				if lastProgressTime.IsZero() || now.Sub(lastProgressTime) > 500*time.Millisecond {
					if keysFetched > lastKeysFetched {
						fmt.Printf("Progress: Listed %d keys\n", keysFetched)
						lastKeysFetched = keysFetched
					}
					
					if keysProcessed > lastKeysProcessed {
						fmt.Printf("Progress: Processed %d/%d keys\n", keysProcessed, total)
						lastKeysProcessed = keysProcessed
					}
					
					if keysMatched > lastKeysMatched {
						fmt.Printf("Progress: Found %d matching keys\n", keysMatched)
						lastKeysMatched = keysMatched
					}
					
					if keysDeleted > lastKeysDeleted {
						fmt.Printf("Progress: Deleted %d keys\n", keysDeleted)
						lastKeysDeleted = keysDeleted
					}
					
					lastProgressTime = now
				}
			}
			
			// Run the purge using upfront loading approach for maximum throughput
			startTime := time.Now()
			count, err := kv.PurgeByMetadataUpfront(client, accountID, namespaceID, metadataField, metadataValue,
				concurrency, dryRun, progressCallback)
			duration := time.Since(startTime)
			
			if err != nil {
				return fmt.Errorf("failed during purge operation: %w", err)
			}
			
			// Output result
			if dryRun {
				// In dry run mode, count is the number of matches found
				if metadataValue != "" {
					fmt.Printf("Dry run: Found %d keys with metadata field '%s' value matching '%s'\n", 
						count, metadataField, metadataValue)
				} else {
					fmt.Printf("Dry run: Found %d keys with metadata field '%s'\n", 
						count, metadataField)
				}
			} else {
				// In actual run mode, count is the number of keys deleted
				if metadataValue != "" {
					fmt.Printf("Successfully deleted %d keys with metadata field '%s' value matching '%s'\n", 
						count, metadataField, metadataValue)
				} else {
					fmt.Printf("Successfully deleted %d keys with metadata field '%s'\n", 
						count, metadataField)
				}
			}
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&metadataField, "field", "cache-tag", "The metadata field to search for")
	cmd.Flags().StringVar(&metadataValue, "value", "", "The metadata value to match (if empty, any value will match)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 500, "Number of concurrent workers (recommend 500-1000 for high rate limits)")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	return cmd
}

// createPurgeByMetadataCmd creates a command to purge KV values by metadata field/value
func createPurgeByMetadataCmd() *cobra.Command {
	var (
		metadataField string
		metadataValue string
		chunkSize     int
		concurrency   int
	)

	cmd := &cobra.Command{
		Use:   "purge-by-metadata",
		Short: "Purge KV values by metadata field/value (deprecated, use purge-by-metadata-only instead)",
		Long:  `Find and delete KV values with matching metadata field and value. This command is deprecated, please use purge-by-metadata-only or purge-by-metadata-upfront instead.`,
		Example: `  # Purge all entries with a specific metadata field value
  cache-kv-purger kv purge-by-metadata --namespace-id your-namespace-id --field "type" --value "temporary"

  # Purge all entries with any value for a metadata field
  cache-kv-purger kv purge-by-metadata --namespace-id your-namespace-id --field "expirable"

  # Preview what would be deleted without actually deleting
  cache-kv-purger kv purge-by-metadata --namespace-id your-namespace-id --field "status" --value "draft" --dry-run`,
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
			
			// Print a message about the deprecated command
			fmt.Println("NOTE: This command is deprecated. Please use 'purge-by-metadata-only' for better performance with metadata operations.")
			
			// Check if dry run
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			
			// Verbose output
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			// Progress tracking
			var lastProgressTime time.Time
			var lastKeysFetched, lastKeysProcessed, lastKeysMatched, lastKeysDeleted int
			
			// Create progress callback
			progressCallback := func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
				if !verbose {
					return
				}
				
				// Only update progress every 0.5 seconds to avoid flooding terminal
				now := time.Now()
				if lastProgressTime.IsZero() || now.Sub(lastProgressTime) > 500*time.Millisecond {
					if keysFetched > lastKeysFetched {
						fmt.Printf("Progress: Listed %d keys\n", keysFetched)
						lastKeysFetched = keysFetched
					}
					
					if keysProcessed > lastKeysProcessed {
						fmt.Printf("Progress: Processed %d/%d keys (%d%%)\n", 
							keysProcessed, total, keysProcessed*100/total)
						lastKeysProcessed = keysProcessed
					}
					
					if keysMatched > lastKeysMatched {
						fmt.Printf("Progress: Found %d matching keys\n", keysMatched)
						lastKeysMatched = keysMatched
					}
					
					if keysDeleted > lastKeysDeleted {
						fmt.Printf("Progress: Deleted %d keys\n", keysDeleted)
						lastKeysDeleted = keysDeleted
					}
					
					lastProgressTime = now
				}
			}
			
			// Use metadata-only implementation which is more efficient
			startTime := time.Now()
			count, err := kv.PurgeByMetadataOnly(client, accountID, namespaceID, metadataField, metadataValue,
				chunkSize, concurrency, dryRun, progressCallback)
			duration := time.Since(startTime)
			
			if err != nil {
				return fmt.Errorf("failed during purge operation: %w", err)
			}
			
			// Output result
			if dryRun {
				// In dry run mode, count is the number of matches found
				if metadataValue != "" {
					fmt.Printf("Dry run: Found %d keys with metadata field '%s' value matching '%s'\n", 
						count, metadataField, metadataValue)
				} else {
					fmt.Printf("Dry run: Found %d keys with metadata field '%s'\n", 
						count, metadataField)
				}
			} else {
				// In actual run mode, count is the number of keys deleted
				if metadataValue != "" {
					fmt.Printf("Successfully deleted %d keys with metadata field '%s' value matching '%s'\n", 
						count, metadataField, metadataValue)
				} else {
					fmt.Printf("Successfully deleted %d keys with metadata field '%s'\n", 
						count, metadataField)
				}
			}
			
			if verbose {
				fmt.Printf("Operation completed in %s\n", duration.Round(time.Millisecond))
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&metadataField, "field", "", "The metadata field to search for")
	cmd.Flags().StringVar(&metadataValue, "value", "", "The metadata value to match (if empty, any value will match)")
	cmd.Flags().IntVar(&chunkSize, "chunk-size", 1000, "Number of keys to process in each chunk (affects memory usage)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 20, "Number of concurrent workers (affects performance)")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	
	// Add a deprecated flag for backward compatibility
	cmd.Flags().Int("limit", 0, "Deprecated: This flag has no effect")
	cmd.Flags().MarkDeprecated("limit", "This flag is deprecated and has no effect")
	
	cmd.MarkFlagRequired("field")
	
	return cmd
}

func init() {
	rootCmd.AddCommand(kvCmd)
	
	// Add common flags to kv command
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.accountID, "account-id", "", "Cloudflare Account ID")
	
	// Add namespace commands
	kvCmd.AddCommand(kvNamespaceCmd)
	kvNamespaceCmd.AddCommand(createNamespaceListCmd())
	kvNamespaceCmd.AddCommand(createNamespaceCreateCmd())
	kvNamespaceCmd.AddCommand(createNamespaceDeleteCmd())
	kvNamespaceCmd.AddCommand(createNamespaceRenameCmd())
	kvNamespaceCmd.AddCommand(createNamespaceBulkDeleteCmd())
	
	// Add values commands
	kvCmd.AddCommand(kvValuesCmd)
	kvValuesCmd.AddCommand(createValuesListCmd())
	kvValuesCmd.AddCommand(createValuesGetCmd())
	kvValuesCmd.AddCommand(createValuesPutCmd())
	kvValuesCmd.AddCommand(createValuesDeleteCmd())
	
	// Add utility commands directly to kvCmd for better discoverability
	kvCmd.AddCommand(createKeyExistsCmd())
	kvCmd.AddCommand(createGetKeyWithMetadataCmd())
	
	// Add core purge commands (newer, optimized versions first)
	kvCmd.AddCommand(createPurgeByMetadataUpfrontCmd()) // Recommended for high rate limits
	kvCmd.AddCommand(createPurgeByMetadataOnlyCmd())    // Best for metadata-based purging
	kvCmd.AddCommand(createStreamingPurgeByTagCmd())    // Best for tag-based purging
	
	// Add legacy purge commands (marked as deprecated)
	kvCmd.AddCommand(createPurgeByTagCmd())             // Legacy, uses streaming now
	kvCmd.AddCommand(createPurgeByMetadataCmd())        // Legacy, uses metadata-only implementation
	kvCmd.AddCommand(createTestMetadataCmd())           // Test utility
	
	// Add upload commands (newer, optimized versions first)
	kvCmd.AddCommand(createBulkUploadConcurrentCmd())   // Recommended bulk uploader
	
	// Add legacy upload commands (marked as deprecated)
	kvCmd.AddCommand(createBulkUploadCmd())             // Legacy, uses concurrent now
	kvCmd.AddCommand(createSimpleUploadCmd())           // Legacy, uses concurrent now
	kvCmd.AddCommand(createBulkUploadBatchCmd())        // Legacy, uses concurrent now
	
	// Add utility commands
	kvCmd.AddCommand(createExportNamespaceCmd())
	kvCmd.AddCommand(createSearchValuesCmd())
	kvCmd.AddCommand(createTestDataWithMetadataCmd())
	
	// Add config command
	kvCmd.AddCommand(createKVConfigCmd())
}
