# AGENTS.md - AI Agent Context

This document provides entry-point context for AI agents working on Drime Shell.

## Quick Start

**What is this?** A Go CLI shell that provides an SSH-like experience for Drime Cloud storage. Commands are translated to API calls, not a real remote shell.

**Key insight:** The shell maintains *virtual* state locally (CWD, file cache). All path lookups go through the cache; mutations invalidate the cache.

## Documentation Index

| Resource | Purpose |
|----------|---------|
| [drime-openapi.yaml](drime-openapi.yaml) | **API source of truth** - all endpoints, schemas |
| [.github/copilot-instructions.md](.github/copilot-instructions.md) | General coding guidelines |
| [.github/instructions/go.instructions.md](.github/instructions/go.instructions.md) | Go patterns and command implementation |

### Specialized Agents

| Agent | Use for |
|-------|---------|
| [planner.agent.md](.github/agents/planner.agent.md) | Implementation planning |
| [reviewer.agent.md](.github/agents/reviewer.agent.md) | Code review |
| [tester.agent.md](.github/agents/tester.agent.md) | Writing tests |

### Domain Knowledge (Skills)

| Skill | Use for |
|-------|---------|
| [drime-api](.github/skills/drime-api/SKILL.md) | API integration, endpoints, S3 uploads |
| [charm-ui](.github/skills/charm-ui/SKILL.md) | Lipgloss styling, Bubbletea components |
| [release-cicd](.github/skills/release-cicd/SKILL.md) | Commits, versioning, GitHub Actions |

### Prompt Templates

| Prompt | Use for |
|--------|---------|
| [add-command.prompt.md](.github/prompts/add-command.prompt.md) | Adding new shell commands |
| [fix-bug.prompt.md](.github/prompts/fix-bug.prompt.md) | Debugging issues |
| [refactor.prompt.md](.github/prompts/refactor.prompt.md) | Code refactoring |

## Architecture Overview

```
cmd/drime/main.go        Entry point
internal/
├── api/                  HTTP client, cache, types
├── commands/             All shell commands
├── session/              Session state (CWD, user, cache)
├── shell/                REPL loop, pipeline, tokenizer
├── ui/                   Charm styling, spinner, table
└── crypto/               Vault AES-256-GCM encryption
```

## Core Concepts

### ID vs Hash

- **Numeric IDs** → mutations (move, delete, copy)
- **Base64 Hashes** → downloads, shareable links
- Hash = `base64(id + "|")` with trailing `=` stripped

### Session State

Commands receive `(ctx, session, env, args)`:
- `session.CWD` - virtual current directory
- `session.Cache` - path→ID mapping
- `session.Client` - API client
- `session.WorkspaceID` - 0 = personal workspace
- `session.InVault` - encrypted context

### Command Pattern

```go
func cmdname(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
    // Parse flags with pflag
    fs := pflag.NewFlagSet("cmdname", pflag.ContinueOnError)
    fs.SetOutput(env.Stderr)
    
    // Resolve paths via session
    resolved, err := s.ResolvePathArg(args[0])
    
    // API calls with spinner
    result, err := ui.WithSpinner(env.Stdout, "", false, func() (T, error) {
        return s.Client.SomeMethod(ctx, ...)
    })
    
    // Update cache after mutations
    s.Cache.Invalidate(resolved)
    
    // Output to env.Stdout (not os.Stdout)
    fmt.Fprintf(env.Stdout, "Result: %v\n", result)
    return nil
}
```

## Decision Summary

These are project-specific decisions that may not be obvious:

1. **Remote-only operations** - `cp`, `mv`, `rm` operate on cloud files only. Use `upload`/`download` for local↔cloud.

2. **Command-based organization** - Trash/Starred/Tracked are managed via commands (`trash ls`, `star ls`), not virtual directories.

3. **S3 presigned uploads** - All uploads go to S3/R2 via presigned URLs, not direct API upload.

4. **MaxPerPage = 9999999999** - Fetch all items in one request, no pagination.

5. **No verbose/interactive flags** - Use spinners/progress bars instead of `-v`. No `-i` confirmations.

6. **Theme: Catppuccin** - Mocha (dark) / Latte (light) palettes with powerline-style prompt.

7. **Vault encryption** - Client-side AES-256-GCM with PBKDF2 key derivation (250k iterations).

## What NOT to do

- Don't commit or push without explicit user approval
- Don't write to `os.Stdout` - use `env.Stdout`
- Don't use `flag` package - use `pflag`
- Don't skip spinners for slow operations
- Don't call API without `WorkspaceID`
- Don't forget cache invalidation after mutations

