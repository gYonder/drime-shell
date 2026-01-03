---
applyTo: '**/*.go'
description: Go coding standards for Drime Shell
---

# Go Coding Standards

## Style

- Follow standard Go formatting (`gofmt`)
- Use `golangci-lint` for linting
- Prefer short variable names in small scopes, descriptive names in larger scopes
- Group imports: stdlib, external, internal

## Error Handling

```go
// Return errors with context
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Use errors.Is/As for type checking
if errors.Is(err, api.ErrTokenExpired) {
    // handle
}
```

## Common Patterns

### Spinner for Async Operations

Any operation >100ms must use the spinner wrapper.

```go
// Signature: WithSpinner[T any](w io.Writer, message string, immediate bool, action func() (T, error))
entries, err := ui.WithSpinner(env.Stdout, "", false, func() ([]api.FileEntry, error) {
    return s.Client.ListByParentIDWithOptions(ctx, parentID, opts)
})
```

### Worker Pool for Bulk Ops

Use `asyncMap` for concurrent operations (uploads, deletes).

```go
results, err := asyncMap(items, func(item T, idx int) (R, error) {
    // process item
}, concurrencyLimit)
```

### Retry Logic

Retry is built into the HTTP client via `DoWithRetry` method.

```go
// internal/api/http.go - retries are automatic for all API calls
// The HTTPClient.DoWithRetry handles exponential backoff internally
resp, err := c.DoWithRetry(req)  // Used by all API methods

// You don't call this directly - it's used inside api package methods
// Just call the high-level API methods which handle retries:
entries, err := s.Client.ListByParentIDWithOptions(ctx, parentID, opts)
```

## Concurrency

- Use `context.Context` for cancellation
- Protect shared state with `sync.Mutex` or `sync.RWMutex`
- Use channels for communication, not synchronization
- Worker pools for bulk operations (see `internal/commands/worker_pool.go`)

## Command Implementation Pattern

```go
func init() {
    Register(&Command{
        Name:        "cmdname",
        Description: "Short description",
        Usage:       "cmdname [options] <args>\n\nDetailed usage...",
        Run:         cmdname,
    })
}

func cmdname(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
    fs := pflag.NewFlagSet("cmdname", pflag.ContinueOnError)
    fs.SetOutput(env.Stderr)
    // define flags
    if err := fs.Parse(args); err != nil {
        return err
    }
    // implementation
    return nil
}
```

## API Client Patterns

```go
// Always use context
result, err := s.Client.SomeMethod(ctx, params)

// Use WithSpinner for slow operations
entries, err := ui.WithSpinner(env.Stdout, "Loading...", false, func() ([]api.FileEntry, error) {
    return s.Client.ListByParentIDWithOptions(ctx, parentID, opts)
})

// Use ListOptions/SearchOptions helpers
opts := api.ListOptions(s.WorkspaceID).WithStarredOnly()
```

## Cache Operations

The cache maps paths to entries (IDs, hashes, metadata). It's initialized at startup with the folder tree.

```go
// Resolve path to entry
entry, ok := s.Cache.Get(resolved)
if !ok {
    return fmt.Errorf("not found: %s", path)
}

// Update cache after mutations
s.Cache.AddChildren(parentPath, newEntries)
s.Cache.Invalidate(path)
s.Cache.Remove(path)
```

### Startup: Folder Tree Loading

On startup, fetch the complete folder hierarchy in one call:

```go
// GET /users/{id}/folders returns all folders with parent relationships
folders, _ := client.GetUserFolders(ctx, userID, workspaceID)

// Build path for each folder by walking up parent chain
for _, folder := range folders {
    path := buildPathFromAncestors(folder, foldersById)
    cache.Set(path, folder)
}
```

### Lazy Loading: Prefetch on `cd`

When user runs `cd`, prefetch children in background:

```go
func (s *Session) ChangeDirectory(path string) error {
    // Validate folder exists (from tree)
    entry, ok := s.Cache.Get(resolved)
    if !ok || entry.Type != "folder" {
        return fmt.Errorf("cd: not a directory")
    }

    s.PreviousDir = s.CWD
    s.CWD = resolved

    // Background prefetch children + one level deeper
    go s.prefetchChildren(resolved, 1)
    return nil
}
```

## List Options Helpers

Use helper functions for consistent API queries:

```go
// ListOptions creates options with defaults (orderBy: "name", asc)
opts := api.ListOptions(s.WorkspaceID)

// SearchOptions for search queries (orderBy: "updated_at", desc)
opts := api.SearchOptions(s.WorkspaceID, "query")

// Chainable filters
opts := api.ListOptions(s.WorkspaceID).
    WithDeletedOnly().     // Trash
    WithStarredOnly().     // Starred
    WithTrackedOnly().     // Send & Track
    WithOrder("file_size", "desc")
```

### Advanced Filters (Base64 JSON)

```go
filters := []api.Filter{
    {Key: api.FilterKeyType, Value: "image", Operator: api.FilterOpEquals},
    {Key: api.FilterKeySharedByMe, Value: true, Operator: api.FilterOpEquals},
}
encoded := api.EncodeFilters(filters)
```

Filter keys: `type`, `public`, `owner_id`, `sharedByMe`, `shareableLink`, `created_at`, `updated_at`

## Testing

### Principles

- Mock at boundaries (DrimeClient interface), not internal functions
- Use realistic API response shapes, not minimal stubs
- Test actual output/behavior, not just that functions were called
- Cover error paths: network failures, 401/403/404, empty responses

### Table-Driven Tests

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "real-looking-input", "expected-output", false},
        {"empty input", "", "", true},
        {"error case", "trigger-error", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Something(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %v, want %v", got, tt.want)
            }
        })
    }
}
```
