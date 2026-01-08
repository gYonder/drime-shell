package commands_test

import (
	"context"
	"testing"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/commands"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDuplicatePolicy_Constants(t *testing.T) {
	// Verify policy constants are defined correctly
	assert.Equal(t, commands.DuplicatePolicy("ask"), commands.DuplicatePolicyAsk)
	assert.Equal(t, commands.DuplicatePolicy("replace"), commands.DuplicatePolicyReplace)
	assert.Equal(t, commands.DuplicatePolicy("rename"), commands.DuplicatePolicyRename)
	assert.Equal(t, commands.DuplicatePolicy("skip"), commands.DuplicatePolicySkip)
}

func TestCheckCollisionsAndResolveWithPolicy_Skip(t *testing.T) {
	mockClient := &api.MockDrimeClient{
		ValidateEntriesFunc: func(ctx context.Context, req api.ValidateRequest) (*api.ValidateResponse, error) {
			return &api.ValidateResponse{
				Duplicates: []string{"/dest/file1.txt"},
			}, nil
		},
	}

	result, err := commands.CheckCollisionsAndResolveWithPolicyForTest(
		context.Background(),
		mockClient,
		0,
		nil,
		"/dest",
		[]string{"file1.txt", "file2.txt"},
		"skip",
	)

	require.NoError(t, err)
	// file1.txt is a duplicate and should be skipped (removed from result)
	_, hasFile1 := result["file1.txt"]
	assert.False(t, hasFile1, "Duplicate file1.txt should be skipped")
	// file2.txt is not a duplicate and should be included
	assert.Equal(t, "file2.txt", result["file2.txt"])
}

func TestCheckCollisionsAndResolveWithPolicy_Replace(t *testing.T) {
	mockClient := &api.MockDrimeClient{
		ValidateEntriesFunc: func(ctx context.Context, req api.ValidateRequest) (*api.ValidateResponse, error) {
			return &api.ValidateResponse{
				Duplicates: []string{"/dest/file1.txt"},
			}, nil
		},
	}

	result, err := commands.CheckCollisionsAndResolveWithPolicyForTest(
		context.Background(),
		mockClient,
		0,
		nil,
		"/dest",
		[]string{"file1.txt"},
		"replace",
	)

	require.NoError(t, err)
	// Replace keeps the same name
	assert.Equal(t, "file1.txt", result["file1.txt"])
}

func TestCheckCollisionsAndResolveWithPolicy_Rename(t *testing.T) {
	mockClient := &api.MockDrimeClient{
		ValidateEntriesFunc: func(ctx context.Context, req api.ValidateRequest) (*api.ValidateResponse, error) {
			return &api.ValidateResponse{
				Duplicates: []string{"/dest/file1.txt"},
			}, nil
		},
		GetAvailableNameFunc: func(ctx context.Context, req api.GetAvailableNameRequest) (*api.GetAvailableNameResponse, error) {
			return &api.GetAvailableNameResponse{
				Name: "file1 (1).txt",
			}, nil
		},
	}

	result, err := commands.CheckCollisionsAndResolveWithPolicyForTest(
		context.Background(),
		mockClient,
		0,
		nil,
		"/dest",
		[]string{"file1.txt"},
		"rename",
	)

	require.NoError(t, err)
	// Rename gets a new name from API
	assert.Equal(t, "file1 (1).txt", result["file1.txt"])
}

func TestCheckCollisionsAndResolveWithPolicy_NoDuplicates(t *testing.T) {
	mockClient := &api.MockDrimeClient{
		ValidateEntriesFunc: func(ctx context.Context, req api.ValidateRequest) (*api.ValidateResponse, error) {
			return &api.ValidateResponse{
				Duplicates: []string{}, // No duplicates
			}, nil
		},
	}

	result, err := commands.CheckCollisionsAndResolveWithPolicyForTest(
		context.Background(),
		mockClient,
		0,
		nil,
		"/dest",
		[]string{"file1.txt", "file2.txt"},
		"skip", // Policy doesn't matter when no duplicates
	)

	require.NoError(t, err)
	assert.Equal(t, "file1.txt", result["file1.txt"])
	assert.Equal(t, "file2.txt", result["file2.txt"])
}
