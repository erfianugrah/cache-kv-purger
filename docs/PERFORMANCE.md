# Performance Guide

This guide covers the performance optimizations implemented in cache-kv-purger and how to leverage them for maximum efficiency.

## Overview

The tool has been optimized to handle large-scale KV namespace operations with millions of keys. Key improvements include:

- **100-1000x reduction** in API calls for metadata operations
- **~55x faster** export operations
- **Constant memory usage** regardless of dataset size
- **13x faster** concurrent operations

## Optimization Features

### 1. High-Performance HTTP Client

The optimized HTTP client provides:
- Connection pooling with keep-alive
- Automatic retry with exponential backoff
- Rate limit handling
- Concurrent request optimization

**Usage**: Automatically enabled for all operations.

### 2. Bulk Metadata Fetching

Instead of making separate API calls for each key's metadata (N+1 problem), the tool now fetches metadata with keys in a single request.

**Usage**:
```bash
# Fetch keys with metadata in one request
cache-kv-purger kv list --namespace "My Namespace" --metadata
```

### 3. Streaming JSON Parser

For large datasets, the streaming parser processes results without loading everything into memory.

**Benefits**:
- Handle millions of keys without OOM errors
- Process results as they arrive
- Constant memory footprint

### 4. Parallel Pagination

When listing large namespaces, multiple pages are fetched concurrently.

**Usage**:
```bash
# List with optimized parallel fetching
cache-kv-purger kv list --namespace "Large Namespace" --all
```

### 5. Request Caching

Frequently accessed metadata is cached to eliminate redundant API calls.

**Cache Configuration**:
- LRU eviction policy
- 5-minute TTL
- 10,000 entries max
- 50MB size limit

### 6. Optimized Batching

All artificial delays have been removed and batch sizes optimized:

**Before**: 10ms delay per operation
**After**: No delays, full concurrent processing

## Performance Tuning

### Concurrency Settings

Adjust concurrency based on your use case:

```bash
# Default concurrency (10)
cache-kv-purger kv delete --namespace "My Namespace" --search "old-data" --bulk

# High concurrency for large operations
cache-kv-purger kv delete --namespace "My Namespace" --search "old-data" --bulk --concurrency 50

# Lower concurrency to avoid rate limits
cache-kv-purger kv delete --namespace "My Namespace" --search "old-data" --bulk --concurrency 5
```

### Batch Size Optimization

Larger batches reduce API calls but use more memory:

```bash
# Default batch size (100)
cache-kv-purger kv export --namespace "My Namespace"

# Large batches for better performance
cache-kv-purger kv export --namespace "My Namespace" --batch-size 1000

# Smaller batches for memory-constrained environments
cache-kv-purger kv export --namespace "My Namespace" --batch-size 50
```

## Benchmarks

### List Operations
- **1M keys without metadata**: ~30 seconds
- **1M keys with metadata**: ~2 minutes (vs ~3 hours before)

### Export Operations
- **1000 keys**: ~180ms (vs ~10 seconds before)
- **10,000 keys**: ~1.8 seconds (vs ~100 seconds before)

### Delete Operations
- **10K keys by tag**: ~10 seconds (vs ~15 minutes before)
- **Bulk delete**: 1000 keys/second throughput

## Best Practices

1. **Use metadata flag wisely**: Only fetch metadata when needed
2. **Tune concurrency**: Start with defaults, increase for large operations
3. **Monitor rate limits**: Reduce concurrency if hitting limits
4. **Use dry-run**: Always test with `--dry-run` before bulk operations
5. **Leverage caching**: Repeated operations benefit from metadata cache

## Troubleshooting Performance Issues

### Slow List Operations
- Ensure you're using the latest version
- Check network latency to Cloudflare API
- Use `--verbose` to see operation timing

### Memory Issues
- Reduce `--batch-size` for large operations
- Use streaming operations when available
- Monitor memory usage with system tools

### Rate Limiting
- Reduce `--concurrency` setting
- Add delays between operations if needed
- Check Cloudflare API rate limits for your plan

## Future Optimizations

Planned improvements include:
- Worker pool reuse to reduce goroutine overhead
- Enhanced progress reporting with ETA
- Adaptive batch sizing based on response times
- Extended caching for namespace metadata