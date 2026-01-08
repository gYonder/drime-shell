package shell

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/chzyer/readline"
	"github.com/gYonder/drime-shell/internal/commands"
	"github.com/gYonder/drime-shell/internal/session"
)

// DrimeCompleter provides tab completion for the shell
type DrimeCompleter struct {
	Session *session.Session
}

// Do implements readline.AutoCompleter
func (c *DrimeCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line[:pos])

	// Split into words
	words := strings.Fields(lineStr)

	// If empty or first word (command completion)
	if len(words) == 0 || (len(words) == 1 && !strings.HasSuffix(lineStr, " ")) {
		prefix := ""
		if len(words) == 1 {
			prefix = words[0]
		}
		return c.completeCommand(prefix)
	}

	// Otherwise, complete paths for the current argument
	// Get the partial path being typed
	lastSpace := strings.LastIndex(lineStr, " ")
	partial := ""
	if lastSpace < len(lineStr)-1 {
		partial = lineStr[lastSpace+1:]
	}

	return c.completePath(partial)
}

// completeCommand returns matching command names
func (c *DrimeCompleter) completeCommand(prefix string) ([][]rune, int) {
	var matches []string

	// Get unique command names (not aliases)
	seen := make(map[string]bool)
	for name, cmd := range commands.Registry {
		if cmd.Name == name && !seen[name] { // Only use primary name
			if strings.HasPrefix(name, prefix) {
				matches = append(matches, name)
				seen[name] = true
			}
		}
	}

	sort.Strings(matches)

	result := make([][]rune, len(matches))
	for i, m := range matches {
		// Return only the suffix that needs to be added
		result[i] = []rune(m[len(prefix):] + " ")
	}

	return result, len(prefix)
}

// completePath returns matching file/folder paths
func (c *DrimeCompleter) completePath(partial string) ([][]rune, int) {
	// Resolve the directory to search in
	var searchDir string
	var searchPrefix string

	if partial == "" {
		searchDir = c.Session.CWD
		searchPrefix = ""
	} else if strings.HasPrefix(partial, "/") {
		// Absolute path
		if strings.HasSuffix(partial, "/") {
			// e.g., "/foo/bar/" - search inside bar
			searchDir = filepath.Clean(partial)
			searchPrefix = ""
		} else {
			searchDir = filepath.Dir(partial)
			searchPrefix = filepath.Base(partial)
			if partial == "/" {
				searchDir = "/"
				searchPrefix = ""
			}
		}
	} else if strings.Contains(partial, "/") {
		// Relative path with directory components
		if strings.HasSuffix(partial, "/") {
			// e.g., "temp/" - search inside temp
			searchDir = c.Session.ResolvePath(strings.TrimSuffix(partial, "/"))
			searchPrefix = ""
		} else {
			// e.g., "temp/Upl" - search in temp for things starting with Upl
			searchDir = c.Session.ResolvePath(filepath.Dir(partial))
			searchPrefix = filepath.Base(partial)
		}
	} else {
		// Simple name in current directory
		searchDir = c.Session.CWD
		searchPrefix = partial
	}

	// Normalize searchDir
	searchDir = filepath.Clean(searchDir)

	// Get entries from cache
	var matches []string

	// Look up the directory in cache (it should exist from folder tree)
	dirEntry, ok := c.Session.Cache.Get(searchDir)
	if !ok || dirEntry.Type != "folder" {
		return nil, 0
	}

	// Iterate through all cached paths to find direct children of searchDir
	allPaths := c.Session.Cache.AllPaths()
	for _, path := range allPaths {
		// Normalize path for comparison
		path = filepath.Clean(path)

		// Check if this path is a direct child of searchDir
		parent := filepath.Dir(path)
		if parent != searchDir {
			continue
		}

		name := filepath.Base(path)
		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(searchPrefix)) {
			// Check if it's a directory to add trailing slash
			entry, ok := c.Session.Cache.Get(path)
			if ok && entry.Type == "folder" {
				matches = append(matches, name+"/")
			} else {
				matches = append(matches, name)
			}
		}
	}

	sort.Strings(matches)

	result := make([][]rune, len(matches))
	for i, m := range matches {
		// Return only the suffix that needs to be added
		suffix := m[len(searchPrefix):]
		// Add space after files, not after directories (user may want to continue path)
		if !strings.HasSuffix(suffix, "/") {
			suffix += " "
		}
		result[i] = []rune(suffix)
	}

	return result, len(searchPrefix)
}

// NewCompleter creates a new DrimeCompleter
func NewCompleter(s *session.Session) readline.AutoCompleter {
	return &DrimeCompleter{Session: s}
}
