# KV Sample Data

This directory contains sample data for testing the Cloudflare Cache KV Purger tool.

## Sample KV Data

The `sample_kv_data.json` file contains 2,000 sample key-value pairs that can be used to test bulk operations with the KV purger. Each entry has the following characteristics:

- Each key is uniquely identified
- Each value is a large JSON object with multiple nested fields
- Every value contains a `cache-tag` field which is one of four values:
  - `homepage` (500 items)
  - `product` (500 items)
  - `blog` (500 items)
  - `user` (500 items)

This data is perfect for testing:
- Bulk uploads to KV namespaces
- Searching keys and values with regex patterns
- Cache tag purging operations

## Generating New Sample Data

To regenerate the sample data or create a new dataset with different characteristics:

```bash
go run examples/generate_sample_data.go
```

## Usage Examples

### Uploading the sample data to a KV namespace

```bash
./cache-kv-purger kv bulk-batch --namespace-id your_namespace_id --file examples/sample_kv_data.json
```

### Purging by cache-tag

```bash
# Purge all items with "homepage" cache-tag
./cache-kv-purger kv purge-by-tag --namespace-id your_namespace_id --tag-value homepage

# Check first what would be deleted without actually deleting
./cache-kv-purger kv purge-by-tag --namespace-id your_namespace_id --tag-value blog --dry-run
```

### Searching for keys with specific cache-tag

```bash
./cache-kv-purger kv search --namespace-id your_namespace_id --value-pattern '"cache-tag":\s*"product"'
```