package session

import (
	"path/filepath"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/crypto"
)

type Session struct {
	Client            api.DrimeClient
	Cache             *api.FileCache
	HistoryGetter     func() []string
	Aliases           map[string]string // User-defined command aliases
	CWD               string
	HomeDir           string
	PreviousDir       string
	Username          string
	Token             string
	UserID            int64
	WorkspaceID       int64           // Current workspace (0 = default)
	WorkspaceName     string          // Name of current workspace (empty = default)
	Workspaces        []api.Workspace // Cached list of available workspaces
	MaxMemoryBufferMB int             // Max MB for in-memory operations before using temp files

	// Vault state
	InVault       bool             // True when vault is the active context
	VaultID       int64            // Vault ID from API
	VaultUnlocked bool             // True when vault encryption key is loaded
	VaultKey      *crypto.VaultKey // In-memory encryption key (nil when locked)
	VaultSalt     []byte           // Salt for key derivation (cached from API)
	VaultCheckIV  []byte           // IV for check value decryption
	VaultCheck    []byte           // Encrypted check value for password verification

	// Saved workspace state (for returning from vault)
	SavedWorkspaceID   int64
	SavedWorkspaceName string
	SavedCWD           string
	SavedCache         *api.FileCache
}

type ViewMode string

const (
	ViewNormal ViewMode = "normal"
)

// MaxMemoryBytes returns the max memory buffer size in bytes
func (s *Session) MaxMemoryBytes() int64 {
	if s.MaxMemoryBufferMB <= 0 {
		return 100 * 1024 * 1024 // Default 100MB
	}
	return int64(s.MaxMemoryBufferMB) * 1024 * 1024
}

func NewSession(client api.DrimeClient, cache *api.FileCache) *Session {
	s := &Session{
		CWD:     "/",
		HomeDir: "/",
		Client:  client,
		Cache:   cache,
		Aliases: make(map[string]string),
	}

	// Default aliases
	s.Aliases["ll"] = "ls -la"
	s.Aliases["la"] = "ls -a"
	s.Aliases["quit"] = "exit"
	s.Aliases["workspace"] = "ws"
	s.Aliases["workspaces"] = "ws"
	s.Aliases["untrack"] = "track off"
	s.Aliases["unstar"] = "star remove"
	s.Aliases["restore"] = "trash restore"

	return s
}

// VirtualCWD returns the user-facing CWD (same as CWD now that virtual views are removed).
func (s *Session) VirtualCWD() string {
	return s.CWD
}

func (s *Session) ResolvePath(path string) string {
	if path == "" {
		return s.CWD
	}

	if path == "-" {
		if s.PreviousDir == "" {
			return s.CWD
		}
		return s.PreviousDir
	}

	if path == "~" {
		return s.HomeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(s.HomeDir, path[2:])
	}

	var absolute string
	if filepath.IsAbs(path) {
		absolute = path
	} else {
		absolute = filepath.Join(s.CWD, path)
	}

	return filepath.Clean(absolute)
}

// ResolvePathArg resolves a user-supplied path argument.
func (s *Session) ResolvePathArg(path string) (string, error) {
	return s.ResolvePath(path), nil
}

// ContextName returns a display name for the current context (workspace or vault).
// Used in the shell prompt. Returns empty string for default workspace.
func (s *Session) ContextName() string {
	if s.InVault {
		if s.VaultUnlocked {
			return "vault:unlocked"
		}
		return "vault:locked"
	}
	// Don't show context for default workspace (ID 0)
	if s.WorkspaceID == 0 {
		return ""
	}
	return s.WorkspaceName
}

// IsVaultUnlocked returns true if the vault is currently unlocked (key is loaded).
func (s *Session) IsVaultUnlocked() bool {
	return s.VaultUnlocked && s.VaultKey != nil
}

// ClearVaultKey securely clears the vault encryption key from memory.
func (s *Session) ClearVaultKey() {
	if s.VaultKey != nil {
		s.VaultKey.Zero()
		s.VaultKey = nil
	}
	s.VaultUnlocked = false
}

// SetVaultKey sets the vault encryption key.
func (s *Session) SetVaultKey(key *crypto.VaultKey) {
	// Clear any existing key first
	s.ClearVaultKey()
	s.VaultKey = key
	s.VaultUnlocked = true
}

// SaveWorkspaceState saves the current workspace state before switching to vault.
func (s *Session) SaveWorkspaceState() {
	s.SavedWorkspaceID = s.WorkspaceID
	s.SavedWorkspaceName = s.WorkspaceName
	s.SavedCWD = s.CWD
	s.SavedCache = s.Cache
}

// RestoreWorkspaceState restores the saved workspace state when leaving vault.
func (s *Session) RestoreWorkspaceState() {
	s.WorkspaceID = s.SavedWorkspaceID
	s.WorkspaceName = s.SavedWorkspaceName
	s.CWD = s.SavedCWD
	if s.SavedCache != nil {
		s.Cache = s.SavedCache
	}
	s.InVault = false
}

// SwitchToVault switches the session to vault context.
// The caller should have already set up VaultKey and loaded the vault cache.
func (s *Session) SwitchToVault(vaultID int64, vaultCache *api.FileCache) {
	// Save current workspace state
	if !s.InVault {
		s.SaveWorkspaceState()
	}

	s.InVault = true
	s.VaultID = vaultID
	s.Cache = vaultCache
	s.CWD = "/"
	s.PreviousDir = ""
}
