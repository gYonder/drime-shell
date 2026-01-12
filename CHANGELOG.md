# Changelog

All notable changes to Drime Shell will be documented here.

---

## [1.1.1](https://github.com/gYonder/drime-shell/compare/v1.1.0...v1.1.1) (2026-01-12)


### Bug Fixes

* run update check in background with proper semver comparison ([7a47334](https://github.com/gYonder/drime-shell/commit/7a47334f928e139f5457960cd4cf6fdf1ab5790f))

## [1.1.0](https://github.com/gYonder/drime-shell/compare/v1.0.0...v1.1.0) (2026-01-12)


### Features

* simplify vault UX - prompt once per session, add vault exit ([fd6afb8](https://github.com/gYonder/drime-shell/commit/fd6afb88b53a83e345eee7b6518b866b1862596c))

## [1.0.0](https://github.com/gYonder/drime-shell/releases/tag/v1.0.0) (2026-01-11)

Initial stable release.

### Features

- SSH-like shell experience with familiar commands (`ls`, `cd`, `cp`, `mv`, `rm`, etc.)
- File transfer with progress bars and duplicate handling
- Workspace support with team collaboration
- Encrypted vault with client-side AES-256-GCM
- Glob patterns, pipes, and redirection
- Tab completion and command history
- Catppuccin theme with Powerline-style prompt
- Checksum verification in install scripts
- Cross-platform support (macOS, Linux, Windows)
