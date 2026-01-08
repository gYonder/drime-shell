package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/session"
)

func TestSearchCommand(t *testing.T) {
	mockClient := &api.MockDrimeClient{}
	sess := &session.Session{
		Client:      mockClient,
		WorkspaceID: 123,
	}
	env := &ExecutionEnv{
		Stdout: &mockWriter{},
		Stderr: &mockWriter{},
	}

	tests := []struct {
		name     string
		args     []string
		expected func(*api.ListEntriesOptions)
	}{
		{
			name: "simple query",
			args: []string{"project"},
			expected: func(opts *api.ListEntriesOptions) {
				if opts.Query != "project" {
					t.Errorf("expected query 'project', got '%s'", opts.Query)
				}
				if opts.Filters != "" {
					t.Errorf("expected empty filters, got '%s'", opts.Filters)
				}
			},
		},
		{
			name: "type filter",
			args: []string{"--type", "image"},
			expected: func(opts *api.ListEntriesOptions) {
				filters := decodeFilters(t, opts.Filters)
				if len(filters) != 1 {
					t.Fatalf("expected 1 filter, got %d", len(filters))
				}
				if filters[0].Key != "type" || filters[0].Value != "image" {
					t.Errorf("unexpected filter: %+v", filters[0])
				}
			},
		},
		{
			name: "shared filter",
			args: []string{"--shared"},
			expected: func(opts *api.ListEntriesOptions) {
				filters := decodeFilters(t, opts.Filters)
				if len(filters) != 1 {
					t.Fatalf("expected 1 filter, got %d", len(filters))
				}
				if filters[0].Key != "sharedByMe" || filters[0].Value != true {
					t.Errorf("unexpected filter: %+v", filters[0])
				}
			},
		},
		{
			name: "link filter",
			args: []string{"--link"},
			expected: func(opts *api.ListEntriesOptions) {
				filters := decodeFilters(t, opts.Filters)
				if len(filters) != 1 {
					t.Fatalf("expected 1 filter, got %d", len(filters))
				}
				if filters[0].Key != "shareableLink" || filters[0].Operator != "has" {
					t.Errorf("unexpected filter: %+v", filters[0])
				}
			},
		},
		{
			name: "complex query",
			args: []string{"report", "--type", "pdf", "--after", "2023-01-01"},
			expected: func(opts *api.ListEntriesOptions) {
				if opts.Query != "report" {
					t.Errorf("expected query 'report', got '%s'", opts.Query)
				}
				filters := decodeFilters(t, opts.Filters)
				if len(filters) != 2 {
					t.Fatalf("expected 2 filters, got %d", len(filters))
				}
				// Order matters in implementation
				if filters[0].Key != "type" || filters[0].Value != "pdf" {
					t.Errorf("unexpected filter 0: %+v", filters[0])
				}
				if filters[1].Key != "created_at" || filters[1].Operator != ">" {
					t.Errorf("unexpected filter 1: %+v", filters[1])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.SearchWithOptionsFunc = func(ctx context.Context, query string, opts *api.ListEntriesOptions) ([]api.FileEntry, error) {
				tt.expected(opts)
				return []api.FileEntry{}, nil
			}

			err := search(context.Background(), sess, env, tt.args)
			if err != nil {
				t.Errorf("search() error = %v", err)
			}
		})
	}
}

func decodeFilters(t *testing.T, encoded string) []api.Filter {
	t.Helper()
	if encoded == "" {
		return nil
	}
	jsonBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	var filters []api.Filter
	if err := json.Unmarshal(jsonBytes, &filters); err != nil {
		t.Fatalf("failed to unmarshal json: %v", err)
	}
	return filters
}

type mockWriter struct{}

func (m *mockWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}
