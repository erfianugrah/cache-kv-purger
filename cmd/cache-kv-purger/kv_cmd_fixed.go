package main

import (
	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/kv"
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// addFixedDeleteCommand adds the fixed kv delete command to a parent command
func addFixedDeleteCommand(parentCmd *cobra.Command) {
	// Get the original delete command
	originalDeleteCmd := cmdutil.NewKVDeleteCommand().Build()

	// Keep the original command but rename it
	originalDeleteCmd.Use = "delete-legacy"
	originalDeleteCmd.Hidden = true // Hide from help output
	parentCmd.AddCommand(originalDeleteCmd)

	// Create a new fixed delete command with the same flags and behavior
	fixedDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete keys or namespaces (fixed version with concurrency fixes)",
		Long:  originalDeleteCmd.Long,
		RunE: func(cmd *cobra.Command, args []string) error {
			// This implementation uses a different approach to inject our fixed code
			// We'll intercept the client creation and wrap it with a patched version

			// Get current flags values
			accountID, _ := cmd.Flags().GetString("account-id")
			namespaceID, _ := cmd.Flags().GetString("namespace-id")
			namespace, _ := cmd.Flags().GetString("namespace")
			// key not used in this implementation
			tagField, _ := cmd.Flags().GetString("tag-field")
			tagValue, _ := cmd.Flags().GetString("tag-value")
			bulk, _ := cmd.Flags().GetBool("bulk")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			batchSize, _ := cmd.Flags().GetInt("batch-size")
			concurrency, _ := cmd.Flags().GetInt("concurrency")
			verbosity, _ := cmd.Flags().GetString("verbosity")

			// Check if this is a tag-based deletion where we need our fix
			isTagBased := bulk && tagField != ""

			if isTagBased {
				// Get the client using the WithConfigAndClient middleware
				// We can't access it directly, so we'll call our own implementation

				// Create client directly
				client, err := api.NewClient()
				if err != nil {
					return err
				}

				// Determine verbosity
				debug := verbosity == "debug"
				// verbose not used but would be:
				// verbose := verbosity == "verbose" || debug

				// Call our fixed implementation directly
				fmt.Printf("[INFO] Using fixed implementation for tag-based deletion\n")

				if accountID == "" {
					return fmt.Errorf("account-id is required")
				}

				// Resolve namespace ID if needed
				if namespaceID == "" && namespace != "" {
					service := kv.NewKVService(client)
					// Create a new background context since we removed the context import
					ctx := context.Background()
					nsID, err := service.ResolveNamespaceID(ctx, accountID, namespace)
					if err != nil {
						return fmt.Errorf("failed to resolve namespace: %w", err)
					}
					namespaceID = nsID
				}

				if namespaceID == "" {
					return fmt.Errorf("namespace-id or namespace is required")
				}

				// Set up a progress callback based on verbosity
				progressCallback := func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int) {
					if debug {
						fetchPercent := 0.0
						procPercent := 0.0
						if total > 0 {
							fetchPercent = float64(keysFetched) / float64(total) * 100
							procPercent = float64(keysProcessed) / float64(total) * 100
						}
						fmt.Printf("[DEBUG] Progress: %d/%d keys fetched (%.1f%%), %d/%d processed (%.1f%%), %d matched, %d deleted\n",
							keysFetched, total, fetchPercent, keysProcessed, total, procPercent, keysMatched, keysDeleted)
					}
				}

				// Call our fixed implementation
				count, err := kv.PurgeByMetadataOnlyFixed(client, accountID, namespaceID, tagField, tagValue,
					batchSize, concurrency, dryRun, progressCallback)

				if err != nil {
					return fmt.Errorf("bulk delete operation failed: %w", err)
				}

				if dryRun {
					fmt.Printf("DRY RUN: Would delete %d keys\n", count)
				} else {
					fmt.Printf("Successfully deleted %d keys\n", count)
				}

				return nil
			}

			// For all other operations, use the original implementation
			return originalDeleteCmd.RunE(cmd, args)
		},
	}

	// Copy all flags from the original command to the fixed command
	originalDeleteCmd.Flags().VisitAll(func(f *pflag.Flag) {
		fixedDeleteCmd.Flags().AddFlag(f)
	})

	// Add the fixed command to the parent
	parentCmd.AddCommand(fixedDeleteCmd)
}
