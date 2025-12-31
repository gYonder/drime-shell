package commands_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/commands"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// LS COMMAND TESTS - Testing multi-argument support post-glob expansion
// ============================================================================

// setupTestEnv creates a test session and execution environment
func setupTestEnv(t *testing.T) (*session.Session, *commands.ExecutionEnv, *bytes.Buffer) {
	cache := api.NewFileCache()
	mockClient := &api.MockDrimeClient{
		ListByParentIDFunc: func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
			return []api.FileEntry{}, nil
		},
	}

	s := session.NewSession(mockClient, cache)
	s.CWD = "/"
	s.HomeDir = "/"
	s.UserID = 123
	s.Username = "testuser"

	// Setup root
	cache.Add(&api.FileEntry{ID: 0, Name: "/", Type: "folder"}, "/")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	env := &commands.ExecutionEnv{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	}

	return s, env, &stdout
}

func TestLs_SingleDirectory(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup /Documents
	docsID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: docsID, Name: "Documents", Type: "folder"}, "/Documents")
	s.Cache.AddChildren("/Documents", []api.FileEntry{
		{ID: 101, Name: "file1.txt", Type: "text", ParentID: &docsID, Size: 100},
		{ID: 102, Name: "file2.txt", Type: "text", ParentID: &docsID, Size: 200},
	})

	s.CWD = "/Documents"

	// Get the ls command
	cmd, ok := commands.Get("ls")
	require.True(t, ok, "ls command should exist")

	err := cmd.Run(context.Background(), s, env, []string{})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file2.txt")
}

func TestLs_MultipleFiles(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup /src with files
	srcID := int64(200)
	s.Cache.Add(&api.FileEntry{ID: srcID, Name: "src", Type: "folder"}, "/src")
	s.Cache.AddChildren("/src", []api.FileEntry{
		{ID: 201, Name: "main.go", Type: "text", ParentID: &srcID, Size: 1500},
		{ID: 202, Name: "util.go", Type: "text", ParentID: &srcID, Size: 800},
		{ID: 203, Name: "test.go", Type: "text", ParentID: &srcID, Size: 1200},
		{ID: 204, Name: "config.yaml", Type: "text", ParentID: &srcID, Size: 300},
	})

	// Also add the individual file entries so they can be resolved
	s.Cache.Add(&api.FileEntry{ID: 201, Name: "main.go", Type: "text", ParentID: &srcID, Size: 1500}, "/src/main.go")
	s.Cache.Add(&api.FileEntry{ID: 202, Name: "util.go", Type: "text", ParentID: &srcID, Size: 800}, "/src/util.go")

	s.CWD = "/src"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// Simulate post-glob expansion: ls main.go util.go
	// This is what happens after ExpandGlobs turns *.go into individual files
	err := cmd.Run(context.Background(), s, env, []string{"main.go", "util.go"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "main.go")
	assert.Contains(t, output, "util.go")
}

func TestLs_MultipleDirectories(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup multiple directories
	docsID := int64(100)
	photosID := int64(200)

	s.Cache.Add(&api.FileEntry{ID: docsID, Name: "Documents", Type: "folder"}, "/Documents")
	s.Cache.AddChildren("/Documents", []api.FileEntry{
		{ID: 101, Name: "report.pdf", Type: "text", ParentID: &docsID},
	})

	s.Cache.Add(&api.FileEntry{ID: photosID, Name: "Photos", Type: "folder"}, "/Photos")
	s.Cache.AddChildren("/Photos", []api.FileEntry{
		{ID: 201, Name: "vacation.jpg", Type: "image", ParentID: &photosID},
	})

	// Add to root
	s.Cache.AddChildren("/", []api.FileEntry{
		{ID: docsID, Name: "Documents", Type: "folder"},
		{ID: photosID, Name: "Photos", Type: "folder"},
	})

	s.CWD = "/"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// ls Documents Photos - should show both with headers
	err := cmd.Run(context.Background(), s, env, []string{"Documents", "Photos"})
	require.NoError(t, err)

	output := stdout.String()
	// Should have headers for each directory
	assert.Contains(t, output, "Documents:")
	assert.Contains(t, output, "Photos:")
	assert.Contains(t, output, "report.pdf")
	assert.Contains(t, output, "vacation.jpg")
}

func TestLs_MixedFilesAndDirectories(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup structure
	srcID := int64(100)
	libID := int64(200)

	s.Cache.Add(&api.FileEntry{ID: srcID, Name: "src", Type: "folder"}, "/src")
	s.Cache.AddChildren("/src", []api.FileEntry{
		{ID: 101, Name: "main.go", Type: "text", ParentID: &srcID},
	})

	s.Cache.Add(&api.FileEntry{ID: libID, Name: "lib", Type: "folder"}, "/lib")
	s.Cache.AddChildren("/lib", []api.FileEntry{
		{ID: 201, Name: "utils.go", Type: "text", ParentID: &libID},
	})

	// Add a file at root
	readmeID := int64(300)
	s.Cache.Add(&api.FileEntry{ID: readmeID, Name: "README.md", Type: "text"}, "/README.md")

	// Add to root
	s.Cache.AddChildren("/", []api.FileEntry{
		{ID: srcID, Name: "src", Type: "folder"},
		{ID: libID, Name: "lib", Type: "folder"},
		{ID: readmeID, Name: "README.md", Type: "text"},
	})

	s.CWD = "/"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// ls README.md src lib - mixed files and directories
	err := cmd.Run(context.Background(), s, env, []string{"README.md", "src", "lib"})
	require.NoError(t, err)

	output := stdout.String()
	// Should list file directly and have headers for directories
	assert.Contains(t, output, "README.md")
	assert.Contains(t, output, "src:")
	assert.Contains(t, output, "lib:")
}

func TestLs_WithFlags(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup directory with hidden files
	homeID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: homeID, Name: "home", Type: "folder"}, "/home")
	s.Cache.AddChildren("/home", []api.FileEntry{
		{ID: 101, Name: ".bashrc", Type: "text", ParentID: &homeID},
		{ID: 102, Name: ".vimrc", Type: "text", ParentID: &homeID},
		{ID: 103, Name: "file.txt", Type: "text", ParentID: &homeID},
	})

	s.CWD = "/home"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// Test -a flag shows hidden files
	err := cmd.Run(context.Background(), s, env, []string{"-a"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, ".bashrc")
	assert.Contains(t, output, ".vimrc")
	assert.Contains(t, output, "file.txt")
}

func TestLs_LongFormat(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup directory
	docsID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: docsID, Name: "docs", Type: "folder"}, "/docs")
	s.Cache.AddChildren("/docs", []api.FileEntry{
		{ID: 101, Name: "report.txt", Type: "text", Size: 1024, ParentID: &docsID},
	})

	s.CWD = "/docs"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// Test -l flag for long format
	err := cmd.Run(context.Background(), s, env, []string{"-l"})
	require.NoError(t, err)

	output := stdout.String()
	// Long format should include "total" line
	assert.Contains(t, output, "total")
	assert.Contains(t, output, "report.txt")
}

func TestLs_NonExistentPath(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Capture stderr separately
	var stderr bytes.Buffer
	env.Stderr = &stderr

	s.CWD = "/"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// ls on non-existent path should write error
	err := cmd.Run(context.Background(), s, env, []string{"/nonexistent"})
	// ls doesn't return error, it writes to stderr
	_ = err

	errorOut := stderr.String()
	assert.Contains(t, errorOut, "No such file or directory")
	_ = stdout
}

func TestLs_EmptyDirectory(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup empty directory
	emptyID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: emptyID, Name: "empty", Type: "folder"}, "/empty")
	s.Cache.MarkChildrenLoaded("/empty") // Mark as loaded with no children

	s.CWD = "/"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// ls on empty directory - should not error
	err := cmd.Run(context.Background(), s, env, []string{"/empty"})
	require.NoError(t, err)

	// Output might just be empty or have header
	_ = stdout
}

func TestLl_MultipleArgs(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup directories
	dir1ID := int64(100)
	dir2ID := int64(200)

	s.Cache.Add(&api.FileEntry{ID: dir1ID, Name: "dir1", Type: "folder"}, "/dir1")
	s.Cache.AddChildren("/dir1", []api.FileEntry{
		{ID: 101, Name: "a.txt", Type: "text", ParentID: &dir1ID, Size: 100},
	})

	s.Cache.Add(&api.FileEntry{ID: dir2ID, Name: "dir2", Type: "folder"}, "/dir2")
	s.Cache.AddChildren("/dir2", []api.FileEntry{
		{ID: 201, Name: "b.txt", Type: "text", ParentID: &dir2ID, Size: 200},
	})

	s.Cache.AddChildren("/", []api.FileEntry{
		{ID: dir1ID, Name: "dir1", Type: "folder"},
		{ID: dir2ID, Name: "dir2", Type: "folder"},
	})

	s.CWD = "/"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// ls -l dir1 dir2 - long format with multiple dirs
	err := cmd.Run(context.Background(), s, env, []string{"-l", "dir1", "dir2"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "dir1:")
	assert.Contains(t, output, "dir2:")
	assert.Contains(t, output, "a.txt")
	assert.Contains(t, output, "b.txt")
}

func TestLa_ShowsHiddenFiles(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup directory with hidden files
	configID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: configID, Name: "config", Type: "folder"}, "/config")
	s.Cache.AddChildren("/config", []api.FileEntry{
		{ID: 101, Name: ".env", Type: "text", ParentID: &configID},
		{ID: 102, Name: ".gitignore", Type: "text", ParentID: &configID},
		{ID: 103, Name: "config.yaml", Type: "text", ParentID: &configID},
	})

	s.CWD = "/config"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"-a"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, ".env")
	assert.Contains(t, output, ".gitignore")
	assert.Contains(t, output, "config.yaml")
}

// ============================================================================
// CAT COMMAND TESTS - Testing multi-argument support
// ============================================================================

func TestCat_MultipleFiles(t *testing.T) {
	s, env, stdout := setupTestEnv(t)

	// Setup files
	docsID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: docsID, Name: "docs", Type: "folder"}, "/docs")
	s.Cache.Add(&api.FileEntry{
		ID:       101,
		Name:     "file1.txt",
		Type:     "text",
		Hash:     "hash1",
		Size:     11,
		ParentID: &docsID,
	}, "/docs/file1.txt")
	s.Cache.Add(&api.FileEntry{
		ID:       102,
		Name:     "file2.txt",
		Type:     "text",
		Hash:     "hash2",
		Size:     11,
		ParentID: &docsID,
	}, "/docs/file2.txt")

	// Mock download to return content
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.DownloadFunc = func(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*api.FileEntry, error) {
		switch hash {
		case "hash1":
			w.Write([]byte("Content 1\n"))
		case "hash2":
			w.Write([]byte("Content 2\n"))
		}
		return nil, nil
	}

	s.CWD = "/docs"

	cmd, ok := commands.Get("cat")
	require.True(t, ok)

	// cat file1.txt file2.txt
	err := cmd.Run(context.Background(), s, env, []string{"file1.txt", "file2.txt"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Content 1")
	assert.Contains(t, output, "Content 2")
}

// ============================================================================
// RM COMMAND TESTS - Testing multi-argument support
// ============================================================================

func TestRm_MultipleFiles(t *testing.T) {
	s, env, _ := setupTestEnv(t)

	// Setup files
	docsID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: docsID, Name: "docs", Type: "folder"}, "/docs")
	s.Cache.Add(&api.FileEntry{ID: 101, Name: "file1.txt", Type: "text", ParentID: &docsID}, "/docs/file1.txt")
	s.Cache.Add(&api.FileEntry{ID: 102, Name: "file2.txt", Type: "text", ParentID: &docsID}, "/docs/file2.txt")
	s.Cache.Add(&api.FileEntry{ID: 103, Name: "file3.txt", Type: "text", ParentID: &docsID}, "/docs/file3.txt")

	// Track deleted IDs
	var deletedIDs []int64
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.DeleteEntriesFunc = func(ctx context.Context, entryIDs []int64, workspaceID int64) error {
		deletedIDs = append(deletedIDs, entryIDs...)
		return nil
	}

	s.CWD = "/docs"

	cmd, ok := commands.Get("rm")
	require.True(t, ok)

	// rm file1.txt file2.txt file3.txt - should batch delete
	err := cmd.Run(context.Background(), s, env, []string{"file1.txt", "file2.txt", "file3.txt"})
	require.NoError(t, err)

	// All three should be deleted
	assert.Len(t, deletedIDs, 3)
	assert.Contains(t, deletedIDs, int64(101))
	assert.Contains(t, deletedIDs, int64(102))
	assert.Contains(t, deletedIDs, int64(103))
}

// ============================================================================
// CP COMMAND TESTS - Brace expansion use case
// ============================================================================

func TestCp_MultipleFiles(t *testing.T) {
	s, env, _ := setupTestEnv(t)

	// Setup source files
	srcID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: srcID, Name: "src", Type: "folder"}, "/src")
	s.Cache.Add(&api.FileEntry{ID: 101, Name: "file1.txt", Type: "text", ParentID: &srcID}, "/src/file1.txt")
	s.Cache.Add(&api.FileEntry{ID: 102, Name: "file2.txt", Type: "text", ParentID: &srcID}, "/src/file2.txt")

	// Setup destination folder
	destID := int64(200)
	s.Cache.Add(&api.FileEntry{ID: destID, Name: "backup", Type: "folder"}, "/backup")

	// Track copied IDs
	var copiedIDs []int64
	var destParentID *int64
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.CopyEntriesFunc = func(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) ([]api.FileEntry, error) {
		copiedIDs = append(copiedIDs, entryIDs...)
		destParentID = destinationParentID
		return []api.FileEntry{}, nil
	}

	s.CWD = "/"

	cmd, ok := commands.Get("cp")
	require.True(t, ok)

	// cp /src/file1.txt /src/file2.txt /backup
	err := cmd.Run(context.Background(), s, env, []string{"/src/file1.txt", "/src/file2.txt", "/backup"})
	require.NoError(t, err)

	// Both files should be copied
	assert.Len(t, copiedIDs, 2)
	assert.Contains(t, copiedIDs, int64(101))
	assert.Contains(t, copiedIDs, int64(102))
	require.NotNil(t, destParentID)
	assert.Equal(t, int64(200), *destParentID)
}

// ============================================================================
// MV COMMAND TESTS
// ============================================================================

func TestMv_MultipleFiles(t *testing.T) {
	s, env, _ := setupTestEnv(t)

	// Setup source files
	srcID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: srcID, Name: "src", Type: "folder"}, "/src")
	s.Cache.Add(&api.FileEntry{ID: 101, Name: "file1.txt", Type: "text", ParentID: &srcID}, "/src/file1.txt")
	s.Cache.Add(&api.FileEntry{ID: 102, Name: "file2.txt", Type: "text", ParentID: &srcID}, "/src/file2.txt")

	// Setup destination folder
	destID := int64(200)
	s.Cache.Add(&api.FileEntry{ID: destID, Name: "dest", Type: "folder"}, "/dest")

	// Track moved IDs
	var movedIDs []int64
	var destParentID *int64
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.MoveEntriesFunc = func(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) error {
		movedIDs = append(movedIDs, entryIDs...)
		destParentID = destinationParentID
		return nil
	}

	s.CWD = "/"

	cmd, ok := commands.Get("mv")
	require.True(t, ok)

	// mv /src/file1.txt /src/file2.txt /dest
	err := cmd.Run(context.Background(), s, env, []string{"/src/file1.txt", "/src/file2.txt", "/dest"})
	require.NoError(t, err)

	// Both files should be moved
	assert.Len(t, movedIDs, 2)
	assert.Contains(t, movedIDs, int64(101))
	assert.Contains(t, movedIDs, int64(102))
	require.NotNil(t, destParentID)
	assert.Equal(t, int64(200), *destParentID)
}

// ============================================================================
// INTEGRATION SCENARIOS - Real-world glob expansion use cases
// ============================================================================

func TestGlobExpansionScenario_LsTxtFiles(t *testing.T) {
	// This test simulates what happens when user types: ls *.txt
	// After ExpandGlobs, it becomes: ls file1.txt file2.txt notes.txt
	s, env, stdout := setupTestEnv(t)

	// Setup directory with various files
	docsID := int64(100)
	s.Cache.Add(&api.FileEntry{ID: docsID, Name: "docs", Type: "folder"}, "/docs")
	s.Cache.AddChildren("/docs", []api.FileEntry{
		{ID: 101, Name: "file1.txt", Type: "text", ParentID: &docsID},
		{ID: 102, Name: "file2.txt", Type: "text", ParentID: &docsID},
		{ID: 103, Name: "notes.txt", Type: "text", ParentID: &docsID},
		{ID: 104, Name: "image.png", Type: "image", ParentID: &docsID},
		{ID: 105, Name: "script.go", Type: "text", ParentID: &docsID},
	})

	// Also add individual file entries
	s.Cache.Add(&api.FileEntry{ID: 101, Name: "file1.txt", Type: "text", ParentID: &docsID}, "/docs/file1.txt")
	s.Cache.Add(&api.FileEntry{ID: 102, Name: "file2.txt", Type: "text", ParentID: &docsID}, "/docs/file2.txt")
	s.Cache.Add(&api.FileEntry{ID: 103, Name: "notes.txt", Type: "text", ParentID: &docsID}, "/docs/notes.txt")

	s.CWD = "/docs"

	cmd, ok := commands.Get("ls")
	require.True(t, ok)

	// Simulate expanded args (as if from *.txt glob)
	err := cmd.Run(context.Background(), s, env, []string{"file1.txt", "file2.txt", "notes.txt"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "file1.txt")
	assert.Contains(t, output, "file2.txt")
	assert.Contains(t, output, "notes.txt")
	// Should NOT contain non-txt files
	assert.NotContains(t, output, "image.png")
	assert.NotContains(t, output, "script.go")
}

func TestGlobExpansionScenario_BraceExpansion(t *testing.T) {
	// This test simulates: cp {app,cmd}/*.go /backup
	// After expansion: cp app/main.go cmd/run.go /backup
	s, env, _ := setupTestEnv(t)

	// Setup structure
	appID := int64(100)
	cmdID := int64(200)
	backupID := int64(300)

	s.Cache.Add(&api.FileEntry{ID: appID, Name: "app", Type: "folder"}, "/app")
	s.Cache.Add(&api.FileEntry{ID: 101, Name: "main.go", Type: "text", ParentID: &appID}, "/app/main.go")

	s.Cache.Add(&api.FileEntry{ID: cmdID, Name: "cmd", Type: "folder"}, "/cmd")
	s.Cache.Add(&api.FileEntry{ID: 201, Name: "run.go", Type: "text", ParentID: &cmdID}, "/cmd/run.go")

	s.Cache.Add(&api.FileEntry{ID: backupID, Name: "backup", Type: "folder"}, "/backup")

	var copiedIDs []int64
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.CopyEntriesFunc = func(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) ([]api.FileEntry, error) {
		copiedIDs = append(copiedIDs, entryIDs...)
		return []api.FileEntry{}, nil
	}

	s.CWD = "/"

	cmd, ok := commands.Get("cp")
	require.True(t, ok)

	// Simulated post-brace-expansion args
	err := cmd.Run(context.Background(), s, env, []string{"/app/main.go", "/cmd/run.go", "/backup"})
	require.NoError(t, err)

	assert.Contains(t, copiedIDs, int64(101))
	assert.Contains(t, copiedIDs, int64(201))
}
