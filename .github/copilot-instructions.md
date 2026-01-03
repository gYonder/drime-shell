# Drime Shell - Copilot Instructions

This is a Go CLI shell application for Drime Cloud storage. It provides an SSH-like experience using virtual filesystem state.

## Project Structure

- `cmd/drime/` - Entry point
- `internal/api/` - HTTP client, caching, API types
- `internal/commands/` - Command implementations
- `internal/shell/` - REPL, pipeline, tokenizer
- `internal/ui/` - Charm libraries for styling
- `internal/session/` - Session state management
- `internal/crypto/` - Vault encryption (AES-256-GCM)

## Key Technical Concepts

1. **Virtual Filesystem**: No remote shell exists. We maintain local CWD and cache, translate commands to API calls.
2. **ID/Hash Mapping**: API uses numeric IDs for mutations, base64 hashes for downloads. Cache maps paths to both.
3. **Folder Tree Caching**: On startup, fetch all folders in one call (`GET /users/{id}/folders`), lazy-load file listings.
4. **Background Prefetching**: When user runs `cd`, prefetch children one level deep using goroutines.

## Coding Standards

- Use `pflag` for command flags (not standard `flag`)
- All commands receive `context.Context`, `*session.Session`, `*ExecutionEnv`, `[]string` args
- Commands write to `env.Stdout`/`env.Stderr`, never directly to `os.Stdout`
- Use `ui.WithSpinner` for operations >100ms
- Resolve paths using `s.ResolvePathArg(path)` before API calls
- Cache invalidation after mutations: `s.Cache.Invalidate(path)` or `s.Cache.AddChildren()`

## Error Handling

- Return `fmt.Errorf("cmd: %v", err)` with command name prefix
- Use `ui.ErrorStyle` for styled error output
- Check for `api.ErrTokenExpired` and suggest `login` command
- Detect SSL errors and provide helpful hints

## UI Guidelines

- Use Charm ecosystem: lipgloss for styling, bubbles for components
- Catppuccin color palette (Mocha for dark, Latte for light)
- Powerline-style prompt segments
- Tables for `ls -l`, `ws members`, etc.
- Progress bars for file transfers

## Testing

- Unit tests alongside source files (`*_test.go`)
- Use table-driven tests with realistic test data
- Mock the `DrimeClient` interface at the boundary, not internal functions
- Test actual behavior and output, not just that functions were called
- Run `make test-race` to catch race conditions
- Existing tests are the spec — don't break them

## Shell Utilities

The shell implements Unix-like utilities operating on remote files:

- **Text**: `cat`, `head`, `tail`, `less`, `wc`, `echo`, `printf`
- **Processing**: `sort`, `uniq`, `diff` (downloads to memory)
- **Metadata**: `stat`, `tree`, `find`, `du`
- **Operations**: `cp`, `mv`, `rm`, `mkdir`, `touch`

### Piping and Redirection

All operations behave as if running on the cloud:

- `|` — Pipes connect stdout→stdin between commands
- `>` — Writes output to **remote** file on Drime Cloud
- `<` — Reads from **remote** file as stdin

Example: `echo "hello" > file.txt` creates `file.txt` on Drime Cloud, not locally.

## Commits & Releases

- Use Conventional Commits: `feat:`, `fix:`, `docs:`, `chore:`, `test:`
- Breaking changes: use `feat!:` or add `BREAKING CHANGE:` footer
- `feat:` → minor version bump, `fix:` → patch bump
- Release-please automates versioning and changelog

## References

- [drime-openapi.yaml](../drime-openapi.yaml) - API specification
