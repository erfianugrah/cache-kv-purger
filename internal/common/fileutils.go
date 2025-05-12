package common

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileType represents the type of file to read
type FileType string

const (
	// FileTypeText indicates a text file with one item per line
	FileTypeText FileType = "text"
	// FileTypeCSV indicates a CSV file
	FileTypeCSV FileType = "csv"
	// FileTypeJSON indicates a JSON file containing an array of strings
	FileTypeJSON FileType = "json_array_of_strings"
	// FileTypeJSONObjects indicates a JSON file containing an array of objects
	FileTypeJSONObjects FileType = "json_array_of_objects_with_field"
)

// ReadItemsFromFileOptions contains options for reading items from a file
type ReadItemsFromFileOptions struct {
	// FieldName is the field name to extract from JSON objects
	FieldName string
	// ColumnIndex is the column index to use for CSV files (0-based)
	ColumnIndex int
	// SkipHeader indicates whether to skip the first line in CSV files
	SkipHeader bool
	// CommentPrefix is the prefix that indicates a comment line (e.g. "#")
	CommentPrefix string
	// TrimSpace indicates whether to trim whitespace from items
	TrimSpace bool
}

// DefaultReadItemsOptions returns default options for ReadItemsFromFile
func DefaultReadItemsOptions() *ReadItemsFromFileOptions {
	return &ReadItemsFromFileOptions{
		ColumnIndex:   0,
		SkipHeader:    false,
		CommentPrefix: "#",
		TrimSpace:     true,
	}
}

// ReadItemsFromFile reads items from a file
// filePath is the path to the file
// fileTypeHint is a hint about the file type (will be auto-detected if empty)
// options are additional options for reading items
func ReadItemsFromFile(filePath string, fileTypeHint FileType, options *ReadItemsFromFileOptions) ([]string, error) {
	// Apply default options if not provided
	if options == nil {
		options = DefaultReadItemsOptions()
	}

	// Make sure the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filePath)
	}

	// Determine file type if not provided
	fileType := fileTypeHint
	if fileType == "" {
		// Auto-detect based on extension
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".json":
			fileType = FileTypeJSON
		case ".csv":
			fileType = FileTypeCSV
		default:
			fileType = FileTypeText
		}
	}

	var items []string
	var err error

	// Process based on file type
	switch fileType {
	case FileTypeJSON, FileTypeJSONObjects:
		items, err = readJSONFile(filePath, fileType, options)
	case FileTypeCSV:
		items, err = readCSVFile(filePath, options)
	default:
		items, err = readTextFile(filePath, options)
	}

	if err != nil {
		return nil, err
	}

	// Remove any empty items
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" {
			result = append(result, item)
		}
	}

	return result, nil
}

// readJSONFile reads items from a JSON file
func readJSONFile(filePath string, fileType FileType, options *ReadItemsFromFileOptions) ([]string, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Handle based on the expected JSON format
	if fileType == FileTypeJSON {
		// Parse as an array of strings
		var items []string
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}

		// Process each item
		if options.TrimSpace {
			for i, item := range items {
				items[i] = strings.TrimSpace(item)
			}
		}

		return items, nil
	} else if fileType == FileTypeJSONObjects {
		// Parse as an array of objects
		if options.FieldName == "" {
			return nil, fmt.Errorf("field name is required for JSON objects")
		}

		var objects []map[string]interface{}
		if err := json.Unmarshal(data, &objects); err != nil {
			return nil, fmt.Errorf("failed to parse JSON objects: %w", err)
		}

		// Extract the field from each object
		items := make([]string, 0, len(objects))
		for _, obj := range objects {
			if val, ok := obj[options.FieldName]; ok {
				// Convert to string
				var strVal string
				switch v := val.(type) {
				case string:
					strVal = v
				case float64:
					strVal = fmt.Sprintf("%g", v)
				case bool:
					strVal = fmt.Sprintf("%t", v)
				default:
					// Skip items that can't be converted to strings
					continue
				}

				if options.TrimSpace {
					strVal = strings.TrimSpace(strVal)
				}

				items = append(items, strVal)
			}
		}

		return items, nil
	}

	return nil, fmt.Errorf("unsupported JSON file type")
}

// readCSVFile reads items from a CSV file
func readCSVFile(filePath string, options *ReadItemsFromFileOptions) ([]string, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV file: %w", err)
	}

	// Process each record
	items := make([]string, 0, len(records))
	for i, record := range records {
		// Skip header if needed
		if i == 0 && options.SkipHeader {
			continue
		}

		// Make sure column index is valid
		if options.ColumnIndex >= len(record) {
			continue
		}

		// Get the value
		value := record[options.ColumnIndex]
		if options.TrimSpace {
			value = strings.TrimSpace(value)
		}

		// Skip comments and empty values
		if value == "" || (options.CommentPrefix != "" && strings.HasPrefix(value, options.CommentPrefix)) {
			continue
		}

		items = append(items, value)
	}

	return items, nil
}

// readTextFile reads items from a text file (one item per line)
func readTextFile(filePath string, options *ReadItemsFromFileOptions) ([]string, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Split into lines
	lines := strings.Split(string(data), "\n")
	items := make([]string, 0, len(lines))

	// Process each line
	for _, line := range lines {
		if options.TrimSpace {
			line = strings.TrimSpace(line)
		}

		// Skip comments and empty lines
		if line == "" || (options.CommentPrefix != "" && strings.HasPrefix(line, options.CommentPrefix)) {
			continue
		}

		items = append(items, line)
	}

	return items, nil
}