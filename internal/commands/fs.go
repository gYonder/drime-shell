package commands

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

func init() {
	Register(&Command{
		Name:        "mkdir",
		Description: "Create a directory",
		Usage:       "mkdir [-p] <path>...\\n\\nOptions:\\n  -p    Create parent directories as needed\\n\\nExamples:\\n  mkdir photos          Create a directory\\n  mkdir -p a/b/c        Create nested directories",
		Run:         mkdir,
	})
	Register(&Command{
		Name:        "rm",
		Description: "Remove files or directories (moves to trash by default)",
		Usage:       "rm [-rf] [--forever|-F] <path>...\n\nOptions:\n  -r, -R        Remove directories recursively\n  -f            Force removal without prompting\n  --forever, -F Permanently delete (bypass trash)\n\nBy default, rm moves files to trash. Use --forever to permanently delete.\nUse 'trash' command to view and restore trashed items.\n\nExamples:\n  rm file.txt           Move file to trash\n  rm -rf folder/        Move folder to trash\n  rm -F file.txt        Permanently delete file\n  rm *.tmp              Move matching files to trash",
		Run:         rm,
	})
}

func mkdir(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// Parse flags
	createParents := false
	var paths []string
	for _, arg := range args {
		if arg == "-p" {
			createParents = true
		} else {
			paths = append(paths, arg)
		}
	}

	if len(paths) < 1 {
		return fmt.Errorf("usage: mkdir [-p] <path>")
	}

	for _, path := range paths {
		if err := mkdirOne(ctx, s, env, path, createParents); err != nil {
			return err
		}
	}
	return nil
}

func mkdirOne(ctx context.Context, s *session.Session, env *ExecutionEnv, path string, createParents bool) error {
	// Resolve to absolute path
	targetPath, err := s.ResolvePathArg(path)
	if err != nil {
		return fmt.Errorf("mkdir: %v", err)
	}

	// Check if already exists
	if _, ok := s.Cache.Get(targetPath); ok {
		if createParents {
			// -p silently succeeds if directory exists
			return nil
		}
		return fmt.Errorf("mkdir: cannot create directory '%s': File exists", path)
	}

	// Split path into components
	// e.g., "/foo/bar/baz" -> ["foo", "bar", "baz"]
	parts := strings.Split(strings.Trim(targetPath, "/"), "/")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return fmt.Errorf("mkdir: invalid path")
	}

	// Find how much of the path already exists
	existingPath := "/"
	startIdx := 0

	for i, part := range parts {
		checkPath := filepath.Join(existingPath, part)
		if entry, ok := s.Cache.Get(checkPath); ok {
			if entry.Type != "folder" {
				return fmt.Errorf("mkdir: '%s' is not a directory", checkPath)
			}
			existingPath = checkPath
			startIdx = i + 1
		} else {
			break
		}
	}

	// Parts that need to be created
	toCreate := parts[startIdx:]

	if len(toCreate) == 0 {
		// Path already fully exists
		if createParents {
			return nil
		}
		return fmt.Errorf("mkdir: cannot create directory '%s': File exists", path)
	}

	// Without -p, we can only create the final directory if parent exists
	if !createParents && len(toCreate) > 1 {
		missingParent := filepath.Join(existingPath, toCreate[0])
		return fmt.Errorf("mkdir: cannot create directory '%s': No such file or directory", missingParent)
	}

	// Create directories one by one
	currentPath := existingPath
	var lastEntry *api.FileEntry

	err = ui.WithSpinnerErr(env.Stderr, "", func() error {
		for _, name := range toCreate {
			// Get parent ID
			var parentID *int64
			if currentPath != "/" {
				if parentEntry, ok := s.Cache.Get(currentPath); ok {
					parentID = &parentEntry.ID
				}
			}

			var newEntry *api.FileEntry
			var createErr error
			if s.InVault {
				newEntry, createErr = s.Client.CreateVaultFolder(ctx, name, parentID, s.VaultID)
			} else {
				newEntry, createErr = s.Client.CreateFolder(ctx, name, parentID, s.WorkspaceID)
			}
			if createErr != nil {
				return fmt.Errorf("mkdir: failed to create '%s': %w", filepath.Join(currentPath, name), createErr)
			}

			// Add to cache
			newPath := filepath.Join(currentPath, name)
			s.Cache.Add(newEntry, newPath)

			currentPath = newPath
			lastEntry = newEntry
		}
		return nil
	})
	if err != nil {
		return err
	}

	if lastEntry != nil {
		fmt.Fprintf(env.Stdout, "%s %s %s\n",
			ui.SuccessStyle.Render("Created directory"),
			ui.DirStyle.Render(targetPath),
			ui.MutedStyle.Render(fmt.Sprintf("(ID: %d)", lastEntry.ID)))
	}
	return nil
}

func rm(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	// Parse flags
	recursive := false
	force := false
	forever := false // Permanently delete (bypass trash)
	var patterns []string

	for _, arg := range args {
		if arg == "-r" || arg == "-R" {
			recursive = true
		} else if arg == "-f" {
			force = true
		} else if arg == "-F" || arg == "--forever" {
			forever = true
		} else if arg == "-rf" || arg == "-fr" || arg == "-Rf" || arg == "-fR" {
			recursive = true
			force = true
		} else if arg == "-rF" || arg == "-Fr" || arg == "-RF" || arg == "-FR" {
			recursive = true
			forever = true
		} else if arg == "-rfF" || arg == "-rFf" || arg == "-fFr" || arg == "-frF" || arg == "-Frf" || arg == "-Ffr" {
			recursive = true
			force = true
			forever = true
		} else if len(arg) > 1 && arg[0] == '-' && arg != "--forever" {
			// Handle combined flags like -rfi, etc.
			for _, c := range arg[1:] {
				switch c {
				case 'r', 'R':
					recursive = true
				case 'f':
					force = true
				case 'F':
					forever = true
				}
			}
		} else {
			patterns = append(patterns, arg)
		}
	}

	if len(patterns) < 1 {
		return fmt.Errorf("usage: rm [-rf] <path>...")
	}

	deletedCount := 0
	movedToTrash := false

	err := ui.WithSpinnerErr(env.Stderr, "", func() error {
		var ids []int64
		var resolvedPaths []string

		for _, pattern := range patterns {
			// Check if pattern contains glob characters
			if strings.ContainsAny(pattern, "*?[") {
				// Glob expansion
				resolvedPattern, err := s.ResolvePathArg(pattern)
				if err != nil {
					return fmt.Errorf("rm: %v", err)
				}
				parentDir := filepath.Dir(resolvedPattern)
				filePattern := filepath.Base(pattern)

				// Ensure parent directory's children are loaded
				if !s.Cache.HasChildren(parentDir) {
					// Need to load children first
					if parentEntry, ok := s.Cache.Get(parentDir); ok {
						var parentID *int64
						if parentEntry.ID != 0 {
							parentID = &parentEntry.ID
						}
						apiOpts := api.ListOptions(s.WorkspaceID)
						children, err := s.Client.ListByParentIDWithOptions(ctx, parentID, apiOpts)
						if err != nil {
							return fmt.Errorf("rm: cannot access '%s': %w", parentDir, err)
						}
						s.Cache.AddChildren(parentDir, children)
					}
				}

				matches := s.Cache.MatchGlob(parentDir, filePattern)
				if len(matches) == 0 {
					if !force {
						return fmt.Errorf("rm: cannot remove '%s': No such file or directory", pattern)
					}
					continue
				}

				for _, resolved := range matches {
					entry, ok := s.Cache.Get(resolved)
					if !ok {
						continue
					}
					if entry.Type == "folder" && !recursive {
						return fmt.Errorf("rm: cannot remove '%s': Is a directory", resolved)
					}
					ids = append(ids, entry.ID)
					resolvedPaths = append(resolvedPaths, resolved)
				}
				continue
			}

			// Regular path (no glob)
			resolved, err := s.ResolvePathArg(pattern)
			if err != nil {
				return fmt.Errorf("rm: %v", err)
			}
			entry, ok := s.Cache.Get(resolved)
			if !ok {
				if force {
					continue // -f ignores non-existent files
				}
				return fmt.Errorf("rm: cannot remove '%s': No such file or directory", pattern)
			}

			// Check if it's a directory and -r wasn't specified
			if entry.Type == "folder" && !recursive {
				return fmt.Errorf("rm: cannot remove '%s': Is a directory", pattern)
			}

			ids = append(ids, entry.ID)
			resolvedPaths = append(resolvedPaths, resolved)
		}

		if len(ids) == 0 {
			return nil // Nothing to delete (all were non-existent with -f)
		}

		if s.InVault {
			// Vault always deletes permanently (no trash)
			if err := s.Client.DeleteVaultEntries(ctx, ids); err != nil {
				return err
			}
			forever = true // Mark as permanent for message display
		} else if forever {
			// Permanently delete (bypass trash)
			if err := s.Client.DeleteEntriesForever(ctx, ids, s.WorkspaceID); err != nil {
				return err
			}
		} else {
			// Move to trash (default)
			if err := s.Client.DeleteEntries(ctx, ids, s.WorkspaceID); err != nil {
				return err
			}
			movedToTrash = true
		}

		// Remove from cache
		for _, resolved := range resolvedPaths {
			s.Cache.Remove(resolved)
		}

		deletedCount = len(ids)
		return nil
	})
	if err != nil {
		return err
	}

	// Unix rm is silent on success, but we'll give a hint about trash
	if movedToTrash && deletedCount == 1 {
		fmt.Fprintln(env.Stderr, ui.MutedStyle.Render("(Moved to trash. Use 'rm -F' to delete permanently)"))
	}
	return nil
}
