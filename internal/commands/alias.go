package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/config"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

func init() {
	Register(&Command{
		Name:        "alias",
		Description: "Create or list command aliases",
		Usage:       "alias [name=value]\n\nWithout arguments, lists all defined aliases.\nWith an argument, creates a new alias.\n\nExamples:\n  alias                   # List all aliases\n  alias ll='ls -la'       # Create alias 'll' for 'ls -la'\n  alias la=ls -a          # Create alias 'la' for 'ls -a'",
		Run:         aliasCmd,
	})
	Register(&Command{
		Name:        "unalias",
		Description: "Remove a command alias",
		Usage:       "unalias <name>\n\nRemoves the specified alias.\n\nExamples:\n  unalias ll",
		Run:         unaliasCmd,
	})
}

func aliasCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// No arguments: list all aliases
	if len(args) == 0 {
		return listAliases(s, env)
	}

	// Join args in case of alias ll=ls -la (space after =)
	def := strings.Join(args, " ")

	name, value, ok := parseAliasDefinition(def)
	if !ok {
		return fmt.Errorf("alias: invalid format. Use: alias name='value' or alias name=value")
	}

	// Check if trying to shadow a built-in command
	if _, exists := Registry[name]; exists {
		fmt.Fprintf(env.Stderr, "Warning: '%s' shadows a built-in command\n", name)
	}

	// Set the alias
	if s.Aliases == nil {
		s.Aliases = make(map[string]string)
	}
	s.Aliases[name] = value

	// Persist to config
	if err := saveAliasesToConfig(s.Aliases); err != nil {
		fmt.Fprintf(env.Stderr, "Warning: failed to save alias to config: %v\n", err)
	}

	fmt.Fprintf(env.Stdout, "alias %s='%s'\n", name, value)
	return nil
}

func unaliasCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: unalias <name>")
	}

	name := args[0]

	if s.Aliases == nil {
		return fmt.Errorf("unalias: %s: not found", name)
	}

	if _, exists := s.Aliases[name]; !exists {
		return fmt.Errorf("unalias: %s: not found", name)
	}

	delete(s.Aliases, name)

	// Persist to config
	if err := saveAliasesToConfig(s.Aliases); err != nil {
		fmt.Fprintf(env.Stderr, "Warning: failed to save config: %v\n", err)
	}

	return nil
}

func listAliases(s *session.Session, env *ExecutionEnv) error {
	if len(s.Aliases) == 0 {
		fmt.Fprintln(env.Stdout, "No aliases defined.")
		fmt.Fprintln(env.Stdout, "")
		fmt.Fprintln(env.Stdout, ui.MutedStyle.Render("Use 'alias name=value' to create an alias."))
		return nil
	}

	// Sort alias names
	names := make([]string, 0, len(s.Aliases))
	for name := range s.Aliases {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		value := s.Aliases[name]
		fmt.Fprintf(env.Stdout, "alias %s='%s'\n", ui.CommandStyle.Render(name), value)
	}
	return nil
}

func saveAliasesToConfig(aliases map[string]string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Aliases = aliases
	return config.Save(cfg)
}

// parseAliasDefinition parses "name=value" or "name='value'" format
// Returns the alias name, value, and whether parsing succeeded.
func parseAliasDefinition(def string) (name, value string, ok bool) {
	// Find the = sign
	idx := strings.Index(def, "=")
	if idx <= 0 {
		return "", "", false
	}

	name = def[:idx]
	value = def[idx+1:]

	// Remove surrounding quotes if present
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '\'' && value[len(value)-1] == '\'') ||
			(value[0] == '"' && value[len(value)-1] == '"') {
			value = value[1 : len(value)-1]
		}
	}

	// Validate name (alphanumeric, underscore, dash only)
	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		isValid := isLower || isUpper || isDigit || r == '_' || r == '-'
		if !isValid {
			return "", "", false
		}
	}

	return name, value, true
}
