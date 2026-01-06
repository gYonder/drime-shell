package commands

import (
	"bufio"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

func init() {
	Register(&Command{
		Name:        "trash",
		Description: "View and manage trash",
		Usage: `trash [command]

Commands:
  trash                    List items in trash (alias: trash ls)
  trash restore <file>...  Restore files from trash by name or #ID (default)
  trash empty              Permanently delete all items in trash

Examples:
  trash                    List all trashed items
  trash restore #123       Restore item with ID 123
  trash restore file.txt   Restore by name
  trash empty              Empty the entire trash (with confirmation)`,
		Run: trashCmd,
	})
}

func trashCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) == 0 {
		return trashList(ctx, s, env)
	}

	subcommand := args[0]
	switch subcommand {
	case "ls", "list":
		return trashList(ctx, s, env)
	case "restore":
		if len(args) < 2 {
			return fmt.Errorf("usage: trash restore <file>")
		}
		return restoreCmd(ctx, s, env, args[1:])
	case "empty":
		return trashEmpty(ctx, s, env)
	default:
		return fmt.Errorf("unknown trash command: %s\nUse: ls, restore, or empty", subcommand)
	}
}

func trashList(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	opts := api.ListOptions(s.WorkspaceID).WithDeletedOnly()

	entries, err := ui.WithSpinner(env.Stdout, "", false, func() ([]api.FileEntry, error) {
		return s.Client.ListByParentIDWithOptions(ctx, nil, opts)
	})
	if err != nil {
		return fmt.Errorf("failed to list trash: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(env.Stdout, "Trash is empty")
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].DeletedAt != nil && entries[j].DeletedAt != nil {
			return entries[i].DeletedAt.After(*entries[j].DeletedAt)
		}
		return entries[i].Name < entries[j].Name
	})

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("ID"),
		ui.HeaderStyle.Render("NAME"),
		ui.HeaderStyle.Render("SIZE"),
		ui.HeaderStyle.Render("TYPE"),
		ui.HeaderStyle.Render("DELETED"),
	)

	for _, e := range entries {
		styledName := ui.StyleName(e.Name, e.Type)
		size := ui.SizeStyle.Render(formatSize(e.Size))
		deletedAt := "-"
		if e.DeletedAt != nil {
			deletedAt = e.DeletedAt.Format("Jan 02 15:04")
		}
		t.AddRow(fmt.Sprintf("#%d", e.ID), styledName, size, e.Type, ui.DateStyle.Render(deletedAt))
	}

	t.Render()
	return nil
}

// resolveTrashEntry finds an entry in trash by name or #ID
func resolveTrashEntry(entries []api.FileEntry, selector string) (*api.FileEntry, error) {
	selector = strings.TrimSpace(selector)

	// Support #<id> syntax
	if strings.HasPrefix(selector, "#") {
		idStr := strings.TrimPrefix(selector, "#")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid ID '%s'", selector)
		}
		for i := range entries {
			if entries[i].ID == id {
				return &entries[i], nil
			}
		}
		return nil, fmt.Errorf("no item with ID %d in trash", id)
	}

	// Match by name
	var match *api.FileEntry
	for i := range entries {
		if entries[i].Name == selector {
			if match != nil {
				return nil, fmt.Errorf("ambiguous name '%s' — use #ID instead (run 'trash' to see IDs)", selector)
			}
			match = &entries[i]
		}
	}
	if match == nil {
		return nil, fmt.Errorf("'%s' not found in trash", selector)
	}
	return match, nil
}

func restoreCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("restore", pflag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()

	if len(files) == 0 {
		return fmt.Errorf("usage: restore <file>\n\nRestore files from trash by name or #ID\nRun 'trash' to see trashed items and their IDs")
	}

	// Fetch trash entries
	opts := api.ListOptions(s.WorkspaceID).WithDeletedOnly()
	entries, err := s.Client.ListByParentIDWithOptions(ctx, nil, opts)
	if err != nil {
		return fmt.Errorf("restore: failed to list trash: %w", err)
	}

	// Collect entry IDs
	var entryIDs []int64
	var names []string

	for _, sel := range files {
		entry, matchErr := resolveTrashEntry(entries, sel)
		if matchErr != nil {
			fmt.Fprintf(env.Stderr, "restore: %v\n", matchErr)
			continue
		}
		entryIDs = append(entryIDs, entry.ID)
		names = append(names, entry.Name)
	}

	if len(entryIDs) == 0 {
		return fmt.Errorf("no valid files to restore")
	}

	err = ui.WithSpinnerErr(env.Stderr, "", false, func() error {
		return s.Client.RestoreEntries(ctx, entryIDs, s.WorkspaceID)
	})
	if err != nil {
		return fmt.Errorf("failed to restore: %w", err)
	}

	if len(names) == 1 {
		fmt.Fprintf(env.Stdout, "Restored '%s'\n", names[0])
	} else {
		fmt.Fprintf(env.Stdout, "Restored %d items\n", len(names))
	}

	return nil
}

func trashEmpty(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	fmt.Fprint(env.Stdout, ui.WarningStyle.Render("⚠ This will permanently delete all items in trash. This cannot be undone.\n"))
	fmt.Fprint(env.Stdout, "Type 'yes' to confirm: ")

	reader := bufio.NewReader(env.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "yes" {
		fmt.Fprintln(env.Stdout, "Cancelled")
		return nil
	}

	err = ui.WithSpinnerErr(env.Stderr, "", false, func() error {
		return s.Client.EmptyTrash(ctx, s.WorkspaceID)
	})
	if err != nil {
		return fmt.Errorf("failed to empty trash: %w", err)
	}

	fmt.Fprintln(env.Stdout, ui.SuccessStyle.Render("✓ Trash emptied"))
	return nil
}
