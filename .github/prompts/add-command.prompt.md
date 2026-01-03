---
name: Add Shell Command
description: Template for implementing a new shell command in drime-shell
---

# Add New Shell Command

Use this prompt when implementing a new command for the drime-shell.

## Input Required

- Command name
- Description
- Expected behavior

## Implementation Checklist

### 1. Create Command Function

In `internal/commands/`, either add to existing file or create new:

```go
func init() {
    Register(&Command{
        Name:        "cmdname",
        Description: "One-line description",
        Usage:       "cmdname [options] <args>\n\nDetailed usage...",
        Run:         cmdname,
    })
}

func cmdname(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
    // Parse flags with pflag
    fs := pflag.NewFlagSet("cmdname", pflag.ContinueOnError)
    fs.SetOutput(env.Stderr)
    
    if err := fs.Parse(args); err != nil {
        return err
    }
    
    // Get positional args
    args = fs.Args()
    
    // Resolve paths
    resolved, err := s.ResolvePathArg(args[0])
    if err != nil {
        return fmt.Errorf("cmdname: %v", err)
    }
    
    // Get entry from cache
    entry, ok := s.Cache.Get(resolved)
    if !ok {
        return fmt.Errorf("cmdname: %s: not found", args[0])
    }
    
    // API call with spinner
    result, err := ui.WithSpinner(env.Stdout, "", false, func() (ResultType, error) {
        return s.Client.SomeMethod(ctx, entry.ID, s.WorkspaceID)
    })
    if err != nil {
        return fmt.Errorf("cmdname: %w", err)
    }
    
    // Update cache if mutation
    s.Cache.Invalidate(resolved)
    
    // Output to env.Stdout
    fmt.Fprintf(env.Stdout, "Result: %v\n", result)
    return nil
}
```

### 2. Add Alias (if needed)

In `internal/commands/alias.go` or the command's init():
```go
Registry["alias"] = Registry["cmdname"]
```

### 3. Handle Vault Context

If command needs different behavior in vault:
```go
if s.InVault {
    // Vault-specific logic
    // Remember: no trash, files are encrypted
}
```

### 4. Add Tests

Create `cmdname_test.go` with table-driven tests:
```go
func TestCmdname(t *testing.T) {
    tests := []struct {
        name    string
        args    []string
        wantErr bool
    }{
        {"basic", []string{"file.txt"}, false},
        {"not found", []string{"missing"}, true},
    }
    // ... test implementation
}
```

### 5. Update Help/Docs

- Ensure Usage string is complete
- Add to README.md command table if user-facing

## Key Patterns

- Use `ResolveEntry(ctx, s, path)` for full entry lookup with cache warming
- Use `api.ListOptions(s.WorkspaceID)` for listing operations
- Use `DownloadAndDecrypt(ctx, s, entry)` for reading file content (handles vault)
- Use `ui.StyleName(name, type)` for colored output
