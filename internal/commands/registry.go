package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

type ExecutionEnv struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Command struct {
	Run         func(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error
	Name        string
	Description string
	Usage       string // Detailed usage info shown by "help <command>"
}

var Registry = make(map[string]*Command)

// ReorderArgsForFlags reorders arguments so flags come before positional args.
// This allows Unix-style interspersed flags like "cmd file.txt -f" to work
// the same as "cmd -f file.txt".
func ReorderArgsForFlags(fs *pflag.FlagSet, args []string) []string {
	var flags []string
	var positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			// Everything after -- is positional
			positional = append(positional, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			// It's a flag
			flags = append(flags, arg)
			// Check if this flag takes a value
			name := strings.TrimLeft(arg, "-")
			if idx := strings.Index(name, "="); idx >= 0 {
				// Flag with = doesn't consume next arg
				i++
				continue
			}
			// Check if the flag is defined and takes a value
			f := fs.Lookup(name)
			if f != nil {
				// Check if it's a bool flag (doesn't need value)
				if f.Value.Type() == "bool" {
					i++
					continue
				}
				// Non-bool flag, consume next arg as value
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flags = append(flags, args[i])
				}
			}
		} else {
			positional = append(positional, arg)
		}
		i++
	}

	return append(flags, positional...)
}

func init() {
	Register(&Command{
		Name:        "help",
		Description: "Show available commands or help for a specific command",
		Usage:       "help [command]\\n\\nExamples:\\n  help         List all commands\\n  help ls      Show detailed help for ls",
		Run:         help,
	})
	Register(&Command{
		Name:        "clear",
		Description: "Clear the screen",
		Usage:       "clear\\n\\nClears the terminal screen and scrollback buffer.",
		Run:         clear,
	})
	Register(&Command{
		Name:        "history",
		Description: "Show command history",
		Usage:       "history\\n\\nDisplays numbered list of previously executed commands.",
		Run:         history,
	})
}

func Register(cmd *Command) {
	Registry[cmd.Name] = cmd
}

func Get(name string) (*Command, bool) {
	cmd, ok := Registry[name]
	return cmd, ok
}

// HasHelpFlag checks if args contain -h or --help
func HasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
		// Stop checking after first non-flag argument
		if len(arg) > 0 && arg[0] != '-' {
			break
		}
	}
	return false
}

// PrintUsage prints usage information for a command to the given writer
func PrintUsage(cmd *Command, w io.Writer) {
	fmt.Fprintf(w, "%s - %s\n", ui.CommandStyle.Render(cmd.Name), cmd.Description)
	if cmd.Usage != "" {
		// Replace escaped newlines with actual newlines for display
		usage := strings.ReplaceAll(cmd.Usage, "\\n", "\n")
		fmt.Fprintf(w, "\nUsage: %s\n", usage)
	}
}

// Parse parses a command line into command name AND args
func Parse(line string) (string, []string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func help(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// If a command name is provided, show detailed help for that command
	if len(args) > 0 {
		cmdName := args[0]
		cmd, ok := Registry[cmdName]
		if !ok {
			return fmt.Errorf("help: unknown command '%s'", cmdName)
		}

		PrintUsage(cmd, env.Stdout)
		return nil
	}

	// Collect unique commands (not aliases)
	seen := make(map[string]bool)
	var cmds []*Command
	for name, cmd := range Registry {
		if cmd.Name == name && !seen[name] {
			cmds = append(cmds, cmd)
			seen[name] = true
		}
	}

	// Sort by name
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})

	fmt.Fprintln(env.Stdout, ui.HeaderStyle.Render("Available commands:"))
	fmt.Fprintln(env.Stdout)
	for _, cmd := range cmds {
		name := ui.CommandStyle.Render(fmt.Sprintf("%-12s", cmd.Name))
		desc := ui.MutedStyle.Render(cmd.Description)
		fmt.Fprintf(env.Stdout, "  %s %s\n", name, desc)
	}
	fmt.Fprintln(env.Stdout)
	return nil
}

func clear(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// ANSI escape sequence: move to top-left, clear entire screen, clear scrollback
	fmt.Fprint(env.Stdout, "\033[H\033[2J\033[3J")
	return nil
}

func history(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if s.HistoryGetter == nil {
		return fmt.Errorf("history not available")
	}

	hist := s.HistoryGetter()
	if len(hist) == 0 {
		fmt.Fprintln(env.Stdout, "No history.")
		return nil
	}

	for i, cmd := range hist {
		num := ui.MutedStyle.Render(fmt.Sprintf("%4d", i+1))
		fmt.Fprintf(env.Stdout, "  %s  %s\n", num, cmd)
	}
	return nil
}
