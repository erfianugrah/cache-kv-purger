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
- [KV Namespace Commands](#kv-namespace-commands)
- [KV Values Commands](#kv-values-commands)
- [KV Utility Commands](#kv-utility-commands)
- [Zone Commands](#zone-commands)
- [Future Enhancements](#future-enhancements)
- [Development](#development)
- [License](#license)

## Features

- **Cache Management**
  - Purge entire cache
  - Purge specific files
  - Purge by cache tags, hosts, or prefixes
  - Multi-zone purging support
  - Batch processing for cache tag purging

- **KV Store Management**
  - Create, list, rename, and delete namespaces
  - Bulk namespace management with pattern matching
  - Read, write, and delete key-value pairs
  - Metadata and expiration support

- **Zone Management**
  - List and query zone information
  - Configure default zones
  - Zone resolution by name or ID

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
go build -o cache-kv-purger ./cmd/cache-kv-purger

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

#### Using Config Command

```bash
# Set default zone
cache-kv-purger config set-defaults --zone example.com

# Set default account ID
cache-kv-purger config set-defaults --account-id your_account_id

# Set custom API endpoint (if needed)
cache-kv-purger config set-defaults --api-endpoint https://custom-api.cloudflare.com/client/v4

# View current configuration
cache-kv-purger config show
```

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

# Purge multiple files
cache-kv-purger cache purge files --zone example.com \
  --file https://example.com/css/styles.css \
  --file https://example.com/js/app.js \
  --file https://example.com/images/logo.png

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

# Purge multiple hosts
cache-kv-purger cache purge hosts --zone example.com \
  --host images.example.com \
  --host api.example.com \
  --host blog.example.com

# Using zone ID with verbose output
cache-kv-purger cache purge hosts --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --host images.example.com \
  --verbose
```

### Purge Prefixes

Purges content with specific URL prefixes.

```bash
# Purge a single prefix
cache-kv-purger cache purge prefixes --zone example.com --prefix /blog/

# Purge multiple prefixes
cache-kv-purger cache purge prefixes --zone example.com \
  --prefix /blog/ \
  --prefix /products/ \
  --prefix /api/v1/

# Using zone ID with verbose output
cache-kv-purger cache purge prefixes --zone-id 01a7362d577a6c3019a474fd6f485823 \
  --prefix /blog/ \
  --verbose
```

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

## Future Enhancements

The codebase contains supporting functions for several advanced operations that may be exposed as commands in future versions:

1. **Bulk uploads**:
   - Parallel processing with concurrent batch operations
   - Optimal batch size and concurrency configuration

2. **Cache tag management**:
   - Tag-based purging with streaming approach
   - Efficient handling of large namespaces

3. **Metadata operations**:
   - Metadata-only purging for improved performance
   - High-throughput upfront loading

4. **Data management**:
   - Export functionality with metadata preservation
   - Search operations with regex pattern matching

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
