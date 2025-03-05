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
func createPurgeByTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purge-by-tag",
		Short: "Purge KV values by cache-tag",
		Long:  `Find and delete KV values with matching cache-tag field value.`,
		Example: `  # Purge all entries with a specific cache-tag value
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id --tag-value "homepage"

  # Purge all entries with any cache-tag value
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id

  # Use a different field name instead of cache-tag
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id --field "category" --tag-value "blog"

  # Use memory-efficient mode for small namespaces
  cache-kv-purger kv purge-by-tag --namespace-id your-namespace-id --tag-value "api" --memory-efficient

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
			
			// Check if memory-efficient mode is enabled
			memoryEfficient, _ := cmd.Flags().GetBool("memory-efficient")
			
			// Get batch size for deletion
			batchSize, _ := cmd.Flags().GetInt("batch-size")
			if batchSize <= 0 {
				batchSize = 25 // Default batch size for deletes
			}
			
			// List keys in the namespace
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				if tagValue != "" {
					fmt.Printf("Finding keys with '%s' field value matching '%s' in namespace '%s'...\n", field, tagValue, namespaceID)
				} else {
					fmt.Printf("Finding all keys with '%s' field in namespace '%s'...\n", field, namespaceID)
				}
			}
			
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
			keys, err := kv.ListAllKeys(client, accountID, namespaceID, progressCallback)
			if err != nil {
				return fmt.Errorf("failed to list keys: %w", err)
			}
			
			if len(keys) == 0 {
				fmt.Println("No keys found in the namespace")
				return nil
			}
			
			if verbose {
				fmt.Printf("Found %d keys to examine\n", len(keys))
			}
			
			// Create a map to store matching keys and their tag values
			keysToDelete := []string{}
			matchedTags := make(map[string]string) // Map of tag value to key
			
			// Choose between memory-efficient vs. high performance mode
			if memoryEfficient {
				// Memory-efficient mode: Check each key individually
				for i, key := range keys {
					if verbose && i%10 == 0 && len(keys) > 20 {
						fmt.Printf("Checking key %d/%d (%d%%)...\n", i+1, len(keys), (i+1)*100/len(keys))
					}
					
					// Get the value for this key
					valueStr, err := kv.GetValue(client, accountID, namespaceID, key.Key)
					if err != nil {
						fmt.Printf("Warning: failed to get value for key '%s': %v\n", key.Key, err)
						continue
					}
					
					// Try to parse it as JSON
					var valueMap map[string]interface{}
					if err := json.Unmarshal([]byte(valueStr), &valueMap); err != nil {
						if verbose {
							fmt.Printf("Key '%s' does not contain valid JSON\n", key.Key)
						}
						continue
					}
					
					// Look for the tag field
					foundTagValue, ok := valueMap[field]
					if !ok {
						if verbose {
							fmt.Printf("Key '%s' does not contain '%s' field\n", key.Key, field)
						}
						continue
					}
					
					// Convert tag value to string
					foundTagStr, ok := foundTagValue.(string)
					if !ok {
						if verbose {
							fmt.Printf("Key '%s' has '%s' field but it's not a string\n", key.Key, field)
						}
						continue
					}
					
					// If a specific tag value was provided, check if it matches
					if tagValue != "" && foundTagStr != tagValue {
						if verbose {
							fmt.Printf("Key '%s' has '%s' field with value '%s' (not matching '%s')\n", key.Key, field, foundTagStr, tagValue)
						}
						continue
					}
					
					if verbose {
						fmt.Printf("Key '%s' has '%s' field with value '%s' - MATCH\n", key.Key, field, foundTagStr)
					}
					
					// Store the match
					matchedTags[foundTagStr] = key.Key
					keysToDelete = append(keysToDelete, key.Key)
				}
			} else {
				// High-performance mode: Load all values into memory first
				if verbose {
					fmt.Println("Loading all values into memory for faster processing...")
				}
				
				// Export with all values (but without metadata to save memory)
				bulkData, err := kv.ExportKeysAndValuesToJSON(client, accountID, namespaceID, false, progressCallback)
				if err != nil {
					return fmt.Errorf("failed to load values: %w", err)
				}
				
				// Process all values in memory
				for _, item := range bulkData {
					// Try to parse value as JSON
					var valueMap map[string]interface{}
					if err := json.Unmarshal([]byte(item.Value), &valueMap); err != nil {
						if verbose {
							fmt.Printf("Key '%s' does not contain valid JSON\n", item.Key)
						}
						continue
					}
					
					// Look for the tag field
					foundTagValue, ok := valueMap[field]
					if !ok {
						if verbose {
							fmt.Printf("Key '%s' does not contain '%s' field\n", item.Key, field)
						}
						continue
					}
					
					// Convert tag value to string
					foundTagStr, ok := foundTagValue.(string)
					if !ok {
						if verbose {
							fmt.Printf("Key '%s' has '%s' field but it's not a string\n", item.Key, field)
						}
						continue
					}
					
					// If a specific tag value was provided, check if it matches
					if tagValue != "" && foundTagStr != tagValue {
						if verbose {
							fmt.Printf("Key '%s' has '%s' field with value '%s' (not matching '%s')\n", item.Key, field, foundTagStr, tagValue)
						}
						continue
					}
					
					if verbose {
						fmt.Printf("Key '%s' has '%s' field with value '%s' - MATCH\n", item.Key, field, foundTagStr)
					}
					
					// Store the match
					matchedTags[foundTagStr] = item.Key
					keysToDelete = append(keysToDelete, item.Key)
				}
			}
			
			// Check if we should use batch deletion
			useBulkDelete, _ := cmd.Flags().GetBool("use-bulk-delete")
			
			// If flag to delete is set, delete the keys
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if len(keysToDelete) > 0 {
				if dryRun {
					if tagValue != "" {
						fmt.Printf("Dry run: Would delete %d keys with '%s' field value matching '%s':\n", len(keysToDelete), field, tagValue)
					} else {
						fmt.Printf("Dry run: Would delete %d keys with '%s' field:\n", len(keysToDelete), field)
					}
					// Display up to 10 keys to avoid overwhelming output
					maxDisplay := 10
					if len(keysToDelete) < maxDisplay {
						maxDisplay = len(keysToDelete)
					}
					
					for i := 0; i < maxDisplay; i++ {
						fmt.Printf("  - Key: %s\n", keysToDelete[i])
					}
					
					if len(keysToDelete) > maxDisplay {
						fmt.Printf("  ...and %d more keys\n", len(keysToDelete)-maxDisplay)
					}
				} else {
					if tagValue != "" {
						fmt.Printf("Deleting %d keys with '%s' field value matching '%s'...\n", len(keysToDelete), field, tagValue)
					} else {
						fmt.Printf("Deleting %d keys with '%s' field...\n", len(keysToDelete), field)
					}
					
					successCount := 0
					
					if useBulkDelete {
						// Use bulk deletions in batches
						for i := 0; i < len(keysToDelete); i += batchSize {
							end := i + batchSize
							if end > len(keysToDelete) {
								end = len(keysToDelete)
							}
							
							batch := keysToDelete[i:end]
							
							if verbose {
								fmt.Printf("Deleting batch %d/%d (%d keys)...\n", 
									(i/batchSize)+1, 
									(len(keysToDelete)+batchSize-1)/batchSize, 
									len(batch))
							}
							
							// Delete batch
							err := kv.DeleteMultipleValues(client, accountID, namespaceID, batch)
							if err != nil {
								fmt.Printf("Warning: batch deletion partially failed: %v\n", err)
							} else {
								successCount += len(batch)
							}
						}
					} else {
						// Use individual deletes
						for i, key := range keysToDelete {
							if verbose && (i+1) % batchSize == 0 {
								fmt.Printf("Progress: %d/%d keys deleted (%d%%)...\n", 
									i+1, len(keysToDelete), (i+1)*100/len(keysToDelete))
							}
							
							// Delete individual key
							if err := kv.DeleteValue(client, accountID, namespaceID, key); err != nil {
								fmt.Printf("Warning: failed to delete key '%s': %v\n", key, err)
							} else {
								successCount++
							}
						}
					}
					
					if verbose && len(keysToDelete) > batchSize {
						fmt.Printf("Completed: %d/%d keys deleted successfully\n", successCount, len(keysToDelete))
					}
					
					if tagValue != "" {
						fmt.Printf("Successfully deleted %d keys with '%s' field value matching '%s'\n", successCount, field, tagValue)
					} else {
						fmt.Printf("Successfully deleted %d keys with '%s' field\n", successCount, field)
					}
				}
			} else {
				if tagValue != "" {
					fmt.Printf("No keys found with '%s' field value matching '%s'\n", field, tagValue)
				} else {
					fmt.Printf("No keys found with '%s' field\n", field)
				}
			}
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().String("field", "cache-tag", "The JSON field to search for in values")
	cmd.Flags().StringVar(&kvFlagsVars.value, "tag-value", "", "The tag value to match (if empty, all values will be purged)")
	cmd.Flags().Bool("dry-run", false, "Show what would be deleted without actually deleting")
	cmd.Flags().Bool("memory-efficient", false, "Process one key at a time instead of loading all values in memory (slower but uses less memory)")
	cmd.Flags().Bool("use-bulk-delete", true, "Use bulk delete API instead of individual deletes (faster for large operations)")
	cmd.Flags().Int("batch-size", 10000, "Number of items to delete in each batch (max 10000)")
	
	return cmd
}

// createBulkUploadBatchCmd creates a command to upload a bulk JSON file to a KV namespace using batch operations
func createBulkUploadBatchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-batch",
		Short: "Upload bulk JSON data to a KV namespace using efficient batch operations",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace using efficient batch operations.`,
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
				Key         string      `json:"key"`
				Value       interface{} `json:"value"`
				Expiration  int64       `json:"expiration,omitempty"`
				ExpirationTTL int64     `json:"expiration_ttl,omitempty"`
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
			
			// Process in batches
			batchSize := 100 // Cloudflare's maximum batch size
			verbose, _ := cmd.Flags().GetBool("verbose")
			
			if verbose {
				fmt.Printf("Uploading %d items to KV namespace '%s' in batches of %d...\n", len(items), namespaceID, batchSize)
			}
			
			successCount := 0
			for i := 0; i < len(items); i += batchSize {
				end := i + batchSize
				if end > len(items) {
					end = len(items)
				}
				
				batch := items[i:end]
				
				if verbose {
					fmt.Printf("Processing batch %d/%d (%d items)...\n", 
						(i/batchSize)+1, 
						(len(items)+batchSize-1)/batchSize, 
						len(batch))
				}
				
				// Convert batch to bulk write items
				bulkItems := make([]kv.BulkWriteItem, len(batch))
				for j, item := range batch {
					// Convert value to JSON string
					valueBytes, err := json.Marshal(item.Value)
					if err != nil {
						fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
						continue
					}
					
					// Add to bulk items
					expiration := int64(0)
					if item.ExpirationTTL > 0 {
						// Use TTL to calculate expiration
						expiration = time.Now().Unix() + item.ExpirationTTL
					}
					
					bulkItems[j] = kv.BulkWriteItem{
						Key:        item.Key,
						Value:      string(valueBytes),
						Expiration: expiration,
					}
				}
				
				// Write batch
				err = kv.WriteMultipleValues(client, accountID, namespaceID, bulkItems)
				if err != nil {
					fmt.Printf("Warning: failed to write batch: %v\n", err)
					continue
				}
				
				successCount += len(batch)
				
				if verbose {
					fmt.Printf("Successfully uploaded batch %d/%d\n", 
						(i/batchSize)+1, 
						(len(items)+batchSize-1)/batchSize)
				}
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
			return nil
		},
	}
	
	cmd.Flags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	cmd.Flags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace (alternative to namespace-id)")
	cmd.Flags().StringVar(&kvFlagsVars.file, "file", "", "JSON file to upload")
	cmd.MarkFlagRequired("file")
	
	return cmd
}

// createSimpleUploadCmd creates a command to upload a bulk JSON file to a KV namespace without expiration
func createSimpleUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "simple-upload",
		Short: "Upload bulk JSON data to a KV namespace (ignoring expiration)",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace, ignoring any expiration settings.`,
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
				Key         string      `json:"key"`
				Value       interface{} `json:"value"`
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
			
			// Upload each item
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Uploading %d items to KV namespace '%s'...\n", len(items), namespaceID)
			}
			
			successCount := 0
			for i, item := range items {
				if verbose {
					fmt.Printf("Uploading item %d/%d: %s\n", i+1, len(items), item.Key)
				}
				
				// Convert value to JSON string
				valueBytes, err := json.Marshal(item.Value)
				if err != nil {
					fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				// Write the value - no expiration
				err = kv.WriteValue(client, accountID, namespaceID, item.Key, string(valueBytes), nil)
				if err != nil {
					fmt.Printf("Warning: failed to write value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				successCount++
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
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
		Short: "Upload bulk JSON data to a KV namespace",
		Long:  `Upload a JSON file containing an array of key-value pairs to a KV namespace.`,
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
				Key         string      `json:"key"`
				Value       interface{} `json:"value"`
				Expiration  int64       `json:"expiration,omitempty"`
				ExpirationTTL int64     `json:"expiration_ttl,omitempty"`
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
			
			// Upload each item
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Printf("Uploading %d items to KV namespace '%s'...\n", len(items), namespaceID)
			}
			
			successCount := 0
			for i, item := range items {
				if verbose {
					fmt.Printf("Uploading item %d/%d: %s\n", i+1, len(items), item.Key)
				}
				
				// Convert value to JSON string
				valueBytes, err := json.Marshal(item.Value)
				if err != nil {
					fmt.Printf("Warning: failed to encode value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				// Create options if expiration is set
				var options *kv.WriteOptions
				if item.Expiration > 0 || item.ExpirationTTL > 0 {
					options = &kv.WriteOptions{}
					
					if item.Expiration > 0 {
						options.Expiration = item.Expiration
					} else if item.ExpirationTTL > 0 {
						// Calculate expiration from TTL
						options.Expiration = time.Now().Unix() + item.ExpirationTTL
					}
				}
				
				// Write the value
				err = kv.WriteValue(client, accountID, namespaceID, item.Key, string(valueBytes), options)
				if err != nil {
					fmt.Printf("Warning: failed to write value for key '%s': %v\n", item.Key, err)
					continue
				}
				
				successCount++
			}
			
			// Output result
			fmt.Printf("Successfully uploaded %d/%d items to KV namespace\n", successCount, len(items))
			
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
	
	// Add values commands
	kvCmd.AddCommand(kvValuesCmd)
	kvValuesCmd.AddCommand(createValuesListCmd())
	kvValuesCmd.AddCommand(createValuesGetCmd())
	kvValuesCmd.AddCommand(createValuesPutCmd())
	kvValuesCmd.AddCommand(createValuesDeleteCmd())
	
	// Add utility commands directly to kvCmd for better discoverability
	kvCmd.AddCommand(createKeyExistsCmd())
	kvCmd.AddCommand(createPurgeByTagCmd())
	kvCmd.AddCommand(createBulkUploadCmd())
	kvCmd.AddCommand(createSimpleUploadCmd())
	kvCmd.AddCommand(createBulkUploadBatchCmd())
	kvCmd.AddCommand(createExportNamespaceCmd())
	kvCmd.AddCommand(createSearchValuesCmd())
	
	// Add config command
	kvCmd.AddCommand(createKVConfigCmd())
}
