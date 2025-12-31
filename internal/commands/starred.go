package commands

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

func invalidateAndRefreshCWD(ctx context.Context, s *session.Session, touchedPaths []string) error {
	parentDirs := make(map[string]struct{})
	for _, p := range touchedPaths {
		parentDirs[filepath.Dir(p)] = struct{}{}
	}

	for dir := range parentDirs {
		s.Cache.InvalidateChildren(dir)
	}

	// If the current working directory was affected, refresh it immediately.
	if _, ok := parentDirs[s.CWD]; !ok {
		return nil
	}

	// Resolve parent ID (nil for root)
	var parentID *int64
	if s.CWD != "/" {
		if parentEntry, ok := s.Cache.Get(s.CWD); ok {
			parentID = &parentEntry.ID
		} else {
			// Can't refresh safely; leave invalidated so next ls will refetch.
			return nil
		}
	}

	opts := api.ListOptions(s.WorkspaceID)
	children, err := s.Client.ListByParentIDWithOptions(ctx, parentID, opts)
	if err != nil {
		// Best-effort refresh. Cache is already invalidated.
		return err
	}

	// Update cache entries for the directory and mark it loaded.
	s.Cache.InvalidateChildren(s.CWD)
	for i := range children {
		childPath := filepath.Join(s.CWD, children[i].Name)
		s.Cache.Add(&children[i], childPath)
	}
	s.Cache.MarkChildrenLoaded(s.CWD)
	return nil
}

func init() {
	Register(&Command{
		Name:        "star",
		Description: "Manage starred files",
		Usage: `star [command] <file>...

Commands:
  star ls                 List all starred files
  star <file>...          Mark files as starred
  star remove <file>...   Remove starred status (alias: unstar)

Examples:
  star file.txt           Star a single file
  star ls                 List starred files
  star remove file.txt    Unstar a file`,
		Run: starCmd,
	})
}

func starCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: star <file>... or star ls")
	}

	// Handle subcommands
	if args[0] == "ls" || args[0] == "list" {
		return starredList(ctx, s, env, args[1:])
	}
	if args[0] == "remove" || args[0] == "rm" {
		if len(args) < 2 {
			return fmt.Errorf("usage: star remove <file>...")
		}
		return unstarCmd(ctx, s, env, args[1:])
	}

	// Collect entry IDs for all specified files
	var entryIDs []int64
	var names []string
	var touchedPaths []string

	for _, arg := range args {
		resolved, err := s.ResolvePathArg(arg)
		if err != nil {
			fmt.Fprintf(env.Stderr, "star: %s: %v\n", arg, err)
			continue
		}
		entry, ok := s.Cache.Get(resolved)
		if !ok {
			fmt.Fprintf(env.Stderr, "star: %s: No such file or directory\n", arg)
			continue
		}
		entryIDs = append(entryIDs, entry.ID)
		names = append(names, entry.Name)
		touchedPaths = append(touchedPaths, resolved)
	}

	if len(entryIDs) == 0 {
		return fmt.Errorf("no valid files to star")
	}

	var refreshErr error
	err := ui.WithSpinnerErr(env.Stderr, "", func() error {
		if err := s.Client.StarEntries(ctx, entryIDs, s.WorkspaceID); err != nil {
			return err
		}
		refreshErr = invalidateAndRefreshCWD(ctx, s, touchedPaths)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to star: %w", err)
	}
	if refreshErr != nil {
		fmt.Fprintf(env.Stderr, "warning: failed to refresh current directory: %v\n", refreshErr)
	}

	// Report success
	if len(names) == 1 {
		fmt.Fprintf(env.Stdout, "Starred '%s'\n", names[0])
	} else {
		fmt.Fprintf(env.Stdout, "Starred %d items\n", len(names))
	}

	return nil
}
func starredList(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	opts := api.ListOptions(s.WorkspaceID).WithStarredOnly()

	entries, err := ui.WithSpinner(env.Stdout, "", func() ([]api.FileEntry, error) {
		return s.Client.ListByParentIDWithOptions(ctx, nil, opts)
	})
	if err != nil {
		return fmt.Errorf("failed to list starred files: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(env.Stdout, "No starred files")
		return nil
	}

	// Sort by name
	// TODO: Maybe sort by starred_at if available?
	// For now, name is fine.
	// sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	// Use standard ls formatting
	return printLong(s, "starred", entries, false, env.Stdout)
}
func unstarCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: unstar <file>...")
	}

	// Collect entry IDs for all specified files
	var entryIDs []int64
	var names []string
	var touchedPaths []string

	for _, arg := range args {
		resolved, err := s.ResolvePathArg(arg)
		if err != nil {
			fmt.Fprintf(env.Stderr, "unstar: %s: %v\n", arg, err)
			continue
		}
		entry, ok := s.Cache.Get(resolved)
		if !ok {
			fmt.Fprintf(env.Stderr, "unstar: %s: No such file or directory\n", arg)
			continue
		}
		entryIDs = append(entryIDs, entry.ID)
		names = append(names, entry.Name)
		touchedPaths = append(touchedPaths, resolved)
	}

	if len(entryIDs) == 0 {
		return fmt.Errorf("no valid files to unstar")
	}

	var refreshErr error
	err := ui.WithSpinnerErr(env.Stderr, "", func() error {
		if err := s.Client.UnstarEntries(ctx, entryIDs, s.WorkspaceID); err != nil {
			return err
		}
		refreshErr = invalidateAndRefreshCWD(ctx, s, touchedPaths)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to unstar: %w", err)
	}
	if refreshErr != nil {
		fmt.Fprintf(env.Stderr, "warning: failed to refresh current directory: %v\n", refreshErr)
	}

	// Report success
	if len(names) == 1 {
		fmt.Fprintf(env.Stdout, "Unstarred '%s'\n", names[0])
	} else {
		fmt.Fprintf(env.Stdout, "Unstarred %d items\n", len(names))
	}

	return nil
}
