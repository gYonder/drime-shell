package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

// Track which paths are currently being prefetched to avoid duplicate requests
var (
	prefetchMu  sync.Mutex
	prefetching = make(map[string]bool)
)

func init() {
	Register(&Command{
		Name:        "ls",
		Description: "List directory contents",
		Usage:       "ls [-l] [-a] [path]\n\nOptions:\n  -l    Long listing format (size, owner, date, name, starred)\n  -a    Show hidden files (starting with .)\n\nExamples:\n  ls           List current directory\n  ls -la       Long format with hidden files\n  ls /Photos   List specific directory",
		Run:         ls,
	})
	Register(&Command{
		Name:        "cd",
		Description: "Change directory",
		Usage:       "cd [path]\n\nSpecial paths:\n  ~            Home directory\n  -            Previous directory\n  ..           Parent directory\n  .            Current directory",
		Run:         cd,
	})
	Register(&Command{
		Name:        "pwd",
		Description: "Print current working directory",
		Usage:       "pwd",
		Run:         pwd,
	})
	Register(&Command{
		Name:        "exit",
		Description: "Exit the shell",
		Usage:       "exit",
		Run:         exitCmd,
	})
}

func ls(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("ls", pflag.ContinueOnError)
	showAll := fs.BoolP("all", "a", false, "show hidden files")
	longFormat := fs.BoolP("long", "l", false, "use long listing format")
	starredOnly := fs.BoolP("starred", "S", false, "show only starred files")

	// Set output of flag set to env.Stderr for usage?
	fs.SetOutput(env.Stderr)

	if err := fs.Parse(args); err != nil {
		return err
	}

	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}

	opts := &listPathOptions{
		showAll:     *showAll,
		longFormat:  *longFormat,
		starredOnly: *starredOnly,
	}

	for i, path := range paths {
		// If multiple args and this is a directory, print header?
		// We can peek at cache.
		resolved, err := s.ResolvePathArg(path)
		if err == nil {
			if entry, ok := s.Cache.Get(resolved); ok && entry.Type == "folder" && len(paths) > 1 {
				fmt.Fprintf(env.Stdout, "%s:\n", path)
			}
		}

		if err := listPathWithOpts(ctx, s, path, opts, env.Stdout); err != nil {
			fmt.Fprintf(env.Stderr, "%v\n", err)
		}

		if i < len(paths)-1 {
			// Add newline between listings if multiple args
			// But only if we printed something?
			// Or if it was a directory listing?
			// Let's add newline if multiple args.
			if err == nil {
				if entry, ok := s.Cache.Get(resolved); ok && entry.Type == "folder" {
					fmt.Fprintln(env.Stdout)
				}
			}
		}
	}
	return nil
}

// listPathOptions controls the behavior of listPathWithOpts
type listPathOptions struct {
	showAll     bool
	longFormat  bool
	starredOnly bool
}

func listPathWithOpts(ctx context.Context, s *session.Session, path string, opts *listPathOptions, w io.Writer) error {
	resolved, err := s.ResolvePathArg(path)
	if err != nil {
		return fmt.Errorf("ls: %v", err)
	}

	// Check if path exists in cache
	entry, ok := s.Cache.Get(resolved)
	if !ok {
		return fmt.Errorf("ls: cannot access '%s': No such file or directory", path)
	}

	var entries []api.FileEntry

	if entry.Type == "folder" {
		// For starred-only listing, always fetch from API with the filter
		if opts.starredOnly {
			var parentID *int64
			if resolved != "/" {
				parentID = &entry.ID
			}
			apiOpts := api.ListOptions(s.WorkspaceID).WithStarredOnly()
			children, err := ui.WithSpinner(w, "", func() ([]api.FileEntry, error) {
				return s.Client.ListByParentIDWithOptions(ctx, parentID, apiOpts)
			})
			if err != nil {
				return err
			}
			entries = children
		} else if cached := s.Cache.GetChildren(resolved); cached != nil {
			// Check if children are already cached
			entries = cached
		} else {
			// Fetch from API (with spinner for slow requests)
			var parentID *int64
			if resolved != "/" {
				parentID = &entry.ID
			}

			var children []api.FileEntry
			if s.InVault {
				// Use vault-specific listing (use hash, empty string for root)
				folderHash := ""
				if resolved != "/" {
					folderHash = entry.Hash
				}
				children, err = ui.WithSpinner(w, "", func() ([]api.FileEntry, error) {
					return s.Client.ListVaultEntries(ctx, folderHash)
				})
			} else {
				apiOpts := api.ListOptions(s.WorkspaceID)
				children, err = ui.WithSpinner(w, "", func() ([]api.FileEntry, error) {
					return s.Client.ListByParentIDWithOptions(ctx, parentID, apiOpts)
				})
			}
			if err != nil {
				return err
			}
			entries = children

			// Update cache with fetched entries
			s.Cache.AddChildren(resolved, children)
		}
	} else {
		// Just list the file itself
		entries = []api.FileEntry{*entry}
	}

	// Filter hidden (but keep . and .. if showAll)
	if !opts.showAll {
		filtered := entries[:0]
		for _, e := range entries {
			if !strings.HasPrefix(e.Name, ".") {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Sort by name
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	if opts.longFormat {
		return printLong(s, resolved, entries, opts.showAll, w)
	}

	// Short format - only show . and .. with -a flag
	var names []string
	if opts.showAll {
		names = append(names, ui.DirStyle.Render("."))
		names = append(names, ui.DirStyle.Render(".."))
	}
	for _, e := range entries {
		names = append(names, ui.StyleName(e.Name, e.Type))
	}

	printColumns(names, w)
	return nil
}

// printColumns prints names in columns, similar to ls (column-major order)
func printColumns(names []string, w io.Writer) {
	if len(names) == 0 {
		return
	}

	// Get terminal width (default to 80 if unknown)
	termWidth := 80

	// Find the longest visible name (excluding ANSI codes)
	maxLen := 0
	for _, name := range names {
		vLen := ui.VisibleLen(name)
		if vLen > maxLen {
			maxLen = vLen
		}
	}

	// Column width = longest name + 2 spaces padding
	colWidth := maxLen + 2
	if colWidth < 1 {
		colWidth = 1
	}

	// Number of columns that fit
	numCols := termWidth / colWidth
	if numCols < 1 {
		numCols = 1
	}

	// Calculate number of rows needed
	numRows := (len(names) + numCols - 1) / numCols

	// Print in column-major order (filling top-to-bottom, then left-to-right)
	for row := 0; row < numRows; row++ {
		for col := 0; col < numCols; col++ {
			idx := col*numRows + row
			if idx >= len(names) {
				continue
			}

			name := names[idx]
			padding := colWidth - ui.VisibleLen(name)
			if padding < 0 {
				padding = 0
			}

			// Last column or last item in row - no padding, just newline
			isLastCol := col == numCols-1
			isLastInRow := (col+1)*numRows+row >= len(names)
			if isLastCol || isLastInRow {
				fmt.Fprint(w, name)
			} else {
				fmt.Fprintf(w, "%s%s", name, strings.Repeat(" ", padding))
			}
		}
		fmt.Fprintln(w)
	}
}

type longRow struct {
	size  string
	owner string
	date  string
	star  string
	name  string
}

func padLeftVisible(s string, width int) string {
	pad := width - ui.VisibleLen(s)
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

func padRightVisible(s string, width int) string {
	pad := width - ui.VisibleLen(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

func buildLongRow(name string, e *api.FileEntry) longRow {
	size := ui.SizeStyle.Render(formatSize(e.Size))
	owner := e.Owner()
	if owner == "" {
		owner = "-"
	}
	owner = ui.OwnerStyle.Render(owner)
	date := ui.DateStyle.Render(e.UpdatedAt.Format("Jan 02 15:04"))
	// Keep this ASCII + unstyled so column math is stable across terminals.
	star := " "
	if e.IsStarred() {
		star = "*"
	}
	styledName := ui.StyleName(name, e.Type)
	return longRow{size: size, owner: owner, date: date, star: star, name: styledName}
}

func printLong(s *session.Session, dirPath string, entries []api.FileEntry, showAll bool, w io.Writer) error {
	// Calculate total size
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	fmt.Fprintf(w, "total %s\n", formatSize(total))

	rows := make([]longRow, 0, len(entries)+2)

	// Show . and .. only with -a flag
	if showAll {
		if currentEntry, ok := s.Cache.Get(dirPath); ok {
			rows = append(rows, buildLongRow(".", currentEntry))
		}
		if dirPath != "/" {
			parentPath := filepath.Dir(dirPath)
			if parentEntry, ok := s.Cache.Get(parentPath); ok {
				rows = append(rows, buildLongRow("..", parentEntry))
			}
		}
	}

	for _, e := range entries {
		entry := e
		rows = append(rows, buildLongRow(entry.Name, &entry))
	}

	// Compute widths based on visible lengths (ANSI stripped)
	wSize, wOwner, wDate, wName := 0, 0, 0, 0
	for _, r := range rows {
		if l := ui.VisibleLen(r.size); l > wSize {
			wSize = l
		}
		if l := ui.VisibleLen(r.owner); l > wOwner {
			wOwner = l
		}
		if l := ui.VisibleLen(r.date); l > wDate {
			wDate = l
		}
		if l := ui.VisibleLen(r.name); l > wName {
			wName = l
		}
	}

	// Render with fixed column start positions regardless of ANSI sequences.
	for _, r := range rows {
		line := padLeftVisible(r.size, wSize) + "  " +
			padRightVisible(r.owner, wOwner) + "  " +
			padRightVisible(r.date, wDate) + "  " +
			padRightVisible(r.name, wName) + "  " +
			r.star
		fmt.Fprintln(w, line)
	}

	return nil
}

// formatSize returns a human-readable size string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func cd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	var target string
	if len(args) < 1 {
		// cd without args goes to home directory
		return cd(ctx, s, env, []string{s.HomeDir})
	} else {
		target = args[0]
	}

	// Handle special cases
	if target == "-" {
		if s.PreviousDir == "" {
			return fmt.Errorf("cd: OLDPWD not set")
		}
		curDir := s.CWD
		s.CWD = s.PreviousDir
		s.PreviousDir = curDir
		go prefetchDirectory(s, s.CWD, 1)
		return nil
	}

	newPath := s.ResolvePath(target)

	// Verify it exists AND is a directory
	entry, ok := s.Cache.Get(newPath)
	if !ok {
		return fmt.Errorf("cd: %s: No such file or directory", target)
	}
	if entry.Type != "folder" {
		return fmt.Errorf("cd: %s: Not a directory", target)
	}

	s.PreviousDir = s.CWD
	s.CWD = newPath

	// Prefetch in background: current dir contents + one level deeper
	go prefetchDirectory(s, newPath, 1)

	return nil
}

// prefetchDirectory fetches directory contents in background.
// depth controls how many levels deep to prefetch (0 = just this dir, 1 = this + children)
func prefetchDirectory(s *session.Session, path string, depth int) {
	// Check if already prefetching or loaded
	prefetchMu.Lock()
	if prefetching[path] || s.Cache.HasChildren(path) {
		prefetchMu.Unlock()
		return
	}
	prefetching[path] = true
	prefetchMu.Unlock()

	defer func() {
		prefetchMu.Lock()
		delete(prefetching, path)
		prefetchMu.Unlock()
	}()

	// Get the folder entry
	entry, ok := s.Cache.Get(path)
	if !ok || entry.Type != "folder" {
		return
	}

	// Fetch children from API
	var children []api.FileEntry
	var err error

	if s.InVault {
		// Use vault-specific listing (use hash, empty string for root)
		folderHash := ""
		if path != "/" {
			folderHash = entry.Hash
		}
		children, err = s.Client.ListVaultEntries(context.Background(), folderHash)
	} else {
		var parentID *int64
		if path != "/" {
			parentID = &entry.ID
		}
		apiOpts := api.ListOptions(s.WorkspaceID)
		children, err = s.Client.ListByParentIDWithOptions(context.Background(), parentID, apiOpts)
	}
	if err != nil {
		return // Silent fail for background ops
	}

	// Add to cache
	s.Cache.AddChildren(path, children)

	// Prefetch subdirectories one level deeper
	if depth > 0 {
		for _, child := range children {
			if child.Type == "folder" {
				var childPath string
				if path == "/" {
					childPath = "/" + child.Name
				} else {
					childPath = path + "/" + child.Name
				}
				go prefetchDirectory(s, childPath, depth-1)
			}
		}
	}
}

func pwd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fmt.Fprintln(env.Stdout, s.VirtualCWD())
	return nil
}

func exitCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	os.Exit(0)
	return nil
}
