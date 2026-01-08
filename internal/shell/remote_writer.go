package shell

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/commands"
	"github.com/gYonder/drime-shell/internal/crypto"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
)

// RemoteFileWriter buffers output to a temporary file and uploads it to Drime Cloud on Close.
// This enables output redirection (">") to work with remote files transparently.
type RemoteFileWriter struct {
	ctx        context.Context
	sess       *session.Session
	tempFile   *os.File
	remotePath string
	closed     bool
	append     bool
}

// NewRemoteFileWriter creates a writer that will upload to the given remote path on Close.
func NewRemoteFileWriter(ctx context.Context, s *session.Session, remotePath string) (*RemoteFileWriter, error) {
	return NewRemoteFileWriterWithMode(ctx, s, remotePath, false)
}

// NewRemoteFileWriterWithMode creates a writer with optional append mode.
// If append is true, the existing file content is downloaded first (>>) behavior.
func NewRemoteFileWriterWithMode(ctx context.Context, s *session.Session, remotePath string, append bool) (*RemoteFileWriter, error) {
	// Vault requires encryption key to be loaded
	if s.InVault {
		if !s.VaultUnlocked || s.VaultKey == nil {
			return nil, fmt.Errorf("cannot redirect output: vault is locked, run 'vault unlock' first")
		}
	}
	f, err := os.CreateTemp("", "drime-redir-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// If append mode and file exists, download existing content first
	if append {
		destResolved, err := s.ResolvePathArg(remotePath)
		if err != nil {
			return nil, err
		}
		if entry, ok := s.Cache.Get(destResolved); ok && entry.Type != "folder" {
			// Download existing content to the temp file
			_, err := s.Client.Download(ctx, entry.Hash, f, nil)
			if err != nil {
				// Non-fatal: if download fails, we just start fresh
				// This handles the case where the file doesn't exist yet
				_ = f.Truncate(0)
				_, _ = f.Seek(0, 0)
			}
			// Position at end for appending
			_, _ = f.Seek(0, io.SeekEnd)
		}
	}

	return &RemoteFileWriter{
		sess:       s,
		remotePath: remotePath,
		tempFile:   f,
		ctx:        ctx,
		append:     append,
	}, nil
}

// Write buffers data to the temporary file.
func (w *RemoteFileWriter) Write(p []byte) (n int, err error) {
	return w.tempFile.Write(p)
}

// Close flushes the buffer and uploads the file to Drime Cloud.
func (w *RemoteFileWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	tempName := w.tempFile.Name()
	w.tempFile.Close()
	defer os.Remove(tempName)

	// Re-open for reading
	f, err := os.Open(tempName)
	if err != nil {
		return fmt.Errorf("failed to read temp file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	// Resolve the destination path
	destResolved, err := w.sess.ResolvePathArg(w.remotePath)
	if err != nil {
		return err
	}
	destName := filepath.Base(destResolved)

	// Check if destination is an existing folder (error case)
	if entry, ok := w.sess.Cache.Get(destResolved); ok && entry.Type == "folder" {
		return fmt.Errorf("cannot redirect to directory '%s'", w.remotePath)
	}

	// Resolve parent folder
	parentDir := filepath.Dir(destResolved)
	var parentID *int64

	if parentEntry, ok := w.sess.Cache.Get(parentDir); ok && parentEntry.Type == "folder" {
		// Use nil for root folder (ID=0 is synthetic)
		if parentEntry.ID != 0 {
			parentID = &parentEntry.ID
		}
	} else if parentDir != "/" && parentDir != "." {
		return fmt.Errorf("directory '%s' does not exist", parentDir)
	}

	// Check for conflict if not appending (skip in vault - no duplicate handling)
	if !w.append && !w.sess.InVault {
		if entry, ok := w.sess.Cache.Get(destResolved); ok && entry.Type != "folder" {
			// File exists, prompt for resolution
			newName, proceed, err := commands.ResolveConflict(w.ctx, w.sess.Client, w.sess.WorkspaceID, parentID, destName)
			if err != nil {
				return err
			}
			if !proceed {
				return nil // User skipped
			}
			destName = newName
		}
	}

	// Upload with spinner for slow operations
	return ui.WithSpinnerErr(os.Stderr, "", false, func() error {
		var newEntry *api.FileEntry
		var uploadErr error

		if w.sess.InVault {
			// Vault: read all content, encrypt, and upload
			content, err := io.ReadAll(f)
			if err != nil {
				return fmt.Errorf("failed to read content: %w", err)
			}

			encryptedContent, iv, err := w.sess.VaultKey.Encrypt(content)
			if err != nil {
				return fmt.Errorf("encryption failed: %w", err)
			}
			ivBase64 := crypto.EncodeBase64(iv)

			newEntry, uploadErr = w.sess.Client.UploadToVault(w.ctx, encryptedContent, destName, parentID, w.sess.VaultID, ivBase64)
		} else {
			// Regular workspace upload
			newEntry, uploadErr = w.sess.Client.Upload(w.ctx, f, destName, parentID, stat.Size(), w.sess.WorkspaceID)
		}

		if uploadErr != nil {
			return fmt.Errorf("failed to upload: %w", uploadErr)
		}
		// Add to cache so it shows up in ls immediately
		if newEntry != nil {
			// Reconstruct path in case name changed (Keep Both)
			finalPath := filepath.Join(parentDir, destName)
			if parentDir == "/" {
				finalPath = "/" + destName
			}
			w.sess.Cache.Add(newEntry, finalPath)
		}
		return nil
	})
}
