# Cloudflare KV Command Documentation

This document provides comprehensive information about the KV commands in the cache-kv-purger CLI tool, including the transition from the legacy structure to the new verb-based approach.

## Table of Contents

1. [Command Structure Overview](#command-structure-overview)
2. [Benefits of the New Structure](#benefits-of-the-new-structure)
3. [Command Reference](#command-reference)
   - [List Command](#list-command)
   - [Get Command](#get-command)
   - [Put Command](#put-command)
   - [Delete Command](#delete-command)
   - [Create Command](#create-command)
   - [Rename Command](#rename-command)
   - [Config Command](#config-command)
4. [Enhanced Search Capabilities](#enhanced-search-capabilities)
5. [Transition Guide](#transition-guide)
6. [Sync Operations](#sync-operations)
7. [Best Practices](#best-practices)

## Command Structure Overview

The KV commands follow a simple, intuitive verb-based structure. Instead of the previous deeply nested subcommands, you now have primary verb commands that work on both namespaces and keys:

```
cache-kv-purger kv <command> [flags]
```

The main commands are:

| Command    | Description                                   |
|------------|-----------------------------------------------|
| `list`     | List namespaces or keys with filtering options |
| `get`      | Get values for keys (single or bulk)           |
| `put`      | Put values for keys (single or bulk)           |
| `delete`   | Delete keys or namespaces (single or bulk)     |
| `create`   | Create namespaces                              |
| `rename`   | Rename namespaces                              |
| `config`   | Configure default settings                     |

> **Note**: The legacy command structure (e.g., `kv namespace list`, `kv values get`, `kv search`) has been completely removed. All KV operations now use the verb-based command structure described in this guide.

## Benefits of the New Structure

The new command structure provides several benefits:

- **Intuitive verb-based commands**: Use verbs as main commands for more natural interaction
- **Namespace resolution by name**: Use names or IDs interchangeably without extra lookup steps
- **Consolidated bulk operations**: Single commands handle both single and bulk operations via flags
- **Simplified command structure**: Flatter command hierarchy for easier discoverability
- **Deep search capabilities**: Powerful recursive metadata search across all structures
- **Consistent flag patterns**: Common flags work the same way across all commands
- **Fewer commands to learn**: Master a few commands instead of many specialized ones

## Command Reference

### List Command

List namespaces or keys in a namespace.

```bash
# List all namespaces
cache-kv-purger kv list --account-id YOUR_ACCOUNT_ID

# List keys in a namespace by ID
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID

# List keys in a namespace by name
cache-kv-purger kv list --namespace "My Namespace"

# List keys with a prefix
cache-kv-purger kv list --namespace "My Namespace" --prefix "product-"

# List keys with metadata
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --metadata

# Deep search for keys with value anywhere in metadata (recursive search)
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-image"
  
# Search for keys with specific metadata field
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --tag-field "status" --tag-value "archived"
```

**Common flags:**
- `--namespace-id`: ID of the namespace to list keys from
- `--namespace`: Name of the namespace to list keys from
- `--prefix`: List only keys with this prefix
- `--pattern`: List only keys matching this regex pattern
- `--search`: Value to search for recursively in metadata (deep search)
- `--tag-field`: Metadata field name to filter by
- `--tag-value`: Value to match in the tag field
- `--metadata`: Include metadata with keys
- `--values`: Include values with keys
- `--json`: Output results as JSON
- `--batch-size`: Size of operation batches
- `--concurrency`: Number of concurrent operations

### Get Command

Get values for one or more keys from a KV namespace.

```bash
# Get a single key
cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --key mykey

# Get a key with metadata
cache-kv-purger kv get --namespace "My Namespace" --key mykey --metadata

# Get multiple keys matching a prefix
cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --bulk --prefix "product-"

# Get keys matching a pattern
cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --bulk --pattern "product-.*-v1"
```

**Common flags:**
- `--namespace-id`: ID of the namespace
- `--namespace`: Name of the namespace
- `--key`: Key to get (required unless bulk operation)
- `--bulk`: Get multiple values based on filters
- `--prefix`: Get keys with this prefix (for bulk)
- `--pattern`: Get keys matching this pattern (for bulk)
- `--metadata`: Include metadata with values
- `--file`: Write output to file instead of stdout
- `--json`: Output results as JSON
- `--batch-size`: Batch size for bulk operations
- `--concurrency`: Concurrency for bulk operations

### Put Command

Put values for one or more keys in a KV namespace.

```bash
# Put a single key
cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --key mykey --value "My value"

# Put a value from a file
cache-kv-purger kv put --namespace "My Namespace" --key config.json --file ./config.json

# Put with expiration
cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --key temp-key --value "temp" --expiration-ttl 3600

# Bulk put from JSON file
cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --bulk --file data.json
```

**Common flags:**
- `--namespace-id`: ID of the namespace
- `--namespace`: Name of the namespace
- `--key`: Key to put
- `--value`: Value to put
- `--file`: Read value from file
- `--expiration`: Expiration timestamp (Unix epoch)
- `--expiration-ttl`: Expiration time-to-live in seconds
- `--bulk`: Put multiple values
- `--batch-size`: Batch size for bulk operations
- `--concurrency`: Concurrency for bulk operations

### Delete Command

Delete one or more keys from a KV namespace, or delete a namespace itself.

```bash
# Delete a single key
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --key mykey

# Delete the namespace itself
cache-kv-purger kv delete --namespace "My Namespace" --namespace-itself

# Delete keys with a prefix (dry run first)
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --prefix "temp-" --dry-run

# Delete ALL keys in a namespace while keeping the namespace itself
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --prefix "" --force

# Delete keys with deep recursive metadata search
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --search "old-data" --force

# Delete keys with specific metadata field/value
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --tag-field "status" --tag-value "archived"
```

**Common flags:**
- `--namespace-id`: ID of the namespace
- `--namespace`: Name of the namespace
- `--key`: Key to delete
- `--namespace-itself`: Delete the namespace itself
- `--bulk`: Delete multiple keys based on filters
- `--prefix`: Delete keys with this prefix
- `--pattern`: Delete keys matching this pattern
- `--search`: Delete keys containing this value (deep recursive search)
- `--tag-field`: Delete keys with this metadata field
- `--tag-value`: Delete keys with this metadata field value
- `--dry-run`: Show what would be deleted without deleting
- `--force`: Skip confirmation prompt
- `--batch-size`: Batch size for bulk operations
- `--concurrency`: Concurrency for bulk operations

### Create Command

Create namespaces.

```bash
# Create a new namespace
cache-kv-purger kv create --account-id YOUR_ACCOUNT_ID --title "My Namespace"
```

**Common flags:**
- `--account-id`: Account ID (required)
- `--title`: Title for the new namespace (required)

### Rename Command

Rename namespaces.

```bash
# Rename a namespace
cache-kv-purger kv rename --namespace-id YOUR_NAMESPACE_ID --title "New Name"

# Rename a namespace by name
cache-kv-purger kv rename --namespace "Old Name" --title "New Name"
```

**Common flags:**
- `--namespace-id`: ID of the namespace
- `--namespace`: Name of the namespace
- `--title`: New title for the namespace (required)

### Config Command

Configure default settings.

```bash
# Set default account ID
cache-kv-purger kv config --account-id YOUR_ACCOUNT_ID

# Show current configuration
cache-kv-purger kv config --show
```

**Common flags:**
- `--account-id`: Set default account ID
- `--show`: Show current configuration

## Enhanced Search Capabilities

The tool now features powerful search capabilities integrated into both the `list` and `delete` commands:

### Deep Recursive Metadata Search

```bash
# Search recursively through nested metadata structures
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-tag"
```

This deep search:
- Searches recursively through all metadata values at any level of nesting
- Works with complex objects, arrays, and scalar values
- Performs case-insensitive matching for better results
- Is much more powerful than the field-specific search with --tag-field and --tag-value

When to use each search method:
- Use `--search` when you don't know the specific structure of the metadata
- Use `--tag-field` and `--tag-value` when you know the exact field path

## Transition Guide

The following table maps the legacy commands to their new consolidated equivalents:

| Legacy Command | New Consolidated Command |
|----------------|--------------------------|
| `kv namespace list` | `kv list` |
| `kv namespace create` | `kv create --title "Name"` |
| `kv namespace delete` | `kv delete --namespace NAME --namespace-itself` |
| `kv namespace rename` | `kv rename --namespace NAME --title "New Name"` |
| `kv values list` | `kv list --namespace NAME` |
| `kv values get` | `kv get --key KEY` |
| `kv values put` | `kv put --key KEY --value VALUE` |
| `kv values delete` | `kv delete --key KEY` |
| `kv get-with-metadata` | `kv get --key KEY --metadata` |
| `kv exists` | `kv get --key KEY --check-exists` |
| `kv search` | `kv list --search VALUE` or `kv delete --bulk --search VALUE` |

## Bulk Operation Format

For bulk `put` operations, the input file should be a JSON array of objects with this structure:

```json
[
  {
    "key": "key1",
    "value": "value1"
  },
  {
    "key": "key2",
    "value": {
      "complex": "object",
      "with": ["nested", "values"]
    },
    "expiration": 1714567890,
    "metadata": {
      "type": "config",
      "tags": ["important", "production"]
    }
  }
]
```

## Sync Operations

The tool provides powerful combined commands that span multiple Cloudflare APIs, enabling more efficient workflows and unified operations.

### Sync Purge Command

The `sync purge` command allows you to:

1. Find KV keys using metadata search or specific tag filtering
2. Delete matching KV keys
3. Purge cache tags for associated content
4. All in a single operation with batch processing and dry-run support

```bash
# Purge KV keys with a specific search value and related cache tags 
cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images

# Use metadata field-specific search and purge cache
cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --tag-field "type" --tag-value "temp" --zone example.com --cache-tag temp-data
  
# Dry run to preview without making changes
cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images --dry-run

# Using namespace name instead of ID
cache-kv-purger sync purge --namespace "My KV Namespace" --search "product-123" --zone example.com --cache-tag product-images
```

This command is ideal for maintaining consistency between your KV storage and CDN cache, particularly useful for:

- Content updates where both storage and cache must be updated
- Maintenance operations requiring synchronized data removal
- Feature deployments that need to ensure fresh cache content

**How it works:**
1. First searches for matching KV keys using either deep recursive search or field-specific matching
2. Shows a preview of matched keys (with sample output for large sets)
3. Deletes the matching KV keys (unless in dry-run mode)
4. Purges specified cache tags from the CDN in the same operation

**Common flags:**
- `--namespace-id` or `--namespace`: Target namespace ID or name
- `--search`: Value to search for in KV metadata (deep recursive search)
- `--tag-field` and `--tag-value`: For field-specific metadata search 
- `--zone`: Zone ID or domain for cache operations
- `--cache-tag`: Cache tags to purge (can be specified multiple times)
- `--dry-run`: Preview changes without making them
- `--concurrency`: Control the number of concurrent operations
- `--batch-size`: Set the batch size for KV operations

## Performance Optimizations

All KV operations now leverage high-performance implementations by default:

### Automatic Optimizations

1. **Metadata Operations**: When using `--metadata` flag, the tool automatically:
   - Fetches metadata in a single API call instead of N+1 queries (100-1000x fewer API calls)
   - Uses LRU caching to eliminate redundant metadata lookups
   - Leverages bulk metadata fetching for multiple keys

2. **List Operations**: Automatically use:
   - Parallel pagination (fetches up to 5 pages concurrently)
   - Streaming JSON parser for constant memory usage with millions of keys
   - Enhanced list API that includes metadata in response when requested

3. **Search Operations**: Optimized with:
   - Single-pass metadata search eliminating separate metadata fetches
   - Efficient deep recursive search through nested structures
   - Streaming processing for large result sets

4. **Bulk Operations**: All bulk operations feature:
   - Configurable concurrency (default: 10 workers)
   - Optimized batching with no artificial delays
   - Connection pooling with HTTP/2 support
   - Automatic retry with exponential backoff

### Performance Tips

- **For large namespaces**: Use `--batch-size 1000` and `--concurrency 20` for optimal throughput
- **When listing with metadata**: The `--metadata` flag no longer causes performance degradation
- **For exports**: Operations are now ~55x faster with optimized batching
- **Cache utilization**: Repeated operations benefit from the 5-minute metadata cache

## Best Practices

1. **Use namespace names**: Use `--namespace` (name) instead of `--namespace-id` for better readability
2. **Safety first**: Always use `--dry-run` before bulk deletion operations to preview changes
3. **Performance tuning**: For large operations, tune `--batch-size` and `--concurrency` parameters
4. **Metadata visibility**: Use `--metadata` with search operations to see matching structures
5. **Deep search**: Use `--search` without `--tag-field` for powerful recursive searches through complex metadata
6. **JSON formatting**: When machine-readable output is needed, use the `--json` flag
7. **Synchronized operations**: Use the `sync purge` command when you need to update both KV and cache in sync
8. **Migration strategy**: 
   - Update scripts to use the new command format
   - Test commands with `--dry-run` or non-destructive flags first
   - Update internal documentation to reference the new command structure