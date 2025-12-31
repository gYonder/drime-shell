package api_test

import (
	"context"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestFileCache_LoadFolderTree(t *testing.T) {
	// Mock folders returned from API
	// In Drime API, top-level folders have nil ParentID
	photosID := int64(1)
	subID := int64(2)
	folders := []api.FileEntry{
		{ID: photosID, Name: "Photos", Type: "folder", ParentID: nil}, // Top-level folder
		{ID: subID, Name: "2024", Type: "folder", ParentID: &photosID},
	}

	// Create mock client
	mockClient := &api.MockDrimeClient{
		GetUserFoldersFunc: func(ctx context.Context, userID int64, workspaceID int64) ([]api.FileEntry, error) {
			return folders, nil
		},
	}

	cache := api.NewFileCache()
	err := cache.LoadFolderTree(context.Background(), mockClient, 123, "testuser", 0)
	assert.NoError(t, err)

	// Check ID lookup
	entry, ok := cache.GetByID(subID)
	assert.True(t, ok, "Should find folder by ID")
	assert.Equal(t, "2024", entry.Name)

	// Check Path lookup - Photos is at /Photos, 2024 is at /Photos/2024
	entry, ok = cache.Get("/Photos")
	assert.True(t, ok, "Should find /Photos")
	assert.Equal(t, "Photos", entry.Name)
	assert.Equal(t, photosID, entry.ID)

	entry, ok = cache.Get("/Photos/2024")
	assert.True(t, ok, "Should find /Photos/2024")
	assert.Equal(t, "2024", entry.Name)
	assert.Equal(t, subID, entry.ID)
}

func TestFileCache_PathResolution(t *testing.T) {
	cache := api.NewFileCache()

	// Manually add structure: / -> Photos -> 2024
	rootID := int64(100)
	photosID := int64(200)
	yearID := int64(300)

	// Note: Cache stores *FileEntry? cache.Add takes (FileEntry, path).
	// Let's check Cache.Add signature.
	// Defined in internal/api/cache.go
	// "func (c *FileCache) Add(entry FileEntry, path string)"

	cache.Add(&api.FileEntry{ID: rootID, Name: "Root", ParentID: nil, Type: "folder"}, "/")
	cache.Add(&api.FileEntry{ID: photosID, Name: "Photos", ParentID: &rootID, Type: "folder"}, "/Photos")
	cache.Add(&api.FileEntry{ID: yearID, Name: "2024", ParentID: &photosID, Type: "folder"}, "/Photos/2024")

	// Test Get
	entry, ok := cache.Get("/Photos/2024")
	assert.True(t, ok)
	assert.Equal(t, yearID, entry.ID)

	// Test case insensitivity or normalization if needed?
	// For now assume case sensitive as per Linux, but Drime might differ. AGENTS.md implies standard shell.
}
