# Verbosity Standardization Plan

## Problem Statement

The codebase currently has inconsistent implementation of verbosity control:

1. **Dual verbosity systems:**
   - Global `--verbosity` flag (string enum with values: quiet, normal, verbose, debug)
   - Command-specific `--verbose` flag (boolean)

2. **Inconsistent implementation:**
   - Some commands check both flags
   - The middleware in `cmdutil.WithVerbose` checks only the global flag
   - This creates unpredictable behavior depending on command implementation

3. **Potential user confusion:**
   - Unclear which flag takes precedence
   - Undocumented relationship between flags
   - Inconsistent results across different commands

## Standardization Goals

1. Provide consistent verbosity behavior across all commands
2. Create clear precedence rules when both flags are used
3. Simplify the verbosity implementation for developers
4. Maintain backward compatibility for existing scripts
5. Set a foundation for future verbosity enhancements

## Implementation Plan

### Phase 1: Core Infrastructure Updates

#### 1.1. Update the Verbosity Middleware (highest priority)

**File:** `/home/erfi/cache-kv-purger/internal/cmdutil/middleware.go`

**Current implementation:**
```go
// WithVerbose adds a verbose flag extractor to simplify checking verbose mode
func WithVerbose(fn func(*cobra.Command, []string, bool, bool) error) func(*cobra.Command, []string) error {
    return func(cmd *cobra.Command, args []string) error {
        verbosityStr, _ := cmd.Flags().GetString("verbosity")
        verbose := false
        debug := false

        switch verbosityStr {
        case "quiet":
            // No output
        case "verbose":
            verbose = true
        case "debug":
            verbose = true
            debug = true
        }

        return fn(cmd, args, verbose, debug)
    }
}
```

**New implementation:**
```go
// WithVerbose adds a verbose flag extractor to simplify checking verbose mode
func WithVerbose(fn func(*cobra.Command, []string, bool, bool) error) func(*cobra.Command, []string) error {
    return func(cmd *cobra.Command, args []string) error {
        // Check global verbosity flag (from root command)
        verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")
        
        // Check command-specific verbose flag
        verboseFlag, _ := cmd.Flags().GetBool("verbose")
        
        // Determine verbose and debug status - either flag can enable verbose mode
        verbose := verboseFlag || verbosityStr == "verbose" || verbosityStr == "debug"
        debug := verbosityStr == "debug"

        // If we have debug logging capabilities in the future, add initialization here
        
        return fn(cmd, args, verbose, debug)
    }
}
```

**Testing requirements:**
- Verify middleware works when only `--verbose` is provided
- Verify middleware works when only `--verbosity=verbose` is provided
- Verify middleware works when both flags are provided
- Verify debug mode is correctly set with `--verbosity=debug`

#### 1.2. Update Command Builder Pattern (if applicable)

**File:** `/home/erfi/cache-kv-purger/internal/cmdutil/builder.go` (if exists)

If the codebase uses a command builder pattern, update it to consistently apply verbosity middleware:

```go
// WithRunE sets the RunE function for the command, automatically wrapping it with verbosity handling
func (b *CommandBuilder) WithRunE(fn func(*cobra.Command, []string) error) *CommandBuilder {
    // Wrap the provided function with verbosity middleware
    b.cmd.RunE = WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
        // Set verbose in the command context or pass to function
        b.opts.verbose = verbose
        b.opts.debug = debug
        return fn(cmd, args)
    })
    return b
}
```

#### 1.3. Update Flag Documentation

**File:** `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/main.go`

Update the global verbosity flag documentation:
```go
rootCmd.PersistentFlags().String("verbosity", "normal", 
    "Verbosity level: quiet, normal, verbose, debug. Overrides command-specific --verbose flags")
```

**File:** `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/cache_cmd.go`

Update the command-specific verbose flag documentation:
```go
purgeCmd.PersistentFlags().BoolP("verbose", "v", false, 
    "Enable verbose output. Can also use global --verbosity=verbose")
```

### Phase 2: Command Inventory and Classification

#### 2.1. Inventory of All Command Implementations

Complete list of all commands that need to be updated:

| File | Command | Current Implementation | Priority |
|------|---------|------------------------|----------|
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/files_cmd.go` | `cache purge files` | Checks both flags directly | High |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/tags_cmd.go` | `cache purge tags` | Uses only --verbose flag | High |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/prefixes_cmd.go` | `cache purge prefixes` | Uses only --verbose flag | High |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/hosts_cmd.go` | `cache purge hosts` | Uses only --verbose flag | High |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/everything_cmd.go` | `cache purge everything` | Uses only --verbose flag | High |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/combined_cmd.go` | `cache purge combined` | To be examined | Medium |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/kv_cmd.go` | `kv` | To be examined | Medium |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/kv_consolidated_cmd.go` | `kv consolidated` | To be examined | Medium |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/zones_cmd.go` | `zones` | To be examined | Medium |
| `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/config_cmd.go` | `config` | To be examined | Low |

#### 2.2. Classification by Implementation Pattern

Based on the examination, we can categorize commands by their current verbosity implementation:

1. **Already using middleware pattern**
   - None found yet

2. **Custom verbosity check (dual flags)**
   - `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/files_cmd.go` (checks both `--verbose` and `--verbosity`)

3. **Custom verbosity check (single flag only)**
   - `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/tags_cmd.go` (uses only `--verbose`)
   - `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/prefixes_cmd.go` (uses only `--verbose`)
   - `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/hosts_cmd.go` (uses only `--verbose`)
   - `/home/erfi/cache-kv-purger/cmd/cache-kv-purger/everything_cmd.go` (uses only `--verbose`)

4. **To be examined**
   - All other commands

This classification helps prioritize and group the conversion work. Since our core middleware (`WithVerbose`) now checks both flags, any commands in category 3 will continue to work but won't respect the global `--verbosity` flag until updated.

### Phase 3: Command Implementation Standardization

#### 3.1. Define Standard Command Pattern

For all commands that need verbosity, use the following pattern:

**Approach 1 - Direct Middleware Use:**
```go
func createAnyCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "command",
        Short: "Description",
        RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
            // Command implementation using the verbose/debug flags directly
            if verbose {
                fmt.Println("Verbose output...")
            }
            if debug {
                fmt.Println("Debug output...")
            }
            return nil
        }),
    }
    return cmd
}
```

**Approach 2 - Options Struct:**
```go
func createAnyCommand() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "command",
        Short: "Description",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Create options struct
            var opts struct {
                verbose bool
                debug   bool
                // other options...
            }
            
            // Extract flags
            verboseFlag, _ := cmd.Flags().GetBool("verbose")
            verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")
            opts.verbose = verboseFlag || verbosityStr == "verbose" || verbosityStr == "debug"
            opts.debug = verbosityStr == "debug"
            
            // Command implementation using opts.verbose
            if opts.verbose {
                fmt.Println("Verbose output...")
            }
            if opts.debug {
                fmt.Println("Debug output...")
            }
            
            return nil
        },
    }
    return cmd
}
```

##### Decision Point
Choose Approach 1 (middleware) as the standard pattern, unless:
- The command has complex flag handling that makes direct middleware use difficult
- The command requires access to verbosity flags before other flag parsing

#### 3.2. Update Command Implementations

For each command:

1. **Update command signature**
   - Change `RunE: func(cmd *cobra.Command, args []string) error {` to 
   - `RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {`

2. **Remove custom verbosity checks**
   - Remove lines that check `--verbose` and `--verbosity` flags
   - Replace `opts.verbose` references with the `verbose` parameter
   - Add `debug` logic if appropriate

3. **Update verbose output statements**
   - Ensure all verbose output uses the `verbose` parameter
   - Add debug-only output using the `debug` parameter

Example conversion:

**Before:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    // Get flags
    var opts struct {
        // flag variables...
        verbose bool
    }
    
    // Extract verbosity flags
    verboseFlag, _ := cmd.Flags().GetBool("verbose")
    verbosityStr, _ := cmd.Root().PersistentFlags().GetString("verbosity")
    opts.verbose = verboseFlag || verbosityStr == "verbose" || verbosityStr == "debug"
    
    // Command logic
    if opts.verbose {
        fmt.Println("Verbose output...")
    }
    
    return nil
}
```

**After:**
```go
RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
    // Get flags
    var opts struct {
        // flag variables...
        // verbose removed from struct
    }
    
    // Command logic
    if verbose {
        fmt.Println("Verbose output...")
    }
    
    if debug {
        fmt.Println("Debug details...")
    }
    
    return nil
}),
```

### Phase 4: Implementation Prioritization and Schedule

#### 4.1. High Priority Commands

1. files_cmd.go - `cache purge files`
2. tags_cmd.go - `cache purge tags`
3. prefixes_cmd.go - `cache purge prefixes`
4. hosts_cmd.go - `cache purge hosts`
5. everything_cmd.go - `cache purge everything`

These are the most commonly used commands and should be updated first.

#### 4.2. Medium Priority Commands

1. combined_cmd.go - `cache purge combined`
2. kv_cmd.go - `kv` related commands
3. zones_cmd.go - `zones` commands

These are frequently used but less critical for immediate conversion.

#### 4.3. Final Pass

1. All remaining commands
2. Any utility functions that process verbosity

### Phase 5: Testing Strategy

#### 5.1. Test Script

Create a test script that exercises all commands with different verbosity combinations:

```bash
#!/bin/bash

# Test with no verbosity flags
echo "=== TESTING WITHOUT VERBOSITY FLAGS ==="
./cache-kv-purger cache purge files --zone erfianugrah.com --files-list 3k_urls.txt --dry-run

# Test with command-specific --verbose flag
echo "=== TESTING WITH --verbose FLAG ==="
./cache-kv-purger cache purge files --zone erfianugrah.com --files-list 3k_urls.txt --dry-run --verbose

# Test with global --verbosity flag
echo "=== TESTING WITH --verbosity=verbose FLAG ==="
./cache-kv-purger --verbosity=verbose cache purge files --zone erfianugrah.com --files-list 3k_urls.txt --dry-run

# Test with debug level verbosity
echo "=== TESTING WITH --verbosity=debug FLAG ==="
./cache-kv-purger --verbosity=debug cache purge files --zone erfianugrah.com --files-list 3k_urls.txt --dry-run

# Repeat for all major commands...
```

#### 5.2. Manual Verification

For each command, manually verify:
1. Default output (no verbosity flags)
2. Verbose output with `--verbose`
3. Verbose output with `--verbosity=verbose`
4. Debug output with `--verbosity=debug`

#### 5.3. Error Case Verification

Test error handling with different verbosity levels:
1. Error with no verbosity flags
2. Error with `--verbose`
3. Error with `--verbosity=verbose`
4. Error with `--verbosity=debug`

### Phase 6: Documentation Updates

#### 6.1 User Documentation

Update README.md and other user-facing documentation:

```markdown
## Verbosity Control

The tool supports two ways to control output verbosity:

1. **Global verbosity flag:**
   ```
   cache-kv-purger --verbosity=verbose cache purge files ...
   ```
   
   Supported values:
   - `quiet`: Minimal output
   - `normal`: Standard output (default)
   - `verbose`: Detailed operation information
   - `debug`: Developer-level debugging information
   
2. **Command-specific verbose flag:**
   ```
   cache-kv-purger cache purge files --verbose ...
   ```
   
   This is equivalent to `--verbosity=verbose` and is provided for convenience.

If both flags are specified, the command will use the most verbose setting.
```

#### 6.2 Developer Documentation

Create or update developer documentation:

```markdown
## Implementing Verbosity in Commands

All commands should use the standard verbosity middleware pattern:

```go
RunE: cmdutil.WithVerbose(func(cmd *cobra.Command, args []string, verbose, debug bool) error {
    // Command implementation using verbose and debug flags directly
    if verbose {
        fmt.Println("Verbose details...")
    }
    
    if debug {
        fmt.Println("Debug details...")
    }
    
    return nil
}),
```

### Guidelines:

1. **Verbose Output**: 
   - Show operational details
   - Include progress information
   - Show input validation
   - List major processing steps
   
2. **Debug Output**:
   - Show internal state
   - Include API request/response details
   - Show detailed decision logic
   - Include timing information
```

### Phase 7: Progress Tracking

| Phase | Task | Status | Notes |
|-------|------|--------|-------|
| 1.1 | Update middleware | Completed | Updated WithVerbose to check both flags |
| 1.2 | Update command builder | Not Started | Not applicable - no command builder pattern found |
| 1.3 | Update flag documentation | Completed | Updated descriptions in main.go and cache_cmd.go |
| 2.1 | Inventory commands | Completed | Identified all commands that need updating |
| 2.2 | Classify implementations | Completed | Categorized commands by implementation pattern |
| 3.1 | Define standard pattern | Completed | |
| 3.2 | Update high priority commands | Completed | Updated tags_cmd.go, prefixes_cmd.go, hosts_cmd.go, everything_cmd.go to use middleware |
| 3.3 | Update medium priority commands | Completed | Updated combined_cmd.go and zones_cmd.go; KV commands use WithConfigAndClient already |
| 3.4 | Update remaining commands | Completed | Not needed - using config values instead |
| 4 | Testing | Completed | Verified verbosity works with both flags |
| 5 | Documentation | Completed | Added verbosity documentation to README.md |

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-04-15 | Use middleware pattern as standard | Simplifies command implementation and ensures consistency |
| 2025-04-15 | Maintain both verbosity flags for now | Preserve backward compatibility |
| 2025-04-15 | Add debug level with additional detail | Support developer troubleshooting |

## Known Gaps and Limitations

### KV Service Verbosity Control

During implementation, we discovered that the KV service component has hardcoded debug and verbose messages with prefixes like `[DEBUG]` and `[VERBOSE]`, but these are not controlled by any verbosity setting. This means that the KV commands may display debug messages regardless of verbosity settings.

To properly fix this in a future update:

1. Add verbosity configuration to the KV service:
   ```go
   type KVService struct {
       client *api.Client
       verbose bool
       debug bool
   }
   ```

2. Update the service methods to check verbosity before printing:
   ```go
   if s.verbose {
       fmt.Println("[VERBOSE] Some detailed information")
   }
   
   if s.debug {
       fmt.Println("[DEBUG] Internal state information")
   }
   ```

3. Pass verbosity settings from middleware to the service:
   ```go
   service := kv.NewKVService(client).
       WithVerbose(verbose).
       WithDebug(debug)
   ```

## Future Considerations

1. **KV Service Verbosity**: Implement proper verbosity control in the KV service as detailed above.

2. **Deprecation Plan**: Consider officially deprecating the command-specific `--verbose` flag in favor of standardizing on `--verbosity`.

3. **Structured Logging**: Consider implementing a structured logging system that supports different output formats (text, JSON) based on verbosity.

4. **Color Support**: Add color output for different types of messages when terminal supports it.

5. **Log Redirection**: Support redirecting verbose/debug output to a file while showing normal output on the console.