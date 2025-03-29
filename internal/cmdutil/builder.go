package cmdutil

import (
	"github.com/spf13/cobra"
)

// CommandBuilder provides a fluent interface for building commands
type CommandBuilder struct {
	cmd *cobra.Command
}

// NewCommand creates a new command builder with the given use, short, and long descriptions
func NewCommand(use, short, long string) *CommandBuilder {
	return &CommandBuilder{
		cmd: &cobra.Command{
			Use:   use,
			Short: short,
			Long:  long,
		},
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