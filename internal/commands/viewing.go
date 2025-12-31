package commands

import (
	"context"
	"fmt"

	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

func init() {
	Register(&Command{
		Name:        "cat",
		Description: "Concatenate and print files to standard output",
		Usage:       "cat <file>...\n\nDisplays the contents of remote files with syntax highlighting.\n\nExamples:\n  cat readme.txt\n  cat file1.txt file2.txt",
		Run:         cat,
	})
}

func cat(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: cat <file>...")
	}

	for _, path := range args {
		entry, err := ResolveEntry(ctx, s, path)
		if err != nil {
			return fmt.Errorf("cat: %v", err)
		}

		if entry.Type == "folder" {
			fmt.Fprintf(env.Stderr, "cat: %s: Is a directory\n", path)
			continue
		}

		// Download content (with vault decryption if needed)
		content, err := ui.WithSpinner(env.Stderr, "", func() ([]byte, error) {
			return DownloadAndDecrypt(ctx, s, entry)
		})
		if err != nil {
			return fmt.Errorf("cat: %s: %w", path, err)
		}

		// Apply syntax highlighting and output
		highlighted := ui.Highlight(string(content), entry.Name)
		fmt.Fprint(env.Stdout, highlighted)

		// Ensure trailing newline
		if len(highlighted) > 0 && highlighted[len(highlighted)-1] != '\n' {
			fmt.Fprintln(env.Stdout)
		}
	}
	return nil
}
