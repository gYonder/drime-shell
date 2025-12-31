package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

// RemoteFileReader downloads a remote file and provides it as an io.Reader.
// This enables input redirection ("<") to work with remote files transparently.
// For files larger than MaxMemoryBufferMB, it uses a temp file instead of memory.
type RemoteFileReader struct {
	reader   io.Reader
	tempFile *os.File // non-nil if using temp file
	closed   bool
}

// NewRemoteFileReader downloads the remote file and returns a reader for its contents.
func NewRemoteFileReader(ctx context.Context, s *session.Session, remotePath string) (*RemoteFileReader, error) {
	// Resolve the path
	resolved, err := s.ResolvePathArg(remotePath)
	if err != nil {
		return nil, err
	}

	entry, ok := s.Cache.Get(resolved)
	if !ok {
		return nil, fmt.Errorf("no such file: %s", remotePath)
	}

	if entry.Type == "folder" {
		return nil, fmt.Errorf("is a directory: %s", remotePath)
	}

	maxMemory := s.MaxMemoryBytes()

	// For large files, use temp file
	if entry.Size > maxMemory {
		return newRemoteFileReaderWithTempFile(ctx, s, entry.Hash)
	}

	// For small files, download into memory
	buf := new(bytes.Buffer)
	err = ui.WithSpinnerErr(os.Stderr, "", func() error {
		_, err := s.Client.Download(ctx, entry.Hash, buf, nil)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	return &RemoteFileReader{
		reader: buf,
	}, nil
}

// newRemoteFileReaderWithTempFile downloads to a temp file for large files
func newRemoteFileReaderWithTempFile(ctx context.Context, s *session.Session, hash string) (*RemoteFileReader, error) {
	tempFile, err := os.CreateTemp("", "drime-input-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	err = ui.WithSpinnerErr(os.Stderr, "", func() error {
		_, err := s.Client.Download(ctx, hash, tempFile, nil)
		return err
	})
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	// Seek back to start for reading
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to seek temp file: %w", err)
	}

	return &RemoteFileReader{
		reader:   tempFile,
		tempFile: tempFile,
	}, nil
}

// Read implements io.Reader.
func (r *RemoteFileReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

// Close implements io.Closer. Cleans up temp file if one was used.
func (r *RemoteFileReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	if r.tempFile != nil {
		name := r.tempFile.Name()
		r.tempFile.Close()
		os.Remove(name)
	}
	return nil
}
