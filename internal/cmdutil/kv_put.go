package cmdutil

import (
	"encoding/json"
	"fmt"
	"os"

	"cache-kv-purger/internal/api"
	"cache-kv-purger/internal/common"
	"cache-kv-purger/internal/config"
	"cache-kv-purger/internal/kv"

	"github.com/spf13/cobra"
)

// NewKVPutCommand creates a new put command for KV
func NewKVPutCommand() *CommandBuilder {
	// Define flag variables
	var opts struct {
		accountID     string
		namespaceID   string
		namespace     string
		key           string
		value         string
		inputFile     string
		metadataJSON  string
		expiration    int64
		expirationTTL int64
		bulk          bool
		bulkFile      string
		batchSize     int
		concurrency   int
	}

	// Create command
	return NewCommand("put", "Put values for keys in a namespace", `
Put values for one or more keys in a KV namespace.

When used with --key and --value or --file, puts a single key value.
When used with --bulk and --bulk-file, puts multiple key values from a file.
`).WithExample(`  # Put a single key
  cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --key mykey --value "My value"

  # Put a value from a file
  cache-kv-purger kv put --namespace "My Namespace" --key config.json --file ./config.json

  # Put with expiration
  cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --key temp-key --value "temp" --expiration-ttl 3600

  # Bulk put from JSON file
  cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --bulk --bulk-file data.json
`).WithStringFlag(
		"account-id", "", "Cloudflare account ID", &opts.accountID,
	).WithStringFlag(
		"namespace-id", "", "Namespace ID", &opts.namespaceID,
	).WithStringFlag(
		"namespace", "", "Namespace name (alternative to namespace-id)", &opts.namespace,
	).WithStringFlag(
		"key", "", "Key to put (required unless bulk operation)", &opts.key,
	).WithStringFlag(
		"value", "", "Value to put (required unless file specified)", &opts.value,
	).WithStringFlag(
		"file", "", "Read value from file instead of --value", &opts.inputFile,
	).WithStringFlag(
		"metadata-json", "", "JSON metadata to associate with the key", &opts.metadataJSON,
	).WithInt64Flag(
		"expiration", 0, "Expiration timestamp (Unix epoch)", &opts.expiration,
	).WithInt64Flag(
		"expiration-ttl", 0, "Expiration TTL in seconds", &opts.expirationTTL,
	).WithBoolFlag(
		"bulk", false, "Put multiple values from file", &opts.bulk,
	).WithStringFlag(
		"bulk-file", "", "File containing key-value pairs (JSON format)", &opts.bulkFile,
	).WithIntFlag(
		"batch-size", 0, "Batch size for bulk operations", &opts.batchSize,
	).WithIntFlag(
		"concurrency", 0, "Concurrency for bulk operations", &opts.concurrency,
	).WithRunE(
		WithConfigAndClient(func(cmd *cobra.Command, args []string, cfg *config.Config, client *api.Client) error {
			// Resolve account ID
			accountID, err := common.ValidateAccountID(cmd, cfg, opts.accountID)
			if err != nil {
				return err
			}

			// Create KV service
			service := kv.NewKVService(client)

			// Handle namespace ID resolution if namespace name is provided
			if opts.namespace != "" && opts.namespaceID == "" {
				nsID, err := service.ResolveNamespaceID(cmd.Context(), accountID, opts.namespace)
				if err != nil {
					return fmt.Errorf("failed to resolve namespace: %w", err)
				}
				opts.namespaceID = nsID
			}

			// Validate that we have a namespace ID
			if opts.namespaceID == "" {
				return fmt.Errorf("namespace-id or namespace is required")
			}

			// Validate operation mode
			if !opts.bulk {
				// Single key mode validation
				if opts.key == "" {
					return fmt.Errorf("key is required for single key operations")
				}

				if opts.value == "" && opts.inputFile == "" {
					return fmt.Errorf("either value or file is required for single key operations")
				}
			} else {
				// Bulk mode validation
				if opts.bulkFile == "" {
					return fmt.Errorf("bulk-file is required for bulk operations")
				}
			}

			// Single key mode
			if !opts.bulk {
				var value string
				if opts.inputFile != "" {
					// Read value from file
					fileData, err := os.ReadFile(opts.inputFile)
					if err != nil {
						return fmt.Errorf("failed to read input file: %w", err)
					}
					value = string(fileData)
				} else {
					value = opts.value
				}

				// Parse metadata if provided
				var metadata kv.KeyValueMetadata
				if opts.metadataJSON != "" {
					if err := json.Unmarshal([]byte(opts.metadataJSON), &metadata); err != nil {
						return fmt.Errorf("failed to parse metadata JSON: %w", err)
					}
				}

				// Create write options
				writeOptions := kv.WriteOptions{
					Expiration:    opts.expiration,
					ExpirationTTL: opts.expirationTTL,
				}
				if opts.metadataJSON != "" {
					writeOptions.Metadata = metadata
				}

				// Put the value
				err := service.Put(cmd.Context(), accountID, opts.namespaceID, opts.key, value, writeOptions)
				if err != nil {
					return fmt.Errorf("failed to put value: %w", err)
				}

				// Format success message with key-value table
				data := make(map[string]string)
				data["Key"] = opts.key
				data["Status"] = "Successfully stored"
				if opts.expiration > 0 {
					data["Expiration"] = fmt.Sprintf("%d", opts.expiration)
				} else if opts.expirationTTL > 0 {
					data["Expiration TTL"] = fmt.Sprintf("%d seconds", opts.expirationTTL)
				}
				
				common.FormatKeyValueTable(data)
				return nil
			}

			// Bulk mode
			// Read bulk file
			bulkData, err := os.ReadFile(opts.bulkFile)
			if err != nil {
				return fmt.Errorf("failed to read bulk file: %w", err)
			}

			// Parse bulk items
			var bulkItems []kv.BulkWriteItem
			if err := json.Unmarshal(bulkData, &bulkItems); err != nil {
				return fmt.Errorf("failed to parse bulk file (must be JSON array of objects): %w", err)
			}

			// Set up bulk write options
			bulkWriteOptions := kv.BulkWriteOptions{
				BatchSize:   opts.batchSize,
				Concurrency: opts.concurrency,
			}

			// Put values in bulk
			count, err := service.BulkPut(cmd.Context(), accountID, opts.namespaceID, bulkItems, bulkWriteOptions)
			if err != nil {
				return fmt.Errorf("bulk put operation failed: %w", err)
			}

			// Format bulk operation result
			data := make(map[string]string)
			data["Operation"] = "Bulk Store"
			data["Success Count"] = fmt.Sprintf("%d", count)
			data["Total Items"] = fmt.Sprintf("%d", len(bulkItems))
			if opts.concurrency > 0 {
				data["Concurrency"] = fmt.Sprintf("%d workers", opts.concurrency)
			}
			if opts.batchSize > 0 {
				data["Batch Size"] = fmt.Sprintf("%d items", opts.batchSize)
			}
			
			common.FormatKeyValueTable(data)
			return nil
		}),
	)
}