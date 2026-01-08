package shell

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
)

// ExpandGlobs expands glob patterns in arguments.
// It returns the expanded arguments, or the original arguments if no expansion occurred.
func ExpandGlobs(ctx context.Context, s *session.Session, w io.Writer, args []string) ([]string, error) {
	var expanded []string
	for _, arg := range args {
		// Check if arg contains glob characters
		// doublestar supports: *, ?, [], {} (brace expansion)
		// Note: !(pattern), +(pattern), @(pattern) are bash extglob, NOT supported
		if !strings.ContainsAny(arg, "*?[]{") {
			expanded = append(expanded, arg)
			continue
		}

		// It's a glob pattern
		// We need to resolve it relative to CWD
		resolvedPath, err := s.ResolvePathArg(arg)
		if err != nil {
			return nil, err
		}
		parentDir := filepath.Dir(resolvedPath)
		filePattern := filepath.Base(resolvedPath)

		// Ensure parent directory is loaded
		if !s.Cache.HasChildren(parentDir) {
			if parentEntry, ok := s.Cache.Get(parentDir); ok {
				var parentID *int64
				if parentEntry.ID != 0 {
					parentID = &parentEntry.ID
				}

				// Fetch children with spinner
				children, err := ui.WithSpinner(w, "", false, func() ([]api.FileEntry, error) {
					apiOpts := api.ListOptions(s.WorkspaceID)
					return s.Client.ListByParentIDWithOptions(ctx, parentID, apiOpts)
				})
				if err == nil {
					s.Cache.AddChildren(parentDir, children)
				}
				// If error, we continue and try to match against what we have (or nothing)
			}
		}

		matches := s.Cache.MatchGlob(parentDir, filePattern)
		if len(matches) == 0 {
			// No matches, keep original arg (bash behavior)
			expanded = append(expanded, arg)
		} else {
			// Add matches
			for _, match := range matches {
				// Try to preserve relativity
				if !filepath.IsAbs(arg) && strings.HasPrefix(match, s.CWD) {
					// Make relative to CWD
					rel, err := filepath.Rel(s.CWD, match)
					if err == nil {
						expanded = append(expanded, rel)
						continue
					}
				}
				expanded = append(expanded, match)
			}
		}
	}
	return expanded, nil
}
