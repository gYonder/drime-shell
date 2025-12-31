package commands_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/commands"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// WORKSPACE COMMAND TESTS
// ============================================================================

// setupWorkspaceTestEnv creates a test session configured for workspace tests
func setupWorkspaceTestEnv(t *testing.T) (*session.Session, *commands.ExecutionEnv, *bytes.Buffer, *bytes.Buffer) {
	cache := api.NewFileCache()
	mockClient := &api.MockDrimeClient{
		ListByParentIDFunc: func(ctx context.Context, parentID *int64) ([]api.FileEntry, error) {
			return []api.FileEntry{}, nil
		},
		GetUserFoldersFunc: func(ctx context.Context, userID int64, workspaceID int64) ([]api.FileEntry, error) {
			return []api.FileEntry{}, nil
		},
		GetWorkspacesFunc: func(ctx context.Context) ([]api.Workspace, error) {
			return []api.Workspace{
				{ID: 1, Name: "Team Project", OwnerID: 100},
				{ID: 2, Name: "Personal", OwnerID: 123},
			}, nil
		},
	}

	s := session.NewSession(mockClient, cache)
	s.CWD = "/"
	s.HomeDir = "/"
	s.UserID = 123
	s.Username = "testuser"
	s.WorkspaceID = 1
	s.WorkspaceName = "Team Project"
	s.Workspaces = []api.Workspace{
		{ID: 1, Name: "Team Project", OwnerID: 100},
		{ID: 2, Name: "Personal", OwnerID: 123},
	}

	cache.Add(&api.FileEntry{ID: 0, Name: "/", Type: "folder"}, "/")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	env := &commands.ExecutionEnv{
		Stdin:  strings.NewReader(""),
		Stdout: &stdout,
		Stderr: &stderr,
	}

	return s, env, &stdout, &stderr
}

// ============================================================================
// WS RM (Delete) Command Tests - With Confirmation
// ============================================================================

func TestWsRm_RequiresTypingWorkspaceName(t *testing.T) {
	s, env, stdout, _ := setupWorkspaceTestEnv(t)

	deleteCalled := false
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.DeleteWorkspaceFunc = func(ctx context.Context, workspaceID int64) error {
		deleteCalled = true
		return nil
	}

	// User types the workspace name correctly
	env.Stdin = strings.NewReader("Team Project\n")

	cmd, ok := commands.Get("ws")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"rm"})
	require.NoError(t, err)

	assert.True(t, deleteCalled)
	assert.Contains(t, stdout.String(), "Deleted")
}

func TestWsRm_CancelledOnWrongName(t *testing.T) {
	s, env, stdout, _ := setupWorkspaceTestEnv(t)

	deleteCalled := false
	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.DeleteWorkspaceFunc = func(ctx context.Context, workspaceID int64) error {
		deleteCalled = true
		return nil
	}

	// User types wrong name
	env.Stdin = strings.NewReader("Wrong Name\n")

	cmd, ok := commands.Get("ws")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"rm"})
	require.NoError(t, err)

	assert.False(t, deleteCalled, "Delete should not be called when name doesn't match")
	assert.Contains(t, stdout.String(), "cancelled")
}

func TestWsRm_CannotDeleteDefaultWorkspace(t *testing.T) {
	s, env, _, _ := setupWorkspaceTestEnv(t)

	s.WorkspaceID = 0

	cmd, ok := commands.Get("ws")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"rm"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default workspace")
}

// ============================================================================
// WS SWITCH Command Tests - With Stats Display
// ============================================================================

func TestWsSwitch_ShowsStats(t *testing.T) {
	s, env, stdout, _ := setupWorkspaceTestEnv(t)

	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.GetWorkspaceStatsFunc = func(ctx context.Context, workspaceID int64) (*api.WorkspaceStats, error) {
		return &api.WorkspaceStats{
			Files: 42,
			Size:  1073741824, // 1 GB
		}, nil
	}
	mockClient.ListByParentIDWithOptionsFunc = func(ctx context.Context, parentID *int64, opts *api.ListEntriesOptions) ([]api.FileEntry, error) {
		return []api.FileEntry{}, nil
	}

	// Start on default workspace
	s.WorkspaceID = 0
	s.WorkspaceName = ""

	cmd, ok := commands.Get("ws")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"Team Project"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Switched to workspace")
	assert.Contains(t, output, "Team Project")
	assert.Contains(t, output, "42 files")
	assert.Contains(t, output, "GB") // Should show size in GB
}

func TestWsSwitch_ByID(t *testing.T) {
	s, env, stdout, _ := setupWorkspaceTestEnv(t)

	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.GetWorkspaceStatsFunc = func(ctx context.Context, workspaceID int64) (*api.WorkspaceStats, error) {
		return &api.WorkspaceStats{Files: 10, Size: 1024}, nil
	}
	mockClient.ListByParentIDWithOptionsFunc = func(ctx context.Context, parentID *int64, opts *api.ListEntriesOptions) ([]api.FileEntry, error) {
		return []api.FileEntry{}, nil
	}

	s.WorkspaceID = 0

	cmd, ok := commands.Get("ws")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"1"})
	require.NoError(t, err)

	assert.Equal(t, int64(1), s.WorkspaceID)
	assert.Contains(t, stdout.String(), "Team Project")
}

func TestWsSwitch_ToDefault(t *testing.T) {
	s, env, stdout, _ := setupWorkspaceTestEnv(t)

	mockClient := s.Client.(*api.MockDrimeClient)
	mockClient.GetWorkspaceStatsFunc = func(ctx context.Context, workspaceID int64) (*api.WorkspaceStats, error) {
		return &api.WorkspaceStats{Files: 5, Size: 512}, nil
	}
	mockClient.ListByParentIDWithOptionsFunc = func(ctx context.Context, parentID *int64, opts *api.ListEntriesOptions) ([]api.FileEntry, error) {
		return []api.FileEntry{}, nil
	}

	cmd, ok := commands.Get("ws")
	require.True(t, ok)

	err := cmd.Run(context.Background(), s, env, []string{"default"})
	require.NoError(t, err)

	assert.Equal(t, int64(0), s.WorkspaceID)
	assert.Contains(t, stdout.String(), "default workspace")
}
