# KV Command Guide

This document provides a guide to using the consolidated KV command structure in the cache-kv-purger CLI tool.

## Overview

The KV commands have been reorganized to follow a simpler, more intuitive verb-based structure. This means that instead of having deeply nested subcommands, you now have primary verb commands (`list`, `get`, `put`, `delete`) that work on both namespaces and keys.

> **Note**: The legacy command structure (e.g., `kv namespace list`, `kv values get`) is still available but marked as deprecated. All legacy commands will display a deprecation notice directing users to the equivalent consolidated command.

## Benefits

- **Intuitive verb-based commands**: Use verbs (`list`, `get`, `put`, `delete`) as main commands
- **Namespace resolution by name**: Use either namespace IDs or names
- **Consolidated bulk operations**: Single commands handle both single and bulk operations based on flags
- **Simplified command structure**: Flatter command hierarchy for easier discoverability

## Command Structure

```
cache-kv-purger kv <command> [flags]
```

The main commands are:

- `list`: List namespaces or keys in a namespace
- `get`: Get values for keys
- `put`: Put values for keys
- `delete`: Delete keys or namespaces
- `namespace`: Operations specific to namespaces (create, rename)

## Common Flags

These flags are available across most commands:

- `--account-id`: Cloudflare Account ID
- `--namespace-id`: ID of the namespace to operate on
- `--namespace`: Name of the namespace to operate on (alternative to namespace-id)
- `--bulk`: Enables bulk operation mode
- `--json`: Output results as JSON

## Command Details

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

# Search for keys containing a value
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-image"
```

Flags:
- `--namespace-id`: ID of the namespace to list keys from
- `--namespace`: Name of the namespace to list keys from
- `--prefix`: List only keys with this prefix
- `--pattern`: List only keys matching this regex pattern
- `--search`: Value to search for anywhere in metadata
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

Flags:
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

Flags:
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

# Delete keys matching a search pattern
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --search "old-data" --force
```

Flags:
- `--namespace-id`: ID of the namespace
- `--namespace`: Name of the namespace
- `--key`: Key to delete
- `--namespace-itself`: Delete the namespace itself
- `--bulk`: Delete multiple keys based on filters
- `--prefix`: Delete keys with this prefix
- `--pattern`: Delete keys matching this pattern
- `--search`: Delete keys containing this value
- `--tag-field`: Delete keys with this metadata field
- `--tag-value`: Delete keys with this metadata field value
- `--dry-run`: Show what would be deleted without deleting
- `--force`: Skip confirmation prompt
- `--batch-size`: Batch size for bulk operations
- `--concurrency`: Concurrency for bulk operations

### Namespace Command

Create, rename, or find namespaces.

```bash
# Create a new namespace
cache-kv-purger kv namespace create --account-id YOUR_ACCOUNT_ID --title "My Namespace"

# Rename a namespace
cache-kv-purger kv namespace rename --namespace-id YOUR_NAMESPACE_ID --title "New Name"

# Rename a namespace by name
cache-kv-purger kv namespace rename --namespace "Old Name" --title "New Name"
```

Flags:
- `--namespace-id`: ID of the namespace
- `--namespace`: Name of the namespace
- `--title`: Title for namespace operations
- `--create`: Create a new namespace
- `--rename`: Rename an existing namespace

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

## Tips

1. **Namespace Resolution**: You can use either `--namespace-id` or `--namespace` (name) for most operations

2. **Bulk Operations**: Add the `--bulk` flag to enable bulk mode for get, put, and delete operations

3. **Safety First**: Use `--dry-run` to preview destructive operations before running them

4. **JSON Output**: Use `--json` for machine-readable output

5. **Performance Tuning**: Adjust `--batch-size` and `--concurrency` for large operations

## Legacy Command Mapping

Here's a mapping of legacy commands to their consolidated equivalents:

| Legacy Command | Consolidated Command |
|----------------|----------------------|
| `kv namespace list` | `kv list` |
| `kv namespace create` | `kv create` |
| `kv namespace delete` | `kv delete --namespace NAME --namespace-itself` |
| `kv namespace rename` | `kv rename` |
| `kv values list` | `kv list --namespace NAME` |
| `kv values get` | `kv get` |
| `kv values put` | `kv put` |
| `kv values delete` | `kv delete` |
| `kv exists` | `kv get --check-exists` |
| `kv get-with-metadata` | `kv get --metadata` |

All legacy commands are marked as deprecated but will continue to work for backward compatibility.