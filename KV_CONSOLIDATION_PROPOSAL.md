# KV Command Unification Proposal

## Current Structure

The current CLI has a hierarchical command structure with separate commands for different operations:

```
kv
├── namespace
│   ├── list
│   ├── create
│   ├── delete
│   ├── rename
│   └── bulk-delete
├── values
│   ├── list
│   ├── get
│   ├── put
│   └── delete
├── config
├── exists
├── get-with-metadata
└── search
```

This approach has several drawbacks:
- Complex command hierarchy with multiple levels
- Related functionality spread across different command paths
- Similar operations implemented with different patterns
- Bulk operations not consistently available

## Proposed Unified Approach

We propose simplifying to a flat command structure based on core operations (verbs) with flags determining scope:

```
kv
├── list       # List namespaces or keys with filtering options
├── get        # Get key values (single or bulk)
├── put        # Put key values (single or bulk)
├── delete     # Delete keys, namespaces (single or bulk)
├── create     # Create resources (namespaces)
├── rename     # Rename resources (namespaces)
└── config     # Configure settings
```

This approach follows these principles:
1. Command verbs indicate the primary action
2. Flags determine what is being acted upon (namespace/key)
3. Bulk operations use the same commands with appropriate flags
4. Consistent filtering mechanisms across commands
5. Logical parameter grouping

## Detailed Command Specifications

### `kv list` - Unified List Command

Lists namespaces, keys, or key details based on flags.

```
kv list [flags]
  # Target selection
  --namespace-id string    # When provided, lists keys in namespace instead of namespaces
  --namespace string       # Namespace name/title instead of ID (will be resolved to ID)
  --all-namespaces         # List keys from all namespaces (requires account-id)
  
  # Key filtering (when listing keys)
  --key string             # Get details about a specific key
  --prefix string          # Filter keys by prefix
  --pattern string         # Filter by regex pattern
  --cursor string          # Pagination cursor
  --limit int              # Maximum items to return
  
  # Advanced filtering
  --search string          # Value to search for anywhere in key/value/metadata
  --tag-field string       # Metadata field to match
  --tag-value string       # Value in metadata field to match
  
  # Output options
  --metadata               # Include metadata with keys
  --values                 # Include values with keys (may be slower for large result sets)
  --expiring               # Show only keys with expiration
  --output-format string   # Format: json, table, etc.
  
  # Performance options
  --batch-size int         # Size of batch operations when retrieving data
  --concurrency int        # Number of concurrent operations for bulk retrieval
```

Examples:
```bash
# List all namespaces
kv list --account-id 01a7362d577a6c3019a474fd6f485823

# List keys in a namespace
kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7

# List keys with a prefix
kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --prefix "product-"

# List keys with metadata and filter by tag
kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --metadata --tag-field "cache-tag" --tag-value "product-images"

# Search for keys containing a value
kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --search "product-image" --metadata

# Get detailed information about a specific key
kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --metadata --values
```

### `kv get` - Unified Get Command

Gets values for one or more keys.

```
kv get [flags]
  # Target selection
  --namespace-id string    # Target namespace (required unless namespace name provided)
  --namespace string       # Namespace name/title instead of ID (will be resolved to ID)
  --key string             # Key to get (required unless bulk operation)
  
  # Bulk operation flags
  --bulk                   # Get multiple values based on filters
  --keys string            # Comma-separated list of keys or @file.txt
  --prefix string          # Get keys with prefix
  --pattern string         # Get keys matching regex pattern
  --search string          # Get keys containing this value
  --tag-field string       # Get keys with this metadata field
  --tag-value string       # Get keys with this metadata field/value
  
  # Output options
  --metadata               # Include metadata with values
  --file string            # Write output to file instead of stdout
  --output-format string   # Output format: json, raw, etc.
  
  # Performance options
  --batch-size int         # Batch size for bulk operations
  --concurrency int        # Concurrency for bulk operations
```

Examples:
```bash
# Get a single key
kv get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Get a key with metadata
kv get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --metadata

# Get multiple keys
kv get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --keys "key1,key2,key3"

# Get keys with prefix
kv get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --prefix "product-" --metadata
```

### `kv put` - Unified Put Command

Puts values for one or more keys.

```
kv put [flags]
  # Target selection
  --namespace-id string    # Target namespace (required unless namespace name provided)
  --namespace string       # Namespace name/title instead of ID (will be resolved to ID)
  --key string             # Key to put (required unless bulk operation)
  --value string           # Value to put (required unless file specified)
  --file string            # Read value from file instead of --value
  
  # Value options
  --metadata-json string   # JSON metadata to associate with the key
  --expiration int         # Expiration timestamp (Unix epoch)
  --expiration-ttl int     # Expiration TTL in seconds
  
  # Bulk operation flags
  --bulk                   # Put multiple values
  --keys-file string       # File containing key-value pairs (JSON format)
  --batch-size int         # Batch size for bulk operations
  --concurrency int        # Concurrency for bulk operations
```

Examples:
```bash
# Put a single key
kv put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --value "My value data"

# Put a value from file
kv put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key config-json --file ./config.json

# Put with metadata
kv put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key product-123 --value "Product data" --metadata-json '{"cache-tag":"products", "version":"1.0"}'

# Put with expiration
kv put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key temp-key --value "Temporary data" --expiration-ttl 3600

# Bulk put from JSON file
kv put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --keys-file data.json --concurrency 20
```

### `kv delete` - Unified Delete Command

Deletes keys or namespaces.

```
kv delete [flags]
  # Target selection
  --namespace-id string    # Target namespace (required unless namespace name provided)
  --namespace string       # Namespace name/title instead of ID (will be resolved to ID)
  --key string             # Key to delete (required unless bulk deletion)
  
  # Namespace deletion
  --namespace-itself       # Delete the namespace itself (not keys)
  
  # Bulk operation flags
  --bulk                   # Delete multiple keys based on filters
  --keys string            # Comma-separated list of keys or @file.txt
  --prefix string          # Delete keys with prefix
  --pattern string         # Delete keys matching regex pattern
  --search string          # Delete keys containing this value
  --tag-field string       # Delete keys with this metadata field
  --tag-value string       # Delete keys with this metadata field/value
  
  # Safety and performance
  --dry-run                # Show what would be deleted without deleting
  --force                  # Skip confirmation prompt
  --batch-size int         # Batch size for bulk operations
  --concurrency int        # Concurrency for bulk operations
```

Examples:
```bash
# Delete a single key
kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Delete multiple specific keys
kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --keys "key1,key2,key3"

# Delete all keys with a prefix (with dry run)
kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --prefix "temp-" --dry-run

# Delete keys by metadata (with confirmation)
kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --tag-field "status" --tag-value "archived"

# Delete the namespace itself
kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --namespace
```

### `kv create` - Create Resources

Creates namespaces.

```
kv create [flags]
  # Namespace creation
  --namespace              # Create a namespace (required)
  --title string           # Title for the new namespace (required)
  --account-id string      # Account ID for the namespace
```

Examples:
```bash
# Create a namespace
kv create --namespace --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache"
```

### `kv rename` - Rename Resources

Renames namespaces.

```
kv rename [flags]
  # Target
  --namespace-id string    # ID of the namespace to rename (required unless namespace name provided)
  --namespace string       # Namespace name/title instead of ID (will be resolved to ID)
  --title string           # New title for the namespace (required)
```

Examples:
```bash
# Rename a namespace
kv rename --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --title "New Namespace Name"
```

### `kv config` - Configure Settings

Configures default settings.

```
kv config [flags]
  --account-id string      # Set default account ID for KV operations
  --show                   # Show current configuration
```

Examples:
```bash
# Set default account ID
kv config --account-id 01a7362d577a6c3019a474fd6f485823

# Show current configuration
kv config --show
```

## Benefits of the Unified Approach

1. **Simplicity**: Reduces command complexity from 11+ commands to 7 core commands
2. **Discoverability**: Users only need to know what action they want to perform (list, get, put, delete)
3. **Consistency**: Similar operations use consistent patterns across commands
4. **Power**: Advanced filtering and bulk operations available for all relevant commands
5. **Less typing**: Common operations require fewer keystrokes
6. **Easier help**: Help documentation is organized by primary action
7. **Intuitive design**: Follows natural language patterns (verb + modifiers)
8. **Flexibility**: Support for namespace lookup by name instead of requiring IDs
9. **Smart autocompletion**: Dynamic completion of namespaces, keys, and other context-aware values
10. **Improved productivity**: Shell autocompletion reduces errors and speeds up command construction

## Implementation Plan

### Phase 1: Core Command Structure

1. Implement the new unified list command
2. Implement the new unified get command
3. Implement the new unified put command
4. Implement the new unified delete command
5. Implement the simplified create and rename commands
6. Update the config command
7. Add backward compatibility aliases

### Phase 2: Advanced Features

1. Add comprehensive filtering mechanisms
2. Implement bulk operations for all commands
3. Add advanced output formatting options
4. Implement performance optimizations for bulk operations
5. Add shell autocompletion for all commands and flags
6. Implement namespace name resolution for all commands

### Phase 3: Shell Autocompletion

1. Implement dynamic autocompletion for:
   - Namespace names based on available namespaces
   - Keys based on keys in the selected namespace
   - Common values for metadata fields
   - Output formats and other fixed-value flags
2. Support bash, zsh, fish, and PowerShell completions
3. Add installation instructions for shell completions

### Phase 4: Documentation and Refinement

1. Update README.md with unified command structure
2. Add detailed examples for common use cases
3. Refine help text and error messages
4. Add deprecation notices for old command paths
5. Document autocompletion installation and use

## Backward Compatibility

For at least one major version cycle, maintain backward compatibility by:

1. Creating aliases from old command paths to new ones with appropriate flag mappings
2. Adding deprecation notices on old command paths
3. Providing migration guides in documentation

## Example Usage Comparison

### Current Approach:
```bash
# List namespaces
cache-kv-purger kv namespace list --account-id 01a7362d577a6c3019a474fd6f485823

# List keys in a namespace
cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --all

# Get a key
cache-kv-purger kv values get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Get key with metadata
cache-kv-purger kv get-with-metadata --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Search for keys
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --value "product-image" --metadata
```

### Unified Approach:
```bash
# List namespaces
cache-kv-purger kv list --account-id 01a7362d577a6c3019a474fd6f485823

# List keys in a namespace
cache-kv-purger kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7

# List keys using namespace name instead of ID
cache-kv-purger kv list --namespace "My Application Cache"

# Get a key
cache-kv-purger kv get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Get key with metadata
cache-kv-purger kv get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --metadata

# Search for keys
cache-kv-purger kv list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --search "product-image" --metadata

# Delete multiple keys matching a pattern with autocompletion
cache-kv-purger kv delete --namespace "My App Cache" --bulk --prefix "temp-" --force
```

### Shell Autocompletion:
```bash
# Install autocompletion for bash
cache-kv-purger completion bash > ~/.cache-kv-purger-completion.bash
echo 'source ~/.cache-kv-purger-completion.bash' >> ~/.bashrc

# Using autocompletion (hit TAB after typing)
cache-kv-purger kv list --namespace <TAB>
# Shows all available namespaces

cache-kv-purger kv get --namespace "My Cache" --key <TAB>
# Shows all keys in the selected namespace

cache-kv-purger kv put --namespace "My Cache" --key mykey --metadata-json <TAB>
# Shows recently used metadata JSON objects
```

## Conclusion

This unified approach drastically simplifies the command structure while maintaining all existing functionality. It makes the tool more intuitive, more consistent, and more powerful by providing a common set of filtering and bulk operation capabilities across all commands.

The implementation can be done incrementally, ensuring backward compatibility while gradually moving users toward the more streamlined interface.

## Code Refactoring Implementation Plan

To implement the consolidated KV command structure, we need to refactor the backend code. Here's a detailed plan for refactoring:

### 1. Service Layer (New)

Create a new service layer in `internal/kv/service.go` that provides a unified interface for all KV operations:

```go
// KVService provides a unified interface for KV operations
type KVService interface {
    // Namespace operations
    ListNamespaces(ctx context.Context, accountID string) ([]Namespace, error)
    CreateNamespace(ctx context.Context, accountID, title string) (*Namespace, error)
    RenameNamespace(ctx context.Context, accountID, namespaceID, newTitle string) (*Namespace, error)
    DeleteNamespace(ctx context.Context, accountID, namespaceID string) error
    
    // Namespace resolution
    ResolveNamespaceID(ctx context.Context, accountID, nameOrID string) (string, error)
    
    // Basic operations
    List(ctx context.Context, accountID, namespaceID string, options ListOptions) (ListKeysResult, error)
    Get(ctx context.Context, accountID, namespaceID, key string, options GetOptions) (*KeyValuePair, error)
    Put(ctx context.Context, accountID, namespaceID, key, value string, options WriteOptions) error
    Delete(ctx context.Context, accountID, namespaceID, key string) error
    
    // Bulk operations
    BulkGet(ctx context.Context, accountID, namespaceID string, keys []string, options BulkGetOptions) ([]KeyValuePair, error)
    BulkPut(ctx context.Context, accountID, namespaceID string, items []BulkWriteItem, options BulkWriteOptions) (int, error)
    BulkDelete(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error)
    
    // Search operations
    Search(ctx context.Context, accountID, namespaceID string, options SearchOptions) ([]KeyValuePair, error)
}

// Various option structs for operations
type ListOptions struct {
    Limit         int
    Cursor        string
    Prefix        string
    Pattern       string
    IncludeValues bool
    IncludeMetadata bool
}

type GetOptions struct {
    IncludeMetadata bool
}

type BulkGetOptions struct {
    IncludeMetadata bool
    BatchSize       int
    Concurrency     int
    Prefix          string
    Pattern         string
}

type BulkWriteOptions struct {
    BatchSize       int
    Concurrency     int
}

type BulkDeleteOptions struct {
    BatchSize       int
    Concurrency     int
    DryRun          bool
    Force           bool
}

type SearchOptions struct {
    TagField        string
    TagValue        string
    SearchValue     string
    IncludeMetadata bool
    BatchSize       int
    Concurrency     int
}
```

### 2. Service Implementation

Implement the service using existing functions from the current codebase:

```go
// cloudflareKVService implements the KVService interface using Cloudflare API
type cloudflareKVService struct {
    client *api.Client
}

// NewKVService creates a new KV service
func NewKVService(client *api.Client) KVService {
    return &cloudflareKVService{
        client: client,
    }
}

// Implement each method using existing functions...
```

### 3. Reorganized Files

Reorganize the code to maintain clean separation of concerns:

- `types.go`: All data structures and common types
- `service.go`: Service interface and implementation
- `namespace.go`: Namespace-specific operations
- `list.go`: List operations (for keys and namespaces)
- `get.go`: Get operations (single and bulk)
- `put.go`: Put operations (single and bulk)
- `delete.go`: Delete operations (single and bulk)
- `search.go`: Advanced search capabilities
- `util.go`: Utility functions

### 4. Command Implementation

Use the command builder pattern to implement the consolidated commands:

```go
// list.go in cmd package
func NewKVListCommand() *cmdutil.CommandBuilder {
    // Define flag variables
    var opts struct {
        namespaceID   string
        namespace     string
        accountID     string
        // other flags...
    }
    
    // Create command
    return cmdutil.NewCommand("list", "List namespaces or keys", "...")
        .WithExample("...")
        .WithStringFlag("namespace-id", "", "Namespace ID", &opts.namespaceID)
        .WithStringFlag("namespace", "", "Namespace name", &opts.namespace)
        // Add other flags...
        .WithRunE(cmdutil.WithConfigAndClient(func(cmd *cobra.Command, args []string, 
                                            cfg *config.Config, client *api.Client) error {
            // Create service
            svc := kv.NewKVService(client)
            
            // Resolve namespace ID if name provided
            if opts.namespace != "" {
                resolved, err := svc.ResolveNamespaceID(cmd.Context(), opts.accountID, opts.namespace)
                if err != nil {
                    return err
                }
                opts.namespaceID = resolved
            }
            
            // Either list namespaces or keys
            if opts.namespaceID == "" {
                // List namespaces
                namespaces, err := svc.ListNamespaces(cmd.Context(), opts.accountID)
                if err != nil {
                    return err
                }
                
                // Format and display namespaces
                // ...
            } else {
                // List keys
                listOpts := kv.ListOptions{
                    // Map command flags to options...
                }
                
                result, err := svc.List(cmd.Context(), opts.accountID, opts.namespaceID, listOpts)
                if err != nil {
                    return err
                }
                
                // Format and display keys
                // ...
            }
            
            return nil
        }))
}

// Similar implementation for other commands...
```

### 5. Command Registration

Update the command registration to use the new consolidated commands:

```go
// In command registration file
func RegisterCommands(rootCmd *cobra.Command) {
    kvCmd := &cobra.Command{
        Use:   "kv",
        Short: "Manage KV store",
        Long:  "Manage Cloudflare Workers KV store namespaces and values",
    }
    
    // Add new verb-based commands
    kvCmd.AddCommand(NewKVListCommand().Build())
    kvCmd.AddCommand(NewKVGetCommand().Build())
    kvCmd.AddCommand(NewKVPutCommand().Build())
    kvCmd.AddCommand(NewKVDeleteCommand().Build())
    kvCmd.AddCommand(NewKVCreateCommand().Build())
    kvCmd.AddCommand(NewKVRenameCommand().Build())
    kvCmd.AddCommand(NewKVConfigCommand().Build())
    
    // Register legacy commands with deprecation notices
    // ...
    
    rootCmd.AddCommand(kvCmd)
}
```

### 6. Backward Compatibility

Ensure backward compatibility with old command structure:

```go
// Create aliases using the old command structure
nsCmd := &cobra.Command{
    Use:        "namespace",
    Short:      "Manage KV namespaces (deprecated)",
    Long:       "Manage KV namespaces - deprecated, use verb commands directly",
    Deprecated: "Use 'kv list', 'kv create', etc. instead",
}

// Add sub-commands that map to the new commands
nsCmd.AddCommand(&cobra.Command{
    Use:        "list",
    Short:      "List namespaces (deprecated)",
    Deprecated: "Use 'kv list' instead",
    Run: func(cmd *cobra.Command, args []string) {
        // Forward to new command by reconstructing args
        kvListCmd := rootCmd.FindCommand([]string{"kv", "list"})
        kvListCmd.Run(cmd, args)
    },
})

// Similar implementation for other legacy commands...
```

This implementation plan provides a structured approach to refactoring the KV code while maintaining all existing functionality and ensuring backward compatibility.