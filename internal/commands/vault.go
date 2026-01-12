package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/crypto"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
	"golang.org/x/term"
)

// readPassword reads a password from stdin with masking.
// Falls back to plain text reading for non-terminal stdin (tests).
func readPassword(env *ExecutionEnv) (string, error) {
	// Check if stdin is a file (e.g., os.Stdin) - terminal needs special handling
	if f, ok := env.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(env.Stdout) // Newline after password
		if err != nil {
			return "", err
		}
		return string(passwordBytes), nil
	}

	// Fallback for non-terminal stdin (tests, pipes)
	// Read byte by byte to avoid bufio buffering issues
	var password []byte
	buf := make([]byte, 1)
	for {
		n, err := env.Stdin.Read(buf)
		if err != nil {
			if len(password) > 0 {
				break // Return what we have
			}
			return "", err
		}
		if n == 0 {
			break
		}
		if buf[0] == '\n' {
			break
		}
		if buf[0] != '\r' { // Skip carriage return
			password = append(password, buf[0])
		}
	}
	return string(password), nil
}

func init() {
	Register(&Command{
		Name:        "vault",
		Description: "Enter encrypted vault or initialize a new vault",
		Usage: `vault [command]

Enter vault:
  vault               Enter encrypted vault (prompts for password on first access)
  vault exit          Return to previous workspace

First-time setup:
  vault init          Initialize a new vault with a password

Cross-transfer (when in vault):
  cp file.txt /path -w <name|id>   Copy from vault to workspace (decrypts)
  mv file.txt /path -w <name|id>   Move from vault to workspace (decrypts)

Cross-transfer (when in workspace):
  cp file.txt /path --vault        Copy from workspace to vault (encrypts)
  mv file.txt /path --vault        Move from workspace to vault (encrypts)

Notes:
  - Vault uses client-side AES-256-GCM encryption
  - Password is prompted once per session on first vault access
  - Files are encrypted before upload and decrypted after download
  - Vault deletes are permanent (no trash)`,
		Run: vaultCmd,
	})
}

func vaultCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) == 0 {
		return switchToVault(ctx, s, env)
	}

	switch strings.ToLower(args[0]) {
	case "exit":
		return exitVault(ctx, s, env)
	case "init", "create":
		return initVault(ctx, s, env)
	default:
		return fmt.Errorf("unknown vault command: %s (use 'help vault' for usage)", args[0])
	}
}

func EnsureVaultUnlocked(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	if s.IsVaultUnlocked() {
		return nil
	}

	vaultMeta, err := ui.WithSpinner(env.Stdout, "", false, func() (*api.VaultMeta, error) {
		return s.Client.GetVaultMetadata(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to check vault: %w", err)
	}

	if vaultMeta == nil {
		return fmt.Errorf("no vault found - run 'vault init' to create one")
	}

	s.VaultID = vaultMeta.ID
	s.VaultSalt = []byte(vaultMeta.Salt)
	s.VaultCheck = []byte(vaultMeta.Check)
	s.VaultCheckIV = []byte(vaultMeta.IV)

	return unlockVaultWithPrompt(ctx, s, env, vaultMeta)
}

// switchToVault switches the session context to the vault.
// If the vault is locked, it prompts for the password first.
func switchToVault(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	// Check if vault exists
	vaultMeta, err := ui.WithSpinner(env.Stdout, "", false, func() (*api.VaultMeta, error) {
		return s.Client.GetVaultMetadata(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to check vault: %w", err)
	}

	if vaultMeta == nil {
		fmt.Fprintln(env.Stdout, "No vault found. Run 'vault init' to create one.")
		return nil
	}

	// Cache vault metadata
	s.VaultID = vaultMeta.ID
	s.VaultSalt = []byte(vaultMeta.Salt)
	s.VaultCheck = []byte(vaultMeta.Check)
	s.VaultCheckIV = []byte(vaultMeta.IV)

	// If vault is not unlocked, prompt for password
	if !s.IsVaultUnlocked() {
		if err := unlockVaultWithPrompt(ctx, s, env, vaultMeta); err != nil {
			return err
		}
	}

	// Load vault folder tree and switch context
	err = ui.WithSpinnerErr(env.Stderr, "", false, func() error {
		vaultCache := api.NewFileCache()
		if err := vaultCache.LoadVaultFolderTree(ctx, s.Client, s.UserID, s.Username); err != nil {
			return fmt.Errorf("failed to load vault folders: %w", err)
		}

		// Prefetch root directory (empty string = root)
		entries, err := s.Client.ListVaultEntries(ctx, "")
		if err == nil {
			vaultCache.AddChildren("/", entries)
		}

		s.SwitchToVault(s.VaultID, vaultCache)
		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(env.Stdout, ui.SuccessStyle.Render("Switched to vault"))
	return nil
}

func exitVault(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	if !s.InVault {
		fmt.Fprintln(env.Stdout, "Not in vault.")
		return nil
	}

	s.RestoreWorkspaceState()

	if s.WorkspaceID == 0 {
		fmt.Fprintln(env.Stdout, ui.SuccessStyle.Render("Returned to personal workspace"))
	} else {
		fmt.Fprintf(env.Stdout, "%s\n", ui.SuccessStyle.Render("Returned to workspace '"+s.WorkspaceName+"'"))
	}
	return nil
}

// unlockVaultWithPrompt prompts for password and unlocks the vault.
func unlockVaultWithPrompt(ctx context.Context, s *session.Session, env *ExecutionEnv, vaultMeta *api.VaultMeta) error {
	// Prompt for password
	fmt.Fprint(env.Stdout, "Vault password: ")

	// Read password with masking (hidden input)
	password, err := readPassword(env)
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	// Derive key and verify password
	var vaultKey *crypto.VaultKey
	err = ui.WithSpinnerErr(env.Stderr, "Unlocking vault...", false, func() error {
		// Decode salt from base64
		salt, err := crypto.DecodeBase64(vaultMeta.Salt)
		if err != nil {
			return fmt.Errorf("invalid vault salt: %w", err)
		}

		// Derive key
		vaultKey = crypto.DeriveKey(password, salt)

		// Verify password by decrypting the check value
		check, err := crypto.DecodeBase64(vaultMeta.Check)
		if err != nil {
			vaultKey.Zero()
			return fmt.Errorf("invalid vault check value: %w", err)
		}

		iv, err := crypto.DecodeBase64(vaultMeta.IV)
		if err != nil {
			vaultKey.Zero()
			return fmt.Errorf("invalid vault IV: %w", err)
		}

		if !crypto.VerifyCheckValue(vaultKey, check, iv) {
			vaultKey.Zero()
			return fmt.Errorf("incorrect password")
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Store the key in session
	s.SetVaultKey(vaultKey)
	s.VaultSalt, _ = crypto.DecodeBase64(vaultMeta.Salt)

	fmt.Fprintln(env.Stdout, ui.SuccessStyle.Render("Vault unlocked for this session"))
	return nil
}

// initVault initializes a new vault with a password.
func initVault(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	// Check if vault already exists
	vaultMeta, err := ui.WithSpinner(env.Stdout, "", false, func() (*api.VaultMeta, error) {
		return s.Client.GetVaultMetadata(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to check vault: %w", err)
	}

	if vaultMeta != nil {
		return fmt.Errorf("vault already exists - use 'vault' to switch to it")
	}

	// Prompt for password
	fmt.Fprint(env.Stdout, "Create vault password: ")
	password1, err := readPassword(env)
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if password1 == "" {
		return fmt.Errorf("password cannot be empty")
	}

	if len(password1) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Confirm password
	fmt.Fprint(env.Stdout, "Confirm password: ")
	password2, err := readPassword(env)
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	if password1 != password2 {
		return fmt.Errorf("passwords do not match")
	}

	// Generate salt and derive key
	var salt, check, iv []byte
	var vaultKey *crypto.VaultKey

	err = ui.WithSpinnerErr(env.Stderr, "Creating vault...", false, func() error {
		// Generate random salt
		var err error
		salt, err = crypto.GenerateSalt()
		if err != nil {
			return fmt.Errorf("failed to generate salt: %w", err)
		}

		// Derive key from password
		vaultKey = crypto.DeriveKey(password1, salt)

		// Create check value for password verification
		check, iv, err = crypto.CreateCheckValue(vaultKey)
		if err != nil {
			vaultKey.Zero()
			return fmt.Errorf("failed to create check value: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Initialize vault on server
	newVault, err := ui.WithSpinner(env.Stdout, "", false, func() (*api.VaultMeta, error) {
		return s.Client.InitializeVault(ctx,
			crypto.EncodeBase64(salt),
			crypto.EncodeBase64(check),
			crypto.EncodeBase64(iv),
		)
	})
	if err != nil {
		vaultKey.Zero()
		return fmt.Errorf("failed to create vault: %w", err)
	}

	// Store vault state in session
	s.VaultID = newVault.ID
	s.SetVaultKey(vaultKey)
	s.VaultSalt = salt

	fmt.Fprintln(env.Stdout, ui.SuccessStyle.Render("Vault created successfully"))
	fmt.Fprintln(env.Stdout, "Use 'vault' to switch to your new vault.")
	return nil
}
