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
  - Bulk operations support
  - Export and import functionality
  - Search keys and values using regex patterns

## Installation

### From Source

```bash
git clone https://github.com/erfianugrah/cache-kv-purger.git
cd cache-kv-purger
go build -o cache-kv-purger .cmd/cache-kv-purger
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

### KV Operations

```bash
# List KV namespaces
./cache-kv-purger kv namespace list

# Create a new namespace
./cache-kv-purger kv namespace create --title "My Namespace"

# List keys in a namespace
./cache-kv-purger kv values list --namespace-id your_namespace_id

# Get a value
./cache-kv-purger kv values get --namespace-id your_namespace_id --key your_key

# Write a value
./cache-kv-purger kv values put --namespace-id your_namespace_id --key your_key --value your_value

# Delete a value
./cache-kv-purger kv values delete --namespace-id your_namespace_id --key your_key

# Export a namespace
./cache-kv-purger kv export --namespace-id your_namespace_id --file export.json

# Search keys and values
./cache-kv-purger kv search --namespace-id your_namespace_id --key-pattern "user_.*" --value-pattern "cache-tag"
```

## Advanced Features

### Bulk Operations

```bash
# Bulk upload data
./cache-kv-purger kv bulk-batch --namespace-id your_namespace_id --file data.json

# Purge by cache tag
./cache-kv-purger kv purge-by-tag --namespace-id your_namespace_id --tag-value homepage
```

### Working with Multiple Zones

```bash
# Purge cache in multiple zones simultaneously
./cache-kv-purger cache purge everything --zones example.com --zones example.org --zones example.net
```

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

## License

This project is licensed under the MIT License - see the LICENSE file for details.
