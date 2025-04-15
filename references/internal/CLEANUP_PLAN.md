# Code Organization and Cleanup Plan

This document outlines the plan for cleaning up and organizing the codebase structure to improve maintainability and reduce duplication.

## Current Issues

1. **Redundant/Duplicate Code**
   - Utility functions spread across multiple files (utils.go, zone_utils.go, autozone_utils.go, file_utils.go)
   - Backup file (tags_cmd.go.bak) should be removed
   - Similar zone handling logic duplicated in multiple files

2. **Inconsistent File Naming**
   - Mixed naming conventions for command files and utility files
   - Some files use plural form while others use singular
   - Some command files don't follow the standard pattern (resolver.go vs *_cmd.go)

3. **Suboptimal Organization**
   - Multiple KV command files that could be consolidated
   - Utility functions that should be in internal packages
   - Command logic mixed with implementation details

4. **Commented-out "Future Enhancement" Code**
   - Commented code blocks for future features that should be properly marked as TODOs

## Cleanup Actions

### Phase 1: Remove Unnecessary Files
- [x] Remove tags_cmd.go.bak backup file

### Phase 2: Consolidate Utility Functions
1. Move common utility functions to internal/common package:
   - [x] Move `splitIntoBatches` and `removeDuplicates` from utils.go to internal/common/batch.go
   - [x] Update imports in all files that use these functions

2. Consolidate zone utilities:
   - [x] Move zone-related functionality from autozone_utils.go and zone_utils.go to internal/zones
   - [x] Create a consistent API for zone operations
   - [x] Update imports in command files

3. Consolidate file utilities:
   - [x] Mark utility functions in utils.go as deprecated, redirecting to common package
   - [x] Mark utility functions in zone_utils.go as deprecated, redirecting to zones package
   - [x] Update consumer code to use the new common package functions

### Phase 3: Rename Files for Consistency
1. [x] Rename resolver.go to resolver_cmd.go
2. [ ] Standardize on singular or plural form for command names (future cleanup)

### Phase 4: Consolidate KV Commands
1. [x] Merge kv_consolidated_cmd.go functionality into kv_cmd.go
2. [x] Mark kv_consolidated_cmd.go as deprecated (to be removed in future)
3. [x] Ensure consistent use of the command builder pattern

### Phase 5: Clean Up Commented Code
1. [ ] Replace commented "future enhancements" with proper TODO comments
2. [ ] Document planned features in a separate document

### Phase 6: Update Documentation
1. [ ] Update README.md to reflect the new organization
2. [ ] Add developer documentation for the command structure

## Implementation Progress

### Completed Tasks

1. **Removed Temporary Files**
   - [x] Removed tags_cmd.go.bak backup file

2. **Consolidated Utility Functions**
   - [x] Added `SplitIntoBatches` and `RemoveDuplicates` to internal/common/batch.go
   - [x] Updated imports in all relevant files
   - [x] Marked original functions as deprecated
   - [x] Added utility methods to zones package for zone operations

3. **Renamed Files for Consistency**
   - [x] Renamed resolver.go to resolver_cmd.go

4. **Consolidated KV Commands**
   - [x] Moved command registration from kv_consolidated_cmd.go to kv_cmd.go
   - [x] Marked kv_consolidated_cmd.go as deprecated

### Pending Tasks

1. **Final Code Cleanup**
   - [ ] Fix any remaining references to deprecated functions
   - [ ] Add proper TODO comments for future enhancements
   - [ ] Remove commented-out code blocks

2. **Documentation Updates**
   - [ ] Update README.md with new organization details
   - [ ] Add developer documentation

## Expected Benefits

- Reduced code duplication
- More consistent naming and organization
- Better separation of concerns
- Improved maintainability
- Easier onboarding for new developers