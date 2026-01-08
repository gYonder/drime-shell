package shell

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/chzyer/readline"
	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/config"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
)

// Shell is the main REPL for the Drime shell.
type Shell struct {
	Session        *session.Session
	RL             *readline.Instance
	sessionHistory []string // Commands from current session (for !!, !-n)
}

// New creates a new Shell with the given session.
func New(s *session.Session) (*Shell, error) {
	completer := NewCompleter(s)

	historyPath, _ := config.HistoryPath()

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "drime> ",
		HistoryFile:       historyPath,
		HistorySearchFold: true,
		AutoComplete:      completer,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
	})
	if err != nil {
		return nil, err
	}

	shell := &Shell{
		Session: s,
		RL:      rl,
	}

	// Set history getter on session so commands can access it
	s.HistoryGetter = shell.GetHistory

	return shell, nil
}

// buildPrompt creates the shell prompt string
func (sh *Shell) buildPrompt() string {
	displayPath := sh.Session.VirtualCWD()
	if displayPath == sh.Session.HomeDir {
		displayPath = "~"
	} else if strings.HasPrefix(displayPath, sh.Session.HomeDir+"/") {
		displayPath = "~" + displayPath[len(sh.Session.HomeDir):]
	}

	contextName := sh.Session.ContextName()
	return ui.RenderPrompt(sh.Session.Username, displayPath, contextName, sh.Session.InVault)
}

// Run starts the REPL loop.
func (sh *Shell) Run() {
	defer sh.RL.Close()

	ctx := context.Background()

	for {
		sh.RL.SetPrompt(sh.buildPrompt())

		line, err := sh.RL.Readline()
		if err != nil { // io.EOF or Ctrl+D
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle history expansion (!n)
		if strings.HasPrefix(line, "!") && len(line) > 1 {
			expanded, err := sh.expandHistory(line)
			if err != nil {
				fmt.Printf("drime: %v\n", err)
				continue
			}
			line = expanded
			fmt.Println(line) // Show the expanded command
		}

		// Handle alias expansion
		if expanded, wasAlias := ExpandAlias(line, sh.Session.Aliases); wasAlias {
			line = expanded
		}

		// Add to session history
		sh.sessionHistory = append(sh.sessionHistory, line)

		// Parse the command line into a command chain
		chain, err := ParseCommandChain(line)
		if err != nil {
			fmt.Printf("drime: %v\n", err)
			continue
		}

		// Execute the command chain
		if err := chain.Execute(ctx, sh.Session); err != nil {
			// Check if token expired - prompt for re-authentication
			if errors.Is(err, api.ErrTokenExpired) {
				fmt.Println("drime: Session expired. Please run 'login' to re-authenticate.")
			} else {
				fmt.Printf("drime: %v\n", err)
			}
		}
	}
}

// expandHistory handles !n and !! syntax for history expansion
func (sh *Shell) expandHistory(line string) (string, error) {
	// For !! and !-n, use session history (current session only)
	// For !n and !prefix, use full history (file + session)

	// !! - last command from current session
	if line == "!!" {
		if len(sh.sessionHistory) == 0 {
			return "", fmt.Errorf("!!: event not found")
		}
		return sh.sessionHistory[len(sh.sessionHistory)-1], nil
	}

	// !-n - nth previous command from current session
	if strings.HasPrefix(line, "!-") {
		nStr := line[2:]
		n, err := strconv.Atoi(nStr)
		if err != nil || n < 1 {
			return "", fmt.Errorf("!%s: event not found", nStr)
		}
		idx := len(sh.sessionHistory) - n
		if idx < 0 {
			return "", fmt.Errorf("!%s: event not found", nStr)
		}
		return sh.sessionHistory[idx], nil
	}

	// For !n and !prefix, use full history
	history := sh.GetHistory()
	if len(history) == 0 {
		return "", fmt.Errorf("no history available")
	}

	// !n - command at position n (1-indexed)
	if strings.HasPrefix(line, "!") {
		nStr := line[1:]
		n, err := strconv.Atoi(nStr)
		if err != nil {
			// !string - search for command starting with string
			prefix := nStr
			for i := len(history) - 1; i >= 0; i-- {
				if strings.HasPrefix(history[i], prefix) {
					return history[i], nil
				}
			}
			return "", fmt.Errorf("!%s: event not found", prefix)
		}
		if n < 1 || n > len(history) {
			return "", fmt.Errorf("!%d: event not found", n)
		}
		return history[n-1], nil
	}

	return line, nil
}

// GetHistory returns the full history from the file (readline keeps it up-to-date)
func (sh *Shell) GetHistory() []string {
	historyPath, err := config.HistoryPath()
	if err != nil {
		return sh.sessionHistory // Fallback to session history
	}

	data, err := os.ReadFile(historyPath)
	if err != nil {
		return sh.sessionHistory // Fallback to session history
	}

	lines := strings.Split(string(data), "\n")
	var history []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			history = append(history, line)
		}
	}
	return history
}
