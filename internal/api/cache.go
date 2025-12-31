package api

import (
	"context"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
)

// FileCache maps paths to their API identifiers and vice-versa
type FileCache struct {
	entries        map[string]*FileEntry // path -> entry
	byID           map[int64]*FileEntry  // id -> entry
	pathByID       map[int64]string      // id -> path (best-effort)
	loadedChildren map[string]bool       // paths whose children have been fetched
	mu             sync.RWMutex
}

func NewFileCache() *FileCache {
	return &FileCache{
		entries:        make(map[string]*FileEntry),
		byID:           make(map[int64]*FileEntry),
		pathByID:       make(map[int64]string),
		loadedChildren: make(map[string]bool),
	}
}

// Add inserts an entry into the cache at specific path
func (c *FileCache) Add(entry *FileEntry, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[path] = entry
	c.byID[entry.ID] = entry
	c.pathByID[entry.ID] = path
}

// Get retrieves an entry by path
func (c *FileCache) Get(path string) (*FileEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[path]
	return e, ok
}

// GetByID retrieves an entry by ID
func (c *FileCache) GetByID(id int64) (*FileEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.byID[id]
	return e, ok
}

// PathForID returns the best-known path for an entry ID.
// For files, this is only available if the entry was cached with a path.
func (c *FileCache) PathForID(id int64) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.pathByID[id]
	return p, ok
}

// Remove deletes an entry from the cache by path
func (c *FileCache) Remove(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[path]; ok {
		delete(c.byID, entry.ID)
		delete(c.pathByID, entry.ID)
		delete(c.entries, path)
	}
}

// AddChildren adds child entries under a parent path and marks it as loaded
func (c *FileCache) AddChildren(parentPath string, children []FileEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range children {
		child := &children[i]
		var childPath string
		if parentPath == "/" {
			childPath = "/" + child.Name
		} else {
			childPath = parentPath + "/" + child.Name
		}
		c.entries[childPath] = child
		c.byID[child.ID] = child
		c.pathByID[child.ID] = childPath
	}
	c.loadedChildren[parentPath] = true
}

// HasChildren returns true if the children of this path have been fetched
func (c *FileCache) HasChildren(path string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loadedChildren[path]
}

// InvalidateChildren marks a path's children as not loaded, forcing a refresh on next access
func (c *FileCache) InvalidateChildren(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.loadedChildren, path)
}

// MarkChildrenLoaded marks a path's children as having been loaded
func (c *FileCache) MarkChildrenLoaded(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loadedChildren[path] = true
}

// MatchGlob returns all cached paths matching a glob pattern in a specific directory.
// Pattern should be just the filename pattern (e.g., "*.txt"), parentPath is the directory to search in.
func (c *FileCache) MatchGlob(parentPath string, pattern string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var matches []string
	prefix := parentPath
	if prefix != "/" {
		prefix += "/"
	}

	for path := range c.entries {
		if path == parentPath {
			continue
		}
		// Check if this is a direct child
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		remainder := path[len(prefix):]
		// Direct child has no more slashes
		if strings.Contains(remainder, "/") {
			continue
		}
		// Match against pattern
		if matched, _ := doublestar.Match(pattern, remainder); matched {
			matches = append(matches, path)
		}
	}
	return matches
}

// GetChildren returns cached children for a path, or nil if not loaded
func (c *FileCache) GetChildren(parentPath string) []FileEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.loadedChildren[parentPath] {
		return nil
	}

	// Return empty slice (not nil) for loaded but empty directories
	children := []FileEntry{}
	prefix := parentPath
	if prefix != "/" {
		prefix += "/"
	}

	for path, entry := range c.entries {
		if path == parentPath {
			continue
		}
		// Check if this is a direct child (not a grandchild)
		if strings.HasPrefix(path, prefix) {
			remainder := path[len(prefix):]
			// Direct child has no more slashes
			if !strings.Contains(remainder, "/") {
				children = append(children, *entry)
			}
		}
	}
	return children
}

// LoadFolderTree fetches all folders and builds the path map
func (c *FileCache) LoadFolderTree(ctx context.Context, client DrimeClient, userID int64, username string, workspaceID int64) error {
	folders, err := client.GetUserFolders(ctx, userID, workspaceID)
	if err != nil {
		return err
	}

	// 1. Index all folders by ID to allow parent lookup
	tempByID := make(map[int64]*FileEntry)
	for i := range folders {
		tempByID[folders[i].ID] = &folders[i]
	}

	// 2. Build paths
	// We need to handle potential disconnects or cycles, but assuming API provides valid tree
	// A simple approach: for each folder, walk up to root

	// Optimization: Sort by ID or depth could help, but map walk is fine for <10k folders

	c.mu.Lock()
	defer c.mu.Unlock()

	// Add synthetic root entry for "/"
	// Root has no ID in Drime API - items at root have parent_id = null
	c.entries["/"] = &FileEntry{
		ID:      0, // Synthetic ID for root
		Name:    "/",
		Type:    "folder",
		OwnerID: userID,
		Users: []FileEntryUser{
			{ID: userID, DisplayName: username, OwnsEntry: true},
		},
	}
	c.byID[0] = c.entries["/"]
	c.pathByID[0] = "/"

	for _, f := range folders {
		path := buildPath(&f, tempByID)
		c.entries[path] = &f
		c.byID[f.ID] = &f
		c.pathByID[f.ID] = path
	}

	return nil
}

// LoadVaultFolderTree fetches all vault folders and builds the path map.
// This is similar to LoadFolderTree but uses the vault-specific API endpoint.
func (c *FileCache) LoadVaultFolderTree(ctx context.Context, client DrimeClient, userID int64, username string) error {
	folders, err := client.GetVaultFolders(ctx, userID)
	if err != nil {
		return err
	}

	// 1. Index all folders by ID to allow parent lookup
	tempByID := make(map[int64]*FileEntry)
	for i := range folders {
		tempByID[folders[i].ID] = &folders[i]
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Add synthetic root entry for "/"
	c.entries["/"] = &FileEntry{
		ID:      0, // Synthetic ID for root
		Name:    "/",
		Type:    "folder",
		OwnerID: userID,
		Users: []FileEntryUser{
			{ID: userID, DisplayName: username, OwnsEntry: true},
		},
	}
	c.byID[0] = c.entries["/"]
	c.pathByID[0] = "/"

	for _, f := range folders {
		path := buildPath(&f, tempByID)
		c.entries[path] = &f
		c.byID[f.ID] = &f
		c.pathByID[f.ID] = path
	}

	return nil
}

// AllPaths returns all paths currently in the cache (for debugging)
func (c *FileCache) AllPaths() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	paths := make([]string, 0, len(c.entries))
	for p := range c.entries {
		paths = append(paths, p)
	}
	return paths
}

func buildPath(entry *FileEntry, idMap map[int64]*FileEntry) string {
	parts := []string{}
	current := entry

	for {
		// Add current entry's name to path
		parts = append([]string{current.Name}, parts...)

		// If no parent, we're at root level - stop here
		if current.ParentID == nil {
			break
		}

		parent, ok := idMap[*current.ParentID]
		if !ok {
			// Orphaned or parent not in set (could be shared root?)
			// Treat as top level
			break
		}
		current = parent
	}

	return "/" + strings.Join(parts, "/")
}
