# AGENTS.md - Technical Context for AI Agents

This document provides comprehensive technical context for AI agents continuing development on the Drime Shell project. It captures all architectural decisions, implementation details, and design rationale.

**For user-facing documentation, see [README.md](README.md).**  
**For API documentation, see [drime-openapi.yaml](drime-openapi.yaml).**  
**For reference implementation patterns, see [drime/](drime/) (TypeScript client).**

---

## Project Overview

**Drime Shell** is a CLI shell application that provides an SSH-like experience for interacting with Drime Cloud storage. Unlike a real shell, there is no actual terminal or shell process running on the remote server. Instead, the shell:

1. Maintains **virtual state** (current working directory, session info) locally
2. Maintains an **internal representation** of the remote filesystem (ID/hash cache)
3. Translates shell commands into **Drime API calls**
4. Formats API responses to look like standard shell output

The goal is to make users feel like they're SSH'd into their cloud storage.

---

## Critical Implementation Insights

### The ID/Hash Problem

The Drime API uses **numeric IDs** for mutations and **base64 hashes** for downloads. Shell users work with **paths**. This requires a local cache mapping paths to IDs/hashes.

**From the TypeScript reference implementation:**
```typescript
// The API requires IDs for operations like move, delete, copy
await api.moveFileEntries(context, [entryIds], destinationId);

// But downloads use the hash
await api.downloadFile(context, hash);

// The hash is computed from the ID
const hashStr = `${fileId}|`;
return Buffer.from(hashStr).toString('base64').replace(/=+$/, '');
```

**Go implementation pattern:**
```go
// FileCache maps paths to their API identifiers
type FileCache struct {
    mu       sync.RWMutex
    entries  map[string]*CachedEntry  // path -> entry
    byID     map[int64]*CachedEntry   // id -> entry (reverse lookup)
}

type CachedEntry struct {
    ID        int64
    Hash      string
    Name      string
    Type      string  // "folder" | "file" | etc.
    ParentID  *int64
    Size      int64
    UpdatedAt time.Time
    Children  []string  // paths of children (for folders)
    Loaded    bool      // true if children have been fetched
}
```

### Startup Strategy: Fetch Folder Tree

**Problem**: We cannot enumerate the entire filesystem on startup (could be thousands of files).

**Solution**: Use `GET /users/{userId}/folders` to fetch the **complete folder hierarchy** in a single API call. This endpoint returns all folders with their parent relationships, allowing us to build the full folder tree instantly.

**From the TypeScript reference:**
```typescript
// Get all folders in one call - this is the key optimization
const { user } = await api.getLoggedUser(ctx);
const folders = await listFolders(ctx, user.id, workspaceId);

// Build path map from parent relationships
const idMap = new Map(folders.map(f => [f.id, f]));
for (const folder of folders) {
    const parts: string[] = [];
    let curr = folder;
    while (curr) {
        parts.unshift(curr.name);
        curr = idMap.get(curr.parentId);
    }
    folderPathMap.set(parts.join('/'), folder.id);
}
```

**Go implementation:**
```go
func (c *FileCache) LoadFolderTree(ctx context.Context, client *Client, userID int64) error {
    folders, err := client.GetUserFolders(ctx, userID, 0)
    if err != nil {
        return err
    }
    
    // Build ID -> folder map for parent lookups
    byID := make(map[int64]*FileEntry)
    for _, f := range folders {
        byID[f.ID] = f
    }
    
    // Build path for each folder by walking up parent chain
    c.mu.Lock()
    defer c.mu.Unlock()
    
    for _, folder := range folders {
        path := c.buildPath(folder, byID)
        c.entries[path] = &CachedEntry{
            ID:       folder.ID,
            Hash:     folder.Hash,
            Name:     folder.Name,
            Type:     "folder",
            ParentID: folder.ParentID,
        }
        c.byID[folder.ID] = c.entries[path]
    }
    return nil
}
```

### Lazy Loading Strategy: Prefetch Children

**Problem**: Listing a directory (`ls`) should be instant, but we can't load all files upfront.

**Solution**: When the user `cd`s into a directory, we:
1. Immediately update CWD (the folder is already in cache from tree load)
2. **Background fetch** the children of that folder
3. **Prefetch** children of subdirectories one level deep (anticipatory loading)

**Go implementation using goroutines:**
```go
func (s *Session) ChangeDirectory(path string) error {
    resolved := s.ResolvePath(path)
    
    entry, ok := s.Cache.Get(resolved)
    if !ok || entry.Type != "folder" {
        return fmt.Errorf("cd: %s: No such directory", path)
    }
    
    s.PreviousDir = s.CWD
    s.CWD = resolved
    
    // Background: fetch children of new CWD
    go s.prefetchChildren(resolved, 1)
    
    return nil
}

func (s *Session) prefetchChildren(path string, depth int) {
    entry, _ := s.Cache.Get(path)
    if entry == nil || entry.Loaded {
        return
    }
    
    // Fetch children from API
    children, err := s.Client.ListByParentID(context.Background(), entry.ID)
    if err != nil {
        return // Silent failure for background ops
    }
    
    s.Cache.SetChildren(path, children)
    
    // Prefetch one level deeper
    if depth > 0 {
        for _, child := range children {
            if child.Type == "folder" {
                go s.prefetchChildren(filepath.Join(path, child.Name), depth-1)
            }
        }
    }
}
```

### Responsive UI with Charm Libraries

**Principle**: Any operation that might take >100ms should show a spinner. Operations >1s should show progress.

**Spinner pattern using Bubbles:**
```go
import "github.com/charmbracelet/bubbles/spinner"

func withSpinner[T any](msg string, fn func() (T, error)) (T, error) {
    s := spinner.New()
    s.Spinner = spinner.Dot
    s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
    
    done := make(chan struct{})
    var result T
    var err error
    
    go func() {
        result, err = fn()
        close(done)
    }()
    
    // Render spinner until done
    for {
        select {
        case <-done:
            fmt.Print("\r" + strings.Repeat(" ", 50) + "\r") // Clear line
            return result, err
        default:
            fmt.Printf("\r%s %s", s.View(), msg)
            s, _ = s.Update(s.Tick())
            time.Sleep(100 * time.Millisecond)
        }
    }
}

// Usage
entries, err := withSpinner("Loading...", func() ([]FileEntry, error) {
    return client.List(ctx, path)
})
```

**Progress bar for transfers:**
```go
import "github.com/charmbracelet/bubbles/progress"

func uploadWithProgress(file *os.File, size int64) {
    p := progress.New(progress.WithDefaultGradient())
    
    uploaded := int64(0)
    for uploaded < size {
        // ... upload chunk ...
        uploaded += chunkSize
        percent := float64(uploaded) / float64(size)
        fmt.Print("\r" + p.ViewAs(percent))
    }
    fmt.Println()
}
```

---

## Implementation Patterns from Reference (drime TypeScript Client)

The [drime/](drime/) folder contains a TypeScript client that serves as our reference implementation. Below are key patterns extracted for Go implementation.

### Retry Logic with Exponential Backoff

All API calls should use retries with exponential backoff and jitter:

```go
func withRetry[T any](operation func() (T, error), maxAttempts int, baseDelay time.Duration) (T, error) {
    var lastErr error
    var zero T
    
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        result, err := operation()
        if err == nil {
            return result, nil
        }
        
        lastErr = err
        if attempt < maxAttempts {
            // Check for rate limit with Retry-After header
            var apiErr *APIError
            if errors.As(err, &apiErr) && apiErr.RetryAfter > 0 {
                time.Sleep(time.Duration(apiErr.RetryAfter) * time.Second)
                continue
            }
            
            // Exponential backoff with jitter
            backoff := baseDelay * time.Duration(1<<(attempt-1))
            jitter := time.Duration(rand.Float64() * 0.25 * float64(backoff))
            time.Sleep(backoff + jitter)
        }
    }
    return zero, lastErr
}
```

### Concurrent Operations with Worker Pool

For bulk operations (folder upload, multi-file delete), use a worker pool pattern:

```go
// asyncMap processes items concurrently with a worker limit
func asyncMap[T, R any](items []T, fn func(T, int) (R, error), concurrency int) ([]R, error) {
    results := make([]R, len(items))
    errors := make([]error, len(items))
    
    sem := make(chan struct{}, concurrency)
    var wg sync.WaitGroup
    
    for i, item := range items {
        wg.Add(1)
        go func(idx int, item T) {
            defer wg.Done()
            sem <- struct{}{}        // Acquire
            defer func() { <-sem }() // Release
            
            results[idx], errors[idx] = fn(item, idx)
        }(i, item)
    }
    
    wg.Wait()
    
    // Collect errors
    for _, err := range errors {
        if err != nil {
            return results, err // Return first error
        }
    }
    return results, nil
}
```

### Folder Path Caching Strategy

The folder cache is critical for performance. Key insights from the reference:

```go
type FolderCache struct {
    mu       sync.RWMutex
    pathToID map[string]int64  // "/Photos/2024" -> 12345
    idToPath map[int64]string  // 12345 -> "/Photos/2024"
}

// EnsureFolderPath creates folders if needed, returns final folder ID
// Uses cache to skip API calls for existing folders
func (c *FolderCache) EnsureFolderPath(ctx context.Context, client *Client, path string, parentID *int64) (int64, error) {
    // Check cache first
    cacheKey := c.makeCacheKey(parentID, path)
    if id, ok := c.get(cacheKey); ok {
        return id, nil
    }
    
    parts := strings.Split(strings.Trim(path, "/"), "/")
    currentParentID := parentID
    
    for i, folderName := range parts {
        partialPath := strings.Join(parts[:i+1], "/")
        partialKey := c.makeCacheKey(parentID, partialPath)
        
        // Check cache for this segment
        if id, ok := c.get(partialKey); ok {
            currentParentID = &id
            continue
        }
        
        // Check if folder exists on server
        entries, err := client.ListByParentID(ctx, currentParentID)
        if err != nil {
            return 0, err
        }
        
        var found *FileEntry
        for _, e := range entries {
            if e.Name == folderName && e.Type == "folder" {
                found = &e
                break
            }
        }
        
        if found != nil {
            c.set(partialKey, found.ID)
            currentParentID = &found.ID
        } else {
            // Create folder
            newFolder, err := client.CreateFolder(ctx, folderName, currentParentID)
            if err != nil {
                // Handle race condition: folder created by another process
                if strings.Contains(err.Error(), "already exists") {
                    // Re-fetch and find it
                    entries, _ := client.ListByParentID(ctx, currentParentID)
                    for _, e := range entries {
                        if e.Name == folderName && e.Type == "folder" {
                            c.set(partialKey, e.ID)
                            currentParentID = &e.ID
                            break
                        }
                    }
                    continue
                }
                return 0, err
            }
            c.set(partialKey, newFolder.ID)
            currentParentID = &newFolder.ID
        }
    }
    
    return *currentParentID, nil
}
```

### Duplicate Detection Optimization

Smart duplicate checking avoids unnecessary API calls:

```go
// If the parent folder doesn't exist in cache, the file can't exist either
func (h *DuplicateHandler) ShouldValidate(relativePath string) bool {
    parentDir := filepath.Dir(relativePath)
    if parentDir == "." || parentDir == "" {
        return true // Root folder always exists
    }
    
    // If parent folder isn't in our cache, it's new, so file is definitely new
    _, exists := h.folderCache.Get(parentDir)
    return exists
}
```

### Multipart Upload Flow

Large files (>65MB) use multipart upload for reliability:

```go
const (
    ChunkSize           = 60 * 1024 * 1024  // 60MB chunks
    MultipartThreshold  = 65 * 1024 * 1024  // Use multipart above this
    BatchSize           = 8                  // Sign URLs in batches
    PartUploadRetries   = 5                  // Retry individual parts
)

func (c *Client) UploadLargeFile(ctx context.Context, file *os.File, remotePath string, onProgress func(int, int)) error {
    stat, _ := file.Stat()
    totalParts := int(math.Ceil(float64(stat.Size()) / float64(ChunkSize)))
    
    // 1. Initialize multipart upload
    upload, err := c.CreateMultipartUpload(ctx, MultipartUploadRequest{
        Filename:  filepath.Base(remotePath),
        Mime:      detectMime(file),
        Size:      stat.Size(),
        Extension: filepath.Ext(remotePath),
    })
    if err != nil {
        return err
    }
    
    // 2. Upload parts in batches (get signed URLs in batches of 8)
    parts := make([]UploadedPart, 0, totalParts)
    partNumbers := makeRange(1, totalParts)
    
    for _, batch := range splitIntoBatches(partNumbers, BatchSize) {
        // Get signed URLs for this batch
        urls, err := c.BatchSignPartUrls(ctx, upload.Key, upload.UploadID, batch)
        if err != nil {
            c.AbortMultipartUpload(ctx, upload.Key, upload.UploadID)
            return err
        }
        
        // Upload each part with retries
        for _, partNum := range batch {
            url := urls[partNum]
            chunk := readChunk(file, partNum, ChunkSize)
            
            etag, err := withRetry(func() (string, error) {
                return uploadChunk(url, chunk)
            }, PartUploadRetries, time.Second)
            
            if err != nil {
                c.AbortMultipartUpload(ctx, upload.Key, upload.UploadID)
                return err
            }
            
            parts = append(parts, UploadedPart{PartNumber: partNum, ETag: etag})
            onProgress(partNum, totalParts)
        }
    }
    
    // 3. Complete upload
    if err := c.CompleteMultipartUpload(ctx, upload.Key, upload.UploadID, parts); err != nil {
        c.AbortMultipartUpload(ctx, upload.Key, upload.UploadID)
        return err
    }
    
    // 4. Create file entry in Drime
    return c.CreateS3Entry(ctx, S3EntryRequest{
        Filename:        upload.Key,
        Size:            stat.Size(),
        ClientMime:      detectMime(file),
        ClientName:      filepath.Base(remotePath),
        ClientExtension: filepath.Ext(remotePath),
        RelativePath:    remotePath,
    })
}
```

### Hash Calculation

The Drime hash is computed from the file ID:

```go
// CalculateDrimeHash computes the hash used for downloads from a file ID
func CalculateDrimeHash(fileID int64) string {
    hashStr := fmt.Sprintf("%d|", fileID)
    encoded := base64.StdEncoding.EncodeToString([]byte(hashStr))
    return strings.TrimRight(encoded, "=")
}

// DecodeDrimeHash extracts the file ID from a hash
func DecodeDrimeHash(hash string) (int64, error) {
    // Add padding if needed
    padding := (4 - len(hash)%4) % 4
    hash += strings.Repeat("=", padding)
    
    decoded, err := base64.StdEncoding.DecodeString(hash)
    if err != nil {
        return 0, err
    }
    
    idStr := strings.TrimSuffix(string(decoded), "|")
    return strconv.ParseInt(idStr, 10, 64)
}
```

---

## Architecture

### Directory Structure

```
drime-shell/
├── cmd/
│   └── drime/
│       └── main.go              # Entry point, CLI flag parsing
├── internal/
│   ├── api/
│   │   ├── client.go            # DrimeClient interface definition
│   │   ├── http.go              # HTTP implementation of DrimeClient
│   │   ├── cache.go             # FileCache and FolderCache
│   │   ├── upload.go            # Upload logic (simple + multipart)
│   │   ├── vault.go             # Vault API methods
│   │   ├── retry.go             # Retry logic with backoff
│   │   └── types.go             # API request/response structs
│   ├── crypto/
│   │   ├── vault.go             # AES-256-GCM encryption for vault
│   │   └── vault_test.go        # Crypto unit tests
│   ├── util/
│   │   ├── memory.go            # Memory detection utilities
│   │   └── memory_test.go       # Memory utility tests
│   ├── shell/
│   │   ├── shell.go             # Main REPL loop
│   │   ├── session.go           # Session state (CWD, user, history, cache)
│   │   ├── parser.go            # Command parsing and glob expansion
│   │   ├── prefetch.go          # Background prefetching logic
│   │   └── completer.go         # Tab completion logic
│   ├── commands/
│   │   ├── registry.go          # Command registration and dispatch
│   │   ├── navigation.go        # cd, pwd, ls, tree, clear
│   │   ├── fileops.go           # mkdir, touch, cp, mv, rm, stat, du
│   │   ├── viewing.go           # cat, head, tail, less, wc
│   │   ├── remote.go            # find, zip (API pass-through)
│   │   ├── transfer.go          # upload, download (vault-aware)
│   │   ├── requests.go          # file requests (create, list, rm)
│   │   ├── vault.go             # vault, vault init/unlock/lock commands
│   │   └── session.go           # whoami, history, help, theme, config, config reload, login, logout, exit
│   ├── session/
│   │   └── session.go           # Session struct with vault state
│   ├── ui/
│   │   ├── theme.go             # Theme definitions (dark/light/auto)
│   │   ├── styles.go            # Lipgloss style definitions
│   │   ├── colors.go            # Semantic color palette
│   │   ├── spinner.go           # Spinner wrapper for async operations
│   │   ├── progress.go          # Progress bar wrapper
│   │   ├── table.go             # Table formatting for ls -l, etc.
│   │   ├── tree.go              # Tree rendering
│   │   └── highlight.go         # Syntax highlighting integration
│   ├── pager/
│   │   ├── pager.go             # Bubbletea viewport wrapper
│   │   └── model.go             # Pager TUI model
│   └── config/
│       ├── config.go            # Config loading/saving
│       └── paths.go             # XDG-style path resolution
├── AGENTS.md                    # This file
├── README.md                    # User documentation
├── go.mod
└── go.sum
```

### Core Components

#### 1. Session State (`internal/shell/session.go`)

The `Session` struct maintains all virtual state:

```go
type Session struct {
    // Virtual filesystem state
    CWD          string       // Current working directory (e.g., "/Photos/2024")
    PreviousDir  string       // For "cd -" support
    HomeDir      string       // User's home directory on Drime (usually "/")
    
    // Cache - critical for path-to-ID resolution
    Cache        *FileCache   // Maps paths to IDs/hashes
    
    // Authentication
    User         *User        // Current user info
    UserID       int64        // User ID for API calls
    Token        string       // API token
    WorkspaceID  int64        // Current workspace (0 = personal)
    
    // Client
    Client       api.DrimeClient
    
    // UI
    Theme        ui.Theme
    
    // History
    History      []string
    HistoryIndex int
    
    // Background operations
    prefetchMu   sync.Mutex
    prefetching  map[string]bool  // Paths currently being prefetched
}
```

#### 2. DrimeClient Interface (`internal/api/client.go`)

Abstract interface for all Drime API operations:

```go
type DrimeClient interface {
    // Authentication
    Whoami(ctx context.Context) (*User, error)
    
    // Navigation & Listing
    GetUserFolders(ctx context.Context, userID int64, workspaceID int64) ([]FileEntry, error)
    ListByParentID(ctx context.Context, parentID *int64) ([]FileEntry, error)
    ListByParentIDWithOptions(ctx context.Context, parentID *int64, opts *ListEntriesOptions) ([]FileEntry, error)
    SearchWithOptions(ctx context.Context, query string, opts *ListEntriesOptions) ([]FileEntry, error)
    
    // File Operations (Remote)
    CreateFolder(ctx context.Context, name string, parentID *int64, workspaceID int64) (*FileEntry, error)
    DeleteEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error
    MoveEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destWorkspaceID *int64) error
    CopyEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destWorkspaceID *int64) ([]FileEntry, error)
    RenameEntry(ctx context.Context, entryID int64, newName string, workspaceID int64) (*FileEntry, error)
    GetSpaceUsage(ctx context.Context, workspaceID int64) (*SpaceUsage, error)
    ExtractEntry(ctx context.Context, entryID int64, parentID *int64, workspaceID int64) error
    GetEntry(ctx context.Context, entryID int64, workspaceID int64) (*FileEntry, error)
    
    // Transfers
    Upload(ctx context.Context, reader io.Reader, name string, parentID *int64, size int64, workspaceID int64) (*FileEntry, error)
    Download(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error)
}
```

#### 2.1 ListEntriesOptions Helper Functions (`internal/api/filters.go`)

To ensure consistency and reduce duplication, use the helper functions:

```go
// ListOptions creates options with sensible defaults for listing
opts := api.ListOptions(workspaceID)  // OrderBy: "name", OrderDir: "asc"

// SearchOptions creates options configured for search
opts := api.SearchOptions(workspaceID, "query")  // OrderBy: "updated_at", OrderDir: "desc"

// Chainable methods for filters
opts := api.ListOptions(workspaceID).
    WithDeletedOnly().     // Only trashed items
    WithStarredOnly().     // Only starred items
    WithTrackedOnly().     // Only tracked items (Send & Track)
    WithOrder("file_size", "desc").
    WithFilters([]api.Filter{
        {Key: api.FilterKeyType, Value: "image", Operator: api.FilterOpEquals},
    })
```

#### 2.2 Advanced Search Filters (`internal/api/filters.go`)

The API supports Base64-encoded JSON filters for advanced search:

**Filter Keys:**
- `type` - File type (image, video, audio, pdf, text, folder, spreadsheet, word, archive)
- `public` - Public visibility (boolean)
- `owner_id` - Owner user ID
- `sharedByMe` - Files shared by current user (boolean)
- `shareableLink` - Files with public link (use `has` operator)
- `created_at` - Creation date
- `updated_at` - Modification date

**Filter Operators:**
- `=`, `!=` - Equality
- `>`, `<` - Comparison
- `between` - Date ranges
- `has` - Existence check (for shareableLink)

```go
// Example: Find all shared PDFs
filters := []api.Filter{
    {Key: api.FilterKeyType, Value: "pdf", Operator: api.FilterOpEquals},
    {Key: api.FilterKeySharedByMe, Value: true, Operator: api.FilterOpEquals},
}
encoded := api.EncodeFilters(filters)  // Base64 JSON
```

#### 3. Command Registry (`internal/commands/registry.go`)

Commands are registered with metadata for help and completion:

```go
type Command struct {
    Name        string
    Aliases     []string
    Description string
    Run         func(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error
}

type ExecutionEnv struct {
    Stdin  io.Reader
    Stdout io.Writer
    Stderr io.Writer
}

var Registry = map[string]*Command{
    "ls":       lsCommand,
    "cd":       cdCommand,
    // ...
}
```

### Shell Utilities Implementation

The shell implements standard Unix-like utilities adapted for the Drime cloud environment:

*   **`echo`, `printf`**: Standard output utilities with escape sequence support.
*   **`diff`**: Compares two remote files using `go-difflib`. Files are downloaded temporarily to memory (limit: configurable via `max_memory_buffer_mb`, default 100MB).
*   **`sort`, `uniq`**: Text processing tools operating on remote files. Support `-r` (reverse) and `-c` (count) flags.
*   **`stat`**: Detailed file metadata including IDs, hashes, and MIME types.
*   **`tree`**: Visual directory tree visualization (limited simple depth).
*   **`find`**: Search files via server-side API with Unix-like syntax (`-name`, `-type`, `-S`, `--trash`, `--shared`).
*   **`wc`, `head`, `tail`**: Text processing utilities.

### Piping and Redirection

The shell supports standard Unix piping `|` and redirection operations:

*   **Pipes (`|`)**: Connect `Stdout` of one command to `Stdin` of another (e.g., `ls | sort`, `cat file.txt | sort | uniq`).
*   **Output Redirection (`>`)**: Write command output to a **remote** file on Drime Cloud. This uses a `RemoteFileWriter` that buffers output to a temp file and uploads it on close.
*   **Implementation**: The REPL parses pipe segments, creates `os.Pipe` instances for inter-command communication, and `RemoteFileWriter` for redirection.

**Design Principle**: All operations behave as if running on the cloud. `echo "hello" > file.txt` creates `file.txt` on Drime Cloud, not locally. This maintains the SSH-like illusion.

---

---

## Key Design Decisions

### 1. Virtual State Model

**Decision**: The shell maintains a "virtual CWD" locally rather than relying on server-side session state.

**Rationale**: 
- The Drime API is stateless (REST)
- Allows offline path manipulation and validation
- Simplifies API design

**Implementation**:
- `cd` validates the path exists via `Stat()` before updating `session.CWD`
- All commands resolve relative paths against `session.CWD` before API calls
- Path utilities handle `.`, `..`, `~`, and `-` (previous directory)

### 2. Remote vs Local Operations

**Decision**: All file operations (`cp`, `mv`, `rm`, `mkdir`, `touch`) operate on remote files only.

**Rationale**: 
- Matches SSH behavior where commands affect the remote system
- Explicit `upload`/`download` commands for local↔remote transfers
- Avoids ambiguity about which filesystem is being modified

**Implementation**:
- `cp src dst` → `POST /api/copy` (server-side copy)
- `upload local remote` → Stream local file to `POST /api/upload`
- `download remote local` → Stream from `GET /api/download` to local file

### 3. Charm Ecosystem for UI

**Decision**: Use Charm libraries exclusively for all UI rendering.

**Rationale**:
- Consistent styling and behavior
- Modern, beautiful output
- Active maintenance and community
- Libraries work well together

**Libraries**:
- `lipgloss` — All styling, colors, tables, trees
- `bubbletea` — TUI framework for pager and interactive prompts
- `bubbles` — Spinner, viewport, progress bar components
- `glamour` — Markdown rendering
- `log` — Styled logging

### 4. Theme System

**Decision**: Auto-detect terminal theme with manual override, persisted in config. Uses **Catppuccin** color palettes (Mocha for dark, Latte for light).

**Implementation**:
```go
type Theme string

const (
    ThemeAuto  Theme = "auto"
    ThemeDark  Theme = "dark"
    ThemeLight Theme = "light"
)

func DetectTheme() Theme {
    if lipgloss.HasDarkBackground() {
        return ThemeDark
    }
    return ThemeLight
}
```

**Prompt Style**:
The shell uses a **Powerline-style prompt** with segments for:
1.  **User** (Mauve background)
2.  **Path** (Surface background)
3.  **Context** (Blue for Workspace, Red for Vault)

**Color Palette** (semantic colors that adapt per theme):
- `ColorPrimary` — Prompt, headers
- `ColorDirectory` — Directory names (blue)
- `ColorFile` — Regular files (default)
- `ColorExecutable` — Executable files (green)
- `ColorSymlink` — Symbolic links (cyan)
- `ColorError` — Errors (red)
- `ColorWarning` — Warnings (yellow)
- `ColorMuted` — Secondary text (gray)

### 5. Pager Strategy

**Decision**: Hybrid approach based on file size.

| Size | Strategy |
|------|----------|
| < 100KB | Inline output with syntax highlighting |
| 100KB–10MB | Built-in `bubbletea` viewport |
| > 10MB | Write to temp file + system `$PAGER` |

**Rationale**:
- Small files: Fast, no interaction overhead
- Medium files: Stay in-shell, consistent UX
- Large files: Don't run out of memory, use user's preferred pager

### 6. Syntax Highlighting

**Decision**: Auto-detect file type and apply highlighting.

**Implementation**:
- **Markdown** (`.md`): Use `glamour` for rich rendering
- **Code files**: Use `chroma` with lexer detection by extension
- **Unknown**: Plain text, no highlighting

```go
func Highlight(content []byte, filename string) (string, error) {
    ext := filepath.Ext(filename)
    
    if ext == ".md" {
        return glamour.Render(string(content), "dark")
    }
    
    lexer := lexers.Match(filename)
    if lexer == nil {
        return string(content), nil
    }
    
    // ... chroma formatting
}
```

### 7. Glob Pattern Expansion

**Decision**: Expand globs client-side against remote directory listings.

**Rationale**:
- API may not support glob syntax
- Consistent behavior across all commands
- Control over which patterns are supported

**Implementation**:
- Use `bmatcuk/doublestar/v4` for pattern matching
- Fetch directory listing, filter with `Match(pattern, filename)`
- Expand before passing to command handler

**Supported patterns**: `*`, `?`, `[abc]`, `[a-z]`, `[!abc]`

### 8. Configuration

**Decision**: Single YAML config file at `~/.drime-shell/config.yaml`, editable manually or via commands.

**Rationale**:

- YAML is human-readable and easy to edit manually
- Single file keeps things simple
- Users can version control their config (minus token) if desired

**Schema**:
```yaml
theme: auto                        # auto | dark | light
token: drm_xxxxxxxxxxxxxxxxxxxx    # API token
api_url: https://api.drime.cloud   # API endpoint
default_path: /                    # Starting directory
history_size: 1000                 # Max history entries
max_memory_buffer_mb: 100          # Max MB buffered in memory before using temp files
```

**Token Priority** (highest to lowest):
1. `DRIME_TOKEN` environment variable
2. Config file `token` field
3. Interactive prompt on startup

**Security**:
- File created with `0600` permissions
- Warning if permissions are too open
- Token masked in `config show` output

### 9. Command-Based Organization (Trash, Starred, Tracked)

**Decision**: Trash, Starred, and Tracked files are managed via dedicated commands rather than virtual filesystem views.

**Rationale**:
- Simpler implementation (no virtual path parsing or cache invalidation complexity)
- More consistent with standard CLI tools
- Clearer separation between navigation and management

**Implementation**:
- **Trash**: `trash` command handles listing, restoring, and emptying.
- **Starred**: `star` command handles listing (`star ls`), adding, and removing.
- **Tracked**: `track` command handles listing (`track ls`), adding, removing, and viewing stats.
- **Shared**: `share` command handles listing (`share ls`), linking, and inviting.

**Pattern**:
- `noun ls` — List items (e.g., `trash ls`, `star ls`)
- `noun <file>` — Apply action (e.g., `star file.txt`)
- `noun verb <file>` — Specific action (e.g., `trash restore file.txt`)
- Top-level aliases provided for common actions (`restore`, `unstar`, `untrack`).

### 10. Piping and Redirection (Remote Semantics)

**Decision**: The shell supports pipes (`|`) and redirection (`>`, `>>`, `<`) and they operate on the **remote** filesystem.

**Implementation notes**:
- `>` / `>>` buffer locally then upload to Drime (remote write).
- `<` downloads the remote file and provides it as stdin (remote read).

### 11. No Verbose/Interactive Flags

**Decision**: Remove `-v` (verbose) and `-i` (interactive) flags from all commands.

**Rationale**:
- Simplifies implementation
- Modern UX doesn't need verbose mode (use progress bars, spinners)
- Interactive confirmations add friction; use explicit commands instead

### 12. S3 Presigned URL Upload Flow

**Decision**: Use S3 presigned URLs for all uploads instead of multipart form-data to the API.

**Rationale**:
- Aligns with the web app implementation pattern
- Offloads bandwidth from API servers to S3/R2
- More reliable for large files
- Supports better retry semantics

**Implementation**:
1. **Simple uploads** (<65MB): `POST /s3/simple/presign` → PUT to S3 → `POST /s3/entries`
2. **Multipart uploads** (>65MB): `POST /s3/multipart/create` → batch sign URLs → PUT parts → `POST /s3/multipart/complete` → `POST /s3/entries`
3. Multipart uploads abort on failure via `POST /s3/multipart/abort`

```go
// Simple upload flow
presign, _ := client.SimplePresign(ctx, SimplePresignRequest{...})
s3Client.PUT(presign.URL, content)
client.CreateS3Entry(ctx, CreateS3EntryRequest{...})
```

### 13. Token Expiration Handling

**Decision**: Return a specific `ErrTokenExpired` error on 401 responses, no retries.

**Rationale**:
- 401 indicates the token is invalid/expired, retrying won't help
- User needs to re-authenticate
- Clear error message guides user to run `login`

**Implementation**:
```go
var ErrTokenExpired = errors.New("authentication token expired or invalid")

// In DoWithRetry:
if resp.StatusCode == http.StatusUnauthorized {
    return nil, ErrTokenExpired
}
```

REPL catches this error and displays: "Session expired. Please run 'login' to re-authenticate."

### 14. MaxPerPage Pagination Constant

**Decision**: Use `MaxPerPage = 9999999999` for all paginated API calls to fetch everything in one request.

**Rationale**:
- Simplifies client logic (no pagination handling needed)
- Folder trees and file lists fit in single responses for typical use
- API handles large result sets efficiently

**Implementation**:
```go
const MaxPerPage int64 = 9999999999

url := fmt.Sprintf("%s/users/%d/folders?workspaceId=%d&perPage=%d", 
    c.BaseURL, userID, workspaceID, MaxPerPage)
```

### 15. Upload Duplicate Handling Policy

**Decision**: Support `--on-duplicate` flag with four policies: `ask`, `replace`, `rename`, `skip`.

**Rationale**:
- Matches web app behavior for consistency
- Provides flexibility for batch uploads
- `ask` (default) ensures user awareness of conflicts
- Non-interactive policies (`replace`, `rename`, `skip`) enable scripting

**Implementation**:
```go
type DuplicatePolicy string

const (
    DuplicatePolicyAsk     DuplicatePolicy = "ask"
    DuplicatePolicyReplace DuplicatePolicy = "replace"
    DuplicatePolicyRename  DuplicatePolicy = "rename"
    DuplicatePolicySkip    DuplicatePolicy = "skip"
)
```

### 16. Path Resolution via API Fallback

**Decision**: When cache doesn't have a folder's path, use `GET /folders/{hash}/path` API endpoint.

**Rationale**:
- Handles edge cases where cache is incomplete
- Returns full ancestor chain for reliable path building
- Warms the cache with all ancestors for future lookups

**Implementation**:
```go
// Fallback when cache miss
ancestors, err := client.GetFolderPath(ctx, entry.Hash, workspaceID)
// Build path from ancestors and warm cache
```

### 17. SSL Error Hints

**Decision**: Detect SSL/TLS errors and provide helpful troubleshooting hints.

**Rationale**:
- SSL errors are common but cryptic
- Users often have VPN, firewall, or certificate issues
- Specific hints reduce support burden

**Patterns detected**:
- `UNEXPECTED_EOF` / `CONNECTION RESET` → "Try reducing parallel workers, checking network, or disabling VPN"
- `CERTIFICATE` → "Check if system certificates are up to date"
- Other SSL → "Check network connection and try again"

### 18. Workspace Member Management

**Decision**: Implement full workspace member management with API as gatekeeper for permissions.

**Rationale**:
- Matches web app functionality for parity
- No client-side permission checking - rely on API errors
- Confirmation prompts for destructive actions (kick, leave, delete)
- Role caching for better UX

**Commands**:
- `ws members` — List members and pending invites in a formatted table
- `ws roles` — List available roles with descriptions
- `ws invite <email> [role]` — Invite user (defaults to "Member" role)
- `ws kick <email>` — Remove member or cancel invite (with confirmation)
- `ws role <email> <role>` — Change member's role
- `ws leave` — Leave workspace (with confirmation, auto-switch to default)
- `ws rm` — Enhanced with confirmation prompt

**Implementation details**:
- Roles are cached in `Session.WorkspaceRoles` after first `ws roles` or `ws invite` call
- Role names can be specified with or without "Workspace " prefix (case-insensitive)
- Member type detection: `member` for active users, `invite` for pending invitations
- Workspace stats displayed on switch: `Switched to workspace 'Name' (42 files, 1.2 GB)`

**API endpoints used**:
- `GET /workspace/{id}` — Workspace with members and invites
- `GET /workspace_roles` — Available roles
- `POST /workspace/{id}/invite` — Send invitations
- `DELETE /workspace/{id}/member/{memberId}` — Remove member
- `DELETE /workspace/invite/{inviteId}` — Cancel invitation
- `POST /workspace/{id}/{type}/{id}/change-role` — Change role

### 19. Encrypted Vault Implementation

**Decision**: Implement vault as a special context (like workspace) with client-side AES-256-GCM encryption.

**Rationale**:
- Matches web app zero-knowledge encryption model
- User holds the only copy of the encryption key
- Files are encrypted before upload, decrypted after download

**Encryption Details**:
- **Algorithm**: AES-256-GCM with 12-byte random IV per file
- **Key Derivation**: PBKDF2-SHA256 with 250,000 iterations (matches web app)
- **Plaintext for verification**: `vault-unlock` encrypted with vault key
- **Auth tag**: 16 bytes appended to ciphertext

**Session State**:
```go
// Vault-related fields in Session
InVault       bool          // Currently operating in vault
VaultID       int64         // Vault ID from API
VaultUnlocked bool          // Key is derived and available
VaultKey      *crypto.VaultKey  // Derived encryption key
VaultSalt     string        // Salt from API (base64)
VaultCheckIV  string        // IV for check value (base64)
VaultCheck    string        // Encrypted check value (base64)

// Saved workspace state for switching back
SavedWorkspaceID   int64
SavedWorkspaceName string
SavedCWD           string
SavedCache         *api.FileCache
```

**Prompt Display**:
- In vault: `[vault:locked]` or `[vault:unlocked]` prefix
- Uses `Session.ContextName()` for consistent display

**Commands**:
- `vault` — Enter vault (prompts unlock if locked)
- `vault unlock` — Unlock with password
- `vault lock` — Clear key from memory, rebuild workspace cache
- `vault init` — Create new vault (password + confirm)

**Cross-Transfer**:
- `cp --vault` / `mv --vault` — From workspace to vault (encrypt)
- `cp -w <id>` / `mv -w <id>` — From vault to workspace (decrypt)

**Vault-Specific Behaviors**:
- No trash — all deletes are permanent
- No duplicate detection — matches web app behavior
- No starred files — not supported in vault
- `upload` encrypts before S3 upload
- `download` decrypts after download
- All file viewing commands (`cat`, `less`, etc.) auto-decrypt

**API Endpoints**:
- `GET /vault` — Get vault metadata (salt, check, iv)
- `POST /vault` — Initialize new vault
- `GET /vault/file-entries` — List vault entries
- `POST /vault/file-entries/move` — Move within vault
- `POST /vault/delete-entries` — Permanent delete
- `POST /folders` with `vaultId` param — Create vault folder
- `GET /file-entries/download/{hash}?encrypted=true` — Download encrypted
- `POST /s3/entries` with `isEncrypted=1`, `vaultId`, `vaultIvs` — Create encrypted entry

### 20. Human-like Coding Style

**Decision**: Avoid "vibe coded" markers in source code and documentation.

**Rationale**:
- Code should look professional and human-written.
- Excessive emojis in comments or documentation (outside of UI output) are distracting.
- Robotic, redundant comments (e.g., `// 1. Do X`) should be replaced with natural language.

**Guidelines**:
- **No emojis in comments**: Keep source code comments clean.
- **Natural comments**: Write comments as if explaining to a colleague, not a step-by-step robot.
- **Professional Docs**: Use standard markdown lists instead of emoji bullet points in technical docs.
- **UI Exceptions**: Emojis in *string literals* for CLI output are allowed and encouraged for the "Beautiful UI" goal.

---

## Command Implementation Details

### Navigation Commands

#### `cd <path>`

1. `cd /` returns to root.
2. `cd -` toggles back to `PreviousDir`.
3. Resolve the target and validate it is a folder from cache (folder tree).
4. Update `PreviousDir`, set `CWD`, and prefetch children.

#### `ls [options] [path]`

Options: `-l`, `-a`, `-h`, `-R`, `-t`, `-S`

1. Resolve path (default to CWD).
2. List children using `ListByParentIDWithOptions`.
3. If `-S` is set, filter for starred files.
4. Format output based on flags:
   - No flags: Names only, columns
   - `-l`: Table with permissions, size, date, name
   - `-a`: Include dotfiles
   - `-h`: Format sizes as 1K, 2M, etc.
   - `-R`: Recursive (multiple API calls)
   - `-t`: Sort by mtime descending
   - `-S`: Show only starred files

### File Viewing Commands

#### `cat [options] <file>`

Options: `-n` (line numbers)

1. Resolve path
2. Call `client.Read(path)` to get content stream
3. Read into memory (check size first via `Stat`)
4. Apply syntax highlighting
5. If `-n`: Prepend line numbers
6. Print to stdout

#### `less <file>`

1. Resolve path
2. Call `client.Stat(path)` for size
3. Based on size:
   - Small: Inline with highlighting
   - Medium: Launch `bubbletea` viewport
   - Large: Write to temp, exec `$PAGER`

### Remote Execution Commands

These pass through to API endpoints that execute on the server.

#### `find [options] [path]`

Server-side search with Unix-like syntax. Uses the API's `query` parameter for substring matching.

Options: `-name`, `-type`, `-S/--starred`, `--trash`, `--shared`

```go
// Build search options
opts := api.ListOptions(s.WorkspaceID)
opts.Query = namePattern  // -name value passed to API

// Type filter: -type d maps to folder
if fileType == "d" {
    filters = append(filters, api.Filter{Key: api.FilterKeyType, Value: "folder"})
}

// -type f: client-side filter (exclude folders from results)
// --starred: opts.WithStarredOnly()
// --trash: opts.WithDeletedOnly()
// --shared: filter with api.FilterKeySharedByMe

// If path specified, restrict to direct children via parentIds
results, err := s.Client.ListByParentIDWithOptions(ctx, parentID, opts)
```

**Limitations:**
- When a path is specified, only direct children are searched (API limitation)
- For recursive search, omit the path to search workspace-wide
- `-name` does substring matching, not glob patterns

#### `search [query] [flags]`

Advanced search using the API's filter system (Base64 encoded JSON).

Options:
- `--type`: Filter by file type (image, video, audio, pdf, text, folder)
- `--owner`: Filter by owner ID
- `--public`: Show only public files
- `--shared`: Show files shared by me (email + links)
- `--link`: Show files with public link
- `--trash`: Show files in trash
- `--starred`: Show starred files
- `--after`/`--before`: Date filtering
- `--sort`: Sort field
- `--asc`/`--desc`: Sort direction

Implementation:
1. Parse flags into `api.Filter` structs.
2. Encode filters using `api.EncodeFilters` (JSON -> Base64).
3. Call `client.SearchWithOptions` with `Filters` string.
4. Render results in a table.

### Transfer Commands

#### `upload [options] <local> [remote]`

Options: `-r` (recursive), `--on-duplicate <ask|replace|rename|skip>`

1. Validate local path exists
2. Parse `--on-duplicate` flag (default: `ask`)
3. Determine remote path (default: CWD + basename)
4. Check for duplicates via `POST /uploads/validate`
5. Resolve duplicates based on policy:
   - `ask`: Interactive prompt for each duplicate
   - `replace`: Overwrite existing files
   - `rename`: Get new name via `POST /uploads/available-name`
   - `skip`: Skip uploading duplicate files
6. Upload via S3 presigned URL flow (see Design Decision #12)
7. Show progress bar with `bubbles/progress`
8. Use worker pool for concurrent multi-file uploads

#### `download [options] <remote> [local]`

Options: `-r` (recursive)

1. Resolve remote path
2. Determine local path (default: current OS directory + basename)
3. If directory and `-r`: List remote tree, download each file
4. Show progress bar

### File Requests

#### `request create <folder> [flags]`

Flags: `--title`, `--desc`, `--expire`, `--password`, `--custom-link`

1. Resolve folder path to ID.
2. Call `CreateFileRequest` (POST `/file-entries/{id}/shareable-link` with `request` payload).
3. If security flags are present, call `UpdateShareableLink` (PUT `/file-entries/{id}/shareable-link`) to apply them.
4. Print the generated link.

#### `request ls`

1. Call `ListFileRequests` (GET `/file-requests/all`).
2. Render table with ID, Name, Folder, Link, Uploads, Expiration.

---

## Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| `github.com/charmbracelet/lipgloss` | v1.x | Styling, colors, tables, trees |
| `github.com/charmbracelet/bubbletea` | v1.x | TUI framework |
| `github.com/charmbracelet/bubbles` | v0.x | Viewport, spinner, progress |
| `github.com/charmbracelet/glamour` | v0.x | Markdown rendering |
| `github.com/charmbracelet/log` | v0.x | Styled logging |
| `github.com/alecthomas/chroma/v2` | v2.x | Syntax highlighting |
| `github.com/bmatcuk/doublestar/v4` | v4.x | Glob pattern matching |
| `github.com/sergi/go-diff` | latest | Diff output |
| `github.com/vbauerster/mpb/v8` | v8.x | Multi-progress bars |
| `gopkg.in/yaml.v3` | v3 | Config file parsing |

---

### API Integration

> **Source of Truth**: See [`drime-openapi.yaml`](drime-openapi.yaml) for complete API documentation including all endpoints, parameters, request/response schemas, and examples.

#### Key Concepts

- **Base URL**: `https://app.drime.cloud/api/v1`
- **Authentication**: `Authorization: Bearer <token>`
- **Workspace ID 0**: The default personal workspace (documented in OpenAPI spec)
- **ID vs Hash**: Use numeric `id` for mutations (move, delete, share); use string `hash` for downloads and shareable links
- **Workspace scoping**: Many endpoints accept optional `workspaceId` query params; always pass `Session.WorkspaceID` for correctness.
- **Trash/Starred**: These are workspace-scoped filters on listings/search, not separate workspaces:
    - Trash view uses `deletedOnly=true`
    - Starred view uses `starredOnly=true`

#### Command-to-Endpoint Mapping

| Shell Command | API Endpoint(s) | Notes |
|--------------|-----------------|-------|
| `whoami` | `GET /cli/loggedUser` | |
| `ls` | `GET /drive/file-entries` | Supports `orderBy`, `orderDir`, `page`, `backup`, filtering |
| `cd` | `GET /users/{id}/folders` + `GET /drive/file-entries` | Uses folder tree + listings; view switching rebuilds cache |
| `pwd` | (local state) | No API call needed |
| `mkdir` | `POST /folders` | |
| `rm` | `POST /file-entries/delete` | Supports `emptyTrash` |
| `mv` | `POST /file-entries/move` | |
| `cp` | `POST /file-entries/duplicate` | |
| `stat` | `GET /file-entries/{entryId}` | |
| `download` | `GET /file-entries/download/{hash}` | Returns binary or ZIP for folders |
| `upload` | S3 flow | validate → presign → transfer → entries |
| `du` | `GET /user/space-usage` | |
| `tree` | `GET /users/{id}/folders` | Get folder hierarchy |
| `unzip` | `POST /file-entries/{entryId}/extract` | Server-side ZIP extraction |

#### Commands Without Direct API Support

These commands must be implemented locally:

| Command | Implementation |
|---------|---------------|
| `cat`, `head`, `tail`, `less` | Download file via `/file-entries/download/{hash}`, display locally |
| `find` | Uses `GET /drive/file-entries?query=...` with filters; path restriction via `parentIds` (direct children only) |
| `touch` | Upload empty file via S3 flow |

#### S3 Upload Flow

The upload process is a 4-step sequence (see OpenAPI spec for full schemas):

1. **Validate**: `POST /uploads/validate` — check quotas and duplicates
2. **Presign**: 
   - Small files: `POST /s3/simple/presign`
   - Large files: `POST /s3/multipart/create` + `/batch-sign-part-urls`
3. **Transfer**: PUT binary to presigned S3/R2 URL
4. **Finalize**: `POST /s3/entries` — create FileEntry record

---

## Testing Strategy

### Unit Tests

- **Parser tests**: Command parsing, glob expansion, path resolution
- **UI tests**: Theme detection, color formatting
- **Config tests**: Loading, saving, defaults, permissions

### Integration Tests

- **Mock API client**: Implement `DrimeClient` with in-memory filesystem
- **Command tests**: Each command with mock client

### Manual Testing

- Test with real Drime API
- Test on different terminals (iTerm2, Terminal.app, VS Code, etc.)
- Test theme detection in light/dark modes

---

## Future Enhancements (Post-MVP)

### Phase 2: Shell Features
- Pipes (`|`) — Connect command stdout to stdin
- Output redirection (`>`, `>>`) — Write to remote files
- Input redirection (`<`) — Read from remote files
- Command substitution (`$(...)`)

### Phase 3: Advanced Commands
- `chmod` / `chown` — If API supports permissions
- `ln -s` — If API supports symlinks
- `diff` — Compare two remote files
- `sort`, `uniq` — Text processing

### Phase 4: Power Features
- Multiple profiles (`--profile work`)
- Bookmarks / aliases
- Shell scripting support
- SSH key authentication

---

## Common Patterns

### Resolving Paths

```go
func (s *Session) ResolvePath(path string) string {
    if path == "" {
        return s.CWD
    }
    if path == "~" {
        return s.HomeDir
    }
    if path == "-" {
        return s.PreviousDir
    }
    if strings.HasPrefix(path, "~/") {
        return filepath.Join(s.HomeDir, path[2:])
    }
    if filepath.IsAbs(path) {
        return filepath.Clean(path)
    }
    return filepath.Clean(filepath.Join(s.CWD, path))
}
```

### Error Handling

Use styled errors with `lipgloss`:

```go
var errorStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("9")).
    Bold(true)

func PrintError(format string, args ...any) {
    msg := fmt.Sprintf(format, args...)
    fmt.Println(errorStyle.Render("Error: " + msg))
}
```

### Progress Bars

```go
func uploadWithProgress(client api.DrimeClient, local, remote string) error {
    file, _ := os.Open(local)
    stat, _ := file.Stat()
    
    bar := progressbar.NewOptions64(stat.Size(),
        progressbar.OptionSetDescription(filepath.Base(local)),
        progressbar.OptionShowBytes(true),
        progressbar.OptionSetTheme(progressbar.Theme{
            Saucer:        "█",
            SaucerPadding: "░",
        }),
    )
    
    reader := progressbar.NewReader(file, bar)
    return client.Upload(ctx, &reader, remote)
}
```

---

## Implementation Plan (Phased)

### Phase 1: Foundation (Week 1)

**Goal**: Basic shell that can authenticate, list files, and navigate directories.

#### 1.1 Project Setup
- [ ] Initialize Go module (`go mod init github.com/user/drime-shell`)
- [ ] Install core dependencies (Charm ecosystem, YAML parser)
- [ ] Create directory structure per Architecture section
- [ ] Set up linting (golangci-lint) and formatting

#### 1.2 Configuration & Authentication
- [ ] Implement config loading/saving (`~/.drime-shell/config.yaml`)
- [ ] Token priority: `DRIME_TOKEN` env → config file → interactive prompt
- [ ] Secure file permissions (0600)

#### 1.3 API Client Foundation
- [ ] Define `DrimeClient` interface
- [ ] Implement HTTP client with:
  - Bearer token authentication
  - Retry logic with exponential backoff
  - Rate limit handling (Retry-After header)
  - Proper error parsing from API responses
- [ ] Implement core endpoints: `getLoggedUser`, `getUserFolders`, `getFileEntries`

#### 1.4 Cache System
- [ ] Implement `FileCache` struct with thread-safe access
- [ ] Implement folder tree loading (`GET /users/{id}/folders`)
- [ ] Build path→ID and ID→path mappings
- [ ] Implement hash calculation (`calculateDrimeHash`)

#### 1.5 Session Management
- [ ] Implement `Session` struct with CWD, cache, client
- [ ] Path resolution (`~`, `-`, `.`, `..`, relative, absolute)
- [ ] Background prefetching framework

#### 1.6 Basic REPL
- [ ] Simple read-eval-print loop
- [ ] Command parsing (split by whitespace)
- [ ] Implement `pwd`, `whoami`, `exit`

### Phase 2: Navigation & Listing (Week 2)

**Goal**: Full navigation with beautiful output.

#### 2.1 Navigation Commands
- [ ] `cd` with path validation, prefetch trigger
- [ ] `ls` with all flags (`-l`, `-a`, `-h`, `-R`, `-t`)
- [ ] `tree` with depth limiting

#### 2.2 UI Components
- [ ] Theme system (auto-detect, dark/light)
- [ ] Styled tables for `ls -l` (lipgloss)
- [ ] Tree rendering (lipgloss)
- [ ] Spinner for loading operations (bubbles/spinner)
- [ ] Error styling

#### 2.3 Background Prefetching
- [ ] Prefetch children on `cd`
- [ ] Prefetch one level deeper (anticipatory)
- [ ] Track in-flight prefetches to avoid duplicates

### Phase 3: File Operations (Week 3)

**Goal**: All file management commands working.

#### 3.1 Read Operations
- [ ] `stat` - Display file metadata
- [ ] `du` - Disk usage (use `/user/space-usage` for root)

#### 3.2 Write Operations
- [ ] `mkdir` with `-p` (create parents)
- [ ] `touch` (upload empty file)
- [ ] `rm` with `-r`, `-f` flags
- [ ] `mv` (move/rename)
- [ ] `cp` (duplicate)

#### 3.3 Cache Invalidation
- [ ] Update cache after mutations
- [ ] Handle race conditions (folder created elsewhere)

### Phase 4: File Viewing (Week 4)

**Goal**: View file contents with syntax highlighting.

#### 4.1 Content Viewing
- [ ] `cat` with line numbers (`-n`)
- [ ] `head` / `tail` with `-n` flag
- [ ] `wc` for line/word/byte counts

#### 4.2 Pager
- [ ] Built-in pager using `bubbles/viewport`
- [ ] Size-based strategy (inline < 100KB, viewport < 10MB, system pager > 10MB)
- [ ] Syntax highlighting with `chroma`
- [ ] Markdown rendering with `glamour`

### Phase 5: Transfers (Week 5)

**Goal**: Upload and download with progress bars.

#### 5.1 Downloads
- [ ] Single file download with progress
- [ ] Recursive folder download (`-r`)
- [ ] Resume support (if API supports range requests)

#### 5.2 Uploads
- [ ] Simple upload for small files (<65MB)
- [ ] Multipart upload for large files
- [ ] Batch URL signing (8 at a time)
- [ ] Per-part retry logic
- [ ] Progress reporting
- [ ] Duplicate detection optimization

#### 5.3 Multi-file Progress
- [ ] Concurrent uploads with worker pool
- [ ] Per-worker status display
- [ ] Overall progress aggregation

### Phase 6: Polish (Week 6)

**Goal**: Production-ready shell.

#### 6.1 Tab Completion
- [ ] Command completion
- [ ] Path completion (use cache)
- [ ] Flag completion

#### 6.2 History
- [ ] Persistent history file
- [ ] Up/down arrow navigation
- [ ] Ctrl+R search

#### 6.3 Additional Commands
- [x] `find` (server-side search with filters)
- [ ] `clear`
- [ ] `help`
- [ ] `config` (show/edit)
- [ ] `theme` (switch themes)

#### 6.4 Error Handling & Edge Cases
- [ ] Network disconnection handling
- [ ] Token expiration
- [ ] Large directory handling (pagination)
- [ ] Glob pattern expansion

### Testing Checklist

- [ ] Unit tests for path resolution
- [ ] Unit tests for cache operations
- [ ] Unit tests for hash calculation
- [ ] Integration tests with mock server
- [ ] Manual testing on different terminals
- [ ] Theme testing (light/dark backgrounds)

1. This document's "Key Design Decisions" section
2. The README.md for user-facing behavior
3. The code comments (once implemented)

When in doubt, prioritize:
1. **User experience** — Beautiful, intuitive output
2. **Simplicity** — Don't over-engineer
3. **SSH-like behavior** — Match user expectations from real shells

---

## Release & Deployment

### Strategy
The project uses a fully automated release pipeline based on **Semantic Versioning** and **Conventional Commits**.

1. **CI/CD**: GitHub Actions run `go test` and `golangci-lint` on every PR and push to `main`.
2. **Versioning**: `release-please` maintains a Release PR using a manifest file (`.release-please-manifest.json`). It is currently configured for **beta releases** (`"prerelease": true` in `release-please-config.json`).
   - Merging the Release PR creates tags like `v1.0.0-beta.1`.
   - To graduate to stable, remove `"prerelease": true` from the config.
3. **Releasing**: When a new tag is pushed, **GoReleaser** builds binaries for Linux, macOS, and Windows (amd64/arm64) and publishes them to GitHub Releases. It automatically detects beta tags and marks them as "Pre-release".
4. **Security**: CodeQL runs on PRs, pushes to `main`, and weekly schedule.

Note: For fully automated releases, configure a repo secret `RELEASE_PLEASE_TOKEN` (PAT) so that CI runs on Release PRs and tag pushes trigger downstream workflows. You must also enable "Allow GitHub Actions to create and approve pull requests" in repo settings.

Workflows are in `.github/workflows/`.

### Installation Scripts

We provide standalone install scripts in `scripts/` that fetch the latest release from GitHub:

- `scripts/install.sh`: For Linux and macOS (Bash/Sh). Installs to `~/.local/bin` by default (override with `BINDIR=...`).
- `scripts/install.ps1`: For Windows (PowerShell).

### Build Targets

- **Linux**: amd64, arm64
- **macOS**: amd64, arm64 (Apple Silicon)
- **Windows**: amd64, arm64

### Conventional Commits

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification to trigger releases:

- `feat` (including `feat(scope): ...`) -> Minor version bump (v1.1.0)
- `fix` (including `fix(scope): ...`) -> Patch version bump (v1.0.1)
- Breaking changes -> Major version bump (use the conventional `!` marker in the header, e.g. `feat!: ...` or `feat(scope)!: ...`)
- `docs:`, `chore:`, `test:` -> No release trigger (no tag created)

Note: Sensitive HTTP capture files (`*.har`) are ignored and should not be committed.

