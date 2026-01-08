package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/gYonder/drime-shell/internal/session"
)

func TestGrepCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantOutput string
		wantErr    bool
		errContain string
	}{
		{
			name:       "simple match",
			args:       []string{"error"},
			stdin:      "line1 error here\nline2 ok\nline3 error again\n",
			wantOutput: "line1 error here\nline3 error again\n",
		},
		{
			name:       "case insensitive",
			args:       []string{"-i", "ERROR"},
			stdin:      "line1 error here\nline2 ok\nline3 Error again\n",
			wantOutput: "line1 error here\nline3 Error again\n",
		},
		{
			name:       "invert match",
			args:       []string{"-v", "error"},
			stdin:      "line1 error here\nline2 ok\nline3 error again\n",
			wantOutput: "line2 ok\n",
		},
		{
			name:       "line numbers",
			args:       []string{"-n", "error"},
			stdin:      "line1 error here\nline2 ok\nline3 error again\n",
			wantOutput: "1:line1 error here\n3:line3 error again\n",
		},
		{
			name:       "count only",
			args:       []string{"-c", "error"},
			stdin:      "line1 error here\nline2 ok\nline3 error again\n",
			wantOutput: "2\n",
		},
		{
			name:       "combined flags",
			args:       []string{"-i", "-n", "ERROR"},
			stdin:      "line1 error here\nline2 ok\nline3 Error again\n",
			wantOutput: "1:line1 error here\n3:line3 Error again\n",
		},
		{
			name:       "regex pattern",
			args:       []string{"err.*here"},
			stdin:      "line1 error here\nline2 ok\nline3 error again\n",
			wantOutput: "line1 error here\n",
		},
		{
			name:       "no matches",
			args:       []string{"notfound"},
			stdin:      "line1 error here\nline2 ok\n",
			wantOutput: "",
		},
		{
			name:       "count zero matches",
			args:       []string{"-c", "notfound"},
			stdin:      "line1 error here\nline2 ok\n",
			wantOutput: "0\n",
		},
		{
			name:       "no pattern",
			args:       []string{},
			stdin:      "some input",
			wantErr:    true,
			errContain: "usage",
		},
		{
			name:       "invalid regex",
			args:       []string{"[invalid"},
			stdin:      "some input",
			wantErr:    true,
			errContain: "invalid pattern",
		},
		{
			name:       "multiple files rejected",
			args:       []string{"pattern", "file1.txt", "file2.txt"},
			stdin:      "",
			wantErr:    true,
			errContain: "multiple files not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			env := &ExecutionEnv{
				Stdout: stdout,
				Stderr: stderr,
				Stdin:  strings.NewReader(tt.stdin),
			}

			// Create minimal session (not needed for stdin-only tests)
			sess := &session.Session{}

			err := grepCmd(context.Background(), sess, env, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContain)
					return
				}
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("expected error containing %q, got %q", tt.errContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got := stdout.String(); got != tt.wantOutput {
				t.Errorf("output mismatch:\ngot:  %q\nwant: %q", got, tt.wantOutput)
			}
		})
	}
}
