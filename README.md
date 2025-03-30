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
- [Unified KV Commands](#unified-kv-commands)
- [KV Namespace Commands](#kv-namespace-commands)
- [KV Values Commands](#kv-values-commands)
- [KV Utility Commands](#kv-utility-commands)
- [Combined API Commands](#combined-api-commands)
- [Zone Commands](#zone-commands)
- [Future Enhancements](#future-enhancements)
- [Development](#development)
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

## Global Commands

All commands support the following global flags:

- `--verbose`: Enable detailed output
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

## KV Namespace Commands

### List Namespaces

Lists all KV namespaces in an account.

```bash
# Basic usage
cache-kv-purger kv namespace list --account-id 01a7362d577a6c3019a474fd6f485823

# Using default account ID from config
cache-kv-purger kv namespace list

# With verbose output
cache-kv-purger kv namespace list --verbose
```

### Create Namespace

Creates a new KV namespace.

```bash
# Basic usage
cache-kv-purger kv namespace create --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache"

# Using default account ID from config
cache-kv-purger kv namespace create --title "My Application Cache"

# With verbose output
cache-kv-purger kv namespace create --title "My Application Cache" --verbose
```

### Delete Namespace

Deletes a KV namespace.

```bash
# Using namespace ID
cache-kv-purger kv namespace delete --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7

# Using namespace title
cache-kv-purger kv namespace delete --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache"

# Using default account ID from config
cache-kv-purger kv namespace delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7

# With verbose output
cache-kv-purger kv namespace delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --verbose
```

### Rename Namespace

Renames a KV namespace.

```bash
# Basic usage
cache-kv-purger kv namespace rename --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --title "New Name"

# Using default account ID from config
cache-kv-purger kv namespace rename --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --title "New Name"

# With verbose output
cache-kv-purger kv namespace rename --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --title "New Name" --verbose
```

### Bulk Delete Namespaces

Deletes multiple KV namespaces matching a pattern or specific IDs.

```bash
# Delete by pattern with dry-run (preview only)
cache-kv-purger kv namespace bulk-delete --account-id 01a7362d577a6c3019a474fd6f485823 --pattern "test-*" --dry-run

# Delete by pattern with confirmation
cache-kv-purger kv namespace bulk-delete --account-id 01a7362d577a6c3019a474fd6f485823 --pattern "test-*"

# Delete by pattern without confirmation
cache-kv-purger kv namespace bulk-delete --account-id 01a7362d577a6c3019a474fd6f485823 --pattern "test-*" --force

# Delete specific namespace IDs
cache-kv-purger kv namespace bulk-delete --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-ids "id1,id2,id3"

# With verbose output
cache-kv-purger kv namespace bulk-delete --account-id 01a7362d577a6c3019a474fd6f485823 --pattern "test-*" --verbose
```

## KV Values Commands

### List Keys

Lists keys in a KV namespace.

```bash
# List first page of keys
cache-kv-purger kv values list --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7

# List all keys (handle pagination automatically)
cache-kv-purger kv values list --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --all

# Using namespace title instead of ID
cache-kv-purger kv values list --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache"

# Using default account ID from config
cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --all

# With verbose output
cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --all --verbose

# Filter keys by prefix
cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --prefix "product-" --all

# List keys with pagination control
cache-kv-purger kv values list --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --page-size 1000 --page 2
```

### Get Value

Gets a value for a key.

```bash
# Basic usage
cache-kv-purger kv values get --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Using namespace title instead of ID
cache-kv-purger kv values get --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache" --key mykey

# Using default account ID from config
cache-kv-purger kv values get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# With verbose output
cache-kv-purger kv values get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --verbose

# Redirect output to file
cache-kv-purger kv values get --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey > output.json
```

### Put Value

Writes a value for a key.

```bash
# Using direct value
cache-kv-purger kv values put --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --value "My value data"

# Using file input
cache-kv-purger kv values put --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key config-json --file ./config.json

# Using namespace title instead of ID
cache-kv-purger kv values put --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache" --key mykey --value "My value data"

# With expiration time (Unix timestamp)
cache-kv-purger kv values put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key temporary-key --value "Temporary data" --expiration 1735689600

# With expiration in seconds from now (TTL)
cache-kv-purger kv values put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key temporary-key --value "Temporary data" --expiration-ttl 3600

# With custom metadata (as JSON)
cache-kv-purger kv values put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key product-123 --value '{"name":"Product 123"}' --metadata '{"cache-tag":"products", "version":"1.0"}'

# With verbose output
cache-kv-purger kv values put --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --value "My value data" --verbose
```

### Delete Value

Deletes a value for a key.

```bash
# Basic usage
cache-kv-purger kv values delete --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Using namespace title instead of ID
cache-kv-purger kv values delete --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache" --key mykey

# Using default account ID from config
cache-kv-purger kv values delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# With verbose output
cache-kv-purger kv values delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --verbose
```

## Unified KV Commands

The tool uses a consolidated verb-based KV command structure for a simpler, more intuitive interface. These commands follow a consistent pattern and support both namespace name and ID resolution. The legacy command structure is still available but marked as deprecated.

For detailed documentation, see the [KV Command Guide](KV_COMMAND_GUIDE.md).

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

### Namespace Command

Operations specific to namespaces.

```bash
# Create a new namespace
cache-kv-purger kv namespace create --account-id YOUR_ACCOUNT_ID --title "My Namespace"

# Rename a namespace
cache-kv-purger kv namespace rename --namespace-id YOUR_NAMESPACE_ID --title "New Name"

# Rename a namespace by name
cache-kv-purger kv namespace rename --namespace "Old Name" --title "New Name"
```

## KV Utility Commands

### Get Key With Metadata

Gets a key's value along with its metadata.

```bash
# Basic usage
cache-kv-purger kv get-with-metadata --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Using namespace title instead of ID
cache-kv-purger kv get-with-metadata --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache" --key mykey

# Using default account ID from config
cache-kv-purger kv get-with-metadata --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# With verbose output
cache-kv-purger kv get-with-metadata --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --verbose
```

### Search and Filter Keys

Search for keys with advanced filtering capabilities.

```bash
# Search for keys containing a value anywhere in metadata
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --value "product-image" --metadata

# Search for keys with a specific tag field
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --tag-field "tags" --tag-value "product-image" --metadata

# Search and purge keys (dry run first)
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --value "product-image" --purge --dry-run

# Search and purge keys (actual deletion)
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --value "product-image" --purge

# Output results as JSON
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --value "product-image" --json

# Control concurrency and chunk size
cache-kv-purger kv search --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --value "product-image" --concurrency 20 --chunk-size 200
```

### Check If Key Exists

Checks if a key exists in a namespace without retrieving its value.

```bash
# Basic usage
cache-kv-purger kv exists --account-id 01a7362d577a6c3019a474fd6f485823 --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# Using namespace title instead of ID
cache-kv-purger kv exists --account-id 01a7362d577a6c3019a474fd6f485823 --title "My Application Cache" --key mykey

# Using default account ID from config
cache-kv-purger kv exists --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey

# With verbose output
cache-kv-purger kv exists --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey --verbose

# Using in scripts (returns non-zero exit code if key doesn't exist)
if cache-kv-purger kv exists --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --key mykey; then
  echo "Key exists"
else
  echo "Key does not exist"
fi
```

### Configure KV Settings

Sets default account ID for KV operations.

```bash
# Set default account ID
cache-kv-purger kv config --account-id 01a7362d577a6c3019a474fd6f485823

# Show current KV configuration
cache-kv-purger kv config
```

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
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --keys-file keys-to-delete.txt

# Delete keys matching a prefix
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --prefix "temp-"

# Delete with custom batch size and concurrency
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --keys-file keys-to-delete.txt --batch-size 1000 --concurrency 10

# Delete keys by metadata tag (keys with metadata containing specific tag value)
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --metadata-field "cache-tag" --metadata-value "products-old"

# Dry run (show what would be deleted without actually deleting)
cache-kv-purger kv bulk-delete --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 --prefix "temp-" --dry-run
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

## Combined API Commands

The combined commands provide functionality that spans across multiple Cloudflare APIs, enabling more efficient workflows and unified operations.

### Purge All (KV and Cache)

Purge both KV keys and cache tags in a single operation. This powerful command allows you to:

1. Find KV keys using smart metadata search or specific tag filtering
2. Delete matching KV keys
3. Purge cache tags for associated content
4. All in a single command with batch processing and dry-run support

```bash
# Purge KV keys with a specific value and cache tags
cache-kv-purger combined purge-all \
  --account-id 01a7362d577a6c3019a474fd6f485823 \
  --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 \
  --value "product-image" \
  --zone example.com \
  --cache-tag product-images

# Purge KV keys with a specific tag field and cache
cache-kv-purger combined purge-all \
  --account-id 01a7362d577a6c3019a474fd6f485823 \
  --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 \
  --tag-field "tags" \
  --tag-value "product-image" \
  --zone example.com \
  --cache-tag product-images \
  --cache-tag image-thumbnails

# Dry run to preview changes without actually purging
cache-kv-purger combined purge-all \
  --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 \
  --value "product-image" \
  --zone example.com \
  --cache-tag product-images \
  --dry-run

# Using namespace title instead of ID
cache-kv-purger combined purge-all \
  --title "My KV Namespace" \
  --value "product-image" \
  --zone example.com \
  --cache-tag product-images

# Control performance parameters
cache-kv-purger combined purge-all \
  --namespace-id 95bc3e9324ac40fa8b71c4a3016c13c7 \
  --value "product-image" \
  --zone example.com \
  --cache-tag product-images \
  --concurrency 20 \
  --chunk-size 200 \
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

### Creating Releases

The project uses GitHub Actions for continuous integration and deployment:

```bash
# Build and test workflow runs on push to main branch and pull requests
git push origin main

# Release workflow is triggered by new tags
git tag v1.0.0
git push origin v1.0.0
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
