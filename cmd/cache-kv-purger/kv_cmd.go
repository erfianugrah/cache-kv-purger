package main

import (
	"cache-kv-purger/internal/cmdutil"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// Common flag variables for KV commands
var kvFlagsVars struct {
	accountID     string
	namespaceID   string
	key           string
	title         string
	file          string
	verbose       bool
	includeValues bool
}

// addMissingValueValidation adds validation for flags that require values
func addMissingValueValidation(cmd *cobra.Command) {
	// Store the original RunE and Run functions
	originalRunE := cmd.RunE
	originalRun := cmd.Run

	// Create a new RunE function that checks for missing values
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// Skip help command
		if cmd.Name() == "help" {
			return nil
		}

		// Check flags for missing values
		var missingValues []string

		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			// Only check flags that are set but have empty values
			if flag.Changed && flag.Value.Type() == "string" && flag.Value.String() == "" {
				missingValues = append(missingValues, flag.Name)
			}
		})

		// Report missing values
		if len(missingValues) > 0 {
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

	// Add direct flags to kvCmd for common use cases
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.namespaceID, "namespace-id", "", "ID of the namespace")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.title, "title", "", "Title of the namespace")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.file, "file", "", "Output or input file path")
	kvCmd.PersistentFlags().StringVar(&kvFlagsVars.key, "key", "", "Key name")
	
	// Add all KV subcommands directly in kv_cmd.go using the builder pattern
	kvCmd.AddCommand(cmdutil.NewKVListCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVGetCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVPutCommand().Build())
	
	// Instead of adding the original delete command, add the fixed version
	// kvCmd.AddCommand(cmdutil.NewKVDeleteCommand().Build())
	addFixedDeleteCommand(kvCmd)
	
	kvCmd.AddCommand(cmdutil.NewKVCreateCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVRenameCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVConfigCommand().Build())
	
	// Demo commands removed for production build
}
