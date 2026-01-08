package commands

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
)

func init() {
	Register(&Command{
		Name:        "du",
		Description: "Show usage statistics",
		Usage:       "du\\n\\nDisplays disk usage: used space, available space, and percentage.",
		Run:         du,
	})
	Register(&Command{
		Name:        "unzip",
		Description: "Extract archive",
		Usage:       "unzip <file>\\n\\nExtracts a ZIP archive on the server (server-side extraction).\\nExtracted files appear in the same directory as the archive.",
		Run:         unzip,
	})
	Register(&Command{
		Name:        "zip",
		Description: "Create a zip archive",
		Usage:       "zip <archive.zip> <file|folder>...\\n\\nCreates a ZIP archive from remote files/folders.\\nThe archive is uploaded to Drime Cloud.\\n\\nExamples:\\n  zip backup.zip file1.txt file2.txt\\n  zip photos.zip /Photos/vacation/\\n  zip all.zip /                      Zip entire storage",
		Run:         zipCmd,
	})
}

func du(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	usage, err := s.Client.GetSpaceUsage(ctx, s.WorkspaceID)
	if err != nil {
		return err
	}

	fmt.Fprintf(env.Stdout, "Used:      %s\n", formatBytes(usage.Used))
	fmt.Fprintf(env.Stdout, "Available: %s\n", formatBytes(usage.Available))

	percent := 0.0
	if usage.Available+usage.Used > 0 {
		percent = float64(usage.Used) / float64(usage.Available+usage.Used) * 100
	}
	fmt.Fprintf(env.Stdout, "Usage:     %.1f%%\n", percent)
	return nil
}

func unzip(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: unzip <file>")
	}

	path := args[0]
	resolved, err := s.ResolvePathArg(path)
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}
	entry, ok := s.Cache.Get(resolved)
	if !ok {
		return fmt.Errorf("unzip: %s: No such file", path)
	}

	if entry.Type == "folder" {
		return fmt.Errorf("unzip: %s: Is a directory", path)
	}

	// Get parent directory info for the refresh
	parentDir := filepath.Dir(resolved)
	parentEntry, _ := s.Cache.Get(parentDir)

	err = ui.WithSpinnerErr(env.Stderr, "", false, func() error {
		// Extract the archive
		if err := s.Client.ExtractEntry(ctx, entry.ID, entry.ParentID, s.WorkspaceID); err != nil {
			return err
		}

		// Refresh the parent directory contents (fetch new files into cache)
		var parentID *int64
		if parentDir == "/" {
			parentID = nil
		} else if parentEntry != nil {
			parentID = &parentEntry.ID
		} else {
			// Parent not found in cache (and not root), can't refresh safely
			return nil
		}

		apiOpts := api.ListOptions(s.WorkspaceID)
		children, err := s.Client.ListByParentIDWithOptions(ctx, parentID, apiOpts)
		if err != nil {
			return err
		}

		// Update cache with fresh data
		s.Cache.InvalidateChildren(parentDir)
		for i := range children {
			childPath := filepath.Join(parentDir, children[i].Name)
			s.Cache.Add(&children[i], childPath)
		}
		s.Cache.MarkChildrenLoaded(parentDir)

		return nil
	})

	return err
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// zipCmd creates a zip archive from remote files/folders.
// Usage: zip archive.zip file1 file2 folder/
func zipCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: zip <archive.zip> <file|folder>")
	}

	archiveName := args[0]
	sources := args[1:]

	// Ensure archive name ends with .zip
	if !strings.HasSuffix(strings.ToLower(archiveName), ".zip") {
		archiveName += ".zip"
	}

	// Resolve destination path
	destResolved, err := s.ResolvePathArg(archiveName)
	if err != nil {
		return fmt.Errorf("zip: %w", err)
	}
	destDir := filepath.Dir(destResolved)

	// Check destination directory exists
	var destDirEntry *api.FileEntry
	if destDir == "/" {
		// Root always exists
	} else {
		var ok bool
		destDirEntry, ok = s.Cache.Get(destDir)
		if !ok || destDirEntry.Type != "folder" {
			return fmt.Errorf("zip: destination directory does not exist: %s", destDir)
		}
	}

	// Collect all source entries and estimate total size
	type sourceInfo struct {
		entry    *api.FileEntry
		resolved string
		baseName string
	}
	var sourcesToZip []sourceInfo
	var estimatedSize int64

	for _, src := range sources {
		resolved, err := s.ResolvePathArg(src)
		if err != nil {
			return fmt.Errorf("zip: %w", err)
		}
		entry, ok := s.Cache.Get(resolved)
		if !ok {
			return fmt.Errorf("zip: %s: No such file or directory", src)
		}

		// Determine a good base name for the entry in the zip
		baseName := filepath.Base(resolved)
		if resolved == "/" || baseName == "/" || baseName == "" {
			// Root folder - use "root" or skip the prefix entirely
			baseName = ""
		}

		sourcesToZip = append(sourcesToZip, sourceInfo{
			entry:    entry,
			resolved: resolved,
			baseName: baseName,
		})
		estimatedSize += entry.Size
	}

	// Determine memory threshold
	maxMemory := s.MaxMemoryBytes()

	// Use temp file if estimated size exceeds threshold
	useTempFile := estimatedSize > maxMemory
	var zipWriter *zip.Writer
	var tempFile *os.File
	var memBuf *bytes.Buffer

	if useTempFile {
		var err error
		tempFile, err = os.CreateTemp("", "drime-zip-*.zip")
		if err != nil {
			return fmt.Errorf("zip: failed to create temp file: %w", err)
		}
		defer func() {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}()
		zipWriter = zip.NewWriter(tempFile)
		fmt.Fprintf(env.Stdout, "Using temp file for large archive...\n")
	} else {
		memBuf = new(bytes.Buffer)
		zipWriter = zip.NewWriter(memBuf)
	}

	for _, src := range sourcesToZip {
		if src.entry.Type == "folder" {
			if err := addFolderToZip(ctx, s, zipWriter, src.entry, src.baseName, env, maxMemory); err != nil {
				zipWriter.Close()
				return fmt.Errorf("zip: error adding folder %s: %w", src.baseName, err)
			}
		} else {
			if err := addFileToZip(ctx, s, zipWriter, src.entry, src.baseName, env, maxMemory); err != nil {
				zipWriter.Close()
				return fmt.Errorf("zip: error adding file %s: %w", src.baseName, err)
			}
		}
	}

	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("zip: error finalizing archive: %w", err)
	}

	// Prepare upload source
	var uploadReader io.Reader
	var uploadSize int64

	if useTempFile {
		// Seek back to beginning and get size
		uploadSize, _ = tempFile.Seek(0, io.SeekEnd)
		_, _ = tempFile.Seek(0, io.SeekStart)
		uploadReader = tempFile
	} else {
		uploadSize = int64(memBuf.Len())
		uploadReader = memBuf
	}

	fmt.Fprintf(env.Stdout, "Uploading %s (%s)...\n", filepath.Base(destResolved), formatBytes(uploadSize))

	var parentID *int64
	if destDirEntry != nil && destDirEntry.ID != 0 {
		parentID = &destDirEntry.ID
	}

	var uploadedEntry *api.FileEntry
	err = ui.WithSpinnerErr(env.Stderr, "", false, func() error {
		var err error
		uploadedEntry, err = s.Client.Upload(ctx, uploadReader, filepath.Base(destResolved), parentID, uploadSize, s.WorkspaceID)
		return err
	})
	if err != nil {
		return err
	}

	if uploadedEntry != nil {
		s.Cache.Add(uploadedEntry, destResolved)
	}

	fmt.Fprintf(env.Stdout, "Created %s\n", archiveName)
	return nil
}

// addFileToZip downloads a single file and adds it to the zip archive
func addFileToZip(ctx context.Context, s *session.Session, zw *zip.Writer, entry *api.FileEntry, nameInZip string, env *ExecutionEnv, maxMemory int64) error {
	fmt.Fprintf(env.Stdout, "  adding: %s\n", nameInZip)

	// For large files, use temp file
	if entry.Size > maxMemory {
		return addLargeFileToZip(ctx, s, zw, entry, nameInZip, env)
	}

	// Download file content to memory
	content := new(bytes.Buffer)
	_, err := ui.WithSpinner(io.Discard, "", false, func() (*api.FileEntry, error) {
		return s.Client.Download(ctx, entry.Hash, content, nil)
	})
	if err != nil {
		return err
	}

	// Create zip entry with proper header
	header := &zip.FileHeader{
		Name:     nameInZip,
		Method:   zip.Deflate,
		Modified: entry.UpdatedAt,
	}

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, content)
	return err
}

// addLargeFileToZip handles files that exceed memory threshold using temp files
func addLargeFileToZip(ctx context.Context, s *session.Session, zw *zip.Writer, entry *api.FileEntry, nameInZip string, _ *ExecutionEnv) error {
	// Create temp file for download
	tempFile, err := os.CreateTemp("", "drime-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		tempFile.Close()
		os.Remove(tempFile.Name())
	}()

	// Download to temp file
	_, err = ui.WithSpinner(io.Discard, "", false, func() (*api.FileEntry, error) {
		return s.Client.Download(ctx, entry.Hash, tempFile, nil)
	})
	if err != nil {
		return err
	}

	// Seek back to start
	_, _ = tempFile.Seek(0, io.SeekStart)

	// Create zip entry
	header := &zip.FileHeader{
		Name:     nameInZip,
		Method:   zip.Deflate,
		Modified: entry.UpdatedAt,
	}

	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, tempFile)
	return err
}

// addFolderToZip downloads a folder (which comes as ZIP from API) and re-adds its contents
func addFolderToZip(ctx context.Context, s *session.Session, zw *zip.Writer, entry *api.FileEntry, folderName string, env *ExecutionEnv, maxMemory int64) error {
	if folderName != "" {
		fmt.Fprintf(env.Stdout, "  adding: %s/\n", folderName)
	}

	// Root folder (ID=0) can't be downloaded - we need to zip its children individually
	if entry.ID == 0 {
		return addRootFolderToZip(ctx, s, zw, folderName, env, maxMemory)
	}

	// For large folders, use temp file
	var zipReader *zip.Reader

	if entry.Size > maxMemory {
		// Download to temp file
		tempFile, err := os.CreateTemp("", "drime-folder-*.zip")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer func() {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}()

		_, err = ui.WithSpinner(io.Discard, "", false, func() (*api.FileEntry, error) {
			return s.Client.Download(ctx, entry.Hash, tempFile, nil)
		})
		if err != nil {
			return err
		}

		// Get file size and create reader
		size, _ := tempFile.Seek(0, io.SeekEnd)
		_, _ = tempFile.Seek(0, io.SeekStart)

		zipReader, err = zip.NewReader(tempFile, size)
		if err != nil {
			return fmt.Errorf("failed to read folder zip: %w", err)
		}
	} else {
		// Download to memory
		content := new(bytes.Buffer)
		_, err := ui.WithSpinner(io.Discard, "", false, func() (*api.FileEntry, error) {
			return s.Client.Download(ctx, entry.Hash, content, nil)
		})
		if err != nil {
			return err
		}

		zipReader, err = zip.NewReader(bytes.NewReader(content.Bytes()), int64(content.Len()))
		if err != nil {
			return fmt.Errorf("failed to read folder zip: %w", err)
		}
	}

	// Copy all entries from the downloaded zip, prefixing with folder name
	for _, f := range zipReader.File {
		// Check for ZipSlip vulnerability - reject paths with traversal
		if strings.Contains(f.Name, "..") {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		newName := filepath.Join(folderName, f.Name)
		fmt.Fprintf(env.Stdout, "  adding: %s\n", newName)

		header := f.FileHeader
		header.Name = newName

		w, err := zw.CreateHeader(&header)
		if err != nil {
			return err
		}

		if !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			_, err = io.Copy(w, rc)
			rc.Close()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// addRootFolderToZip handles the special case of zipping the root folder (ID=0)
// which can't be downloaded directly - we zip its children instead
func addRootFolderToZip(ctx context.Context, s *session.Session, zw *zip.Writer, prefix string, env *ExecutionEnv, maxMemory int64) error {
	// List children of root
	apiOpts := api.ListOptions(s.WorkspaceID)
	children, err := s.Client.ListByParentIDWithOptions(ctx, nil, apiOpts)
	if err != nil {
		return fmt.Errorf("failed to list root folder: %w", err)
	}

	for _, child := range children {
		childName := child.Name
		if prefix != "" {
			childName = filepath.Join(prefix, child.Name)
		}

		if child.Type == "folder" {
			if err := addFolderToZip(ctx, s, zw, &child, childName, env, maxMemory); err != nil {
				return err
			}
		} else {
			if err := addFileToZip(ctx, s, zw, &child, childName, env, maxMemory); err != nil {
				return err
			}
		}
	}

	return nil
}
