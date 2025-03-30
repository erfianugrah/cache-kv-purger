package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandBuilder provides a fluent interface for building commands
type CommandBuilder struct {
	cmd *cobra.Command
}

// NewCommand creates a new command builder with the given use, short, and long descriptions
func NewCommand(use, short, long string) *CommandBuilder {
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
	}
	
	// Add a pre-run hook that will:
	// 1. Check if --help is present and prioritize it
	// 2. Validate that flags have values if they're specified
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// If this is a help request, skip validation
		if helpEnabled, _ := cmd.Flags().GetBool("help"); helpEnabled {
			// We're handling this specially because --help might be combined with invalid flags
			cmd.Help()
			return fmt.Errorf("help requested")
		}
		
		// Validate that all flags provided have values
		var missingValues []string
		cmd.Flags().Visit(func(f *pflag.Flag) {
			// Check if the flag was provided but with an empty value
			// This happens when a flag that requires a value is followed by another flag
			// e.g., --flag1 --flag2
			if f.Value.Type() == "string" && f.Value.String() == "" {
				// Check if it starts with a dash, which indicates it might be another flag
				// We make an exception for explicitly empty strings like --flag=""
				if cmd.ArgsLenAtDash() > 0 || strings.HasPrefix(f.Value.String(), "-") {
					missingValues = append(missingValues, f.Name)
				}
			}
		})
		
		if len(missingValues) > 0 {
			return fmt.Errorf("the following flags require values: %s", strings.Join(missingValues, ", "))
		}
		
		// If parent has PreRunE, run it
		if cmd.Parent() != nil && cmd.Parent().PersistentPreRunE != nil {
			// We can't compare functions directly, but we can just run the parent's
			return cmd.Parent().PersistentPreRunE(cmd, args)
		}
		
		return nil
	}
	
	return &CommandBuilder{
		cmd: cmd,
	}
}

// WithRunE sets the RunE function for the command
func (b *CommandBuilder) WithRunE(runE func(*cobra.Command, []string) error) *CommandBuilder {
	b.cmd.RunE = runE
	return b
}

// WithRun sets the Run function for the command
func (b *CommandBuilder) WithRun(run func(*cobra.Command, []string)) *CommandBuilder {
	b.cmd.Run = run
	return b
}

// WithExample sets the example for the command
func (b *CommandBuilder) WithExample(example string) *CommandBuilder {
	b.cmd.Example = example
	return b
}

// WithAliases sets the aliases for the command
func (b *CommandBuilder) WithAliases(aliases ...string) *CommandBuilder {
	b.cmd.Aliases = aliases
	return b
}

// WithStringFlag adds a string flag to the command
func (b *CommandBuilder) WithStringFlag(name, value, usage string, variable *string) *CommandBuilder {
	b.cmd.Flags().StringVar(variable, name, value, usage)
	return b
}

// WithStringSliceFlag adds a string slice flag to the command
func (b *CommandBuilder) WithStringSliceFlag(name string, value []string, usage string, variable *[]string) *CommandBuilder {
	b.cmd.Flags().StringSliceVar(variable, name, value, usage)
	return b
}

// WithBoolFlag adds a boolean flag to the command
func (b *CommandBuilder) WithBoolFlag(name string, value bool, usage string, variable *bool) *CommandBuilder {
	b.cmd.Flags().BoolVar(variable, name, value, usage)
	return b
}

// WithIntFlag adds an integer flag to the command
func (b *CommandBuilder) WithIntFlag(name string, value int, usage string, variable *int) *CommandBuilder {
	b.cmd.Flags().IntVar(variable, name, value, usage)
	return b
}

// WithInt64Flag adds an int64 flag to the command
func (b *CommandBuilder) WithInt64Flag(name string, value int64, usage string, variable *int64) *CommandBuilder {
	b.cmd.Flags().Int64Var(variable, name, value, usage)
	return b
}

// WithRequiredFlag marks a flag as required
func (b *CommandBuilder) WithRequiredFlag(name string) *CommandBuilder {
	_ = b.cmd.MarkFlagRequired(name)
	return b
}

// WithSubCommand adds a subcommand to the command
func (b *CommandBuilder) WithSubCommand(subCmd *cobra.Command) *CommandBuilder {
	b.cmd.AddCommand(subCmd)
	return b
}

// WithPersistentFlags adds persistent flags to the command
func (b *CommandBuilder) WithPersistentStringFlag(name, value, usage string, variable *string) *CommandBuilder {
	b.cmd.PersistentFlags().StringVar(variable, name, value, usage)
	return b
}

// Build returns the built cobra.Command
func (b *CommandBuilder) Build() *cobra.Command {
	return b.cmd
}

// AddFlagValidation adds flag validation to an existing cobra.Command
// This can be used for commands not created with CommandBuilder
func AddFlagValidation(cmd *cobra.Command) {
	// Store the original PreRun/PreRunE if they exist
	originalPreRun := cmd.PreRun
	originalPreRunE := cmd.PreRunE
	
	// Add our validation logic
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// If this is a help request, skip validation
		if helpFlag := cmd.Flags().Lookup("help"); helpFlag != nil && helpFlag.Changed {
			cmd.Help()
			return fmt.Errorf("help requested")
		}
		
		// Validate that all flags provided have values
		var missingValues []string
		cmd.Flags().Visit(func(f *pflag.Flag) {
			// Check if the flag was provided but with an empty value
			if f.Value.Type() == "string" && f.Value.String() == "" {
				// Check if it starts with a dash, which indicates another flag
				if strings.HasPrefix(f.Value.String(), "-") {
					missingValues = append(missingValues, f.Name)
				}
			}
		})
		
		if len(missingValues) > 0 {
			return fmt.Errorf("the following flags require values: %s", strings.Join(missingValues, ", "))
		}
		
		// Run the original PreRunE if it exists
		if originalPreRunE != nil {
			return originalPreRunE(cmd, args)
		}
		
		// Run the original PreRun if it exists
		if originalPreRun != nil {
			originalPreRun(cmd, args)
		}
		
		return nil
	}
	
	// Apply the same validation to all subcommands
	for _, subCmd := range cmd.Commands() {
		AddFlagValidation(subCmd)
	}
}