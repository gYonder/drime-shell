package commands

import (
	"context"
	"fmt"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

func init() {
	Register(&Command{
		Name:        "find",
		Description: "Search for files in a directory hierarchy",
		Usage: `find [path] [expression]

Search for files using server-side search.

Flags:
  -name <pattern>   File name contains pattern (wildcard search).
  -type <c>         File is of type c:
                      f: regular file
                      d: directory
  -S, --starred     Only show starred files.
  --trash           Show items in trash.
  --shared          Show files shared by me.

Examples:
  find -name "vacation"           Find files containing 'vacation'
  find -name ".go" -type f        Find files containing '.go'
  find /Photos -type d            Find folders in /Photos (direct children only)
  find -S -name "important"       Find starred files containing 'important'
  find --shared                   Find all files I've shared

Note: When a path is specified, only direct children of that folder are searched.
      For recursive search, omit the path to search the entire workspace.`,
		Run: find,
	})
}

func find(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("find", pflag.ContinueOnError)
	fs.SetOutput(env.Stderr)

	namePattern := fs.String("name", "", "File name pattern (substring match)")
	fileType := fs.String("type", "", "File type (f=file, d=directory)")
	starred := fs.BoolP("starred", "S", false, "Only show starred files")
	trash := fs.Bool("trash", false, "Show items in trash")
	shared := fs.Bool("shared", false, "Show files shared by me")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check for path argument
	var parentID *int64
	if fs.NArg() > 0 {
		searchPath := fs.Arg(0)
		resolvedPath, err := s.ResolvePathArg(searchPath)
		if err != nil {
			return fmt.Errorf("find: %v", err)
		}
		entry, ok := s.Cache.Get(resolvedPath)
		if !ok {
			return fmt.Errorf("find: %s: No such file or directory", searchPath)
		}
		if entry.Type != "folder" {
			return fmt.Errorf("find: %s: Not a directory", searchPath)
		}
		parentID = &entry.ID
	}

	// Build search options
	opts := api.ListOptions(s.WorkspaceID)

	// Set query from -name pattern
	if *namePattern != "" {
		opts.Query = *namePattern
	}

	// Build filters
	var filters []api.Filter

	// Type filter: -type d maps to folder
	if *fileType == "d" {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyType,
			Value:    "folder",
			Operator: api.FilterOpEquals,
		})
	}

	// Starred filter
	if *starred {
		opts = opts.WithStarredOnly()
	}

	// Trash filter
	if *trash {
		opts = opts.WithDeletedOnly()
	}

	// Shared filter
	if *shared {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeySharedByMe,
			Value:    true,
			Operator: api.FilterOpEquals,
		})
	}

	// Encode filters if any
	if len(filters) > 0 {
		opts.Filters = api.EncodeFilters(filters)
	}

	// Perform search
	var results []api.FileEntry
	var err error

	results, err = ui.WithSpinner(env.Stdout, "", false, func() ([]api.FileEntry, error) {
		if parentID != nil {
			// Search within specific folder (direct children only)
			return s.Client.ListByParentIDWithOptions(ctx, parentID, opts)
		}
		// Workspace-wide search
		return s.Client.ListByParentIDWithOptions(ctx, nil, opts)
	})
	if err != nil {
		return fmt.Errorf("find: %v", err)
	}

	// Client-side filtering for -type f (exclude folders)
	if *fileType == "f" {
		filtered := make([]api.FileEntry, 0, len(results))
		for _, r := range results {
			if r.Type != "folder" {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// Filter out trash items unless --trash specified
	if !*trash {
		filtered := make([]api.FileEntry, 0, len(results))
		for _, r := range results {
			if !r.IsInTrash() {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	if len(results) == 0 {
		return nil // No output for empty results (Unix find behavior)
	}

	// Output results - one per line for piping
	for _, r := range results {
		fmt.Fprintln(env.Stdout, r.Name)
	}

	return nil
}
