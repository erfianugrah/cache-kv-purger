# Cloudflare Cache KV Purger

[![Build and Test](https://github.com/erfianugrah/cache-kv-purger/actions/workflows/build.yml/badge.svg)](https://github.com/erfianugrah/cache-kv-purger/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/erfianugrah/cache-kv-purger)](https://goreportcard.com/report/github.com/erfianugrah/cache-kv-purger)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A command-line interface tool for managing Cloudflare cache purging and KV store operations.

## Features

- **Cache Management**
  - Purge entire cache
  - Purge specific files
  - Purge by cache tags, hosts, or prefixes
  - Multi-zone purging support

- **KV Store Management**
  - Create, list, rename, and delete namespaces
  - Read, write, and delete key-value pairs
  - High-performance bulk operations
  - Optimized metadata and cache-tag purging
  - Export and import functionality
  - Search keys and values using regex patterns

- **Performance Optimizations**
  - Concurrent batch processing for uploads
  - Streaming operations for large datasets
  - Metadata-optimized purging algorithms
  - Configurable concurrency and batch sizes
  - Memory-efficient processing for large namespaces
  - Comprehensive progress reporting with `--verbose` flag

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/erfianugrah/cache-kv-purger.git
cd cache-kv-purger

# Build the binary
go build -o cache-kv-purger cmd/cache-kv-purger/main.go cmd/cache-kv-purger/cache_cmd.go cmd/cache-kv-purger/config_cmd.go cmd/cache-kv-purger/kv_cmd.go cmd/cache-kv-purger/zones_cmd.go
```

### Binary Releases

Download the latest release for your platform from the [Releases page](https://github.com/erfianugrah/cache-kv-purger/releases).

## Configuration

### API Token

You'll need a Cloudflare API token with the appropriate permissions:

```bash
# Set environment variable
export CF_API_TOKEN=your_cloudflare_api_token

# Or configure using the tool
./cache-kv-purger config set --token your_cloudflare_api_token
```

### Account ID and Zone ID

For KV operations, you'll need your Cloudflare Account ID:

```bash
# Set environment variable
export CLOUDFLARE_ACCOUNT_ID=your_account_id

# Or configure using the tool
./cache-kv-purger kv config --account-id your_account_id
```

For cache operations, you'll need your Zone ID or domain:

```bash
# Use with commands
./cache-kv-purger cache purge --zone example.com ...

# Or set default zone
export CLOUDFLARE_ZONE_ID=your_zone_id
```

## Usage Examples

### Cache Operations

```bash
# Purge everything in a zone
./cache-kv-purger cache purge everything --zone example.com

# Purge specific files
./cache-kv-purger cache purge files --zone example.com --file https://example.com/path/to/file.jpg --file https://example.com/path/to/another.css

# Purge by cache tags
./cache-kv-purger cache purge tags --zone example.com --tag product-listing --tag user-profile

# Purge by hosts
./cache-kv-purger cache purge hosts --zone example.com --host subdomain.example.com

# Purge by URL prefixes
./cache-kv-purger cache purge prefixes --zone example.com --prefix /blog/ --prefix /products/
```

### KV Namespace Operations

```bash
# List all KV namespaces in your account
./cache-kv-purger kv namespace list --verbose

# Create a new namespace
./cache-kv-purger kv namespace create --title "My Namespace"

# Delete a namespace
./cache-kv-purger kv namespace delete --namespace-id your_namespace_id

# Rename a namespace
./cache-kv-purger kv namespace rename --namespace-id your_namespace_id --title "New Name"

# Bulk delete multiple namespaces matching a pattern
./cache-kv-purger kv namespace bulk-delete --pattern "test-*" --dry-run
```

### KV Key-Value Operations

```bash
# List keys in a namespace (first page)
./cache-kv-purger kv values list --namespace-id your_namespace_id

# List all keys in a namespace (handles pagination)
./cache-kv-purger kv values list --namespace-id your_namespace_id --all

# Get a value
./cache-kv-purger kv values get --namespace-id your_namespace_id --key your_key

# Get a value with metadata
./cache-kv-purger kv get-with-metadata --namespace-id your_namespace_id --key your_key

# Check if a key exists
./cache-kv-purger kv exists --namespace-id your_namespace_id --key your_key

# Write a value
./cache-kv-purger kv values put --namespace-id your_namespace_id --key your_key --value your_value

# Write a value with metadata and expiration
./cache-kv-purger kv values put --namespace-id your_namespace_id --key your_key --value your_value --expiration 1735689600

# Write a value from a file
./cache-kv-purger kv values put --namespace-id your_namespace_id --key your_key --file ./data.json

# Delete a value
./cache-kv-purger kv values delete --namespace-id your_namespace_id --key your_key

# Export a namespace to a JSON file
./cache-kv-purger kv export --namespace-id your_namespace_id --file export.json --include-metadata

# Search keys and values with regex patterns
./cache-kv-purger kv search --namespace-id your_namespace_id --key-pattern "user_.*" --value-pattern "cache-tag"

# Search with different output formats
./cache-kv-purger kv search --namespace-id your_namespace_id --key-pattern "test-.*" --format json
```

## Advanced Features

### Bulk Import Operations

```bash
# Generate test data with metadata for testing
./cache-kv-purger kv generate-test-data --count 100 --output test-data.json --tag-field cache-tag --tag-values product,blog,homepage,api

# Optimized bulk upload with concurrent processing (recommended)
./cache-kv-purger kv bulk-concurrent --namespace-id your_namespace_id --file data.json --concurrency 30 --batch-size 100 --verbose

# For high-throughput environments with high API rate limits
./cache-kv-purger kv bulk-concurrent --namespace-id your_namespace_id --file data.json --concurrency 50 --batch-size 50

# Legacy bulk upload methods (not recommended, use bulk-concurrent instead)
./cache-kv-purger kv bulk-batch --namespace-id your_namespace_id --file data.json
./cache-kv-purger kv bulk-upload --namespace-id your_namespace_id --file data.json
./cache-kv-purger kv simple-upload --namespace-id your_namespace_id --file data.json
```

### Cache Tag Purging

```bash
# Optimized streaming purge by cache-tag (recommended for large namespaces)
./cache-kv-purger kv purge-by-tag-streaming --namespace-id your_namespace_id --tag-value homepage --chunk-size 200 --concurrency 20

# Preview what would be deleted with dry-run mode
./cache-kv-purger kv purge-by-tag-streaming --namespace-id your_namespace_id --tag-value homepage --dry-run --verbose

# Delete all entries with any value for a specific field
./cache-kv-purger kv purge-by-tag-streaming --namespace-id your_namespace_id --field cache-tag

# Legacy purge by cache-tag (not recommended)
./cache-kv-purger kv purge-by-tag --namespace-id your_namespace_id --tag-value homepage
```

### Metadata Operations

```bash
# Extremely fast metadata purging (recommended for most use cases)
./cache-kv-purger kv purge-by-metadata-only --namespace-id your_namespace_id --field cache-tag --value blog --verbose

# High throughput metadata purging (optimized for high API rate limits)
./cache-kv-purger kv purge-by-metadata-upfront --namespace-id your_namespace_id --field cache-tag --value blog --concurrency 500

# Preview what would be deleted without making changes
./cache-kv-purger kv purge-by-metadata-only --namespace-id your_namespace_id --field cache-tag --value blog --dry-run

# Test metadata functionality on a small set of keys
./cache-kv-purger kv test-metadata --namespace-id your_namespace_id --field cache-tag --limit 10

# Legacy metadata purging command (not recommended)
./cache-kv-purger kv purge-by-metadata --namespace-id your_namespace_id --field cache-tag --value blog
```

### Working with Multiple Zones

```bash
# Purge cache in multiple zones simultaneously
./cache-kv-purger cache purge everything --zones example.com --zones example.org --zones example.net
```

## Performance Optimizations

The tool includes several performance-optimized commands that are designed to handle large-scale Cloudflare KV operations efficiently:

### KV Bulk Operations
- **bulk-concurrent**: Uploads JSON data using concurrent batch operations, supporting high throughput with configurable concurrency and batch sizes.
- **write-multiple-values-concurrently**: Internal implementation that processes items with parallel operations, maximizing API rate limit usage.
- **Performance metrics**: Displays operations per second and execution time with `--verbose` flag.

### KV Purging Operations
- **purge-by-metadata-only**: Extremely efficient metadata-based purging that focuses only on metadata operations without loading values.
- **purge-by-metadata-upfront**: High-throughput approach that loads all metadata first and then processes in memory, designed for environments with high API rate limits.
- **purge-by-tag-streaming**: Streaming approach to find and delete entries with specific cache-tag values, optimized for working with large namespaces.

### Data Management Operations
- **export**: Efficiently exports keys, values, and metadata with parallel processing.
- **search**: Performs regex-based search on keys and values with multiple output formats (simple, table, json, keys).
- **get-with-metadata**: Retrieves both value and metadata in a single operation.

### Implementation Details
- The tool automatically uses batching, concurrency, and streaming patterns to maximize performance.
- Legacy commands like `bulk-batch`, `purge-by-tag`, and `simple-upload` now internally use the optimized implementations.
- All operations include detailed progress reporting when used with `--verbose` flag.
- Commands support `--dry-run` flags for testing operations without making changes.
- Memory-efficient streaming operations for handling large datasets.

## Development

### GitHub Workflows

This project uses GitHub Actions for continuous integration and deployment:

1. **Build and Test** - Runs on every push to `main` and on pull requests:
   - Builds the project
   - Runs all tests
   - Performs linting with golangci-lint

2. **Release** - Triggered when a new tag is pushed:
   - Automatically builds binaries for multiple platforms (Linux, Windows, macOS)
   - Creates GitHub releases with the built binaries
   - Generates checksums for verification

To create a new release:

```bash
git tag v1.0.0
git push origin v1.0.0
```

### Contributing

1. Fork the repository
2. Create your feature branch: `git checkout -b feature/amazing-feature`
3. Commit your changes: `git commit -m 'Add some amazing feature'`
4. Push to the branch: `git push origin feature/amazing-feature`
5. Open a Pull Request

### Performance Testing

When contributing performance improvements:

1. Test with small, medium, and large data sets
2. Use the `--verbose` flag to monitor progress and performance metrics
3. Use `--dry-run` when testing purge operations to avoid accidental data loss
4. Compare the performance of both legacy and optimized implementations
5. Document performance gains with metrics in your PR

## Command Reference

### KV Commands

| Command | Description | Usage |
|---------|-------------|-------|
| `namespace list` | List all KV namespaces | `kv namespace list` |
| `namespace create` | Create a new namespace | `kv namespace create --title "Name"` |
| `namespace delete` | Delete a namespace | `kv namespace delete --namespace-id ID` |
| `namespace rename` | Rename a namespace | `kv namespace rename --namespace-id ID --title "New Name"` |
| `namespace bulk-delete` | Delete multiple namespaces | `kv namespace bulk-delete --pattern "test-*"` |
| `values list` | List keys in a namespace | `kv values list --namespace-id ID [--all]` |
| `values get` | Get a value | `kv values get --namespace-id ID --key KEY` |
| `values put` | Write a value | `kv values put --namespace-id ID --key KEY --value VAL` |
| `values delete` | Delete a value | `kv values delete --namespace-id ID --key KEY` |
| `get-with-metadata` | Get value with metadata | `kv get-with-metadata --namespace-id ID --key KEY` |
| `exists` | Check if a key exists | `kv exists --namespace-id ID --key KEY` |
| `export` | Export namespace to JSON | `kv export --namespace-id ID --file FILE.json` |
| `search` | Search keys and values | `kv search --namespace-id ID --key-pattern PAT` |
| `bulk-concurrent` | Upload data in concurrent batches | `kv bulk-concurrent --namespace-id ID --file FILE.json` |
| `purge-by-metadata-only` | Purge by metadata efficiently | `kv purge-by-metadata-only --namespace-id ID --field F` |
| `purge-by-metadata-upfront` | Purge with high throughput | `kv purge-by-metadata-upfront --namespace-id ID --field F` |
| `purge-by-tag-streaming` | Purge by cache-tag | `kv purge-by-tag-streaming --namespace-id ID --tag-value V` |
| `test-metadata` | Test metadata operations | `kv test-metadata --namespace-id ID --field F` |
| `generate-test-data` | Generate test data | `kv generate-test-data --count 100 --output FILE.json` |

### Cache Commands

| Command | Description | Usage |
|---------|-------------|-------|
| `purge everything` | Purge entire cache | `cache purge everything --zone ZONE` |
| `purge files` | Purge specific files | `cache purge files --zone ZONE --file URL` |
| `purge tags` | Purge by cache tags | `cache purge tags --zone ZONE --tag TAG` |
| `purge hosts` | Purge by hosts | `cache purge hosts --zone ZONE --host HOST` |
| `purge prefixes` | Purge by URL prefixes | `cache purge prefixes --zone ZONE --prefix PREFIX` |

## License

This project is licensed under the MIT License - see the LICENSE file for details.
