package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/crypto"
	"github.com/gYonder/drime-shell/internal/session"
)

// TestVaultCommandInit tests the vault init command
func TestVaultCommandInit(t *testing.T) {
	// Setup mock client
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	// Mock: No vault exists initially
	mockClient.GetVaultMetadataFunc = func(ctx context.Context) (*api.VaultMeta, error) {
		return nil, nil
	}

	// Mock: Initialize vault succeeds
	var capturedSalt, capturedCheck, capturedIV string
	mockClient.InitializeVaultFunc = func(ctx context.Context, salt, check, iv string) (*api.VaultMeta, error) {
		capturedSalt = salt
		capturedCheck = check
		capturedIV = iv
		return &api.VaultMeta{
			ID:    1,
			Salt:  salt,
			Check: check,
			IV:    iv,
		}, nil
	}

	// Create execution environment with password input
	stdin := strings.NewReader("testpassword123\ntestpassword123\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &ExecutionEnv{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	// Run vault init
	cmd, _ := Get("vault")
	err := cmd.Run(context.Background(), sess, env, []string{"init"})
	if err != nil {
		t.Fatalf("vault init failed: %v", err)
	}

	// Verify salt, check, and IV were captured
	if capturedSalt == "" {
		t.Error("expected salt to be captured")
	}
	if capturedCheck == "" {
		t.Error("expected check value to be captured")
	}
	if capturedIV == "" {
		t.Error("expected IV to be captured")
	}

	// Verify vault state
	if sess.VaultID != 1 {
		t.Errorf("expected VaultID to be 1, got %d", sess.VaultID)
	}
	if !sess.VaultUnlocked {
		t.Error("expected vault to be unlocked after init")
	}
	if sess.VaultKey == nil {
		t.Error("expected VaultKey to be set after init")
	}
}

// TestVaultCommandEnterExit tests entering vault and exiting back to previous workspace
func TestVaultCommandEnterExit(t *testing.T) {
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	password := "testpassword123"
	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey(password, salt)
	check, iv, _ := crypto.CreateCheckValue(key)
	key.Zero()

	saltB64 := crypto.EncodeBase64(salt)
	checkB64 := crypto.EncodeBase64(check)
	ivB64 := crypto.EncodeBase64(iv)

	mockClient.GetVaultMetadataFunc = func(ctx context.Context) (*api.VaultMeta, error) {
		return &api.VaultMeta{
			ID:    1,
			Salt:  saltB64,
			Check: checkB64,
			IV:    ivB64,
		}, nil
	}

	mockClient.ListVaultEntriesFunc = func(ctx context.Context, path string) ([]api.FileEntry, error) {
		return []api.FileEntry{}, nil
	}

	mockClient.GetVaultFoldersFunc = func(ctx context.Context, userID int64) ([]api.FileEntry, error) {
		return []api.FileEntry{}, nil
	}

	sess.WorkspaceID = 123
	sess.WorkspaceName = "TestWorkspace"
	sess.CWD = "/Documents"
	sess.UserID = 1
	sess.Username = "testuser"

	stdin := strings.NewReader(password + "\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &ExecutionEnv{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	cmd, _ := Get("vault")
	err := cmd.Run(context.Background(), sess, env, []string{})
	if err != nil {
		t.Fatalf("vault enter failed: %v", err)
	}

	if !sess.InVault {
		t.Error("expected to be in vault after entering")
	}
	if !sess.VaultUnlocked {
		t.Error("expected vault to be unlocked")
	}

	stdout.Reset()
	stderr.Reset()
	err = cmd.Run(context.Background(), sess, env, []string{"exit"})
	if err != nil {
		t.Fatalf("vault exit failed: %v", err)
	}

	if sess.InVault {
		t.Error("expected to not be in vault after exit")
	}
	if sess.WorkspaceID != 123 {
		t.Errorf("expected to return to workspace 123, got %d", sess.WorkspaceID)
	}
	if sess.CWD != "/Documents" {
		t.Errorf("expected to return to /Documents, got %s", sess.CWD)
	}
}

func TestVaultCommandWrongPassword(t *testing.T) {
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	correctPassword := "correctpassword"
	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey(correctPassword, salt)
	check, iv, _ := crypto.CreateCheckValue(key)
	key.Zero()

	saltB64 := crypto.EncodeBase64(salt)
	checkB64 := crypto.EncodeBase64(check)
	ivB64 := crypto.EncodeBase64(iv)

	mockClient.GetVaultMetadataFunc = func(ctx context.Context) (*api.VaultMeta, error) {
		return &api.VaultMeta{
			ID:    1,
			Salt:  saltB64,
			Check: checkB64,
			IV:    ivB64,
		}, nil
	}

	stdin := strings.NewReader("wrongpassword\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &ExecutionEnv{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	cmd, _ := Get("vault")
	err := cmd.Run(context.Background(), sess, env, []string{})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "incorrect password") {
		t.Errorf("expected 'incorrect password' error, got: %v", err)
	}

	if sess.VaultUnlocked {
		t.Error("vault should not be unlocked with wrong password")
	}
}

// TestVaultContextName tests the ContextName method
func TestVaultContextName(t *testing.T) {
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	// Default workspace (ID 0)
	if name := sess.ContextName(); name != "" {
		t.Errorf("expected empty context name for default workspace, got %q", name)
	}

	// Named workspace (non-zero ID)
	sess.WorkspaceID = 123
	sess.WorkspaceName = "MyWorkspace"
	if name := sess.ContextName(); name != "MyWorkspace" {
		t.Errorf("expected 'MyWorkspace', got %q", name)
	}

	// In vault - should just show "vault" (no lock state)
	sess.InVault = true
	if name := sess.ContextName(); name != "vault" {
		t.Errorf("expected 'vault', got %q", name)
	}
}

// TestVaultSessionSwitch tests switching to vault and back
func TestVaultSessionSwitch(t *testing.T) {
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	// Set up workspace state
	sess.WorkspaceID = 123
	sess.WorkspaceName = "TestWorkspace"
	sess.CWD = "/Documents"

	// Create vault cache
	vaultCache := api.NewFileCache()
	vaultCache.Add(&api.FileEntry{ID: 0, Name: "/", Type: "folder"}, "/")

	// Switch to vault
	sess.SwitchToVault(456, vaultCache)

	// Verify vault state
	if !sess.InVault {
		t.Error("expected InVault to be true")
	}
	if sess.VaultID != 456 {
		t.Errorf("expected VaultID 456, got %d", sess.VaultID)
	}
	if sess.CWD != "/" {
		t.Errorf("expected CWD '/', got %q", sess.CWD)
	}
	if sess.Cache != vaultCache {
		t.Error("expected cache to be vaultCache")
	}

	// Verify workspace state was saved
	if sess.SavedWorkspaceID != 123 {
		t.Errorf("expected SavedWorkspaceID 123, got %d", sess.SavedWorkspaceID)
	}
	if sess.SavedWorkspaceName != "TestWorkspace" {
		t.Errorf("expected SavedWorkspaceName 'TestWorkspace', got %q", sess.SavedWorkspaceName)
	}
	if sess.SavedCWD != "/Documents" {
		t.Errorf("expected SavedCWD '/Documents', got %q", sess.SavedCWD)
	}

	// Restore workspace state
	sess.RestoreWorkspaceState()

	// Verify restoration
	if sess.InVault {
		t.Error("expected InVault to be false after restore")
	}
	if sess.WorkspaceID != 123 {
		t.Errorf("expected WorkspaceID 123, got %d", sess.WorkspaceID)
	}
	if sess.WorkspaceName != "TestWorkspace" {
		t.Errorf("expected WorkspaceName 'TestWorkspace', got %q", sess.WorkspaceName)
	}
	if sess.CWD != "/Documents" {
		t.Errorf("expected CWD '/Documents', got %q", sess.CWD)
	}
}
