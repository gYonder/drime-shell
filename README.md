# Drime Shell

A modern, beautiful CLI shell for Drime Cloud built in Go. Provides an SSH-like experience for navigating and managing files on your Drime Cloud storage through an intuitive command-line interface.

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![CI](https://github.com/mikael.mansson2/drime-shell/actions/workflows/ci.yml/badge.svg)
![CodeQL](https://github.com/mikael.mansson2/drime-shell/actions/workflows/codeql.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/mikael.mansson2/drime-shell)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

## Features

- **SSH-like Experience** - Navigate your cloud storage with familiar commands (`ls`, `cd`, `mkdir`, `rm`, etc.)
- **Beautiful UI** - Powered by the Charm ecosystem with syntax highlighting, colored output, and styled tables
- **Theme Support** - Auto-detects terminal theme with manual dark/light override
- **Full File Operations** - Copy, move, delete, create files and directories remotely
- **Remote Execution** - `find` and `zip` execute server-side for performance
- **File Transfer** - Upload and download files with progress bars
- **Tab Completion** - Auto-complete paths and commands as you type
- **Command History** - Persistent history across sessions
- **Glob Patterns** - Wildcard support (`*.txt`, `[a-z]*`, `*.{go,rs}`)
- **Workspaces** - Create, switch, and manage multiple workspaces
- **Encrypted Vault** - Zero-knowledge encrypted storage with client-side AES-256-GCM encryption
- **Starred Files** - Mark and filter important files
- **Trash Management** - Safe deletion with restore capability

## Installation
 
### macOS & Linux
 
```bash
curl -fsSL https://raw.githubusercontent.com/mikael.mansson2/drime-shell/main/scripts/install.sh | sh
```
 
### Windows (PowerShell)
 
```powershell
iwr https://raw.githubusercontent.com/mikael.mansson2/drime-shell/main/scripts/install.ps1 | iex
```

## Quick Start

### 1. Configure Authentication

On first run, you'll be prompted for your Drime API token. Alternatively, set it up beforehand:

```bash
# Option 1: Environment variable
export DRIME_TOKEN=drm_your_token_here

# Option 2: Config file (~/.drime-shell/config.yaml)
mkdir -p ~/.drime-shell
echo "token: drm_your_token_here" > ~/.drime-shell/config.yaml
chmod 600 ~/.drime-shell/config.yaml
```

### 2. Launch the Shell

```bash
drime
```

You'll see a prompt like:

```
user@drime:~ $
```

### 3. Start Exploring

```bash
user@drime:~ $ ls
Documents/  Photos/  Projects/  README.md

user@drime:~ $ cd Projects
user@drime:~/Projects $ ls -la
total 3
drwxr-xr-x  user  4.0K  Dec 14 10:30  .
drwxr-xr-x  user  4.0K  Dec 10 09:15  ..
drwxr-xr-x  user  4.0K  Dec 12 14:22  my-app/
-rw-r--r--  user  2.1K  Dec 14 10:30  notes.md
```

## Commands

### Navigation

| Command | Description | Example |
|---------|-------------|---------|
| `pwd` | Print current working directory | `pwd` |
| `cd <path>` | Change directory | `cd ~/Documents`, `cd ..`, `cd -` |
| `ls [options] [path]` | List directory contents | `ls -la`, `ls -Rh` |
| `tree [options] [path]` | Display directory tree | `tree -L 2 -d` |
| `clear` | Clear the terminal screen | `clear` |

#### `ls` Options
- `-l` — Long format (permissions, size, date)
- `-a` — Show hidden files (dotfiles)
- `-h` — Human-readable sizes (1K, 2M, 3G)
- `-R` — Recursive listing
- `-t` — Sort by modification time

#### `tree` Options
- `-L <depth>` — Limit depth
- `-d` — Directories only
- `-a` — Show hidden files

### File Operations

| Command | Description | Example |
|---------|-------------|---------|
| `mkdir [options] <path>` | Create directory | `mkdir -p foo/bar/baz` |
| `touch <file>` | Create empty file or update timestamp | `touch newfile.txt` |
| `cp [options] <src> <dst>` | Copy files/directories (remote) | `cp -r folder/ backup/` |
| `mv [options] <src> <dst>` | Move/rename files (remote) | `mv old.txt new.txt` |
| `rm [options] <path>` | Remove files/directories | `rm -rf old_folder/` |
| `stat <path>` | Display file metadata | `stat myfile.txt` |
| `du [options] [path]` | Show disk usage | `du -sh Projects/` |

#### `mkdir` Options
- `-p` — Create parent directories as needed

#### `cp` Options
- `-r` — Recursive (for directories)
- `-w <id>` — Target workspace ID (for copying across workspaces)

#### `mv` Options
- `-w <id>` — Target workspace ID (for moving across workspaces)

#### `rm` Options
- `-r` — Recursive (for directories)
- `-f` — Force (no confirmation)

#### `du` Options
- `-h` — Human-readable sizes
- `-s` — Summary only
- `-d <depth>` — Limit depth

### Organization & Sharing

| Command | Description | Example |
|---------|-------------|---------|
| `star [command] <file>` | Manage starred files | `star file.txt`, `star ls` |
| `track [command] <file>` | Manage file tracking | `track file.pdf`, `track ls` |
| `trash [command]` | Manage trash | `trash`, `trash restore file.txt` |
| `share [command] <file>` | Share files and manage links | `share link file.txt`, `share invite file.txt user@example.com` |

#### `star` Commands
- `star <file>` — Star a file
- `star ls` — List all starred files
- `star remove <file>` — Unstar a file (alias: `unstar`)

#### `track` Commands
- `track <file>` — Start tracking a file
- `track ls` — List all tracked files
- `track stats <file>` — Show tracking statistics
- `track off <file>` — Stop tracking a file (alias: `untrack`)

#### `trash` Commands
- `trash` (or `trash ls`) — List items in trash
- `trash restore <file>` — Restore file from trash (alias: `restore`)
- `trash empty` — Permanently delete all items in trash

#### `share` Commands
- `share ls` — List shared files
- `share link <file>` — Create/manage shareable link
- `share invite <file> <email>` — Invite users via email

**Share List Options:**
- `--by-me` — Files shared by me (default)
- `--with-me` — Files shared with me
- `--public` — Files with public links

### File Viewing & Processing

| Command | Description | Example |
|---------|-------------|---------|
| `cat [options] <file>` | Display file contents | `cat -n script.py` |
| `head [options] <file>` | Show first lines | `head -n 20 log.txt` |
| `tail [options] <file>` | Show last lines | `tail -n 50 log.txt` |
| `less <file>` | View file with pager | `less largefile.log` |
| `wc [options] <file>` | Count lines/words/bytes | `wc -l data.csv` |
| `diff <file1> <file2>` | Compare two remote files | `diff v1.txt v2.txt` |
| `sort [options] <file>` | Sort lines of a file | `sort -r names.txt` |
| `uniq [options] <file>` | Filter adjacent duplicate lines | `uniq -c sorted_names.txt` |

#### `cat` Options
- `-n` — Number all output lines

#### `head` / `tail` Options
- `-n <num>` — Number of lines (default: 10)

#### `wc` Options
- `-l` — Count lines
- `-w` — Count words
- `-c` — Count bytes

#### `sort` Options
- `-r` — Reverse sort order

#### `uniq` Options
- `-c` — Count occurrences

### Piping and Redirection

Drime Shell supports standard Unix piping and redirection:

*   **Pipes (`|`)**: Chain commands together. Output from the left command becomes input for the right command.
    ```bash
    ls | sort -r
    cat names.txt | sort | uniq -c
    ```

*   **Redirection (`>`)**: Save command output to a **remote** file on Drime Cloud.
    ```bash
    # Create/overwrite a file on Drime Cloud
    ls -l > file_list.txt

    # Combine with pipes
    cat names.txt | sort | uniq > unique_names.txt
    ```

**Note**: Output redirection (`>`) creates files on Drime Cloud, not locally. This maintains the illusion that you're working directly on the remote filesystem. The append operator (`>>`) is not yet supported for remote files.

### Remote Execution

These commands execute on the Drime server for optimal performance:

| Command | Description | Example |
|---------|-------------|---------|
| `find [options] [path]` | Find files (server-side) | `find -name "vacation" -type f` |
| `search [query] [flags]` | Advanced file search | `search "report" --type pdf` |
| `zip <archive> <files...>` | Create zip archive | `zip backup.zip *.txt` |
| `unzip <file>` | Extract zip archive | `unzip backup.zip` |

#### `find` Options

- `-name <pattern>` — File name contains pattern (substring match)
- `-type <f|d>` — File type (f=file, d=directory)
- `-S, --starred` — Only show starred files
- `--trash` — Show items in trash
- `--shared` — Show files shared by me

**Note:** When a path is specified, only direct children of that folder are searched.
For recursive search, omit the path to search the entire workspace.

#### `search` Options

- `--type <type>` — Filter by type (image, video, audio, pdf, text, folder)
- `--owner <id>` — Filter by owner ID
- `--public` — Show only public files
- `--shared` — Show files shared by me (email + links)
- `--link` — Show files with public link
- `--trash` — Show files in trash
- `--starred` — Show starred files
- `--after <date>` — Created after date
- `--before <date>` — Created before date
- `--sort <field>` — Sort by: name, size, created, updated
- `--asc` / `--desc` — Sort direction

### File Transfer

| Command | Description | Example |
|---------|-------------|---------|
| `upload [options] <local> [remote]` | Upload to Drime | `upload -r ./myproject ~/Projects/` |
| `download [options] <remote> [local]` | Download from Drime | `download -r ~/Photos ./backup/` |

#### `upload` Options
- `-r` — Recursive (for directories)
- `--on-duplicate <action>` — How to handle duplicate files:
  - `ask` — Prompt for each duplicate (default)
  - `replace` — Overwrite existing files
  - `rename` — Keep both with numbered suffix (e.g., `file (1).txt`)
  - `skip` — Skip uploading duplicates

#### `download` Options
- `-r` — Recursive (for directories)

Transfers display a progress bar with ETA and transfer speed.

#### Handling Duplicates

When uploading files that already exist at the destination, use `--on-duplicate` to control behavior:

```bash
# Prompt for each duplicate (default)
upload photo.jpg

# Automatically skip existing files
upload --on-duplicate skip ./photos /Photos/

# Replace all existing files
upload --on-duplicate replace backup.zip

# Keep both - duplicates get numbered suffix
upload --on-duplicate rename document.pdf
```

### Workspaces

Workspaces allow you to organize files into separate spaces, each with its own folder structure and optional team collaboration.

| Command | Description | Example |
|---------|-------------|---------|
| `ws` | List all workspaces | `ws` |
| `ws <name\|id>` | Switch to a workspace | `ws MyProject`, `ws 1234` |
| `ws 0` or `ws default` | Switch to default workspace | `ws default` |
| `ws new <name>` | Create a new workspace | `ws new "Team Project"` |
| `ws rename <name>` | Rename current workspace | `ws rename "New Name"` |
| `ws rm [id]` | Delete a workspace (with confirmation) | `ws rm`, `ws rm 1234` |

**Member Management:**

| Command | Description | Example |
|---------|-------------|---------|
| `ws members` | List members and pending invites | `ws members` |
| `ws roles` | List available roles | `ws roles` |
| `ws invite <email> [role]` | Invite a user | `ws invite user@example.com`, `ws invite user@example.com admin` |
| `ws kick <email>` | Remove member or cancel invite | `ws kick user@example.com` |
| `ws role <email> <role>` | Change member's role | `ws role user@example.com admin` |
| `ws leave` | Leave the current workspace | `ws leave` |

**Notes:**
- The current workspace is shown in your prompt: `[MyWorkspace] user@drime:~ $`
- Switching workspaces resets your working directory to `/` and shows stats
- Cannot rename or delete the default workspace (ID 0)
- Deleting the current workspace automatically switches to default
- Destructive actions (rm) require confirmation

### Shell Built-ins

| Command | Description | Example |
|---------|-------------|---------|
| `alias [name=value]` | Create or list aliases | `alias ll='ls -la'` |
| `unalias <name>` | Remove an alias | `unalias ll` |
| `whoami` | Show current user info | `whoami` |
| `history` | Show command history | `history` |
| `clear` | Clear screen | `clear` |
| `help [command]` | Show help | `help ls` |

### Starred Files

Mark important files for quick access and filtering.

| Command | Description | Example |
|---------|-------------|---------|
| `star <file>...` | Mark files as starred | `star important.pdf`, `star *.doc` |
| `unstar <file>...` | Remove starred status | `unstar old.pdf` |
| `ls -S` | List only starred files | `ls -lS` |
| `find -S -name <query>` | Search only starred files | `find -S -name report` |

**Notes:**
- Starred files show a ★ indicator in `ls -l` output
- Glob patterns work with star/unstar commands
- Starred status persists across sessions

### Trash Management

Deleted files go to trash first, allowing recovery before permanent deletion.

| Command | Description | Example |
|---------|-------------|---------|
| `rm <file>` | Move to trash (default) | `rm oldfile.txt` |
| `rm -F <file>` | Delete permanently (skip trash) | `rm -F --forever temp.log` |
| `trash` or `trash ls` | List items in trash | `trash` |
| `trash restore <file>` | Restore from trash | `trash restore document.pdf` |
| `trash empty` | Permanently delete all trash | `trash empty` |

**Notes:**
- `rm` moves to trash by default (use `-F` or `--forever` to skip)
- Trash is per-workspace
- Use `trash restore` (or alias `restore`) to recover files

### File Requests

Create public upload links where others can upload files directly to a folder.

| Command | Description | Example |
| --- | --- | --- |
| `request ls` | List active file requests | `request ls` |
| `request create <folder>` | Create a new file request | `request create ./Uploads --title "Project X"` |
| `request rm <id>` | Remove a file request | `request rm 12345` |

**Options for `create`:**

- `--title`: Set a custom title (defaults to folder name)
- `--desc`: Set a description
- `--expire`: Set expiration date (YYYY-MM-DD)
- `--password`: Set a password
- `--custom-link`: Set a custom URL suffix

### Encrypted Vault

The vault is a zero-knowledge encrypted storage space where your files are encrypted client-side before upload. Only you hold the encryption key - not even Drime can read your vault contents.

#### Encryption Details
- **Algorithm**: AES-256-GCM with random 12-byte IV per file
- **Key Derivation**: PBKDF2-SHA256 with 250,000 iterations
- **Security**: Files are encrypted locally before upload and decrypted after download

| Command | Description | Example |
|---------|-------------|---------|
| `vault` | Enter/switch to vault (prompts unlock if locked) | `vault` |
| `vault unlock` | Unlock vault with password | `vault unlock` |
| `vault lock` | Lock vault (clears key from memory) | `vault lock` |
| `vault init` | Initialize a new vault (first time setup) | `vault init` |

#### Vault Prompt

When in the vault, the prompt shows the vault status:
```bash
[vault:locked] user@drime:~ $      # Vault is locked
[vault:unlocked] user@drime:~ $    # Vault is unlocked and ready
```

#### Cross-Transfer: Moving Files To/From Vault

Transfer files between regular workspace and vault using special flags:

```bash
# When in a workspace, copy files TO vault
user@drime:~ $ cp --vault document.pdf /
Copied: document.pdf -> vault:/document.pdf (encrypted)

# When in a workspace, move files TO vault (copy + delete source)
user@drime:~ $ mv --vault sensitive.txt /private/
Copied: sensitive.txt -> vault:/private/sensitive.txt (encrypted)
Deleted: sensitive.txt

# When in vault, copy files TO a workspace
[vault:unlocked] user@drime:~ $ cp -w 0 secret.txt /Documents/
Copied: vault:secret.txt -> workspace 0 (decrypted)

# When in vault, move files TO a workspace
[vault:unlocked] user@drime:~ $ mv -w 0 report.pdf /
Copied: vault:report.pdf -> workspace 0 (decrypted)
Deleted: vault:report.pdf
```

#### Working in the Vault

When inside the vault, standard commands work normally:

```bash
[vault:unlocked] user@drime:~ $ ls
Documents/  Photos/  private-key.pem

[vault:unlocked] user@drime:~ $ cd Documents
[vault:unlocked] user@drime:~/Documents $ upload ~/tax-return.pdf
Uploaded: /Documents/tax-return.pdf (encrypted)

[vault:unlocked] user@drime:~/Documents $ download tax-return.pdf ~/
Downloaded: ~/tax-return.pdf (decrypted)

[vault:unlocked] user@drime:~/Documents $ cat notes.txt
(file is decrypted and displayed)
```

**Key Differences from Regular Workspaces:**
- No trash - all deletes are permanent (`rm` behaves like `rm -F`)
- No duplicate detection (matches web app behavior)
- No starred files support
- Files are encrypted on upload, decrypted on download/view
- Must unlock vault before any file operations

**Security Best Practices:**
- Use `vault lock` when stepping away
- Vault key is stored only in memory - restart clears it
- Password is never stored; only used to derive the encryption key
- Each file has a unique IV for added security

### Session & Configuration

| Command | Description | Example |
|---------|-------------|---------|
| `whoami` | Display current user and workspace | `whoami` |
| `history` | Show command history | `history` |
| `help [command]` | Show help | `help ls`, `help ws` |
| `theme <auto\|dark\|light>` | Change color theme | `theme dark` |
| `config show` | Display configuration | `config show` |
| `config set <key> <value>` | Update configuration | `config set theme light` |
| `config path` | Show config file location | `config path` |
| `exit` / `quit` | Exit the shell | `exit` |

### Authentication

| Command | Description | Example |
|---------|-------------|---------|
| `login [email]` | Log in to Drime Cloud | `login`, `login user@example.com` |
| `logout` | Remove stored credentials | `logout` |

#### `login`
Authenticates with Drime Cloud using email and password. The token is saved to `~/.drime-shell/config.yaml`.

```bash
# Interactive login (prompts for email and password)
user@drime:~ $ login
Email: user@example.com
Password: ********
✓ Logged in as user@example.com
Token saved to ~/.drime-shell/config.yaml

# Provide email upfront (only prompts for password)
user@drime:~ $ login user@example.com
Password: ********
✓ Logged in as user@example.com
```

**Note:** You can also set `DRIME_TOKEN` environment variable to skip interactive login.

### Glob Patterns

Wildcards are expanded against the remote filesystem before commands execute:

| Pattern | Description | Example |
|---------|-------------|---------|
| `*` | Any characters (zero or more) | `*.txt` matches all text files |
| `?` | Exactly one character | `file?.go` matches `file1.go`, `fileA.go` |
| `[abc]` | One character in set | `[abc].txt` matches `a.txt`, `b.txt`, `c.txt` |
| `[a-z]` | One character in range | `[A-Z]*.md` matches uppercase-starting markdown |
| `[!abc]` or `[^abc]` | One character NOT in set | `[!.]*.txt` matches non-hidden text files |
| `{a,b,c}` | Brace expansion (alternatives) | `*.{go,rs}` matches `.go` and `.rs` files |

#### Examples

```bash
# Basic wildcards
ls *.txt                    # All .txt files
rm data_?.csv               # data_1.csv, data_2.csv, etc.
cat [A-Z]*.md               # Markdown files starting with uppercase

# Brace expansion
ls *.{yaml,json,toml}       # All config files
cp {main,util}.go backup/   # Copy main.go and util.go
cat config.{yaml,json}      # View multiple config files

# Character classes
rm [0-9]*.log               # Remove logs starting with numbers
ls [!.]*                    # List non-hidden files

# Combining patterns
mv backup/*.{jpg,png} photos/   # Move all images from backup
```

#### Not Supported

The following bash extended glob patterns (`extglob`) are **not** supported:

| Pattern | Description |
|---------|-------------|
| `!(pattern)` | Negation (everything except pattern) |
| `+(pattern)` | One or more occurrences |
| `?(pattern)` | Zero or one occurrence |
| `@(pattern)` | Exactly one of patterns |

If you need these, use multiple commands or `find` with filters instead.

## Configuration

Configuration is stored in `~/.drime-shell/config.yaml`. You can edit this file manually with any text editor, or use the built-in `config` commands.

### Config File Format

```yaml
# Drime Shell Configuration

# Color theme: auto, dark, or light
theme: auto

# Authentication token (keep this file secure!)
token: drm_xxxxxxxxxxxxxxxxxxxx

# API endpoint (optional, defaults to production)
api_url: https://api.drime.cloud

# Starting directory when shell launches
default_path: /

# Number of commands to keep in history
history_size: 1000
```

### Manual Editing

You can edit the config file directly:

```bash
# Open with your preferred editor
nano ~/.drime-shell/config.yaml
vim ~/.drime-shell/config.yaml
code ~/.drime-shell/config.yaml
```

Changes take effect on the next shell startup. If you're already in a Drime Shell session, use `config reload` to apply changes without restarting.

**Tips:**
- Keep a backup before making changes
- Ensure proper YAML syntax (spaces, not tabs)
- Don't commit this file to version control (contains your token)

### Token Priority

The authentication token is loaded in this order:

1. `DRIME_TOKEN` environment variable (highest priority)
2. `~/.drime-shell/config.yaml` file
3. Interactive prompt (if neither exists)

### Security

- Config file is created with `0600` permissions (owner read/write only)
- A warning is displayed if permissions are too open
- Token is masked in `config show` output

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Tab` | Auto-complete paths and commands |
| `↑` / `↓` | Navigate command history |
| `Ctrl+C` | Cancel current command |
| `Ctrl+D` | Exit shell |
| `Ctrl+L` | Clear screen |
| `Ctrl+R` | Search command history |

### In Pager (`less`)

| Shortcut | Action |
|----------|--------|
| `↑` / `↓` | Scroll line by line |
| `Page Up` / `Page Down` | Scroll by page |
| `g` / `G` | Go to top / bottom |
| `/` | Search (if enabled) |
| `q` | Quit pager |

## Themes

Drime Shell auto-detects your terminal's color scheme and adapts accordingly. Override with:

```bash
# Set theme for current session
theme dark
theme light
theme auto

# Set theme permanently
config set theme dark
```

### Color Coding

- 📁 **Blue** — Directories
- 📄 **White/Default** — Regular files
- ⚡ **Green** — Executable files
- 🔗 **Cyan** — Symbolic links
- 🔴 **Red** — Errors and warnings

## Syntax Highlighting

File contents are automatically syntax-highlighted based on extension:

- **Markdown** (`.md`) — Rendered with Glamour
- **Code** (`.go`, `.py`, `.js`, `.json`, `.yaml`, etc.) — Highlighted with Chroma
- **Plain text** — No highlighting

## Architecture

Drime Shell operates as a **virtual filesystem shell**. There is no real shell running on the server — instead:

1. **Local state**: Maintains CWD, command history, and a cache of folder/file IDs
2. **API translation**: Converts shell commands to Drime Cloud REST API calls
3. **Smart caching**: On startup, fetches the complete folder tree in one call, then lazily loads file listings as needed
4. **Background prefetching**: When you `cd` into a folder, the shell prefetches children one level deep for instant navigation

```text
drime-shell/
├── cmd/drime/           # Entry point
├── internal/
│   ├── api/             # DrimeClient interface, HTTP client, caching
│   ├── shell/           # REPL loop, session state, prefetching
│   ├── commands/        # Individual command implementations
│   ├── ui/              # Lipgloss styles, themes, spinners, progress bars
│   └── pager/           # Bubbletea viewport wrapper
├── AGENTS.md            # Technical context for AI agents
├── drime-openapi.yaml   # API specification (source of truth)
├── README.md            # This file
└── go.mod
```

### Performance Design

| Feature | Implementation |
|---------|----------------|
| Startup | Single API call fetches all folders via `GET /users/{id}/folders` |
| Navigation | Folder tree already cached, `cd` is instant |
| Listing | Background prefetch on `cd`, `ls` uses cached data |
| Large files | Multipart upload with 60MB chunks, batched URL signing |
| Reliability | Automatic retries with exponential backoff and jitter |

For detailed implementation patterns, see [AGENTS.md](AGENTS.md).

## Development

### Prerequisites

- Go 1.21 or later
- Access to Drime Cloud API

### Building

```bash
# Development build
go build -o drime ./cmd/drime

# Run tests
go test ./...

# Run with race detector
go run -race ./cmd/drime
```

### Dependencies

| Library | Purpose |
|---------|---------|
| `charmbracelet/lipgloss` | Terminal styling and layouts |
| `charmbracelet/bubbletea` | TUI framework |
| `charmbracelet/bubbles` | UI components (viewport, spinner, progress) |
| `charmbracelet/glamour` | Markdown rendering |
| `charmbracelet/log` | Styled logging |
| `alecthomas/chroma` | Syntax highlighting |
| `bmatcuk/doublestar/v4` | Glob pattern matching |
| `sergi/go-diff` | Diff output |

## Releases

This repo uses **release-please** + **GoReleaser**:

1. Merge Conventional Commits into `main`.
2. `release-please` opens/updates a Release PR.
3. Merge the Release PR to create a SemVer tag (GoReleaser expects tags starting with `v`, e.g. `v1.2.3`).
4. GoReleaser runs on the tag and publishes a GitHub Release + binaries.

To make this fully automated, set a repo secret `RELEASE_PLEASE_TOKEN` (a PAT) so that tags/PRs created by release-please can trigger downstream workflows.

Security scanning is handled by CodeQL on PRs, pushes to `main`, and a weekly schedule.

## Troubleshooting

### "Permission denied" when reading config

```bash
chmod 600 ~/.drime-shell/config.yaml
```

### Token not working

1. Verify token in environment: `echo $DRIME_TOKEN`
2. Check config file: `cat ~/.drime-shell/config.yaml`
3. Test with `whoami` command after connecting

### "Session expired" message

Your authentication token has expired or become invalid. Re-authenticate:

```bash
user@drime:~ $ login
```

This will prompt for email and password and save a new token.

### Colors not displaying

- Ensure your terminal supports ANSI colors
- Try `theme dark` or `theme light` to override auto-detection
- Check `TERM` environment variable

### Command not found

Run `help` to see all available commands. Some commands may require specific API permissions.

### SSL/TLS connection errors

If you see SSL-related errors:

- Check your network connection and try again
- If using a VPN, try temporarily disabling it
- Ensure your system certificates are up to date
- The shell will provide specific hints for common SSL issues

## Contributing

Contributions are welcome! Please see [AGENTS.md](AGENTS.md) for technical details about the architecture and implementation decisions.

## License

MIT License - see LICENSE file for details.

## Acknowledgments

- [Charm](https://charm.sh/) for the beautiful TUI libraries
- The Go community for excellent tooling
