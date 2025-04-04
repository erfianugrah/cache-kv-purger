package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"
)

// OutputJSON marshals the given data to JSON and outputs it to stdout
func OutputJSON(data interface{}) error {
	jsonData, err := ToJSON(data)
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))
	return nil
}

// ToJSON marshals the given data to JSON
func ToJSON(data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// FormatTable formats tabular data with properly aligned columns
// headers: slice of column headers
// rows: slice of slices containing row data (each inner slice is a row)
func FormatTable(headers []string, rows [][]string) {
	// Create a new tabwriter that writes to stdout
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Write headers
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Calculate total width for the separator line
	totalWidth := 0
	for _, h := range headers {
		totalWidth += utf8.RuneCountInString(h) + 3 // Add padding
	}

	// Create separator line matching the width of the content
	fmt.Fprintln(w, strings.Repeat("-", totalWidth))

	// Write rows
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}

	// Flush the writer to ensure all content is written
	w.Flush()
}

// FormatKeyValueTable formats data as a 2-column key-value table
func FormatKeyValueTable(data map[string]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	// Find the longest key to determine separator width
	maxKeyLength := 0
	for key := range data {
		if len(key) > maxKeyLength {
			maxKeyLength = len(key)
		}
	}

	// Add some padding
	separatorWidth := maxKeyLength + 20

	// Print separator
	fmt.Fprintln(w, strings.Repeat("-", separatorWidth))

	// Print key-value pairs
	for key, value := range data {
		fmt.Fprintf(w, "%s\t%s\n", key, value)
	}

	// Print separator
	fmt.Fprintln(w, strings.Repeat("-", separatorWidth))

	w.Flush()
}
