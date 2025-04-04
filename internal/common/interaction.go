package common

import (
	"fmt"
)

// ConfirmAction prompts the user for confirmation of an action
func ConfirmAction(message string) bool {
	fmt.Printf("%s [y/N]: ", message)
	var confirm string
	if _, err := fmt.Scanln(&confirm); err != nil || (confirm != "y" && confirm != "Y") {
		return false
	}
	return true
}

// ConfirmDeletion is a specialized confirmation for deletion operations
func ConfirmDeletion(count int, itemType string) bool {
	return ConfirmAction(fmt.Sprintf("\nAre you sure you want to delete these %d %s? This cannot be undone.", count, itemType))
}

// DisplayItemSample shows a sample of items when there are too many to display all
func DisplayItemSample(items []interface{}, verbose bool, itemFormatter func(interface{}) string) {
	if len(items) == 0 {
		fmt.Println("No items found")
		return
	}

	fmt.Printf("Found %d items\n", len(items))

	if verbose {
		// Show all items in verbose mode
		for i, item := range items {
			fmt.Printf("  %d. %s\n", i+1, itemFormatter(item))
		}
	} else {
		// Show sample in normal mode
		displayCount := 5
		if len(items) < displayCount {
			displayCount = len(items)
		}

		for i := 0; i < displayCount; i++ {
			fmt.Printf("  %d. %s\n", i+1, itemFormatter(items[i]))
		}

		if len(items) > displayCount {
			fmt.Printf("  ... and %d more items\n", len(items)-displayCount)
		}
	}
}

// StringsDisplaySample shows a sample of string items
func StringsDisplaySample(items []string, verbose bool) {
	if len(items) == 0 {
		fmt.Println("No items found")
		return
	}

	fmt.Printf("Found %d items\n", len(items))

	if verbose {
		// Show all items in verbose mode
		for i, item := range items {
			fmt.Printf("  %d. %s\n", i+1, item)
		}
	} else {
		// Show sample in normal mode
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
}
