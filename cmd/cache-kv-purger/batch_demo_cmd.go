package main

import (
	"cache-kv-purger/internal/common"
	"fmt"
	"github.com/spf13/cobra"
	"strconv"
	"time"
)

// KeyValuePair is a simple key-value pair for demonstrating generic batch processing
type KeyValuePair struct {
	Key   string
	Value string
}

// createBatchDemoCmd creates a command to demonstrate the new batch processor
func createBatchDemoCmd() *cobra.Command {
	var batchSize int
	var concurrency int
	var itemCount int

	cmd := &cobra.Command{
		Use:   "batch-demo",
		Short: "Demonstrate the generic batch processor",
		Long:  `Shows how the new generic batch processor works with different data types.`,
		Example: `  # Run with default parameters
  cache-kv-purger batch-demo 

  # Run with custom parameters
  cache-kv-purger batch-demo --batch-size 10 --concurrency 5 --items 100`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create some sample data
			fmt.Println("Creating sample data...")
			
			// Example 1: Process strings
			stringItems := make([]string, itemCount)
			for i := 0; i < itemCount; i++ {
				stringItems[i] = fmt.Sprintf("item-%d", i)
			}
			
			// Example 2: Process structs
			structItems := make([]KeyValuePair, itemCount)
			for i := 0; i < itemCount; i++ {
				structItems[i] = KeyValuePair{
					Key:   fmt.Sprintf("key-%d", i),
					Value: fmt.Sprintf("value-%d", i),
				}
			}
			
			fmt.Printf("Generated %d sample items\n", itemCount)
			
			// Create progress callback
			progressCallback := func(completed, total, successful int) {
				fmt.Printf("Progress: %d/%d batches completed, %d items processed successfully\n", 
					completed, total, successful)
			}
			
			// Example 1: Process strings (original batch processor)
			fmt.Println("\n------ Original BatchProcessor (strings only) ------")
			fmt.Printf("Processing %d string items with batch size %d and concurrency %d\n", 
				len(stringItems), batchSize, concurrency)
			
			processor := common.NewBatchProcessor().
				WithBatchSize(batchSize).
				WithConcurrency(concurrency).
				WithProgressCallback(progressCallback)
			
			start := time.Now()
			results, errors := processor.ProcessStrings(stringItems, func(batch []string) ([]string, error) {
				// Simulate work
				time.Sleep(500 * time.Millisecond)
				processed := make([]string, 0, len(batch))
				
				for _, item := range batch {
					processed = append(processed, "processed-"+item)
				}
				
				return processed, nil
			})
			
			duration := time.Since(start)
			fmt.Printf("Original processor finished in %v: %d results, %d errors\n", 
				duration, len(results), len(errors))
			
			// Example 2: Process strings with generic processor
			fmt.Println("\n------ Generic BatchProcessor (strings) ------")
			fmt.Printf("Processing %d string items with batch size %d and concurrency %d\n",
				len(stringItems), batchSize, concurrency)

			genericStringProcessor := common.NewGenericBatchProcessor[string, string]().
				WithBatchSize(batchSize).
				WithConcurrency(concurrency).
				WithProgressCallback(progressCallback)
			
			start = time.Now()
			genResults, genErrors := genericStringProcessor.ProcessItems(stringItems, func(batch []string) ([]string, error) {
				// Simulate work
				time.Sleep(500 * time.Millisecond)
				processed := make([]string, 0, len(batch))
				
				for _, item := range batch {
					processed = append(processed, "generic-processed-"+item)
				}
				
				return processed, nil
			})
			
			duration = time.Since(start)
			fmt.Printf("Generic string processor finished in %v: %d results, %d errors\n", 
				duration, len(genResults), len(genErrors))
			
			// Example 3: Process structs with generic processor
			fmt.Println("\n------ Generic BatchProcessor (structs) ------")
			fmt.Printf("Processing %d struct items with batch size %d and concurrency %d\n",
				len(structItems), batchSize, concurrency)

			genericStructProcessor := common.NewGenericBatchProcessor[KeyValuePair, int]().
				WithBatchSize(batchSize).
				WithConcurrency(concurrency).
				WithProgressCallback(progressCallback)
			
			start = time.Now()
			numResults, structErrors := genericStructProcessor.ProcessItems(structItems, func(batch []KeyValuePair) ([]int, error) {
				// Simulate work
				time.Sleep(500 * time.Millisecond)
				processed := make([]int, 0, len(batch))
				
				for _, item := range batch {
					// Extract number from the key
					numStr := item.Key[4:] // Remove "key-" prefix
					num, _ := strconv.Atoi(numStr)
					processed = append(processed, num*2) // Double the number
				}
				
				return processed, nil
			})
			
			duration = time.Since(start)
			fmt.Printf("Generic struct processor finished in %v: %d results, %d errors\n", 
				duration, len(numResults), len(structErrors))
			
			return nil
		},
	}

	cmd.Flags().IntVar(&batchSize, "batch-size", 10, "Number of items to process in each batch")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Number of concurrent batch operations")
	cmd.Flags().IntVar(&itemCount, "items", 50, "Number of sample items to generate")

	return cmd
}