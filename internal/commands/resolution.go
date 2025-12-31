package commands

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/crypto"
	"github.com/mikael.mansson2/drime-shell/internal/session"
)

// ResolveEntry resolves a file entry from a user argument by path.
func ResolveEntry(ctx context.Context, s *session.Session, arg string) (*api.FileEntry, error) {
	path, err := s.ResolvePathArg(arg)
	if err != nil {
		return nil, err
	}

	entry, ok := s.Cache.Get(path)
	if !ok {
		return nil, fmt.Errorf("%s: No such file or directory", arg)
	}
	return entry, nil
}

// DownloadAndDecrypt downloads a file, handling vault decryption automatically.
// Returns the plaintext content as bytes.
func DownloadAndDecrypt(ctx context.Context, s *session.Session, entry *api.FileEntry) ([]byte, error) {
	var buf bytes.Buffer

	if s.InVault {
		// Vault: download encrypted and decrypt
		if !s.VaultUnlocked {
			return nil, fmt.Errorf("vault is locked, run 'vault unlock' first")
		}
		if entry.IV == "" {
			return nil, fmt.Errorf("file has no IV (not encrypted?)")
		}
		iv, err := crypto.DecodeBase64(entry.IV)
		if err != nil {
			return nil, fmt.Errorf("invalid IV: %w", err)
		}

		if _, err := s.Client.DownloadEncrypted(ctx, entry.Hash, &buf, nil); err != nil {
			return nil, fmt.Errorf("download failed: %w", err)
		}

		// Decrypt
		plaintext, err := s.VaultKey.Decrypt(buf.Bytes(), iv)
		if err != nil {
			return nil, fmt.Errorf("decryption failed: %w", err)
		}
		return plaintext, nil
	}

	// Regular download
	if _, err := s.Client.Download(ctx, entry.Hash, &buf, nil); err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	return buf.Bytes(), nil
}

// DownloadAndDecryptToWriter downloads a file to a writer, handling vault decryption.
// For vault files, decrypts in memory then writes to w.
// For regular files, streams directly to w.
func DownloadAndDecryptToWriter(ctx context.Context, s *session.Session, entry *api.FileEntry, w io.Writer, progress func(int64, int64)) error {
	if s.InVault {
		// Vault: must buffer for decryption
		content, err := DownloadAndDecrypt(ctx, s, entry)
		if err != nil {
			return err
		}
		_, err = w.Write(content)
		return err
	}

	// Regular: stream directly
	_, err := s.Client.Download(ctx, entry.Hash, w, progress)
	return err
}
