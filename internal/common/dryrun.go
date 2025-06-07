package common

import (
	"fmt"
)

// DryRunOptions contains options for handling dry run behavior
type DryRunOptions struct {
	// Enabled indicates whether dry run mode is active
	Enabled bool

	// Verbose enables detailed output
	Verbose bool

	// ItemType is the type of items being processed (e.g., "files", "keys", "hosts")
	ItemType string

	// ActionVerb is the action being performed (e.g., "purge", "delete", "update")
	ActionVerb string

	// BatchSize is the size of batches when processing in batches
	BatchSize int
}

// HandleDryRun processes dry run mode for a slice of items
// Returns true if execution should continue (false if in dry run mode)
func HandleDryRun(opts DryRunOptions, items []string, batches [][]string) bool {
	if !opts.Enabled {
		return true
	}

	totalCount := len(items)
	batchCount := len(batches)

	// Display dry run header
	fmt.Printf("DRY RUN: Would %s %d %s", opts.ActionVerb, totalCount, opts.ItemType)

	// Add batch information if relevant
	if batchCount > 1 {
		fmt.Printf(" in %d batches (batch size: %d)", batchCount, opts.BatchSize)
	}
	fmt.Println()

	// In verbose mode, display the items
	if opts.Verbose {
		displayItems(items, batches, opts.Verbose)
	}

	// Return false to indicate processing should stop (dry run only)
	return false
}

// HandleDryRunWithSample processes dry run mode with a sample preview
// Returns true if execution should continue (false if in dry run mode)
func HandleDryRunWithSample(opts DryRunOptions, items []string, batches [][]string) bool {
	if !opts.Enabled {
		return true
	}

	totalCount := len(items)
	batchCount := len(batches)

	// Display dry run header
	fmt.Printf("DRY RUN: Would %s %d %s", opts.ActionVerb, totalCount, opts.ItemType)

	// Add batch information if relevant
	if batchCount > 1 {
		fmt.Printf(" in %d batches (batch size: %d)", batchCount, opts.BatchSize)
	}
	fmt.Println()

	// Always show a sample of items in this mode
	displaySampleItems(items, batches, opts.Verbose)

	// Return false to indicate processing should stop (dry run only)
	return false
}

// displayItems shows all items, optionally grouped by batch
func displayItems(items []string, batches [][]string, verbose bool) {
	if len(batches) <= 1 {
		// Single batch or no batching, just display the items
		for i, item := range items {
			fmt.Printf("  %d. %s\n", i+1, item)
		}
		return
	}

	// Display items by batch
	for i, batch := range batches {
		fmt.Printf("Batch %d: %d items\n", i+1, len(batch))
		if verbose {
			for j, item := range batch {
				fmt.Printf("  %d. %s\n", j+1, item)
			}
		} else {
			// Show summary only for non-verbose mode
			fmt.Printf("  First item: %s\n", batch[0])
			if len(batch) > 1 {
				fmt.Printf("  Last item: %s\n", batch[len(batch)-1])
			}
		}
	}
}

// displaySampleItems shows a representative sample of items
func displaySampleItems(items []string, batches [][]string, verbose bool) {
	if len(batches) <= 1 {
		// No batches or single batch
		if verbose {
			// Show all items in verbose mode
			for i, item := range items {
				fmt.Printf("  %d. %s\n", i+1, item)
			}
		} else {
			// Show sample otherwise
			displayCount := 5
			if len(items) < displayCount {
				displayCount = len(items)
			}

			for i := 0; i < displayCount; i++ {
				fmt.Printf("  %d. %s\n", i+1, items[i])
			}

			if len(items) > displayCount {
				fmt.Printf("  ... and %d more items\n", len(items)-displayCount)
			}
		}
		return
	}

	// With batches, show batch summary and samples from first and last batch
	fmt.Printf("Processing in %d batches:\n", len(batches))

	// Show first batch
	fmt.Printf("First batch: %d items\n", len(batches[0]))
	displayCount := 3
	if len(batches[0]) < displayCount {
		displayCount = len(batches[0])
	}
	for i := 0; i < displayCount; i++ {
		fmt.Printf("  %d. %s\n", i+1, batches[0][i])
	}
	if len(batches[0]) > displayCount {
		fmt.Printf("  ... and %d more items in this batch\n", len(batches[0])-displayCount)
	}

	// If there are multiple batches, show the last batch
	if len(batches) > 1 {
		lastBatch := batches[len(batches)-1]
		fmt.Printf("Last batch: %d items\n", len(lastBatch))
		if len(lastBatch) > 0 {
			displayCount = 2
			if len(lastBatch) < displayCount {
				displayCount = len(lastBatch)
			}
			for i := 0; i < displayCount; i++ {
				fmt.Printf("  %d. %s\n", i+1, lastBatch[i])
			}
			if len(lastBatch) > displayCount {
				fmt.Printf("  ... and %d more items in this batch\n", len(lastBatch)-displayCount)
			}
		}
	}

	// Show batch summary
	fmt.Printf("DRY RUN SUMMARY: Would process %d total items across %d batches\n", len(items), len(batches))
}

// ConfirmBatchOperation asks the user to confirm a batch operation
// Returns true if the user confirms, or if force is true
func ConfirmBatchOperation(itemCount int, itemType string, actionVerb string, force bool) bool {
	if force {
		return true
	}

	fmt.Printf("\nYou are about to %s %d %s.\n", actionVerb, itemCount, itemType)
	fmt.Print("This operation cannot be undone. Are you sure? [y/N]: ")

	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil || (confirm != "y" && confirm != "Y") {
		fmt.Println("Operation cancelled.")
		return false
	}

	return true
}
