# Migration Plan for Race Condition Fixes

This document outlines the step-by-step approach to safely implement the race condition fixes for the KV delete operations.

## Phase 1: Integration Preparation

1. **Run Unit Tests**
   ```
   cd /home/erfi/cache-kv-purger
   go test ./internal/kv/... -v
   ```

2. **Verify New Files**
   - Ensure the following files are present:
     - `/internal/kv/purge_fix.go`
     - `/internal/kv/service_fix.go`
     - `/internal/kv/purge_fix_test.go`
     - `/internal/kv/README_RACE_FIXES.md`

## Phase 2: Service Layer Integration

1. **Update CloudflareKVService Implementation**

   Edit `/internal/kv/service.go` to replace the `bulkDeleteWithAdvancedFiltering` method with the fixed version:

   ```go
   // Replace the current implementation with:
   func (s *CloudflareKVService) bulkDeleteWithAdvancedFiltering(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
       // Call the fixed implementation
       return s.bulkDeleteWithAdvancedFilteringFixed(ctx, accountID, namespaceID, keys, options)
   }
   ```

2. **Update BulkDelete Method**

   Edit `/internal/kv/service.go` to update the `BulkDelete` method:

   ```go
   // Replace the current implementation with:
   func (s *CloudflareKVService) BulkDelete(ctx context.Context, accountID, namespaceID string, keys []string, options BulkDeleteOptions) (int, error) {
       // Call the fixed implementation
       return s.BulkDeleteFixed(ctx, accountID, namespaceID, keys, options)
   }
   ```

## Phase 3: Update Core Purge Functions

1. **Update Function References in purge.go**

   Add the following code at the end of `/internal/kv/purge.go`:

   ```go
   // The following functions are deprecated and will be removed in a future release.
   // They are kept for backward compatibility. Use the Fixed versions instead.

   // Deprecated: Use StreamingPurgeByTagFixed instead
   func StreamingPurgeByTag(client *api.Client, accountID, namespaceID, tagField, tagValue string,
       chunkSize int, concurrency int, dryRun bool,
       progressCallback func(keysFetched, keysProcessed, keysDeleted, total int)) (int, error) {
       return StreamingPurgeByTagFixed(client, accountID, namespaceID, tagField, tagValue, 
           chunkSize, concurrency, dryRun, progressCallback)
   }

   // Deprecated: Use PurgeByMetadataOnlyFixed instead
   func PurgeByMetadataOnly(client *api.Client, accountID, namespaceID, metadataField, metadataValue string,
       chunkSize int, concurrency int, dryRun bool,
       progressCallback func(keysFetched, keysProcessed, keysMatched, keysDeleted, total int)) (int, error) {
       return PurgeByMetadataOnlyFixed(client, accountID, namespaceID, metadataField, metadataValue,
           chunkSize, concurrency, dryRun, progressCallback)
   }
   ```

## Phase 4: Integration Testing

1. **Run Integration Tests**
   ```
   cd /home/erfi/cache-kv-purger
   go test ./... -v
   ```

2. **Manual Testing**
   - Test the KV delete command with various options
   - Verify progress reporting works correctly
   - Test with large datasets to ensure no race conditions occur

## Phase 5: Validation

1. **Load Testing**
   - Run concurrent delete operations with high volume
   - Monitor for any race conditions or crashes
   - Verify counters are correctly updated

2. **Performance Analysis**
   - Compare performance before and after changes
   - Ensure the synchronization doesn't introduce significant overhead

## Phase 6: Documentation and Cleanup

1. **Update Main Documentation**
   - Add a section about the race condition fixes to the main README
   - Document any API changes for users

2. **Code Review**
   - Perform a thorough code review to ensure all race conditions are fixed
   - Check for any remaining issues or edge cases

## Phase 7: Final Deployment

1. **Tag Release**
   - Create a new version tag for the fixes
   - Update CHANGELOG.md with the race condition fixes

2. **Monitor Production Use**
   - Watch for any issues in production
   - Collect feedback from users

## Rollback Plan

In case of issues, the following rollback plan can be executed:

1. **Revert Service Methods**
   - Restore the original `bulkDeleteWithAdvancedFiltering` method
   - Restore the original `BulkDelete` method

2. **Remove Function Wrappers**
   - Remove the wrappers added to `StreamingPurgeByTag` and `PurgeByMetadataOnly`

3. **Keep the Fix Files**
   - Keep the `*_fix.go` files for future reference and debugging

## Timeline

- **Phase 1-2:** Day 1
- **Phase 3-4:** Day 2
- **Phase 5-6:** Day 3
- **Phase 7:** Day 4-5

Total estimated time: 5 days