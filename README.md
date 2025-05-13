# Cloudflare Cache KV Purger

[![Build and Test](https://github.com/erfianugrah/cache-kv-purger/actions/workflows/build.yml/badge.svg)](https://github.com/erfianugrah/cache-kv-purger/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/erfianugrah/cache-kv-purger)](https://goreportcard.com/report/github.com/erfianugrah/cache-kv-purger)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A command-line interface tool for managing Cloudflare cache purging and Workers KV store operations.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Global Commands](#global-commands)
- [Cache Commands](#cache-commands)
- [KV Commands Overview](#kv-commands-overview)
- [Sync Operations](#sync-operations)
- [Zone Commands](#zone-commands)
- [Advanced Features](#advanced-features)
- [Future Enhancements](#future-enhancements)
- [Development](#development)
- [Documentation & References](#documentation--references)
- [License](#license)

## Features

- **Cache Management**
  - Purge entire cache
  - Purge specific files
  - Purge files with custom headers (for cache variants)
  - Purge by cache tags, hosts, or prefixes
  - Multi-zone purging support
  - Batch processing for cache tag purging

- **KV Store Management**
  - Create, list, rename, and delete namespaces
  - Bulk namespace management with pattern matching
  - Read, write, and delete key-value pairs
  - Metadata and expiration support
  - Smart key searching with metadata filtering
  - Recursive searching through nested metadata
  - **NEW** Intuitive verb-based commands (list, get, put, delete)
  - **NEW** Support for both single and bulk operations in the same commands
  - **NEW** Namespace resolution by name or ID without explicit lookups

- **Combined API Operations**
  - Execute operations across multiple Cloudflare APIs in one command
  - Purge both KV keys and cache in a single operation
  - Smart metadata search integrated with cache purging
  - Efficient cross-API workflows with unified interface

- **Zone Management**
  - List and query zone information
  - Configure default zones
  - Zone resolution by name or ID

- **Technical Architecture**
  - Modular codebase with separation of concerns
  - Command builder pattern for easy extension
  - Middleware for common operations
  - Standardized validation and error handling
  - Configurable batch processing with progress reporting

## Installation

### From Binary Releases (Recommended)

Download the latest release for your platform from the [Releases page](https://github.com/erfianugrah/cache-kv-purger/releases).

```bash
# Download (replace X.Y.Z with version and OS with your platform)
curl -LO https://github.com/erfianugrah/cache-kv-purger/releases/download/vX.Y.Z/cache-kv-purger_Linux_x86_64.tar.gz

# Extract
tar -xzf cache-kv-purger_Linux_x86_64.tar.gz

# Move to a directory in your PATH
sudo mv cache-kv-purger /usr/local/bin/

# Verify installation
cache-kv-purger --version
```

### From Source

```bash
# Clone the repository
git clone https://github.com/erfianugrah/cache-kv-purger.git
cd cache-kv-purger

# Build the binary
go build -o cache-kv-purger ./cmd/cli

# Run the tool
./cache-kv-purger
```

## Authentication

The tool uses Cloudflare API credentials for authentication.

### API Token (Recommended)

```bash
# Set environment variable
export CLOUDFLARE_API_TOKEN=your_cloudflare_api_token
```

#### Required API Token Permissions

When creating your Cloudflare API token, make sure to include the following permissions:

**For Cache Operations:**
- Zone > Cache Purge > Purge

**For KV Operations:**
- Account > Workers KV Storage > Edit
- Account > Workers KV Storage > Read

**For Zone Operations:**
- Zone > Zone > Read
- Zone > Zone Settings > Read

Without these permissions, certain commands may fail with authorization errors. You can create different tokens with specific permissions if you only need to use a subset of the functionality.

### API Key (Legacy)

```bash
# Set environment variables
export CLOUDFLARE_API_KEY=your_cloudflare_api_key
export CLOUDFLARE_EMAIL=your_email@example.com
```

## Configuration

### Account ID and Zone ID

For KV operations, you'll need your Cloudflare Account ID. For cache operations, you'll need your Zone ID or domain name.

#### Using Environment Variables

```bash
# Set environment variables
export CLOUDFLARE_ACCOUNT_ID=your_account_id
export CLOUDFLARE_ZONE_ID=your_zone_id
```

#### Advanced Environment Variables

The tool supports these additional environment variables for customizing behavior:

```bash
# API request timeout in seconds (default: 60)
export CLOUDFLARE_API_TIMEOUT=120

# Maximum concurrent requests for batch operations (default varies by operation)
export CLOUDFLARE_MAX_CONCURRENCY=30

# Cache purge concurrency (default: 10, max: 20)
export CLOUDFLARE_CACHE_CONCURRENCY=15

# Multi-zone concurrent operations (default: 3)
export CLOUDFLARE_MULTI_ZONE_CONCURRENCY=5 

# Default batch size for operations (default varies by operation)
export CLOUDFLARE_BATCH_SIZE=500

# Retry count for failed API requests (default: 3)
export CLOUDFLARE_RETRY_COUNT=5

# Base backoff delay in milliseconds for retries (default: 1000)
export CLOUDFLARE_BACKOFF_DELAY=2000
```

#### Using Config Command

```bash
# Set default zone
cache-kv-purger config set-defaults --zone example.com

# Set default account ID
cache-kv-purger config set-defaults --account-id your_account_id

# Set custom API endpoint (if needed)
cache-kv-purger config set-defaults --api-endpoint https://custom-api.cloudflare.com/client/v4

# Set default request timeout
cache-kv-purger config set-defaults --timeout 120

# Set default batch parameters
cache-kv-purger config set-defaults --batch-size 250 --concurrency 20

# Set cache-specific concurrency limits
cache-kv-purger config set-defaults --concurrency 15 --zone-concurrency 5

# View current configuration
cache-kv-purger config show
```

#### Configuration Precedence

The tool prioritizes configuration sources in the following order:
1. Command-line flags (highest priority)
2. Environment variables 
3. Configuration file (created with `config set-defaults`)
4. Built-in defaults (lowest priority)

## Verbosity Controls

The tool provides two ways to control verbosity level:

### Global Verbosity Flag

```bash
cache-kv-purger --verbosity=<level> <command>
```

Supported verbosity levels:
- `quiet`: Minimal output showing only results and errors
- `normal`: Standard output (default)
- `verbose`: Detailed output including progress and operation details
- `debug`: Developer-level debug information

### Command-specific Verbose Flag

Each command also supports a command-specific verbose flag for convenience:

```bash
cache-kv-purger <command> --verbose
```

This is equivalent to using `--verbosity=verbose` and is provided for ease of use.

If both flags are specified, the more verbose setting will be used. For example, if you use both `--verbose` and `--verbosity=debug`, the debug level will be applied.

## Global Commands

All commands support the following global flags:

- `--verbosity`: Control output level as described above
- `--verbose`: Enable detailed output (shorthand for --verbosity=verbose)
- `--zone`: Specify a zone ID or domain name

### Config Command

```bash
# Show current configuration
cache-kv-purger config show

# Set default values
cache-kv-purger config set-defaults --zone example.com --account-id 01a7362d577a6c3019a474fd6f485823
```

## Cache Commands

### Purge Everything

Purges all cached content for a zone.

```bash
# Using zone ID
cache-kv-purger cache purge everything --zone 01a7362d577a6c3019a474fd6f485823

# Using domain name
cache-kv-purger cache purge everything --zone example.com

# Using default zone from config or environment variable
cache-kv-purger cache purge everything

# With verbose output
cache-kv-purger cache purge everything --zone example.com --verbose

# Purge multiple zones at once
cache-kv-purger cache purge everything --zones example.com --zones example.org
cache-kv-purger cache purge everything --zone-list "example.com,example.org,example.net"
```

### Purge Files

Purges specific files from the cache by URL.

```bash
# Purge a single file
cache-kv-purger cache purge files --zone example.com --file https://example.com/css/styles.css

# Purge multiple files using individual flags
cache-kv-purger cache purge files --zone example.com \
  --file https://example.com/css/styles.css \
  --file https://example.com/js/app.js \
  --file https://example.com/images/logo.png

# Purge multiple files using comma-delimited list
cache-kv-purger cache purge files --zone example.com \
  --files "https://example.com/css/styles.css,https://example.com/js/app.js,https://example.com/images/logo.png"

# Purge files from a text file (one URL per line)
cache-kv-purger cache purge files --zone example.com --files-list urls.txt

# Auto-zone detection (happens automatically when no zone is specified)
cache-kv-purger cache purge files \
  --file https://example.com/css/styles.css \
  --file https://example2.com/js/app.js

# Using zone ID with verbose output
cache-kv-purger cache purge files --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --file https://example.com/css/styles.css \
  --verbose
```

### Purge Cache Tags

Purges content associated with specific cache tags.

```bash
# Purge a single tag
cache-kv-purger cache purge tags --zone example.com --tag product-listing

# Purge multiple tags
cache-kv-purger cache purge tags --zone example.com \
  --tag product-listing \
  --tag blog-posts \
  --tag user-profile

# Using zone ID with verbose output
cache-kv-purger cache purge tags --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --tag product-listing \
  --verbose
```

### Purge Cache Tags in Batches

Cloudflare limits tag purging to 30 tags per API call. This command automatically handles batch processing for larger tag sets.

```bash
# Purge a large number of tags using comma-delimited list (automatically batched in groups of 30)
cache-kv-purger cache purge tags-batch --zone example.com \
  --tags "product-tag-1,product-tag-2,product-tag-3,product-tag-4,product-tag-5"

# You can also use multiple --tag flags if preferred
cache-kv-purger cache purge tags-batch --zone example.com \
  --tag product-tag-1 \
  --tag product-tag-2 \
  --tag product-tag-3

# Purge the same set of tags across multiple zones
cache-kv-purger cache purge tags-batch --zones example.com --zones example.org \
  --tag product-tag-1 \
  --tag product-tag-2 \
  --tag product-tag-3

# With verbose output for detailed progress
cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.txt --verbose
```

#### Multiple File Formats for Tags

The `tags-batch` command supports several file formats for providing tag lists:

```bash
# Text file format (one tag per line)
cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.txt

# CSV file format
cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.csv --csv-column "tag_name"

# JSON file format (array of strings)
cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.json

# JSON file format (array of objects)
cache-kv-purger cache purge tags-batch --zone example.com --tags-file tags.json --json-field "name"
```

File format examples:

**Text file (tags.txt):**
```
product-tag-1
product-tag-2
product-tag-3
```

**CSV file (tags.csv):**
```csv
tag_name,description,created_at
product-tag-1,Description 1,2023-01-01
product-tag-2,Description 2,2023-01-02
product-tag-3,Description 3,2023-01-03
```

**JSON file - array of strings (tags.json):**
```json
[
  "product-tag-1",
  "product-tag-2",
  "product-tag-3"
]
```

**JSON file - array of objects (tags.json):**
```json
[
  {"name": "product-tag-1", "description": "Description 1"},
  {"name": "product-tag-2", "description": "Description 2"},
  {"name": "product-tag-3", "description": "Description 3"}
]
```

### Purge Hosts

Purges content from specific hosts within a zone.

```bash
# Purge a single host
cache-kv-purger cache purge hosts --zone example.com --host images.example.com

# Purge multiple hosts using individual flags
cache-kv-purger cache purge hosts --zone example.com \
  --host images.example.com \
  --host api.example.com \
  --host blog.example.com

# Purge multiple hosts using comma-delimited list
cache-kv-purger cache purge hosts --zone example.com \
  --hosts "images.example.com,api.example.com,blog.example.com"

# Purge hosts from a text file (one host per line)
cache-kv-purger cache purge hosts --zone example.com --hosts-file hosts.txt

# Auto-zone detection (happens automatically when no zone is specified)
cache-kv-purger cache purge hosts \
  --host images.example.com \
  --host api.example2.com

# Using zone ID with verbose output
cache-kv-purger cache purge hosts --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --host images.example.com \
  --verbose
  
# Control concurrency settings
cache-kv-purger cache purge hosts --zone example.com \
  --hosts "images.example.com,api.example.com,blog.example.com" \
  --concurrency 15 --zone-concurrency 5
```

### Purge Prefixes

Purges content with specific URL prefixes.

```bash
# Purge a single prefix
cache-kv-purger cache purge prefixes --zone example.com --prefix /blog/

# Purge multiple prefixes using individual flags
cache-kv-purger cache purge prefixes --zone example.com \
  --prefix /blog/ \
  --prefix /products/ \
  --prefix /api/v1/

# Purge multiple prefixes using comma-delimited list
cache-kv-purger cache purge prefixes --zone example.com \
  --prefixes "/blog/,/products/,/api/v1/"

# Purge prefixes from a text file (one prefix per line)
cache-kv-purger cache purge prefixes --zone example.com --prefixes-file prefixes.txt

# Auto-zone detection with full URLs (happens automatically when no zone is specified)
cache-kv-purger cache purge prefixes \
  --prefix https://example.com/blog/ \
  --prefix https://example2.com/assets/

# Using zone ID with verbose output
cache-kv-purger cache purge prefixes --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --prefix /blog/ \
  --verbose
```

### Purge Files With Headers

Purges specific files from the cache with custom request headers to target specific cache variants.

```bash
# Purge a specific URL with a single header
cache-kv-purger cache purge files-with-headers --zone example.com \
  --file https://example.com/image.jpg \
  --header "Accept-Language:en-US"

# Purge a specific URL with multiple headers
cache-kv-purger cache purge files-with-headers --zone example.com \
  --file https://example.com/image.jpg \
  --header "CF-IPCountry:US" \
  --header "CF-Device-Type:desktop" \
  --header "Accept-Language:en-US"

# Purge multiple URLs with the same set of headers
cache-kv-purger cache purge files-with-headers --zone example.com \
  --file https://example.com/image1.jpg \
  --file https://example.com/image2.jpg \
  --header "CF-IPCountry:US" \
  --header "CF-Device-Type:desktop"

# Using zone ID with verbose output
cache-kv-purger cache purge files-with-headers --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --file https://example.com/image.jpg \
  --header "CF-IPCountry:US" \
  --verbose
```

#### Batch Purging for Files with Headers

When purging a large number of URLs with headers, the tool automatically handles batch processing with concurrent API calls to comply with Cloudflare API limits while providing optimal performance.

```bash
# Purge a large number of URLs (automatically processed in parallel batches)
cache-kv-purger cache purge files-with-headers --zone example.com \
  --file https://example.com/image1.jpg \
  --file https://example.com/image2.jpg \
  --file https://example.com/image3.jpg \
  --file https://example.com/image4.jpg \
  ... (many more files) \
  --header "CF-IPCountry:US" \
  --header "CF-Device-Type:desktop"

# Purge using a comma-delimited list of URLs
cache-kv-purger cache purge files-with-headers --zone example.com \
  --files "https://example.com/image1.jpg,https://example.com/image2.jpg,https://example.com/image3.jpg" \
  --header "CF-IPCountry:US"

# Loading files from a text file (one URL per line)
cache-kv-purger cache purge files-with-headers --zone example.com \
  --files-list urls.txt \
  --header "CF-IPCountry:US" \
  --header "CF-Device-Type:desktop"

# Auto-zone detection (happens automatically when no zone is specified)
cache-kv-purger cache purge files-with-headers \
  --file https://example.com/image.jpg \
  --file https://example2.com/images/logo.png \
  --header "CF-IPCountry:US"

# Purge across multiple zones with concurrent processing
cache-kv-purger cache purge files-with-headers \
  --zones example.com --zones example.org \
  --file https://example.com/image1.jpg \
  --file https://example.com/image2.jpg \
  ... (many more files) \
  --header "CF-IPCountry:US" \
  --verbose
```

The concurrent processing automatically:
- Batches requests to comply with API limits (30 URLs per request)
- Processes batches in parallel (up to 10 concurrent requests)
- Processes up to 3 zones in parallel for multi-zone operations
- Auto-detects zones based on URL hostnames when no zone is specified
- Reports progress during long-running batch operations

### Purge Custom

Purges cache with custom options, allowing multiple purge types at once.

```bash
# Purge specific files and prefixes
cache-kv-purger cache purge custom --zone example.com \
  --file https://example.com/css/styles.css \
  --file https://example.com/js/app.js \
  --prefix /blog/ \
  --prefix /products/

# Purge everything along with specific hosts
cache-kv-purger cache purge custom --zone example.com \
  --everything \
  --host images.example.com

# Complex example with multiple options
cache-kv-purger cache purge custom --zone example.com \
  --file https://example.com/css/styles.css \
  --tag product-listing \
  --host images.example.com \
  --prefix /blog/ \
  --verbose
```

## KV Commands Overview

The tool uses a verb-based command structure for KV operations that follows intuitive naming patterns. This provides a simplified, more discoverable interface for managing KV namespaces and key-value pairs.

> **Note**: The legacy KV command structure (namespace, values, etc.) has been completely removed from the codebase.

For comprehensive documentation, see the [KV Documentation](KV_DOCUMENTATION.md).

### Command Structure

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

### Key Features

- **Unified verb-based commands** for intuitive operation
- **Powerful search capabilities** including deep recursive metadata search
- **Namespace resolution by name** - use names or IDs interchangeably
- **Combined single and bulk operations** within the same commands
- **Consistent flag patterns** across all commands
- **Advanced filtering options** for efficient operations

### Example Usage

List, search, and filter operations:
```bash
# List all namespaces
cache-kv-purger kv list --account-id YOUR_ACCOUNT_ID

# List keys in a namespace (by name or ID)
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID
cache-kv-purger kv list --namespace "My Namespace"

# Deep recursive metadata search
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-image"

# Filter by metadata field and value
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --tag-field "status" --tag-value "archived"
```

Get operations:
```bash
# Get a single key with metadata
cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --key mykey --metadata

# Bulk get with pattern matching
cache-kv-purger kv get --namespace-id YOUR_NAMESPACE_ID --bulk --prefix "config-" --metadata
```

Write operations:
```bash
# Write a single value
cache-kv-purger kv put --namespace-id YOUR_NAMESPACE_ID --key mykey --value "My value"

# Write from file with expiration
cache-kv-purger kv put --namespace "My Namespace" --key config.json --file ./config.json --expiration-ttl 3600
```

Delete operations:
```bash
# Delete a single key
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --key mykey

# Delete namespace
cache-kv-purger kv delete --namespace "My Namespace" --namespace-itself

# Bulk delete with search (dry run first)
cache-kv-purger kv delete --namespace-id YOUR_NAMESPACE_ID --bulk --search "old-data" --dry-run
```

Namespace operations:
```bash
# Create namespace
cache-kv-purger kv create --title "My New Namespace"

# Rename namespace
cache-kv-purger kv rename --namespace "Old Name" --title "New Name"
```

### Deep Search Capabilities

Both the `list` and `delete` commands now feature advanced recursive metadata search:

```bash
# Search recursively through complex nested metadata structures
cache-kv-purger kv list --namespace-id YOUR_NAMESPACE_ID --search "product-tag" --metadata
```

This searches through:
- All levels of nested objects and arrays
- All value types (strings, numbers, booleans)
- Case-insensitive matching for better results

### Tips for KV Operations

1. Use `--namespace` (name) instead of `--namespace-id` for better readability
2. Always use `--dry-run` before bulk deletion operations
3. For large operations, tune `--batch-size` and `--concurrency` 
4. Use `--metadata` with search operations to see matching structures
5. When json formatting is needed, use the `--json` flag

For detailed command options and more examples, see the [KV Command Guide](KV_COMMAND_GUIDE.md).

### Bulk Operations

The tool supports efficient bulk operations for managing large volumes of key-value pairs.

#### Bulk Write

Writes multiple key-value pairs in a single operation or in optimized batches.

```bash
# Write multiple key-value pairs from a JSON file
cache-kv-purger kv bulk-write --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --file data.json

# Write with custom batch size and concurrency
cache-kv-purger kv bulk-write --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --file data.json --batch-size 100 --concurrency 20

# With metadata for all keys in the batch
cache-kv-purger kv bulk-write --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --file data.json --metadata '{"source":"import-job", "timestamp":"2023-06-01"}'

# With expiration TTL for all keys in the batch
cache-kv-purger kv bulk-write --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --file data.json --expiration-ttl 86400
```

Format of the JSON file for bulk writes:
```json
[
  {
    "key": "product-1",
    "value": "Product 1 data",
    "metadata": {
      "cache-tag": "products",
      "category": "electronics"
    },
    "expiration": 1735689600
  },
  {
    "key": "product-2",
    "value": "Product 2 data",
    "metadata": {
      "cache-tag": "products",
      "category": "clothing"
    },
    "expiration_ttl": 86400
  }
]
```

#### Bulk Delete

Deletes multiple keys in optimized batches.

```bash
# Delete keys from a text file (one key per line)
cache-kv-purger kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --keys-file keys-to-delete.txt

# Delete keys matching a prefix
cache-kv-purger kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --prefix "temp-"

# Delete with custom batch size and concurrency
cache-kv-purger kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --keys-file keys-to-delete.txt --batch-size 1000 --concurrency 10

# Delete keys by metadata tag
cache-kv-purger kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --tag-field "cache-tag" --tag-value "products-old"

# Delete keys with deep recursive metadata search
cache-kv-purger kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --search "product-old" --dry-run

# Dry run (show what would be deleted without actually deleting)
cache-kv-purger kv delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --bulk --prefix "temp-" --dry-run
```

#### Export and Import

These commands help with backing up and restoring KV data across environments.

```bash
# Export entire namespace with metadata to a file
cache-kv-purger kv export --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --output namespace-backup.json

# Export with filtering
cache-kv-purger kv export --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --prefix "config-" --output config-backup.json

# Import from backup file
cache-kv-purger kv import --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --file namespace-backup.json

# Import with custom concurrency
cache-kv-purger kv import --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --file namespace-backup.json --concurrency 20
```

## Sync Operations

The sync commands provide functionality that spans across multiple Cloudflare APIs, enabling more efficient workflows and synchronized operations.

### Sync Purge 

Synchronize KV and cache by purging both KV keys and cache tags in a single operation. This powerful command allows you to:

1. Find KV keys using smart metadata search or specific tag filtering
2. Delete matching KV keys
3. Purge cache tags for associated content
4. All in a single command with batch processing and dry-run support

```bash
# Purge KV keys with a specific search value and related cache tags 
cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images

# Use metadata field-specific search and purge cache
cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --tag-field "type" --tag-value "temp" --zone example.com --cache-tag temp-data
  
# Dry run to preview without making changes
cache-kv-purger sync purge --namespace-id YOUR_NAMESPACE_ID --search "product-123" --zone example.com --cache-tag product-images --dry-run

# Using namespace name instead of ID for better readability
cache-kv-purger sync purge --namespace "My KV Namespace" --search "product-123" --zone example.com --cache-tag product-images

# Control performance parameters for large operations
cache-kv-purger sync purge \
  --namespace-id YOUR_NAMESPACE_ID \
  --search "product-123" \
  --zone example.com \
  --cache-tag product-images \
  --concurrency 20 \
  --batch-size 200 \
  --verbose
```

## Zone Commands

### List Zones

Lists all zones in an account.

```bash
# Basic usage
cache-kv-purger zones list

# Specify account ID
cache-kv-purger zones list --account-id 01a7362d577a6c3019a474fd6f485823

# With verbose output
cache-kv-purger zones list --verbose
```

### Get Zone

Gets details for a specific zone.

```bash
# Using domain name
cache-kv-purger zones get example.com

# Using account ID for lookup
cache-kv-purger zones get example.com --account-id 01a7362d577a6c3019a474fd6f485823

# With verbose output
cache-kv-purger zones get example.com --verbose
```

### Configure Zone

Sets a default zone for cache operations.

```bash
# Set default zone by domain name
cache-kv-purger zones config --zone example.com

# Set default zone by ID
cache-kv-purger zones config --zone-id 01a7362d577a6c3019a474fd6f485823

# Show current zone configuration
cache-kv-purger zones config
```

## Advanced Features

The tool includes several advanced features for optimizing performance and handling large-scale operations:

### Performance Optimization

#### Batch Processing
- **KV Write Operations**: Optimizes performance with concurrent batch operations
  - Default batch size: 100 items per batch
  - Maximum batch size: 10,000 items per API call
  - Default concurrency: 10 parallel requests
  - Maximum recommended concurrency: 50 parallel requests

- **Cache Purge Operations**: For efficient purging of large sets
  - Default batch size for tags: 30 tags per request (Cloudflare limit)
  - Default batch size for URLs: 30 URLs per request
  - Default concurrency: 10 parallel requests (configurable via `--concurrency` flag or `CLOUDFLARE_CACHE_CONCURRENCY`)
  - Maximum concurrency: 20 parallel requests
  - Uses parallelism to maximize throughput while respecting API limits

#### Cross-Zone Operations
- **Multi-Zone Purging**: For purging the same content across multiple zones
  - Default zone concurrency: 3 zones processed in parallel (configurable via `--zone-concurrency` flag or `CLOUDFLARE_MULTI_ZONE_CONCURRENCY`)
  - Automatically distributes files to appropriate zones based on hostname
  - Optimizes API calls to minimize rate limit impacts

#### Combined API Operations
- **Cross-API Operations**: For operations that span multiple Cloudflare APIs
  - Purge both KV keys and cache tags in a single operation
  - Smart KV search capabilities with cache tag purging
  - Consistent interface for multi-API operations
  - Preview mode with dry-run flag

#### Metadata-Only Operations
- **Efficient KV Operations**: The tool can perform metadata-only operations that avoid retrieving values
  - Faster performance for purge-by-tag and metadata filtering operations
  - Supports upfront metadata loading for large namespaces to reduce API calls

#### Advanced Architecture
- **Command Builder Pattern**: Fluent interface for creating commands
  ```go
  cmdutil.NewCommand("action", "Short desc", "Long desc")
    .WithRunE(handler)
    .WithStringFlag("flag-name", "default", "description", &variable)
    .WithBoolFlag("dry-run", false, "Run without making changes", &dryRun)
    .Build()
  ```
- **Middleware Pattern**: Pre-processing for common operations
  ```go
  // Automatically loads config and creates client
  cmdutil.WithConfigAndClient(func(cmd *cobra.Command, args []string, 
                                  cfg *config.Config, client *api.Client) error {
    // Command implementation with config and client ready to use
  })
  ```
- **Batch Processing**: Configurable processing for large operations
  ```go
  processor := common.NewBatchProcessor()
    .WithBatchSize(batchSize)
    .WithConcurrency(concurrency)
    .WithProgressCallback(progressFunc)
  
  successful, errors := processor.ProcessStrings(items, processFunc)
  ```

### API Behavior and Limitations

#### Rate Limit Handling
- The tool automatically handles Cloudflare API rate limits through:
  - Smart batching of large operations
  - Concurrent request throttling
  - Error handling with informative messages

#### Cloudflare API Limits
- Cache tag purging: Maximum 30 tags per API call
- KV bulk operations: Maximum 10,000 items per API call
- KV namespace listing: Paginated in sets of 100 namespaces
- Cache purging: Rate limited to approximately 1,000 purges per hour

#### Partial Success Handling
For batch operations that partially succeed, the tool reports:
- Number of successful items
- Errors for failed batches
- With `--verbose`, detailed information about each batch

### Troubleshooting

#### Common Issues
- **Rate Limit Exceeded**: Reduce concurrency or batch size, or add delays between operations
- **Authentication Failures**: Verify your API token has the correct permissions (see [Required API Token Permissions](#required-api-token-permissions))
- **Zone Not Found**: Ensure the zone ID or domain name is correct and belongs to your account
- **Permission Denied**: Different operations require different permissions. If you receive a "Permission Denied" error, check that your token has all the required permissions for that specific operation

#### Improving Performance
- For very large KV namespaces (>100,000 keys), use pagination options
- For bulk uploads of >1 million items, consider splitting into multiple operations
- When purging thousands of cache items, use the batch operations with appropriate batch sizes

#### Error Handling and Recovery

The tool implements sophisticated error handling for large operations:

- **Partial Success**: For batch operations, the tool reports how many items succeeded before failure
- **Resumable Operations**: Failed batch operations can be resumed by skipping already processed items
- **Rate Limit Recovery**: The tool automatically implements backoff strategies when rate limits are hit
- **Detailed Error Messages**: With `--verbose`, you get specific error details for troubleshooting

**Example of Resuming a Failed Bulk Operation**:
```bash
# Original command that processed 5000 items before hitting an error
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --keys-file all-keys.txt --verbose

# Resume by skipping the first 5000 items
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --keys-file all-keys.txt --skip 5000 --verbose
```

## Future Enhancements

### Planned Features

The codebase contains supporting functions for several advanced operations that may be exposed as additional commands in future versions:

1. **Bulk uploads**:
   - Additional export/import formats
   - Enhanced concurrency models

2. **Cache tag management**:
   - Tag-based purging with advanced filtering
   - Custom scheduling for recurring purges

3. **Metadata operations**:
   - Advanced metadata search and filtering
   - Bulk metadata updates without value changes

4. **Data management**:
   - Improved export functionality with metadata preservation
   - Search operations with regex pattern matching
   - Cross-namespace operations

### Architectural Enhancements

The codebase has undergone significant architectural improvements that provide a foundation for further enhancements:

1. **Command Organization and Management**:
   - Centralized command initialization via `commands.go`
   - Consistent command registration order
   - Proper initialization of root command and global flags
   - Structured command setup functions

2. **Command Builder Pattern**:
   - Fluent interface for creating and configuring commands
   - Consistent flag definition and command setup
   - Enhanced readability and maintainability
   - Simplified command creation process
   ```go
   cmdutil.NewCommand("command-name", "Short desc", "Long desc")
     .WithStringFlag("flag-name", "default", "description", &variable)
     .WithRunE(handler)
     .Build()
   ```

3. **Middleware Pattern**:
   - Common pre-processing for command handlers
   - Automatic config loading and API client initialization
   - Consistent error handling patterns
   - Dependency injection for command implementations
   ```go
   cmdutil.WithConfigAndClient(func(cmd *cobra.Command, args []string, 
                                 cfg *config.Config, client *api.Client) error {
     // Command implementation with config and client ready to use
   })
   ```

4. **Common Utilities**:
   - Standardized validation logic (`common/validation.go`)
   - Consistent user interaction patterns (`common/interaction.go`)
   - Optimized batch processing (`common/batch.go`)
   - Common error handling and formatting (`common/errors.go`)
   - File and URL utilities (`common/cache.go`)

5. **Batch Processing Improvements**:
   - Configurable batch sizes and concurrency
   - Fluent configuration interface
   - Progress reporting callbacks
   - Error handling and aggregation
   ```go
   processor := common.NewBatchProcessor()
     .WithBatchSize(100)
     .WithConcurrency(10)
     .WithProgressCallback(progressFunc)
   
   successful, errors := processor.ProcessStrings(items, processFunc)
   ```

6. **Additional Planned Enhancements**:
   - Complete command migration to builder pattern
   - Enhanced batch processing with full concurrency support
   - Improved testing infrastructure
   - Developer tools for command generation
   - Documentation generation from code comments

## Development

### Local Setup

```bash
# Clone the repository
git clone https://github.com/erfianugrah/cache-kv-purger.git
cd cache-kv-purger

# Install dependencies
go mod download

# Run tests
go test ./...

# Build the binary
go build -o cache-kv-purger ./cmd/cache-kv-purger
```

### Adding New Commands

The tool now includes a command builder pattern that makes it easy to add new commands:

```go
// 1. Define flag variables
var commandFlags struct {
    stringFlag  string
    boolFlag    bool
    intFlag     int
    stringSlice []string
}

// 2. Create the command with the builder
func createNewCommand() *cmdutil.CommandBuilder {
    return cmdutil.NewCommand(
        "command-name",
        "Short description",
        "Long description of the command.",
    ).WithExample(`  # Example of using the command
  cache-kv-purger parent-command command-name --flag value
    
  # Another example
  cache-kv-purger parent-command command-name --other-flag`,
    ).WithRunE(
        // 3. Use middleware for common pre-processing
        cmdutil.WithConfigAndClient(commandImplementation),
    ).WithStringFlag(
        "string-flag", "default", "Description of the flag", &commandFlags.stringFlag,
    ).WithBoolFlag(
        "bool-flag", false, "Description of the boolean flag", &commandFlags.boolFlag,
    )
}

// 4. Implement the command function with dependencies injected
func commandImplementation(cmd *cobra.Command, args []string, cfg *config.Config, client *api.Client) error {
    // Implementation here
    
    // Use common validation
    accountID, err := common.ValidateAccountID(cmd, cfg)
    if err != nil {
        return err
    }
    
    // Use batch processing for large operations
    processor := common.NewBatchProcessor().
        WithBatchSize(100).
        WithProgressCallback(func(completed, total, successful int) {
            fmt.Printf("Progress: %d/%d\n", completed, total)
        })
    
    // Process items
    return nil
}
```

### Code Organization

The codebase is organized into several packages with clear separation of concerns:

#### Command Structure

- **`cmd/cache-kv-purger/`**: Contains the CLI commands and entry points
  - Command files follow the naming pattern `*_cmd.go`
  - Utility functions are being migrated to appropriate internal packages

#### Internal Packages

- **`internal/api/`**: Cloudflare API client and request handling
- **`internal/auth/`**: Authentication mechanisms
- **`internal/cache/`**: Cache-specific operations
- **`internal/cmdutil/`**: Command creation utilities and middleware
- **`internal/common/`**: Shared utilities and helpers
  - `batch.go`: Batch processing functions
  - `cache.go`: Cache-related utilities
  - `errors.go`: Error handling
  - `validation.go`: Input validation
- **`internal/config/`**: Configuration management
- **`internal/kv/`**: KV operations implementation
- **`internal/zones/`**: Zone management utilities

#### Recent Improvements

The codebase has undergone organization improvements to:
- Reduce code duplication by moving utility functions to appropriate packages
- Create consistent naming patterns for files and functions
- Improve separation of concerns
- Standardize on the command builder pattern
- Add proper documentation for future enhancements (see `references/internal/PLANNED_FEATURES.md`)

### Code Quality

To ensure code quality during development:

```bash
# Install golangci-lint (if not already installed)
# Visit https://golangci-lint.run/usage/install/ for more installation options
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2

# Run the linter
golangci-lint run ./...

# Run the linter with specific configuration
golangci-lint run --config .golangci.yml ./...

# Fix auto-fixable issues
golangci-lint run --fix ./...
```

The linter will check for:
- Code style issues
- Potential bugs
- Performance issues
- Unnecessary code
- And many other quality concerns

### CI/CD Workflows

The project uses GitHub Actions for continuous integration and deployment:

#### Build and Test Workflow

This workflow runs on every push to the main branch and on pull requests:
- Builds the application on Ubuntu, macOS, and Windows
- Runs tests on Ubuntu
- Performs linting checks

The workflow is defined in `.github/workflows/build.yml`.

#### Release Workflow

This workflow is triggered when a new tag is pushed:
- Automatically builds binaries for multiple platforms
- Creates a GitHub release with the binaries
- Uses GoReleaser for release management

```bash
# To trigger the release workflow
git tag v1.0.0
git push origin v1.0.0
```

The workflow is defined in `.github/workflows/release.yml`.

## Documentation & References

- **[KV Documentation](KV_DOCUMENTATION.md)** - Comprehensive guide to the KV commands
- **[LICENSE](LICENSE)** - MIT License details
- **Additional Reference Materials** - Located in the `references/` directory:
  - `references/internal/PLANNED_FEATURES.md` - Future enhancements and features roadmap
  - `references/internal/CLEANUP_PLAN.md` - Code organization and cleanup plan
  - `references/internal/CLEANUP_SUMMARY.md` - Summary of completed cleanup work
  - `references/internal/IMPROVEMENTS.md` - General improvement proposals
  - `references/internal/UX_IMPROVEMENTS.md` - User experience improvement proposals

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
