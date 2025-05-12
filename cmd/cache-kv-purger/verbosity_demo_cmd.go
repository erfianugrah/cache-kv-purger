package main

import (
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/common"
	"fmt"
	"github.com/spf13/cobra"
)

// createVerbosityDemoCmd creates a command to demonstrate the new verbosity feature
func createVerbosityDemoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verbosity-demo",
		Short: "Demonstrate the new verbosity features",
		Long:  `Shows how the new verbosity handler works at different levels.`,
		Example: `  # Run with normal verbosity
  cache-kv-purger verbosity-demo 

  # Run with verbose level
  cache-kv-purger verbosity-demo --verbose
  cache-kv-purger verbosity-demo --verbosity verbose

  # Run with debug level
  cache-kv-purger verbosity-demo --verbosity debug

  # Run with quiet level
  cache-kv-purger verbosity-demo --verbosity quiet`,
		RunE: cmdutil.WithVerbosity(func(cmd *cobra.Command, args []string, verbosity *common.Verbosity) error {
			// Print messages at different verbosity levels
			verbosity.Println("This is a normal message visible at normal level")
			verbosity.Verboseln("This is a verbose message only visible at verbose or debug level")
			verbosity.Debugln("This is a debug message only visible at debug level")

			// Show some conditional formatting
			verbosity.Println("Current verbosity level: %s", verbosity.Level.String())
			
			// Demonstrate boolean checks
			if verbosity.IsVerbose() {
				fmt.Println("Verbosity check passed: verbose mode is enabled")
			}
			
			if verbosity.IsDebug() {
				fmt.Println("Debug check passed: debug mode is enabled")
			}
			
			if verbosity.IsQuiet() {
				fmt.Println("This message won't be seen in quiet mode because it's checked separately")
			} else {
				fmt.Println("Not in quiet mode")
			}
			
			// Demonstrate progress updates
			for i := 0; i < 5; i++ {
				// In verbose mode, this will show as separate lines
				verbosity.Verboseln("Processing item %d of 5", i+1)
				
				// In normal mode, this will update in-place
				verbosity.ProgressUpdate("Processing item %d of 5...", i+1)
			}
			
			// End the progress line
			verbosity.ProgressFinish()
			
			verbosity.Println("Demo completed")
			
			return nil
		}),
	}

	// Note: We don't need to add a --verbose flag anymore since the middleware
	// already handles the global --verbosity flag from the root command

	return cmd
}