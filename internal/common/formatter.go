package common

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// OutputFormat defines the format for command output
type OutputFormat string

const (
	// OutputFormatText is the standard human-readable text format
	OutputFormatText OutputFormat = "text"

	// OutputFormatJSON is the JSON format
	OutputFormatJSON OutputFormat = "json"

	// OutputFormatTable is the tabular format
	OutputFormatTable OutputFormat = "table"
)

// OutputFormatter provides standardized output formatting
type OutputFormatter struct {
	// Format defines the output format (text, json, table)
	Format OutputFormat

	// Writer is where the output will be written (defaults to os.Stdout)
	Writer io.Writer

	// Verbosity controls the level of detail in the output
	Verbosity *Verbosity

	// TimestampFormat defines the format for timestamps (empty = no timestamps)
	TimestampFormat string
}

// NewOutputFormatter creates a new formatter with default settings
func NewOutputFormatter() *OutputFormatter {
	return &OutputFormatter{
		Format:          OutputFormatText,
		Writer:          os.Stdout,
		TimestampFormat: "",
	}
}

// WithFormat sets the output format
func (f *OutputFormatter) WithFormat(format OutputFormat) *OutputFormatter {
	f.Format = format
	return f
}

// WithWriter sets the output writer
func (f *OutputFormatter) WithWriter(writer io.Writer) *OutputFormatter {
	if writer != nil {
		f.Writer = writer
	}
	return f
}

// WithVerbosity sets the verbosity level
func (f *OutputFormatter) WithVerbosity(verbosity *Verbosity) *OutputFormatter {
	f.Verbosity = verbosity
	return f
}

// WithTimestamps enables output timestamps in the specified format
func (f *OutputFormatter) WithTimestamps(format string) *OutputFormatter {
	f.TimestampFormat = format
	return f
}

// FormatHeader prints a section header
func (f *OutputFormatter) FormatHeader(title string) {
	width := len(title) + 10
	line := strings.Repeat("=", width)

	f.writeLine(line)
	f.writeLine(fmt.Sprintf("    %s    ", title))
	f.writeLine(line)
}

// FormatSubHeader prints a subsection header
func (f *OutputFormatter) FormatSubHeader(title string) {
	width := len(title) + 6
	line := strings.Repeat("-", width)

	f.writeLine("")
	f.writeLine(fmt.Sprintf("  %s  ", title))
	f.writeLine(line)
}

// FormatResult formats an operation result
func (f *OutputFormatter) FormatResult(operation string, result string, details map[string]string) {
	// For JSON output, format as structured data
	if f.Format == OutputFormatJSON {
		data := map[string]interface{}{
			"operation": operation,
			"result":    result,
			"details":   details,
		}

		jsonData, err := ToJSON(data)
		if err != nil {
			fmt.Fprintf(f.Writer, "Error formatting JSON: %v\n", err)
			return
		}

		fmt.Fprintln(f.Writer, string(jsonData))
		return
	}

	// For text and table output, use the key-value table
	resultData := make(map[string]string)
	resultData["Operation"] = operation
	resultData["Result"] = result

	// Add any additional details
	for k, v := range details {
		resultData[k] = v
	}

	f.FormatKeyValueTable(resultData)
}

// FormatSuccess formats a success message
func (f *OutputFormatter) FormatSuccess(operation string, items int, itemType string, details map[string]string) {
	successMsg := fmt.Sprintf("Successfully %s %d %s", operation, items, itemType)
	if items == 1 {
		// Handle singular case
		successMsg = fmt.Sprintf("Successfully %s %d %s", operation, items, strings.TrimSuffix(itemType, "s"))
	}

	f.FormatResult(operation, successMsg, details)
}

// FormatKeyValueTable formats data as a 2-column key-value table
func (f *OutputFormatter) FormatKeyValueTable(data map[string]string) {
	// For JSON output, format as structured data
	if f.Format == OutputFormatJSON {
		jsonData, err := ToJSON(data)
		if err != nil {
			fmt.Fprintf(f.Writer, "Error formatting JSON: %v\n", err)
			return
		}

		fmt.Fprintln(f.Writer, string(jsonData))
		return
	}

	// For text and table output, use tabwriter for alignment
	w := tabwriter.NewWriter(f.Writer, 0, 0, 3, ' ', 0)

	// Print separator
	fmt.Fprintln(w, strings.Repeat("-", 50))

	// Print key-value pairs
	for key, value := range data {
		fmt.Fprintf(w, "%s\t%s\n", key, value)
	}

	// Print separator
	fmt.Fprintln(w, strings.Repeat("-", 50))

	w.Flush()
}

// FormatTable formats tabular data with headers
func (f *OutputFormatter) FormatTable(headers []string, rows [][]string) {
	// For JSON output, format as structured data
	if f.Format == OutputFormatJSON {
		// Create a slice of maps, where each map represents a row
		jsonRows := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			rowMap := make(map[string]string)
			for i, cell := range row {
				if i < len(headers) {
					rowMap[headers[i]] = cell
				}
			}
			jsonRows = append(jsonRows, rowMap)
		}

		jsonData, err := ToJSON(jsonRows)
		if err != nil {
			fmt.Fprintf(f.Writer, "Error formatting JSON: %v\n", err)
			return
		}

		fmt.Fprintln(f.Writer, string(jsonData))
		return
	}

	// For text and table output, use tabwriter for alignment
	w := tabwriter.NewWriter(f.Writer, 0, 0, 3, ' ', 0)

	// Write headers
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Repeat("-", 50))

	// Write rows
	for _, row := range rows {
		// Ensure we don't go out of bounds if a row has fewer cells than headers
		cells := make([]string, len(headers))
		for i := range cells {
			if i < len(row) {
				cells[i] = row[i]
			} else {
				cells[i] = ""
			}
		}
		fmt.Fprintln(w, strings.Join(cells, "\t"))
	}

	w.Flush()
}

// FormatProgressStart starts a progress reporting process
func (f *OutputFormatter) FormatProgressStart(operation string, total int, itemType string) {
	// Skip for JSON output
	if f.Format == OutputFormatJSON {
		return
	}

	// Skip for quiet mode
	if f.Verbosity != nil && f.Verbosity.IsQuiet() {
		return
	}

	fmt.Fprintf(f.Writer, "%s %d %s... ", operation, total, itemType)
}

// FormatProgressUpdate updates a progress report
func (f *OutputFormatter) FormatProgressUpdate(completed, total int) {
	// Skip for JSON output
	if f.Format == OutputFormatJSON {
		return
	}

	// Skip for quiet mode
	if f.Verbosity != nil && f.Verbosity.IsQuiet() {
		return
	}

	// Skip for verbose mode (which gets more detailed updates)
	if f.Verbosity != nil && f.Verbosity.IsVerbose() {
		return
	}

	percent := float64(completed) / float64(total) * 100
	fmt.Fprintf(f.Writer, "\rProgress: %d/%d (%.1f%%)... ", completed, total, percent)
}

// FormatProgressComplete completes a progress report
func (f *OutputFormatter) FormatProgressComplete() {
	// Skip for JSON output
	if f.Format == OutputFormatJSON {
		return
	}

	// Skip for quiet mode
	if f.Verbosity != nil && f.Verbosity.IsQuiet() {
		return
	}

	// Skip for verbose mode (which gets more detailed updates)
	if f.Verbosity != nil && f.Verbosity.IsVerbose() {
		return
	}

	fmt.Fprintln(f.Writer, "Done!")
}

// FormatList formats a simple list of items
func (f *OutputFormatter) FormatList(items []string, title string) {
	// For JSON output, format as an array
	if f.Format == OutputFormatJSON {
		data := map[string]interface{}{
			"title": title,
			"items": items,
			"count": len(items),
		}

		jsonData, err := ToJSON(data)
		if err != nil {
			fmt.Fprintf(f.Writer, "Error formatting JSON: %v\n", err)
			return
		}

		fmt.Fprintln(f.Writer, string(jsonData))
		return
	}

	// For text output, print as numbered list with title
	if title != "" {
		f.writeLine(fmt.Sprintf("%s (%d items):", title, len(items)))
	} else {
		f.writeLine(fmt.Sprintf("Items (%d):", len(items)))
	}

	// Handle empty list
	if len(items) == 0 {
		f.writeLine("  (no items)")
		return
	}

	// Determine how many items to display based on verbosity
	displayCount := len(items)
	if f.Verbosity != nil && !f.Verbosity.IsVerbose() && displayCount > 5 {
		displayCount = 5
	}

	// Print items with numbers
	for i := 0; i < displayCount; i++ {
		f.writeLine(fmt.Sprintf("  %d. %s", i+1, items[i]))
	}

	// Show count of remaining items
	if displayCount < len(items) {
		f.writeLine(fmt.Sprintf("  ... and %d more items", len(items)-displayCount))
	}
}

// writeLine writes a line of text with an optional timestamp
func (f *OutputFormatter) writeLine(line string) {
	// Add timestamp if configured
	if f.TimestampFormat != "" {
		timestamp := time.Now().Format(f.TimestampFormat)
		fmt.Fprintf(f.Writer, "[%s] %s\n", timestamp, line)
	} else {
		fmt.Fprintln(f.Writer, line)
	}
}
