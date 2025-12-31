package commands

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

func init() {
	Register(&Command{
		Name:        "diff",
		Description: "Show changes between two files",
		Usage:       "diff <file1> <file2>\\n\\nShows unified diff between two remote files.",
		Run:         diffCmd,
	})
	Register(&Command{
		Name:        "sort",
		Description: "Sort lines of text files",
		Usage:       "sort [-r] <file>\\nsort [-r] (reads from stdin when piped)\\n\\nOptions:\\n  -r    Reverse sort order\\n\\nExamples:\\n  sort names.txt             Sort file alphabetically\\n  sort -r names.txt          Sort in reverse order\\n  cat file.txt | sort        Sort piped input",
		Run:         sortCmd,
	})
	Register(&Command{
		Name:        "uniq",
		Description: "Report or omit repeated lines",
		Usage:       "uniq [-c] <file>\\nuniq [-c] (reads from stdin when piped)\\n\\nOptions:\\n  -c    Prefix lines with occurrence count\\n\\nExamples:\\n  uniq names.txt             Remove adjacent duplicates\\n  sort file.txt | uniq -c    Count unique lines",
		Run:         uniqCmd,
	})
	Register(&Command{
		Name:        "wc",
		Description: "Print newline, word, and byte counts",
		Usage:       "wc [-lwc] <file>\\nwc [-lwc] (reads from stdin when piped)\\n\\nOptions:\\n  -l    Print line count only\\n  -w    Print word count only\\n  -c    Print byte count only\\n\\nWith no options, prints lines, words, and bytes.",
		Run:         wcCmd,
	})
	Register(&Command{
		Name:        "head",
		Description: "Output the first part of files",
		Usage:       "head [-n lines] <file>\\nhead [-n lines] (reads from stdin when piped)\\n\\nOptions:\\n  -n N    Show first N lines (default: 10)\\n\\nExamples:\\n  head file.txt         Show first 10 lines\\n  head -n 5 file.txt    Show first 5 lines",
		Run:         headCmd,
	})
	Register(&Command{
		Name:        "tail",
		Description: "Output the last part of files",
		Usage:       "tail [-n lines] <file>\\ntail [-n lines] (reads from stdin when piped)\\n\\nOptions:\\n  -n N    Show last N lines (default: 10)\\n\\nExamples:\\n  tail file.txt         Show last 10 lines\\n  tail -n 20 log.txt    Show last 20 lines",
		Run:         tailCmd,
	})
}

// Helper to read file content to string (memory intensive if large, but diff usually is)
func readFileToString(ctx context.Context, s *session.Session, path string) (string, error) {
	entry, err := ResolveEntry(ctx, s, path)
	if err != nil {
		return "", err
	}
	if entry.Type == "folder" {
		return "", fmt.Errorf("%s: Is a directory", path)
	}

	// Check against configurable memory limit
	maxSize := s.MaxMemoryBytes()
	if entry.Size > maxSize {
		return "", fmt.Errorf("%s: File too large (>%dMB) for text processing", path, maxSize/(1024*1024))
	}

	// Download with vault decryption if needed
	content, err := ui.WithSpinner(os.Stderr, "", func() ([]byte, error) {
		return DownloadAndDecrypt(ctx, s, entry)
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	return string(content), nil
}

// Helper to read file to lines
func readFileLines(ctx context.Context, s *session.Session, path string) ([]string, error) {
	content, err := readFileToString(ctx, s, path)
	if err != nil {
		return nil, err
	}
	// Split by newline
	// Handle Windows/Unix newlines?
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(content, "\n"), nil
}

func diffCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: diff <file1> <file2>")
	}

	file1 := args[0]
	file2 := args[1]

	content1, err := readFileLines(ctx, s, file1)
	if err != nil {
		return err
	}
	content2, err := readFileLines(ctx, s, file2)
	if err != nil {
		return err
	}

	diff := difflib.UnifiedDiff{
		A:        content1,
		B:        content2,
		FromFile: file1,
		ToFile:   file2,
		Context:  3,
	}

	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return err
	}

	fmt.Fprint(env.Stdout, text)
	return nil
}

func sortCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("sort", pflag.ContinueOnError)
	reversed := fs.BoolP("reverse", "r", false, "reverse sort order")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// If no args and stdin is a TTY, show usage
	var lines []string
	var err error

	if fs.NArg() < 1 {
		if isStdinTTY() {
			return fmt.Errorf("usage: sort [-r] <file>\n       sort [-r] (reads from stdin when piped)")
		}
		// Read from stdin
		// Warning: reading all stdin into memory
		bytes, err := io.ReadAll(env.Stdin)
		if err != nil {
			return err
		}
		lines = strings.Split(string(bytes), "\n")
	} else {
		path := fs.Arg(0)
		lines, err = readFileLines(ctx, s, path)
		if err != nil {
			return err
		}
	}

	// Remove last empty line from split if ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	sort.Strings(lines)
	if *reversed {
		// Reverse in place
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}

	for _, line := range lines {
		fmt.Fprintln(env.Stdout, line)
	}
	return nil
}

func uniqCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("uniq", pflag.ContinueOnError)
	count := fs.BoolP("count", "c", false, "count occurrences")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var content string
	var err error

	if fs.NArg() < 1 {
		if isStdinTTY() {
			return fmt.Errorf("usage: uniq [-c] <file>\n       uniq [-c] (reads from stdin when piped)")
		}
		// Read from stdin
		bytes, err := io.ReadAll(env.Stdin)
		if err != nil {
			return err
		}
		content = string(bytes)
	} else {
		path := fs.Arg(0)
		content, err = readFileToString(ctx, s, path)
		if err != nil {
			return err
		}
	}

	scanner := bufio.NewScanner(strings.NewReader(content))

	var prevLine string
	var occurrences int
	first := true

	for scanner.Scan() {
		line := scanner.Text()

		if first {
			prevLine = line
			occurrences = 1
			first = false
			continue
		}

		if line == prevLine {
			occurrences++
		} else {
			printUniq(prevLine, occurrences, *count, env.Stdout)
			prevLine = line
			occurrences = 1
		}
	}

	if !first {
		printUniq(prevLine, occurrences, *count, env.Stdout)
	}

	return nil
}

func printUniq(line string, count int, showCount bool, w io.Writer) {
	if showCount {
		fmt.Fprintf(w, "%4d %s\n", count, line)
	} else {
		fmt.Fprintln(w, line)
	}
}

func wcCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("wc", pflag.ContinueOnError)
	linesOnly := fs.BoolP("lines", "l", false, "print line count only")
	wordsOnly := fs.BoolP("words", "w", false, "print word count only")
	bytesOnly := fs.BoolP("bytes", "c", false, "print byte count only")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// If no flags, show all
	showAll := !*linesOnly && !*wordsOnly && !*bytesOnly

	var content string
	var filename string

	if fs.NArg() < 1 {
		if isStdinTTY() {
			return fmt.Errorf("usage: wc [-lwc] <file>\n       wc [-lwc] (reads from stdin when piped)")
		}
		// Read from stdin
		data, err := io.ReadAll(env.Stdin)
		if err != nil {
			return err
		}
		content = string(data)
	} else {
		filename = fs.Arg(0)
		var err error
		content, err = readFileToString(ctx, s, filename)
		if err != nil {
			return err
		}
	}

	lines := strings.Count(content, "\n")
	// If content doesn't end with newline but has content, count that as a line
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		lines++
	}
	words := len(strings.Fields(content))
	bytes := len(content)

	var parts []string
	if showAll || *linesOnly {
		parts = append(parts, fmt.Sprintf("%d", lines))
	}
	if showAll || *wordsOnly {
		parts = append(parts, fmt.Sprintf("%d", words))
	}
	if showAll || *bytesOnly {
		parts = append(parts, fmt.Sprintf("%d", bytes))
	}

	output := strings.Join(parts, "\t")
	if filename != "" {
		output += "\t" + filename
	}
	fmt.Fprintln(env.Stdout, output)
	return nil
}

func headCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("head", pflag.ContinueOnError)
	numLines := fs.IntP("lines", "n", 10, "number of lines to show")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var lines []string
	var err error

	if fs.NArg() < 1 {
		if isStdinTTY() {
			return fmt.Errorf("usage: head [-n lines] <file>\n       head [-n lines] (reads from stdin when piped)")
		}
		// Read from stdin
		data, err := io.ReadAll(env.Stdin)
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines = strings.Split(content, "\n")
	} else {
		path := fs.Arg(0)
		lines, err = readFileLines(ctx, s, path)
		if err != nil {
			return err
		}
	}

	// Remove trailing empty line from split if content ended with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	count := *numLines
	if count > len(lines) {
		count = len(lines)
	}

	for i := 0; i < count; i++ {
		fmt.Fprintln(env.Stdout, lines[i])
	}
	return nil
}

func tailCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("tail", pflag.ContinueOnError)
	numLines := fs.IntP("lines", "n", 10, "number of lines to show")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var lines []string
	var err error

	if fs.NArg() < 1 {
		if isStdinTTY() {
			return fmt.Errorf("usage: tail [-n lines] <file>\n       tail [-n lines] (reads from stdin when piped)")
		}
		// Read from stdin
		data, err := io.ReadAll(env.Stdin)
		if err != nil {
			return err
		}
		content := strings.ReplaceAll(string(data), "\r\n", "\n")
		lines = strings.Split(content, "\n")
	} else {
		path := fs.Arg(0)
		lines, err = readFileLines(ctx, s, path)
		if err != nil {
			return err
		}
	}

	// Remove trailing empty line from split if content ended with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	count := *numLines
	start := len(lines) - count
	if start < 0 {
		start = 0
	}

	for i := start; i < len(lines); i++ {
		fmt.Fprintln(env.Stdout, lines[i])
	}
	return nil
}

// isStdinTTY returns true if stdin is a terminal (not piped)
func isStdinTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
