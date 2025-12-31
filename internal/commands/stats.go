package commands

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

func init() {
	Register(&Command{
		Name:        "stat",
		Description: "Display file status",
		Usage: `stat <file>

Shows detailed metadata about a file or folder:
  - File name and type
  - Size in bytes
  - Internal ID and hash
  - Created and modified timestamps
  - MIME type (for media files)

Examples:
  stat document.pdf       Show info about a file
  stat Photos/            Show info about a folder`,
		Run: stat,
	})

	Register(&Command{
		Name:        "tree",
		Description: "List contents in a tree-like format",
		Usage: `tree [path]

Displays directory structure as a tree.
Defaults to current directory if no path specified.

Examples:
  tree              Show tree from current directory
  tree Photos/      Show tree starting from Photos folder
  tree /            Show tree from root

Note: Limited to 20 levels deep to prevent excessive API calls.`,
		Run: tree,
	})
}

func stat(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: stat <file>")
	}

	path := args[0]
	cached, err := ResolveEntry(ctx, s, path)
	if err != nil {
		return fmt.Errorf("stat: %v", err)
	}

	// Fetch fresh details from API (with spinner for slow requests)
	entry, _ := ui.WithSpinner(env.Stdout, "", func() (*api.FileEntry, error) {
		return s.Client.GetEntry(ctx, cached.ID, s.WorkspaceID)
	})
	if entry == nil {
		// Could be 404 (deleted remotely), network error, etc.
		// Silently use cached data - it's still useful
		entry = cached
	}

	label := ui.MutedStyle.Render
	fmt.Fprintf(env.Stdout, "%s %s\n", label("  File:"), ui.StyleName(entry.Name, entry.Type))
	fmt.Fprintf(env.Stdout, "%s %s\n", label("  Size:"), ui.SizeStyle.Render(fmt.Sprintf("%d", entry.Size)))
	fmt.Fprintf(env.Stdout, "%s %s\n", label("  Type:"), ui.StyleForType(entry.Type).Render(entry.Type))
	fmt.Fprintf(env.Stdout, "%s %s\n", label("    ID:"), ui.MutedStyle.Render(fmt.Sprintf("%d", entry.ID)))
	fmt.Fprintf(env.Stdout, "%s %s\n", label("  Hash:"), ui.MutedStyle.Render(entry.Hash))
	if entry.UpdatedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "%s %s\n", label("Modify:"), ui.MutedStyle.Render("<unknown>"))
	} else {
		fmt.Fprintf(env.Stdout, "%s %s\n", label("Modify:"), ui.DateStyle.Render(entry.UpdatedAt.String()))
	}
	if entry.CreatedAt.IsZero() {
		fmt.Fprintf(env.Stdout, "%s %s\n", label("Create:"), ui.MutedStyle.Render("<unknown>"))
	} else {
		fmt.Fprintf(env.Stdout, "%s %s\n", label("Create:"), ui.DateStyle.Render(entry.CreatedAt.String()))
	}
	if entry.Type == "image" || entry.Type == "video" {
		fmt.Fprintf(env.Stdout, "%s %s\n", label("  Mime:"), ui.MutedStyle.Render(entry.Mime))
	}

	return nil
}

func tree(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	rootPath := "."
	if len(args) > 0 {
		rootPath = args[0]
	}

	resolved, err := s.ResolvePathArg(rootPath)
	if err != nil {
		return fmt.Errorf("tree: %v", err)
	}
	rootEntry, ok := s.Cache.Get(resolved)
	if !ok {
		return fmt.Errorf("tree: %s: No such directory", rootPath)
	}

	if rootEntry.Type != "folder" {
		fmt.Fprintf(env.Stderr, "%s [error opening dir]\n", rootPath)
		return nil
	}

	fmt.Fprintln(env.Stdout, rootPath)
	return walkTree(ctx, s, rootEntry, "", 0, env.Stdout)
}

func walkTree(ctx context.Context, s *session.Session, parent *api.FileEntry, prefix string, depth int, w io.Writer) error {
	// Hard limit on recursion depth to prevent infinite loops or API spam
	if depth > 20 {
		fmt.Fprintf(w, "%s... (max depth reached)\n", prefix)
		return nil
	}

	// API call for children - use vault API if in vault
	var children []api.FileEntry
	var err error
	if s.InVault {
		children, err = s.Client.ListVaultEntries(ctx, parent.Hash)
	} else {
		apiOpts := api.ListOptions(s.WorkspaceID)
		children, err = s.Client.ListByParentIDWithOptions(ctx, &parent.ID, apiOpts)
	}
	if err != nil {
		return err
	}

	// Sort by name
	sort.Slice(children, func(i, j int) bool {
		return children[i].Name < children[j].Name
	})

	for i, child := range children {
		isLast := i == len(children)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		fmt.Fprintf(w, "%s%s%s\n", ui.MutedStyle.Render(prefix), ui.MutedStyle.Render(connector), ui.StyleName(child.Name, child.Type))

		if child.Type == "folder" {
			newPrefix := prefix + "│   "
			if isLast {
				newPrefix = prefix + "    "
			}
			err := walkTree(ctx, s, &child, newPrefix, depth+1, w)
			if err != nil {
				// Warn but continue
				fmt.Fprintf(w, "%s[Error: %v]\n", newPrefix, err)
			}
		}
	}
	return nil
}
