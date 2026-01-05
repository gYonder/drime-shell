# Drime Shell

A modern CLI shell for Drime Cloud built in Go. Provides an SSH-like experience for navigating and managing files on your cloud storage.

![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)
![CI](https://github.com/mikael-mansson/drime-shell/actions/workflows/ci.yml/badge.svg)
![CodeQL](https://github.com/mikael-mansson/drime-shell/actions/workflows/codeql.yml/badge.svg)
![Release](https://img.shields.io/github/v/release/mikael-mansson/drime-shell)
![License](https://img.shields.io/badge/License-MIT-blue.svg)

## Features

- **SSH-like Experience** — Familiar commands: `ls`, `cd`, `mkdir`, `rm`, `cp`, `mv`, etc.
- **Beautiful UI** — Syntax highlighting, colored output, Powerline-style prompt
- **File Transfer** — Upload/download with progress bars and duplicate handling
- **Workspaces** — Organize files into separate spaces with team collaboration
- **Encrypted Vault** — Zero-knowledge AES-256-GCM encryption
- **Glob Patterns** — Wildcards: `*.txt`, `[a-z]*`, `*.{go,rs}`
- **Pipes & Redirection** — `ls | sort`, `cat file.txt > output.txt`
- **Tab Completion** — Auto-complete paths and commands

## Installation
 
**macOS & Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/mikael-mansson/drime-shell/main/scripts/install.sh | sh
```
 
**Windows (PowerShell):**
```powershell
iwr https://raw.githubusercontent.com/mikael-mansson/drime-shell/main/scripts/install.ps1 | iex
```

## Quick Start

### Authentication

On first run, you'll be prompted for your Drime API token. Or set it up beforehand:

```bash
# Environment variable
export DRIME_TOKEN=drm_your_token_here

# Or config file
mkdir -p ~/.drime-shell && echo "token: drm_your_token_here" > ~/.drime-shell/config.yaml
```

### Launch

```bash
drime
```

The shell uses a Powerline-style prompt showing your username and current path:

<pre style="background:#1e1e2e;color:#cdd6f4;padding:12px;border-radius:8px;font-family:monospace;">
<span style="background:#89b4fa;color:#1e1e2e"> mikael.maansson </span><span style="background:#313244;color:#89b4fa">&#xe0b0;</span><span style="background:#313244;color:#cdd6f4"> ~/Projects </span><span style="color:#313244">&#xe0b0;</span>
</pre>

When in a workspace or vault, additional segments appear:

<pre style="background:#1e1e2e;color:#cdd6f4;padding:12px;border-radius:8px;font-family:monospace;">
<span style="background:#89b4fa;color:#1e1e2e"> mikael.maansson </span><span style="background:#313244;color:#89b4fa">&#xe0b0;</span><span style="background:#313244;color:#cdd6f4"> ~ </span><span style="background:#cba6f7;color:#313244">&#xe0b0;</span><span style="background:#cba6f7;color:#1e1e2e"> test </span><span style="color:#cba6f7">&#xe0b0;</span>
</pre>

<pre style="background:#1e1e2e;color:#cdd6f4;padding:12px;border-radius:8px;font-family:monospace;">
<span style="background:#89b4fa;color:#1e1e2e"> mikael.maansson </span><span style="background:#313244;color:#89b4fa">&#xe0b0;</span><span style="background:#313244;color:#cdd6f4"> ~ </span><span style="background:#a6e3a1;color:#313244">&#xe0b0;</span><span style="background:#a6e3a1;color:#1e1e2e"> vault:unlocked </span><span style="color:#a6e3a1">&#xe0b0;</span>
</pre>

### Example Session

<pre style="background:#1e1e2e;color:#cdd6f4;padding:12px;border-radius:8px;font-family:monospace;">
<span style="background:#89b4fa;color:#1e1e2e"> mikael.maansson </span><span style="background:#313244;color:#89b4fa">&#xe0b0;</span><span style="background:#313244;color:#cdd6f4"> ~ </span><span style="color:#313244">&#xe0b0;</span> ls
<span style="color:#cba6f7">222704736.pdf.zip</span>                           ex.txt
<span style="color:#cba6f7">6089912.pdf</span>                                 ex.txt - Copy
<span style="color:#89b4fa">Bildsamling</span>                                 foo
<span style="color:#cba6f7">Firefox 144.0.2.dmg</span>                         foobar
<span style="color:#f9e2af">IMG_50291.jpg</span>                               <span style="color:#a6e3a1">rename.ts</span>
<span style="color:#cdd6f4">README.md</span>                                   <span style="color:#89b4fa">temp</span>
<span style="color:#cba6f7">Tipsruta till Tomtepromenaden 2025.pdf</span>      <span style="color:#cba6f7">test.zip</span>
<span style="color:#89b4fa">UploadFromMobile</span>                            <span style="color:#89b4fa">xxx</span>
<span style="background:#89b4fa;color:#1e1e2e"> mikael.maansson </span><span style="background:#313244;color:#89b4fa">&#xe0b0;</span><span style="background:#313244;color:#cdd6f4"> ~ </span><span style="color:#313244">&#xe0b0;</span> cat ex.txt | head -n 5
sdlkfjkadsjf

sfasdfdsf

adsff
</pre>

## Commands

Use `<command> -h` for detailed help on any command.

### Navigation

| Command | Description |
|---------|-------------|
| `ls` | List directory contents (`-l` long, `-a` hidden, `-S` starred) |
| `cd` | Change directory (`~` home, `-` previous, `..` parent) |
| `pwd` | Print working directory |
| `tree` | Display directory tree |

### File Operations

| Command | Description |
|---------|-------------|
| `mkdir` | Create directories (`-p` for parents) |
| `touch` | Create empty file |
| `cp` | Copy files (`-r` recursive, `-w` cross-workspace, `--vault`) |
| `mv` | Move/rename files (`-w` cross-workspace, `--vault`) |
| `rm` | Remove files (`-r` recursive, `-F` permanent) |
| `stat` | Display file metadata |

### File Viewing

| Command | Description |
|---------|-------------|
| `cat` | Display file contents |
| `head` / `tail` | Show first/last lines |
| `wc` | Count lines/words/bytes |
| `diff` | Compare two files |
| `sort` / `uniq` | Sort lines, filter duplicates |
| `edit` | Edit file in built-in editor |

### Search

| Command | Description |
|---------|-------------|
| `find` | Search files (`-name`, `-type f/d`, `-S` starred) |
| `search` | Advanced search (`--type`, `--after`, `--shared`, etc.) |

### Transfer

| Command | Description |
|---------|-------------|
| `upload` | Upload local files (`--on-duplicate ask/replace/rename/skip`) |
| `download` | Download to local filesystem |

### Organization

| Command | Description |
|---------|-------------|
| `star` / `unstar` | Star/unstar files |
| `trash` / `restore` | Manage trash |
| `track` / `untrack` | Track file views/downloads |
| `share` | Share files (links, email invites) |
| `request` | Manage file upload requests |
| `ws` | List/switch workspaces, manage members |

Use `ws -h` for full workspace management options (create, rename, delete, invite, kick, etc.).

### Encrypted Vault

Zero-knowledge encrypted storage with client-side AES-256-GCM encryption.

| Command | Description |
|---------|-------------|
| `vault` | Enter vault (prompts unlock if locked) |
| `vault unlock` | Unlock with password |
| `vault lock` | Lock (clears key from memory) |
| `vault init` | First-time setup |

When in vault, the prompt shows status:

<pre style="background:#1e1e2e;color:#cdd6f4;padding:12px;border-radius:8px;font-family:monospace;">
<span style="background:#89b4fa;color:#1e1e2e"> mikael.maansson </span><span style="background:#313244;color:#89b4fa">&#xe0b0;</span><span style="background:#313244;color:#cdd6f4"> ~ </span><span style="background:#a6e3a1;color:#313244">&#xe0b0;</span><span style="background:#a6e3a1;color:#1e1e2e"> vault:unlocked </span><span style="color:#a6e3a1">&#xe0b0;</span>
</pre>

Cross-transfer using `--vault` flag or `-w <workspace>`:
```bash
cp --vault secret.pdf /          # Workspace → Vault (encrypts)
cp -w 0 secret.pdf /Documents/   # Vault → Workspace (decrypts)
```

**Vault differences:** No trash (deletes are permanent), no starring, files encrypted on upload.

### Other Commands

| Command | Description |
|---------|-------------|
| `alias` / `unalias` | Manage command aliases |
| `whoami` | Show current user |
| `history` | Show command history |
| `theme` | Set color theme (`auto`, `dark`, `light`) |
| `config` | View/edit configuration |
| `login` / `logout` | Manage authentication |
| `zip` / `unzip` | Create/extract archives (server-side) |
| `echo` / `printf` | Output text |
| `help` | Show help |
| `exit` | Exit shell |

## Glob Patterns

| Pattern | Description |
|---------|-------------|
| `*` | Any characters |
| `?` | Single character |
| `[abc]` / `[a-z]` | Character class |
| `{a,b}` | Alternatives |

```bash
ls *.txt                    # All text files
rm *.{log,tmp}              # Remove logs and temp files
cp [A-Z]*.md backup/        # Copy capitalized markdown files
```

## Pipes & Redirection

```bash
ls | sort -r                        # Pipe to sort
cat names.txt | sort | uniq -c      # Chain multiple commands
ls -l > listing.txt                 # Redirect to remote file
```

**Note:** Output redirection creates files on Drime Cloud, not locally.

## Configuration

Stored in `~/.drime-shell/config.yaml`:

```yaml
theme: auto
token: drm_xxxxxxxxxxxxxxxxxxxx
history_size: 1000
```

Token priority: `DRIME_TOKEN` env var → config file → interactive prompt.

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Tab` | Auto-complete |
| `↑` / `↓` | History navigation |
| `Ctrl+C` | Cancel |
| `Ctrl+D` | Exit |
| `Ctrl+L` | Clear screen |

## Troubleshooting

**Permission denied:** `chmod 600 ~/.drime-shell/config.yaml`

**Session expired:** Run `login` to re-authenticate.

**Colors broken:** Try `theme dark` or check `TERM` variable.

## Development

```bash
go build -o drime ./cmd/drime   # Build
go test ./...                    # Test
```

See [AGENTS.md](AGENTS.md) for architecture details.

## License

MIT
