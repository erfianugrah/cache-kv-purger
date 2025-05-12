package main

import (
	"cache-kv-purger/internal/cmdutil"
	"cache-kv-purger/internal/common"
	"github.com/spf13/cobra"
	"time"
)

// createFormatDemoCmd creates a command to demonstrate the output formatter
func createFormatDemoCmd() *cobra.Command {
	var outputFormat string
	var withTimestamps bool
	
	cmd := &cobra.Command{
		Use:   "format-demo",
		Short: "Demonstrate the output formatter",
		Long:  `Shows how the output formatter works with different formats and options.`,
		Example: `  # Run with default text format
  cache-kv-purger format-demo

  # Run with JSON format
  cache-kv-purger format-demo --format json

  # Run with timestamps
  cache-kv-purger format-demo --timestamps
  
  # Run with verbosity levels
  cache-kv-purger format-demo --verbosity verbose`,
		RunE: cmdutil.WithVerbosity(func(cmd *cobra.Command, args []string, verbosity *common.Verbosity) error {
			// Create formatter based on options
			formatter := common.NewOutputFormatter().
				WithVerbosity(verbosity)
			
			// Set format if specified
			switch outputFormat {
			case "json":
				formatter.WithFormat(common.OutputFormatJSON)
			case "table":
				formatter.WithFormat(common.OutputFormatTable)
			default:
				formatter.WithFormat(common.OutputFormatText)
			}
			
			// Add timestamps if requested
			if withTimestamps {
				formatter.WithTimestamps(time.RFC3339)
			}
			
			// Demo 1: Header and subheader
			formatter.FormatHeader("Output Formatter Demo")
			
			// Demo 2: Key-value table
			formatter.FormatSubHeader("Key-Value Table Demo")
			kvData := map[string]string{
				"Name":        "Example Project",
				"Version":     "1.0.0",
				"Description": "Demonstrates the output formatter",
				"Author":      "John Doe",
				"License":     "MIT",
			}
			formatter.FormatKeyValueTable(kvData)
			
			// Demo 3: Table with rows and columns
			formatter.FormatSubHeader("Table Demo")
			headers := []string{"ID", "Name", "Type", "Size", "Created"}
			rows := [][]string{
				{"1", "file1.txt", "text", "10 KB", "2023-01-01"},
				{"2", "image.png", "image", "500 KB", "2023-01-02"},
				{"3", "document.pdf", "document", "1.2 MB", "2023-01-03"},
				{"4", "script.js", "code", "5 KB", "2023-01-04"},
				{"5", "styles.css", "code", "8 KB", "2023-01-05"},
			}
			formatter.FormatTable(headers, rows)
			
			// Demo 4: Success message
			formatter.FormatSubHeader("Success Message Demo")
			details := map[string]string{
				"Duration": "2.5s",
				"Status":   "Completed",
				"ID":       "op-12345",
			}
			formatter.FormatSuccess("processed", 5, "files", details)
			
			// Demo 5: List formatting
			formatter.FormatSubHeader("List Demo")
			items := []string{
				"Item 1: First example item",
				"Item 2: Second example item",
				"Item 3: Third example item with longer text to demonstrate wrapping",
				"Item 4: Fourth example item",
				"Item.5: Fifth example item",
				"Item 6: Sixth example item",
				"Item 7: Seventh example item",
				"Item 8: Eighth example item",
			}
			formatter.FormatList(items, "Sample Items")
			
			// Demo 6: Progress reporting
			formatter.FormatSubHeader("Progress Demo")
			total := 10
			
			formatter.FormatProgressStart("Processing", total, "items")
			for i := 1; i <= total; i++ {
				// Simulate work
				time.Sleep(200 * time.Millisecond)
				
				// Update progress
				formatter.FormatProgressUpdate(i, total)
				
				// Verbose output gets more details
				if verbosity.IsVerbose() {
					verbosity.Verboseln("Processed item %d of %d", i, total)
				}
			}
			formatter.FormatProgressComplete()
			
			return nil
		}),
	}
	
	// Add command-specific flags
	cmd.Flags().StringVar(&outputFormat, "format", "text", "Output format (text, json, table)")
	cmd.Flags().BoolVar(&withTimestamps, "timestamps", false, "Include timestamps in output")
	
	return cmd
}