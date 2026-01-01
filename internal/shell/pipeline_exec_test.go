package shell_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/commands"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/shell"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupMockCommands registers temporary commands for testing pipelines.
// Returns a cleanup function to remove them.
func setupMockCommands() func() {
	// mock-echo: writes args joined by space to stdout
	commands.Register(&commands.Command{
		Name: "mock-echo",
		Run: func(ctx context.Context, s *session.Session, env *commands.ExecutionEnv, args []string) error {
			fmt.Fprintln(env.Stdout, strings.Join(args, " "))
			return nil
		},
	})

	// mock-reverse: reverses each line from stdin
	commands.Register(&commands.Command{
		Name: "mock-reverse",
		Run: func(ctx context.Context, s *session.Session, env *commands.ExecutionEnv, args []string) error {
			buf, err := io.ReadAll(env.Stdin)
			if err != nil {
				return err
			}
			input := strings.TrimRight(string(buf), "\n")
			lines := strings.Split(input, "\n")
			for i, line := range lines {
				runes := []rune(line)
				for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
					runes[i], runes[j] = runes[j], runes[i]
				}
				lines[i] = string(runes)
			}
			fmt.Fprintln(env.Stdout, strings.Join(lines, "\n"))
			return nil
		},
	})

	// mock-upper: converts stdin to uppercase
	commands.Register(&commands.Command{
		Name: "mock-upper",
		Run: func(ctx context.Context, s *session.Session, env *commands.ExecutionEnv, args []string) error {
			buf, err := io.ReadAll(env.Stdin)
			if err != nil {
				return err
			}
			fmt.Fprint(env.Stdout, strings.ToUpper(string(buf)))
			return nil
		},
	})

	// mock-wc: counts lines
	commands.Register(&commands.Command{
		Name: "mock-wc",
		Run: func(ctx context.Context, s *session.Session, env *commands.ExecutionEnv, args []string) error {
			buf, err := io.ReadAll(env.Stdin)
			if err != nil {
				return err
			}
			input := strings.TrimSpace(string(buf))
			if input == "" {
				fmt.Fprintln(env.Stdout, "0")
				return nil
			}
			lines := strings.Split(input, "\n")
			fmt.Fprintf(env.Stdout, "%d\n", len(lines))
			return nil
		},
	})

	return func() {
		delete(commands.Registry, "mock-echo")
		delete(commands.Registry, "mock-reverse")
		delete(commands.Registry, "mock-upper")
		delete(commands.Registry, "mock-wc")
	}
}



func TestPipeline_Execute_FourCommands(t *testing.T) {
	cleanup := setupMockCommands()
	defer cleanup()

	// Capture output via MockClient intercepting "output.txt" upload
	var capturedOutput bytes.Buffer
	mockClient := &api.MockDrimeClient{
		UploadFunc: func(ctx context.Context, r io.Reader, name string, parentID *int64, size int64, wid int64) (*api.FileEntry, error) {
			if name == "output.txt" {
				io.Copy(&capturedOutput, r)
				return &api.FileEntry{Name: name}, nil
			}
			return nil, fmt.Errorf("unexpected upload: %s", name)
		},
	}

	cache := api.NewFileCache()
	s := session.NewSession(mockClient, cache)

	// Command: echo "hello world" -> reverse -> upper -> wc -> output.txt
	// 1. "hello world"
	// 2. "dlrow olleh"
	// 3. "DLROW OLLEH"
	// 4. "1" (line count)
	input := "mock-echo hello world | mock-reverse | mock-upper | mock-wc > output.txt"

	pipeline, err := shell.ParsePipeline(input)
	require.NoError(t, err)

	err = pipeline.Execute(context.Background(), s)
	require.NoError(t, err)

	assert.Equal(t, "1\n", capturedOutput.String())
}

func TestPipeline_Execute_DataTransformation(t *testing.T) {
	cleanup := setupMockCommands()
	defer cleanup()

	var capturedOutput bytes.Buffer
	mockClient := &api.MockDrimeClient{
		UploadFunc: func(ctx context.Context, r io.Reader, name string, parentID *int64, size int64, wid int64) (*api.FileEntry, error) {
			if name == "output.txt" {
				io.Copy(&capturedOutput, r)
				return &api.FileEntry{Name: name}, nil
			}
			return nil, fmt.Errorf("unexpected upload: %s", name)
		},
	}

	cache := api.NewFileCache()
	s := session.NewSession(mockClient, cache)

	// Command: echo "abc" -> reverse -> upper -> output.txt
	// 1. "abc"
	// 2. "cba"
	// 3. "CBA"
	input := "mock-echo abc | mock-reverse | mock-upper > output.txt"

	pipeline, err := shell.ParsePipeline(input)
	require.NoError(t, err)

	err = pipeline.Execute(context.Background(), s)
	require.NoError(t, err)

	assert.Equal(t, "CBA\n", capturedOutput.String())
}
