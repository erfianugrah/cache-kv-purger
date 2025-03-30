package cmdutil

import (
	"fmt"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"

	"github.com/spf13/cobra"
)

// NewKVCreateCommand creates a new command for creating namespaces
func NewKVCreateCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID     string
		title         string
		namespace     bool
		outputJSON    bool
	}

	// Create command
	return NewCommand("create", "Create a namespace", `
Create a KV namespace with the specified title.
`).WithExample(`  # Create a namespace
  cache-kv-purger kv create --account-id YOUR_ACCOUNT_ID --title "My Application Cache" --namespace
`).WithStringFlag(
		"account-id", "", "Cloudflare account ID", &opts.accountID,
	).WithStringFlag(
		"title", "", "Title for the new namespace (required)", &opts.title,
	).WithBoolFlag(
		"namespace", true, "Create a namespace (required)", &opts.namespace,
	).WithBoolFlag(
		"json", false, "Output as JSON", &opts.outputJSON,
	).WithRunE(
		WithConfigAndClient(func(cmd *cobra.Command, args []string, cfg *config.Config, client *api.Client) error {
			// Resolve account ID
			accountID, err := common.ValidateAccountID(cmd, cfg, opts.accountID)
			if err != nil {
				return err
			}

			// Validate inputs
			if !opts.namespace {
				return fmt.Errorf("--namespace flag is required")
			}

			if opts.title == "" {
				return fmt.Errorf("title is required")
			}

			// Create KV service
			service := kv.NewKVService(client)

			// Create the namespace
			ns, err := service.CreateNamespace(cmd.Context(), accountID, opts.title)
			if err != nil {
				return fmt.Errorf("failed to create namespace: %w", err)
			}

			// Display results
			if opts.outputJSON {
				return common.OutputJSON(ns)
			}

			fmt.Printf("Successfully created namespace:\n")
			fmt.Printf("ID: %s\n", ns.ID)
			fmt.Printf("Title: %s\n", ns.Title)
			return nil
		}),
	)
}

// NewKVRenameCommand creates a new command for renaming namespaces
func NewKVRenameCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID     string
		namespaceID   string
		namespace     string
		title         string
		outputJSON    bool
	}

	// Create command
	return NewCommand("rename", "Rename a namespace", `
Rename a KV namespace to the specified title.
`).WithExample(`  # Rename a namespace
  cache-kv-purger kv rename --namespace-id YOUR_NAMESPACE_ID --title "New Name"

  # Rename a namespace by name
  cache-kv-purger kv rename --namespace "Old Name" --title "New Name"
`).WithStringFlag(
		"account-id", "", "Cloudflare account ID", &opts.accountID,
	).WithStringFlag(
		"namespace-id", "", "ID of the namespace to rename", &opts.namespaceID,
	).WithStringFlag(
		"namespace", "", "Namespace name (alternative to namespace-id)", &opts.namespace,
	).WithStringFlag(
		"title", "", "New title for the namespace (required)", &opts.title,
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

			// Validate inputs
			if opts.namespaceID == "" {
				return fmt.Errorf("namespace-id or namespace is required")
			}

			if opts.title == "" {
				return fmt.Errorf("title is required")
			}

			// Rename the namespace
			ns, err := service.RenameNamespace(cmd.Context(), accountID, opts.namespaceID, opts.title)
			if err != nil {
				return fmt.Errorf("failed to rename namespace: %w", err)
			}

			// Display results
			if opts.outputJSON {
				return common.OutputJSON(ns)
			}

			fmt.Printf("Successfully renamed namespace:\n")
			fmt.Printf("ID: %s\n", ns.ID)
			fmt.Printf("New Title: %s\n", ns.Title)
			return nil
		}),
	)
}