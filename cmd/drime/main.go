package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/build"
	"github.com/mikael.mansson2/drime-shell/internal/config"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/shell"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"golang.org/x/term"

	// Register commands
	_ "github.com/mikael.mansson2/drime-shell/internal/commands"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("drime-shell version %s (commit: %s, date: %s)\n", build.Version, build.Commit, build.Date)
		os.Exit(0)
	}

	// Show immediate feedback - gets cleared before any prompts or replaced by spinner
	fmt.Fprint(os.Stderr, "Initializing... ⠋")

	// Load configuration from file or environment
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r\033[KError loading config: %v\n", err)
		os.Exit(1)
	}

	// If the token isn't set, we need to ask the user for it
	if cfg.Token == "" {
		// Clear the "Starting..." message before prompting
		fmt.Fprint(os.Stderr, "\r\033[K")
		token, err := promptForToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		cfg.Token = token

		// Offer to save the token
		if shouldSave := promptYesNo("Save token to config file?"); shouldSave {
			if err := config.Save(cfg); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to save config: %v\n", err)
			} else {
				fmt.Println("Token saved to ~/.drime-shell/config.yaml")
			}
		}
	}

	// Set up the API client
	client := api.NewHTTPClient(cfg.APIURL, cfg.Token)

	// check connectivity and initialize shell
	// We wrap all network activity in a spinner so it looks nice
	type initData struct {
		user    *api.User
		cache   *api.FileCache
		entries []api.FileEntry
	}

	data, err := ui.WithSpinner(os.Stderr, "Initializing...", true, func() (*initData, error) {
		// 1. Verify token & get user info
		user, err := client.Whoami(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to connect to Drime Cloud: %w", err)
		}

		// 2. Load folder tree (single massive API call)
		cache := api.NewFileCache()
		if err := cache.LoadFolderTree(context.Background(), client, user.ID, user.Name(), 0); err != nil {
			return nil, fmt.Errorf("failed to load folder tree: %w", err)
		}

		// 3. Prefetch root directory contents
		entries, err := client.ListByParentIDWithOptions(context.Background(), nil, api.ListOptions(0))
		if err != nil {
			// Not critical, just return empty
			entries = []api.FileEntry{}
		}

		return &initData{user, cache, entries}, nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	user := data.user
	cache := data.cache
	entries := data.entries

	// Setup Session
	sess := session.NewSession(client, cache)
	sess.UserID = user.ID
	sess.Username = user.Name()
	sess.Token = cfg.Token
	sess.MaxMemoryBufferMB = cfg.MaxMemoryBufferMB
	if cfg.Aliases != nil {
		for k, v := range cfg.Aliases {
			sess.Aliases[k] = v
		}
	}

	// Apply prefetched entries
	if len(entries) > 0 {
		cache.AddChildren("/", entries)
		// Prefetch children of root folders in background (one level deeper)
		for _, entry := range entries {
			if entry.Type == "folder" {
				go func(folderID int64, folderName string) {
					children, err := client.ListByParentIDWithOptions(context.Background(), &folderID, api.ListOptions(0))
					if err == nil {
						cache.AddChildren("/"+folderName, children)
					}
				}(entry.ID, entry.Name)
			}
		}
	}

	// 6. Start Shell
	sh, err := shell.New(sess)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start shell: %v\n", err)
		os.Exit(1)
	}

	sh.Run()
}

func promptForToken() (string, error) {
	fmt.Println("No Drime API token found.")
	fmt.Println()
	fmt.Println("Choose authentication method:")
	fmt.Println("  1) Enter API token (from https://app.drime.cloud/settings/api)")
	fmt.Println("  2) Log in with email and password")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Enter choice [1/2]: ")
		choice, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			return promptForTokenDirect(reader)
		case "2":
			return promptLoginFlow(reader)
		default:
			fmt.Println("Please enter 1 or 2")
		}
	}
}

func promptForTokenDirect(reader *bufio.Reader) (string, error) {
	fmt.Println()
	fmt.Println("Get your API token from: https://app.drime.cloud/settings/api")
	fmt.Print("Enter your Drime API token: ")

	token, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}

	return token, nil
}

func promptLoginFlow(reader *bufio.Reader) (string, error) {
	fmt.Println()

	// Get email
	fmt.Print("Email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read email: %w", err)
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	// Get password (hidden input)
	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // Newline after password
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	password := string(passwordBytes)
	if password == "" {
		return "", fmt.Errorf("password is required")
	}

	// Get device name for the token
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "drime-shell"
	}
	deviceName := fmt.Sprintf("drime-shell@%s", hostname)

	// Need a temporary client to call login
	cfg, _ := config.Load()
	tempClient := api.NewHTTPClient(cfg.APIURL, "")

	fmt.Print("Logging in... ")
	user, err := tempClient.Login(context.Background(), email, password, deviceName)
	if err != nil {
		fmt.Println("Failed")
		return "", fmt.Errorf("login failed: %w", err)
	}
	fmt.Println("Done")

	if user.AccessToken == "" {
		return "", fmt.Errorf("login succeeded but no token returned")
	}

	fmt.Printf("Logged in as %s\n", email)
	return user.AccessToken, nil
}

func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [Y/n]: ", question)

	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "" || answer == "y" || answer == "yes"
}
