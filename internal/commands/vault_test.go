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

// TestVaultCommandUnlockLock tests the vault unlock and lock commands
func TestVaultCommandUnlockLock(t *testing.T) {
	// Setup mock client with existing vault
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	// Generate valid vault metadata
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

	// Test unlock
	stdin := strings.NewReader(password + "\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &ExecutionEnv{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	cmd, _ := Get("vault")
	err := cmd.Run(context.Background(), sess, env, []string{"unlock"})
	if err != nil {
		t.Fatalf("vault unlock failed: %v", err)
	}

	if !sess.VaultUnlocked {
		t.Error("expected vault to be unlocked")
	}
	if sess.VaultKey == nil {
		t.Error("expected VaultKey to be set")
	}

	// Test lock
	stdout.Reset()
	stderr.Reset()
	err = cmd.Run(context.Background(), sess, env, []string{"lock"})
	if err != nil {
		t.Fatalf("vault lock failed: %v", err)
	}

	if sess.VaultUnlocked {
		t.Error("expected vault to be locked after lock command")
	}
	if sess.VaultKey != nil {
		t.Error("expected VaultKey to be nil after lock")
	}
}

// TestVaultCommandWrongPassword tests unlocking with wrong password
func TestVaultCommandWrongPassword(t *testing.T) {
	// Setup mock client with existing vault
	mockClient := &api.MockDrimeClient{}
	cache := api.NewFileCache()
	sess := session.NewSession(mockClient, cache)

	// Generate valid vault metadata with one password
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

	// Try to unlock with wrong password
	stdin := strings.NewReader("wrongpassword\n")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	env := &ExecutionEnv{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}

	cmd, _ := Get("vault")
	err := cmd.Run(context.Background(), sess, env, []string{"unlock"})
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

	// In vault, locked
	sess.InVault = true
	sess.VaultUnlocked = false
	if name := sess.ContextName(); name != "vault:locked" {
		t.Errorf("expected 'vault:locked', got %q", name)
	}

	// In vault, unlocked
	sess.VaultUnlocked = true
	sess.VaultKey = &crypto.VaultKey{} // Non-nil key
	if name := sess.ContextName(); name != "vault:unlocked" {
		t.Errorf("expected 'vault:unlocked', got %q", name)
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
