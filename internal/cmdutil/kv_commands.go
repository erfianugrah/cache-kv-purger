package cmdutil

import (
	"fmt"

	"github.com/spf13/cobra"
)

// RegisterKVCommands registers all KV commands with the given root command
func RegisterKVCommands(rootCmd *cobra.Command) {
	// Main KV command
	kvCmd := &cobra.Command{
		Use:   "kv",
		Short: "Manage KV store",
		Long:  "Manage Cloudflare Workers KV store namespaces and values",
	}

	// Add new verb-based commands
	kvCmd.AddCommand(NewKVListCommand().Build())
	kvCmd.AddCommand(NewKVGetCommand().Build())
	kvCmd.AddCommand(NewKVPutCommand().Build())
	kvCmd.AddCommand(NewKVDeleteCommand().Build())
	kvCmd.AddCommand(NewKVCreateCommand().Build())
	kvCmd.AddCommand(NewKVRenameCommand().Build())
	kvCmd.AddCommand(NewKVConfigCommand().Build())

	// Register legacy commands with deprecation notices
	registerLegacyKVCommands(kvCmd)

	rootCmd.AddCommand(kvCmd)
}

// registerLegacyKVCommands registers the old command structure with deprecation notices
func registerLegacyKVCommands(kvCmd *cobra.Command) {
	// Legacy namespace commands
	nsCmd := &cobra.Command{
		Use:        "namespace",
		Short:      "Manage KV namespaces (deprecated)",
		Long:       "Manage KV namespaces - deprecated, use verb commands directly",
		Deprecated: "Use 'kv list', 'kv create', 'kv delete', etc. instead",
	}

	// List namespace legacy command
	nsCmd.AddCommand(&cobra.Command{
		Use:        "list",
		Short:      "List namespaces (deprecated)",
		Long:       "List namespaces - deprecated, use 'kv list' instead",
		Deprecated: "Use 'kv list' instead",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("This command is deprecated. Please use 'kv list' instead.")
			fmt.Println("Example: cache-kv-purger kv list --account-id YOUR_ACCOUNT_ID")
		},
	})

	// Add other legacy namespace commands...
	// (create, delete, rename, bulk-delete)

	// Legacy values commands
	valuesCmd := &cobra.Command{
		Use:        "values",
		Short:      "Manage KV key-values (deprecated)",
		Long:       "Manage KV key-values - deprecated, use verb commands directly",
		Deprecated: "Use 'kv list', 'kv get', 'kv put', 'kv delete' instead",
	}

	// List values legacy command
	valuesCmd.AddCommand(&cobra.Command{
		Use:        "list",
		Short:      "List keys (deprecated)",
		Long:       "List keys - deprecated, use 'kv list --namespace-id ID' instead",
		Deprecated: "Use 'kv list --namespace-id ID' instead",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("This command is deprecated. Please use 'kv list' instead.")
			fmt.Println("Example: cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID")
		},
	})

	// Add other legacy values commands...
	// (get, put, delete)

	// Legacy utility commands
	// (get-with-metadata, search, exists)

	// Add all legacy command groups to the main KV command
	kvCmd.AddCommand(nsCmd)
	kvCmd.AddCommand(valuesCmd)
}
