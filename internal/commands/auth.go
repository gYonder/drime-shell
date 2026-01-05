package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/mikael.mansson2/drime-shell/internal/config"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"golang.org/x/term"
)

func init() {
	Register(&Command{
		Name:        "login",
		Description: "Log in to Drime Cloud",
		Usage: `login [email]

Authenticates with Drime Cloud using email and password.
The token is saved to ~/.drime-shell/config.yaml.

If email is provided, only prompts for password.
Password input is hidden for security.

Examples:
  login                    Prompt for email and password
  login user@example.com   Prompt for password only

Note: You can also set DRIME_TOKEN environment variable to skip login.`,
		Run: loginCmd,
	})
	Register(&Command{
		Name:        "logout",
		Description: "Log out from Drime Cloud",
		Usage: `logout

Removes the saved authentication token from ~/.drime-shell/config.yaml.
You'll need to log in again to use the shell.

Note: This does not invalidate the token on the server.`,
		Run: logoutCmd,
	})
	Register(&Command{
		Name:        "whoami",
		Description: "Show current user",
		Usage: `whoami

Displays information about the currently logged-in user:
  - Email address
  - User ID
  - Current workspace (if not default)`,
		Run: whoamiCmd,
	})
}

func loginCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// Note: Login must use os.Stdin directly since it's typically run before the REPL
	// and needs actual terminal input for the password prompt
	reader := bufio.NewReader(os.Stdin)

	// Get email
	var email string
	if len(args) > 0 {
		email = args[0]
	} else {
		fmt.Fprint(env.Stdout, "Email: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read email: %w", err)
		}
		email = strings.TrimSpace(input)
	}

	if email == "" {
		return fmt.Errorf("email is required")
	}

	// Get password (hidden input)
	fmt.Fprint(env.Stdout, "Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(env.Stdout) // Newline after password
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)

	if password == "" {
		return fmt.Errorf("password is required")
	}

	// Get device name for the token
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "drime-shell"
	}
	deviceName := fmt.Sprintf("drime-shell@%s", hostname)

	// Call login API
	user, err := ui.WithSpinner(env.Stdout, "", false, func() (*struct {
		Email       string
		ID          int64
		AccessToken string
	}, error) {
		u, err := s.Client.Login(ctx, email, password, deviceName)
		if err != nil {
			return nil, err
		}
		return &struct {
			Email       string
			ID          int64
			AccessToken string
		}{
			Email:       u.Email,
			ID:          u.ID,
			AccessToken: u.AccessToken,
		}, nil
	})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	if user.AccessToken == "" {
		return fmt.Errorf("login succeeded but no token returned")
	}

	// Save token to config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	cfg.Token = user.AccessToken
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	// Update session
	s.Token = user.AccessToken
	s.Username = user.Email
	s.UserID = user.ID

	fmt.Fprintf(env.Stdout, "%s Logged in as %s\n",
		ui.SuccessStyle.Render("✓"),
		ui.PromptUserStyle.Render(user.Email))
	fmt.Fprintf(env.Stdout, "%s\n", ui.MutedStyle.Render("Token saved to ~/.drime-shell/config.yaml"))

	return nil
}

func logoutCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// Load current config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Token == "" {
		fmt.Fprintln(env.Stdout, "Not logged in.")
		return nil
	}

	// Clear token
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Clear session
	s.Token = ""
	s.Username = ""
	s.UserID = 0

	fmt.Fprintf(env.Stdout, "%s Logged out. Token removed from config.\n",
		ui.SuccessStyle.Render("✓"))
	fmt.Fprintf(env.Stdout, "%s\n", ui.WarningStyle.Render("Note: You'll need to log in again to use the shell."))

	return nil
}

func whoamiCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if s.Username == "" {
		fmt.Fprintln(env.Stdout, "Not logged in.")
		return nil
	}

	fmt.Fprintf(env.Stdout, "%s (ID: %d)\n",
		ui.PromptUserStyle.Render(s.Username),
		s.UserID)

	if s.WorkspaceName != "" {
		fmt.Fprintf(env.Stdout, "Workspace: %s\n", ui.WorkspaceStyle.Render(s.WorkspaceName))
	}

	return nil
}
