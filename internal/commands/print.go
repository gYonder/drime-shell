package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/session"
)

func init() {
	Register(&Command{
		Name:        "echo",
		Description: "Output arguments to standard output",
		Usage:       "echo [-n] [string]...\\n\\nOptions:\\n  -n    Do not output trailing newline\\n\\nExamples:\\n  echo hello world\\n  echo -n no newline",
		Run:         echo,
	})
	Register(&Command{
		Name:        "printf",
		Description: "Format and print data",
		Usage:       "printf <format> [arguments]...\\n\\nSupports escape sequences: \\\\n (newline), \\\\t (tab), \\\\r (return)\\n\\nExamples:\\n  printf \"Hello %s\\\\n\" world\\n  printf \"Count: %d\\\\n\" 42",
		Run:         printf,
	})
}

func echo(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// Simple implementation: join with spaces and print with newline
	// Does not handle -n yet, usually echo handles it.

	newline := true
	if len(args) > 0 && args[0] == "-n" {
		newline = false
		args = args[1:]
	}

	fmt.Fprint(env.Stdout, strings.Join(args, " "))
	if newline {
		fmt.Fprintln(env.Stdout)
	}
	return nil
}

func printf(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: printf <format> [arguments...]")
	}

	format := args[0]
	// ... (omitted comment)

	// Convert args to interface{}
	params := make([]interface{}, len(args)-1)
	for i, v := range args[1:] {
		params[i] = v
	}

	format = unescape(format)

	_, err := fmt.Fprintf(env.Stdout, format, params...)
	return err
}

func unescape(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
