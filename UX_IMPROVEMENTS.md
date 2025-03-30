# UX Improvements in CLI

## Command Help Prioritization

The CLI now provides a better user experience when using the `--help` flag with other arguments or incomplete flags.

### What's Changed

1. **Help Always Takes Priority**
   - When `--help` appears anywhere in the command, it takes precedence over validation errors
   - Even with incomplete or invalid flags, help text is shown instead of errors

2. **Descriptive Flag Requirements**
   - Flags requiring values are clearly marked with "(requires a value)"
   - This helps users understand which flags need arguments

3. **Better Error Messages**
   - When a flag is missing a required value, a clear error shows which flags need values
   - Error messages are more helpful and point users to specific issues

### Examples

**Before:**
```
$ cache-kv-purger kv list --namespace-id --help
Error: failed to list keys: API error (HTTP 400): ...
```

**After:**
```
$ cache-kv-purger kv list --namespace-id --help
List KV namespaces or keys in a namespace.
...
Usage and examples shown
...
```

### Implementation Notes

This improvement was implemented through:

1. Custom pre-run validation that checks for the help flag first
2. Enhanced flag validation to detect missing values
3. Help prioritization in the command execution flow
4. Improved error messages that specify which flags need values

These changes make the CLI more forgiving and user-friendly, especially for new users exploring the command structure.