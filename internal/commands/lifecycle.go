package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
)

const (
	installURLUnix = "https://raw.githubusercontent.com/gYonder/drime-shell/main/scripts/install.sh"
	installURLWin  = "https://raw.githubusercontent.com/gYonder/drime-shell/main/scripts/install.ps1"
)

func init() {
	Register(&Command{
		Name:        "update",
		Description: "Update Drime Shell to the latest version",
		Run:         runUpdate,
	})
	Register(&Command{
		Name:        "uninstall",
		Description: "Uninstall Drime Shell",
		Run:         runUninstall,
	})
}

func runUpdate(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fmt.Fprintln(env.Stdout, "Checking for updates and installing...")

	if runtime.GOOS == "windows" {
		// On Windows, start a detached PowerShell process to download and run the installer
		// We use -NoProfile -ExecutionPolicy Bypass for robustness
		cmdStr := fmt.Sprintf("irm %s | iex", installURLWin)
		cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", cmdStr)

		// Start detached so we can exit and release the lock on the binary
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start update process: %w", err)
		}

		fmt.Fprintln(env.Stdout, "Update process started. Drime Shell will close now.")
		os.Exit(0)
		return nil
	}

	// On Unix, use syscall.Exec to replace the current process with the installer
	// This ensures we don't have issues with the shell running
	curlCmd := fmt.Sprintf("curl -fsSL %s | bash", installURLUnix)
	shPath, err := exec.LookPath("sh")
	if err != nil {
		return fmt.Errorf("sh not found: %w", err)
	}

	fmt.Fprintln(env.Stdout, "Downloading and running installer...")
	err = syscall.Exec(shPath, []string{"sh", "-c", curlCmd}, os.Environ())
	if err != nil {
		return fmt.Errorf("failed to exec installer: %w", err)
	}

	return nil
}

func runUninstall(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	confirm, err := ui.Prompt(fmt.Sprintf("%s\nAre you sure you want to uninstall Drime Shell? [y/N] ", ui.ErrorStyle.Render("WARNING: This will remove the drime-shell binary.")))
	if err != nil {
		return err
	}
	if confirm != "y" && confirm != "Y" {
		fmt.Fprintln(env.Stdout, "Uninstall cancelled.")
		return nil
	}

	fmt.Fprintln(env.Stdout, "Uninstalling...")

	if runtime.GOOS == "windows" {
		// On Windows, download script to temp file and run with -Uninstall switch
		tmpScript := os.Getenv("TEMP") + "\\drime-uninstall.ps1"

		// Setup the command to download script, run it with -Uninstall, and then try to delete itself (best effort)
		psCmd := fmt.Sprintf(`
			try {
				irm %s -OutFile "%s"
				& "%s" -Uninstall
			} catch {
				Write-Error $_
				Read-Host "Press Enter to exit"
			}
		`, installURLWin, tmpScript, tmpScript)

		cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psCmd)

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start uninstall process: %w", err)
		}

		fmt.Fprintln(env.Stdout, "Uninstall process started. Drime Shell will close now.")
		os.Exit(0)
		return nil
	}

	// On Unix, exec the install script with --uninstall
	curlCmd := fmt.Sprintf("curl -fsSL %s | bash -s -- --uninstall", installURLUnix)
	shPath, err := exec.LookPath("sh")
	if err != nil {
		return fmt.Errorf("sh not found: %w", err)
	}

	err = syscall.Exec(shPath, []string{"sh", "-c", curlCmd}, os.Environ())
	if err != nil {
		return fmt.Errorf("failed to exec uninstaller: %w", err)
	}

	return nil
}
