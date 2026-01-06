package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gabriel-vasile/mimetype"
)

// detectMimeType detects MIME type from content using magic bytes.
// Falls back to extension-based detection, then application/octet-stream.
func detectMimeType(reader io.Reader, filename string) (mimeType string, buffered *bytes.Reader, err error) {
	// Read first 3072 bytes for magic number detection (mimetype lib default)
	header := make([]byte, 3072)
	n, err := io.ReadFull(reader, header)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", nil, err
	}
	header = header[:n]

	// Detect from content using magic bytes
	mtype := mimetype.Detect(header)
	mimeType = mtype.String()

	// If detection returned generic octet-stream, try extension as fallback
	if mimeType == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".txt":
			mimeType = "text/plain"
		case ".json":
			mimeType = "application/json"
		case ".yaml", ".yml":
			mimeType = "application/x-yaml"
		case ".md":
			mimeType = "text/markdown"
		case ".csv":
			mimeType = "text/csv"
		case ".html", ".htm":
			mimeType = "text/html"
		case ".css":
			mimeType = "text/css"
		case ".js":
			mimeType = "text/javascript"
		case ".ts":
			mimeType = "text/typescript"
		case ".go":
			mimeType = "text/x-go"
		case ".py":
			mimeType = "text/x-python"
		case ".rs":
			mimeType = "text/x-rust"
		case ".sh":
			mimeType = "text/x-shellscript"
		}
	}

	return mimeType, bytes.NewReader(header), nil
}

const (
	ChunkSize       = 60 * 1024 * 1024 // 60MB
	MultipartThresh = 65 * 1024 * 1024 // 65MB - use multipart above this
	BatchSize       = 8                // Sign URLs in batches
	S3MaxRetries    = 5                // Max retries for S3 operations
	S3RetryDelay    = time.Second      // Base delay for S3 retries
)

// SimplePresignRequest is the request body for /s3/simple/presign
type SimplePresignRequest struct {
	Filename     string `json:"filename"`
	Mime         string `json:"mime"`
	Size         int64  `json:"size"`
	Extension    string `json:"extension,omitempty"`
	ParentID     *int64 `json:"parentId,omitempty"`
	WorkspaceID  int64  `json:"workspaceId"`
	RelativePath string `json:"relativePath,omitempty"`
}

// SimplePresignResponse is the response from /s3/simple/presign
type SimplePresignResponse struct {
	URL string `json:"url"`
	ACL string `json:"acl"`
	Key string `json:"key"`
}

// AbortMultipartRequest is the request body for /s3/multipart/abort
type AbortMultipartRequest struct {
	Key      string `json:"key"`
	UploadID string `json:"uploadId"`
}

// Multipart Requests/Responses
type CreateMultipartRequest struct {
	ParentID     *int64 `json:"parentId,omitempty"` // API might not support this directly here?
	Filename     string `json:"filename"`
	Mime         string `json:"mime"`
	Extension    string `json:"extension"`
	RelativePath string `json:"relativePath,omitempty"` // Used if Auto-create
	Size         int64  `json:"size"`
	WorkspaceID  int64  `json:"workspaceId"`
}

type CreateMultipartResponse struct {
	UploadID string `json:"uploadId"`
	Key      string `json:"key"`
}

type BatchSignRequest struct {
	Key         string `json:"key"`
	UploadID    string `json:"uploadId"`
	PartNumbers []int  `json:"partNumbers"`
}

type BatchSignResponse struct {
	URLs []struct {
		URL        string `json:"url"`
		PartNumber int    `json:"partNumber"`
	} `json:"urls"`
}

type CompleteMultipartRequest struct {
	Key      string         `json:"key"`
	UploadID string         `json:"uploadId"`
	Parts    []UploadedPart `json:"parts"`
}

type UploadedPart struct {
	ETag       string `json:"ETag"`
	PartNumber int    `json:"PartNumber"`
}

type CreateS3EntryRequest struct {
	ParentID        *int64 `json:"parentId,omitempty"`
	Filename        string `json:"filename"`
	ClientMime      string `json:"clientMime"`
	ClientName      string `json:"clientName"`
	ClientExtension string `json:"clientExtension"`
	RelativePath    string `json:"relativePath,omitempty"`
	Size            int64  `json:"size"`
	WorkspaceID     int64  `json:"workspaceId"`
}

type CreateS3EntryResponse struct {
	Status    string    `json:"status"`
	FileEntry FileEntry `json:"fileEntry"`
}

func (c *HTTPClient) Upload(ctx context.Context, reader io.Reader, name string, parentID *int64, size int64, workspaceID int64) (*FileEntry, error) {
	// We can't easily stat io.Reader or use File-specific logic easily.
	// We MUST adapt.
	// Multipart S3 Upload requires random access for parallel uploads usually (ReadAt).
	// If input is io.Reader (pipe), we can't seek.
	// So we might be limited to Simple Upload if it supports streaming?
	// OR we read into temp file / memory.
	// User said "upload don't make sense" for pipe.
	// But `cat file | upload` implies streaming.
	// The new interface `Upload` takes `io.Reader`.

	// If size is unknown (e.g. from pipe), we pass -1 or similar?
	// But `upload` command does `f.Stat()` and passes size.
	// For piping, `cat` sends size? No.
	// If pipe, size might be unknown.

	// However, my `Upload` command impl calls `f.Stat()` even for pipe?
	// No, `upload` command currently opens a file by path.
	// IF we support piping, we'd pass `os.Stdin` and size=0?
	// My `Upload` method in `client.go` signature: `Upload(ctx, reader, name, parentID, size)`.
	// If simple upload, we stream.
	// If multipart, we need random access OR sequential upload of parts.

	// For now, let's keep it simple:
	// If size < Threshold (65MB), buffer in memory (if reader) and use simple.
	// If size > Threshold, we need a file or we fail for pipes?
	// The current implementation takes `size`.

	if size > MultipartThresh {
		// Multipart Upload
		// We need to cast reader to `io.ReaderAt`? or `*os.File`?
		if f, ok := reader.(*os.File); ok {
			stat, err := f.Stat()
			if err == nil {
				return c.uploadMultipart(ctx, f, stat, name, parentID, nil, workspaceID)
			}
		}
		// For bytes.Reader, we can use uploadMultipartFromReader
		if br, ok := reader.(*bytes.Reader); ok {
			return c.uploadMultipartFromReader(ctx, br, name, size, parentID, workspaceID)
		}
		return nil, fmt.Errorf("multipart upload only supported for files and byte readers currently")
	} else {
		// Simple Upload
		return c.uploadSimple(ctx, reader, name, size, parentID, workspaceID)
	}
}

func (c *HTTPClient) uploadSimple(ctx context.Context, reader io.Reader, name string, size int64, parentID *int64, workspaceID int64) (*FileEntry, error) {
	// Detect MIME type from content using magic bytes
	mimeType, headerReader, err := detectMimeType(reader, name)
	if err != nil {
		return nil, fmt.Errorf("failed to detect mime type: %w", err)
	}

	// Chain header back with rest of reader
	combinedReader := io.MultiReader(headerReader, reader)

	// Read entire content into memory for S3 upload (we need content length)
	// For simple uploads (<65MB), this is acceptable
	content, err := io.ReadAll(combinedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read file content: %w", err)
	}
	actualSize := int64(len(content))
	if size <= 0 {
		size = actualSize
	}

	// Extract extension
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:] // remove dot
	}

	// 1. Get presigned URL from API
	presignReq := SimplePresignRequest{
		Filename:    name,
		Mime:        mimeType,
		Size:        size,
		Extension:   ext,
		ParentID:    parentID,
		WorkspaceID: workspaceID,
	}

	presignBody, _ := json.Marshal(presignReq)
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/simple/presign", bytes.NewReader(presignBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("presign request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("presign failed (%s): %s", resp.Status, extractAPIError(b))
	}

	var presignRes SimplePresignResponse
	if err := json.NewDecoder(resp.Body).Decode(&presignRes); err != nil {
		return nil, fmt.Errorf("failed to decode presign response: %w", err)
	}

	// 2. Upload directly to S3 using presigned URL (with retries)
	var putResp *http.Response
	var lastErr error
	for attempt := 0; attempt <= S3MaxRetries; attempt++ {
		putReq, _ := http.NewRequestWithContext(ctx, "PUT", presignRes.URL, bytes.NewReader(content))
		putReq.ContentLength = actualSize
		putReq.Header.Set("Content-Type", mimeType)
		if presignRes.ACL != "" {
			putReq.Header.Set("x-amz-acl", presignRes.ACL)
		}

		// Use a separate client for S3 (no auth header, longer timeout)
		s3Client := &http.Client{Timeout: 5 * time.Minute}
		putResp, lastErr = s3Client.Do(putReq)

		if lastErr == nil && putResp.StatusCode == http.StatusOK {
			putResp.Body.Close()
			break
		}

		if putResp != nil {
			putResp.Body.Close()
		}

		if attempt < S3MaxRetries {
			backoff := S3RetryDelay * time.Duration(1<<attempt)
			jitter := time.Duration(float64(backoff) * 0.25 * (2*rand.Float64() - 1))
			select {
			case <-time.After(backoff + jitter):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("S3 upload failed after %d retries: %w", S3MaxRetries, lastErr)
	}
	if putResp != nil && putResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("S3 upload failed with status: %s", putResp.Status)
	}

	// 3. Create file entry in Drime
	s3Filename := filepath.Base(presignRes.Key)
	entryReq := CreateS3EntryRequest{
		Filename:        s3Filename,
		Size:            size,
		ClientMime:      mimeType,
		ClientName:      name,
		ClientExtension: ext,
		ParentID:        parentID,
		WorkspaceID:     workspaceID,
	}

	entryBody, _ := json.Marshal(entryReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/entries", bytes.NewReader(entryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("create entry failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create entry failed (%s): %s", resp.Status, extractAPIError(b))
	}

	var entryRes CreateS3EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&entryRes); err != nil {
		return nil, err
	}

	return &entryRes.FileEntry, nil
}

// uploadMultipartFromReader handles multipart upload for bytes.Reader
func (c *HTTPClient) uploadMultipartFromReader(ctx context.Context, reader *bytes.Reader, name string, size int64, parentID *int64, workspaceID int64) (*FileEntry, error) {
	// Detect MIME type from content using magic bytes
	mimeType, headerReader, err := detectMimeType(reader, name)
	if err != nil {
		return nil, fmt.Errorf("failed to detect mime type: %w", err)
	}
	// Reset reader position after detection (bytes.Reader supports this)
	_, _ = reader.Seek(0, io.SeekStart)
	_ = headerReader // Not needed since we can seek

	// 1. Initialize multipart upload
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:] // remove dot
	}

	initReq := CreateMultipartRequest{
		Filename:    name,
		Mime:        mimeType,
		Size:        size,
		Extension:   ext,
		WorkspaceID: workspaceID,
	}

	initBody, _ := json.Marshal(initReq)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/create", bytes.NewReader(initBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("multipart init failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("multipart init failed (%s): %s", resp.Status, string(b))
	}

	var initRes CreateMultipartResponse
	if err := json.NewDecoder(resp.Body).Decode(&initRes); err != nil {
		return nil, err
	}

	// 2. Sign URL for single part
	signReq := BatchSignRequest{
		Key:         initRes.Key,
		UploadID:    initRes.UploadID,
		PartNumbers: []int{1},
	}
	signBody, _ := json.Marshal(signReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/batch-sign-part-urls", bytes.NewReader(signBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("sign part URL failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sign part URL failed (%s): %s", resp.Status, string(b))
	}

	var signRes BatchSignResponse
	if err := json.NewDecoder(resp.Body).Decode(&signRes); err != nil {
		return nil, err
	}
	if len(signRes.URLs) == 0 {
		return nil, fmt.Errorf("no signed URLs returned")
	}

	// 3. Upload part to S3
	partURL := signRes.URLs[0].URL
	_, _ = reader.Seek(0, io.SeekStart)
	content, _ := io.ReadAll(reader)

	putReq, _ := http.NewRequestWithContext(ctx, "PUT", partURL, bytes.NewReader(content))
	putReq.ContentLength = int64(len(content))
	putReq.Header.Set("Content-Type", "application/octet-stream")

	s3Client := &http.Client{Timeout: 60 * time.Second}
	putResp, err := s3Client.Do(putReq)
	if err != nil {
		return nil, fmt.Errorf("S3 upload failed: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != 200 {
		b, _ := io.ReadAll(putResp.Body)
		return nil, fmt.Errorf("S3 upload failed (%s): %s", putResp.Status, string(b))
	}

	etag := putResp.Header.Get("ETag")

	// 4. Complete multipart upload
	completeReq := CompleteMultipartRequest{
		Key:      initRes.Key,
		UploadID: initRes.UploadID,
		Parts:    []UploadedPart{{PartNumber: 1, ETag: etag}},
	}
	completeBody, _ := json.Marshal(completeReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/complete", bytes.NewReader(completeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("complete multipart failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("complete multipart failed (%s): %s", resp.Status, string(b))
	}

	// 5. Create file entry
	s3Filename := filepath.Base(initRes.Key)
	entryReq := CreateS3EntryRequest{
		Filename:        s3Filename,
		Size:            size,
		ClientMime:      mimeType,
		ClientName:      name,
		ClientExtension: ext,
		ParentID:        parentID,
		WorkspaceID:     workspaceID,
	}
	entryBody, _ := json.Marshal(entryReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/entries", bytes.NewReader(entryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("create entry failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create entry failed (%s): %s", resp.Status, string(b))
	}

	var entryRes CreateS3EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&entryRes); err != nil {
		return nil, err
	}

	return &entryRes.FileEntry, nil
}

func (c *HTTPClient) uploadMultipart(ctx context.Context, file *os.File, stat os.FileInfo, name string, parentID *int64, progress func(int64, int64), workspaceID int64) (*FileEntry, error) {
	// Detect MIME type from file content using magic bytes
	mtype, err := mimetype.DetectFile(file.Name())
	mimeType := "application/octet-stream"
	if err == nil {
		mimeType = mtype.String()
	}

	// If detection returned generic octet-stream, try extension fallback
	if mimeType == "application/octet-stream" {
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".txt":
			mimeType = "text/plain"
		case ".json":
			mimeType = "application/json"
		case ".yaml", ".yml":
			mimeType = "application/x-yaml"
		case ".md":
			mimeType = "text/markdown"
		case ".csv":
			mimeType = "text/csv"
		case ".html", ".htm":
			mimeType = "text/html"
		case ".css":
			mimeType = "text/css"
		case ".js":
			mimeType = "text/javascript"
		case ".ts":
			mimeType = "text/typescript"
		case ".go":
			mimeType = "text/x-go"
		case ".py":
			mimeType = "text/x-python"
		case ".rs":
			mimeType = "text/x-rust"
		case ".sh":
			mimeType = "text/x-shellscript"
		}
	}

	// 1. Initialize
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:] // remove dot
	}

	initReq := CreateMultipartRequest{
		Filename:    name,
		Mime:        mimeType,
		Size:        stat.Size(),
		Extension:   ext,
		WorkspaceID: workspaceID,
		// ParentID: Not sent to /create endpoint according to schema
	}

	initBody, _ := json.Marshal(initReq)
	req, _ := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/create", bytes.NewReader(initBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("multipart init failed: %s", resp.Status)
	}

	var initRes CreateMultipartResponse
	if err := json.NewDecoder(resp.Body).Decode(&initRes); err != nil {
		return nil, err
	}

	// 2. Upload Parts
	// Calculate parts
	totalParts := int(math.Ceil(float64(stat.Size()) / float64(ChunkSize)))
	uploadedParts := make([]UploadedPart, totalParts)

	// Worker pool for uploads?
	// Let's do batch sequential for safety first, or small concurrency.
	// AGENTS.md suggests batch signing.

	var uploadedBytes int64
	var mu sync.Mutex

	for i := 0; i < totalParts; i += BatchSize {
		end := i + BatchSize
		if end > totalParts {
			end = totalParts
		}

		// Prepare batch
		batchParts := make([]int, 0, end-i)
		for j := i; j < end; j++ {
			batchParts = append(batchParts, j+1) // 1-based index
		}

		// Sign URLs
		signReq := BatchSignRequest{
			Key:         initRes.Key,
			UploadID:    initRes.UploadID,
			PartNumbers: batchParts,
		}
		signBody, _ := json.Marshal(signReq)
		req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/batch-sign-part-urls", bytes.NewReader(signBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.Token)

		resp, err = c.DoWithRetry(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var signRes BatchSignResponse
		if err := json.NewDecoder(resp.Body).Decode(&signRes); err != nil {
			return nil, err
		}

		// Upload this batch
		// Can be concurrent
		var wg sync.WaitGroup
		errChan := make(chan error, len(batchParts))

		for _, signedPart := range signRes.URLs {
			wg.Add(1)
			go func(partNum int, url string) {
				defer wg.Done()

				// Read chunk
				offset := int64(partNum-1) * ChunkSize

				// We need a SectionReader that is thread safe?
				// os.File ReadAt is thread safe.
				chunkSize := int64(ChunkSize)
				if offset+chunkSize > stat.Size() {
					chunkSize = stat.Size() - offset
				}

				// Read data
				buf := make([]byte, chunkSize)
				_, err := file.ReadAt(buf, offset)
				if err != nil && err != io.EOF {
					errChan <- err
					return
				}

				// Upload to S3 (PUT)
				// Retry logic is needed here specifically for S3 usually
				putReq, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(buf))
				// S3 requires Content-Length usually, set automatically by Go for BytesReader

				// Use bare http client for S3 signed URL? Or c.Client?
				// c.Client has timeouts. Signed URLs have expiry.
				// S3 urls don't need Bearer token.
				// DoWithRetry adds Token? YES. We should NOT use DoWithRetry for S3 calls if it adds token.
				// c.DoWithRetry adds token? NO, `DoWithRetry` just retries. `Upload` caller added token.
				// But `DoWithRetry` implementation in `http.go` DOES NOT add token. The CALLER does.
				// Ah, wait. `Whoami` added token. I checked `http.go`.
				// `DoWithRetry` takes `req`. It does NOT add token. Good.

				putResp, err := c.DoWithRetry(putReq)
				if err != nil {
					errChan <- err
					return
				}
				defer putResp.Body.Close()

				if putResp.StatusCode != 200 {
					errChan <- fmt.Errorf("S3 upload failed: %s", putResp.Status)
					return
				}

				etag := putResp.Header.Get("ETag")
				// S3 ETags are quoted usually
				// struct needs strict string?

				mu.Lock()
				uploadedParts[partNum-1] = UploadedPart{
					PartNumber: partNum,
					ETag:       etag, // clean quotes?
				}
				uploadedBytes += chunkSize
				if progress != nil {
					progress(uploadedBytes, stat.Size())
				}
				mu.Unlock()

			}(signedPart.PartNumber, signedPart.URL)
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			// Abort the multipart upload on failure
			_ = c.AbortMultipart(ctx, initRes.Key, initRes.UploadID)
			return nil, err
		}
	}

	// 3. Complete
	// Clean ETags if needed (usually API handles it, or we trim quotes)
	// Some interfaces expect ETag "hash" others "\"hash\"".
	// Let's rely on standard AWS SDK behavior or raw string.
	// Drime API probably expects what S3 returns.

	compReq := CompleteMultipartRequest{
		Key:      initRes.Key,
		UploadID: initRes.UploadID,
		Parts:    uploadedParts,
	}
	compBody, _ := json.Marshal(compReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/complete", bytes.NewReader(compBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("complete failed: %s", resp.Status)
	}

	// 4. Create Entry
	// Extract just the filename from the S3 key (e.g., \"uploads/uuid/uuid\" -> \"uuid\")
	s3Filename := filepath.Base(initRes.Key)
	entryReq := CreateS3EntryRequest{
		Filename:        s3Filename,
		Size:            stat.Size(),
		ClientMime:      mimeType,
		ClientName:      name,
		ClientExtension: ext,
		ParentID:        parentID,
		WorkspaceID:     workspaceID,
	}

	entryBody, _ := json.Marshal(entryReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/entries", bytes.NewReader(entryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("CreateEntry failed: %s", resp.Status)
	}

	var res CreateS3EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return &res.FileEntry, nil
}

// AbortMultipart aborts an in-progress multipart upload
func (c *HTTPClient) AbortMultipart(ctx context.Context, key, uploadID string) error {
	abortReq := AbortMultipartRequest{
		Key:      key,
		UploadID: uploadID,
	}
	abortBody, _ := json.Marshal(abortReq)
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/multipart/abort", bytes.NewReader(abortBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return fmt.Errorf("abort multipart failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("abort multipart failed (%s): %s", resp.Status, extractAPIError(b))
	}

	return nil
}

// ValidateEntries checks for duplicates and quota before upload/move/copy
func (c *HTTPClient) ValidateEntries(ctx context.Context, req ValidateRequest) (*ValidateResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validate request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/uploads/validate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add workspaceId query param as well, just in case (some endpoints require it in query)
	q := httpReq.URL.Query()
	q.Add("workspaceId", fmt.Sprintf("%d", req.WorkspaceID))
	httpReq.URL.RawQuery = q.Encode()

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("validate failed: %s", resp.Status)
	}

	var validateResp ValidateResponse
	if err := json.NewDecoder(resp.Body).Decode(&validateResp); err != nil {
		return nil, fmt.Errorf("failed to decode validate response: %w", err)
	}

	return &validateResp, nil
}

// GetAvailableName gets an alternative filename if the original exists
func (c *HTTPClient) GetAvailableName(ctx context.Context, req GetAvailableNameRequest) (*GetAvailableNameResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/entry/getAvailableName", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add workspaceId query param
	q := httpReq.URL.Query()
	q.Add("workspaceId", fmt.Sprintf("%d", req.WorkspaceID))
	httpReq.URL.RawQuery = q.Encode()

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("getAvailableName failed: %s", resp.Status)
	}

	var availResp GetAvailableNameResponse
	if err := json.NewDecoder(resp.Body).Decode(&availResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &availResp, nil
}

// VaultSimplePresignRequest is the request body for vault file presigning
type VaultSimplePresignRequest struct {
	Filename     string `json:"filename"`
	Mime         string `json:"mime"`
	Size         int64  `json:"size"`
	Extension    string `json:"extension,omitempty"`
	ParentID     *int64 `json:"parentId,omitempty"`
	RelativePath string `json:"relativePath,omitempty"`
	IsEncrypted  int    `json:"isEncrypted"`
	VaultIvs     string `json:"vaultIvs"`
	VaultID      int64  `json:"vaultId"`
	WorkspaceID  *int64 `json:"workspaceId"` // null for vault
}

// VaultCreateS3EntryRequest is the request body for creating a vault file entry
type VaultCreateS3EntryRequest struct {
	ParentID        *int64 `json:"parentId,omitempty"`
	Filename        string `json:"filename"`
	ClientMime      string `json:"clientMime"`
	ClientName      string `json:"clientName"`
	ClientExtension string `json:"clientExtension"`
	RelativePath    string `json:"relativePath,omitempty"`
	Size            int64  `json:"size"`
	WorkspaceID     *int64 `json:"workspaceId"` // null for vault
	IsEncrypted     int    `json:"isEncrypted"`
	VaultIvs        string `json:"vaultIvs"`
	VaultID         int64  `json:"vaultId"`
}

// UploadToVault uploads an already-encrypted file to the vault.
// The encryptedContent should be the AES-GCM encrypted data.
// ivBase64 is the base64-encoded IV used for encryption.
func (c *HTTPClient) UploadToVault(ctx context.Context, encryptedContent []byte, name string, parentID *int64, vaultID int64, ivBase64 string) (*FileEntry, error) {
	size := int64(len(encryptedContent))

	// Extract extension
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:] // remove dot
	}

	// For vault uploads, mime is always application/octet-stream (encrypted blob)
	mimeType := "application/octet-stream"

	// vaultIvs format: ",<base64IV>" (leading comma, one IV per file)
	vaultIvs := "," + ivBase64

	// 1. Get presigned URL from API
	presignReq := VaultSimplePresignRequest{
		Filename:    name,
		Mime:        mimeType,
		Size:        size,
		Extension:   ext,
		ParentID:    parentID,
		IsEncrypted: 1,
		VaultIvs:    vaultIvs,
		VaultID:     vaultID,
		WorkspaceID: nil, // null for vault
	}

	presignBody, _ := json.Marshal(presignReq)
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/simple/presign", bytes.NewReader(presignBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("presign request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("presign failed (%s): %s", resp.Status, extractAPIError(b))
	}

	var presignRes SimplePresignResponse
	if err := json.NewDecoder(resp.Body).Decode(&presignRes); err != nil {
		return nil, fmt.Errorf("failed to decode presign response: %w", err)
	}

	// 2. Upload encrypted content directly to S3
	var putResp *http.Response
	var lastErr error
	for attempt := 0; attempt <= S3MaxRetries; attempt++ {
		putReq, _ := http.NewRequestWithContext(ctx, "PUT", presignRes.URL, bytes.NewReader(encryptedContent))
		putReq.ContentLength = size
		putReq.Header.Set("Content-Type", mimeType)
		if presignRes.ACL != "" {
			putReq.Header.Set("x-amz-acl", presignRes.ACL)
		}

		s3Client := &http.Client{Timeout: 5 * time.Minute}
		putResp, lastErr = s3Client.Do(putReq)

		if lastErr == nil && putResp.StatusCode == http.StatusOK {
			putResp.Body.Close()
			break
		}

		if putResp != nil {
			putResp.Body.Close()
		}

		if attempt < S3MaxRetries {
			backoff := S3RetryDelay * time.Duration(1<<attempt)
			jitter := time.Duration(float64(backoff) * 0.25 * (2*rand.Float64() - 1))
			select {
			case <-time.After(backoff + jitter):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("S3 upload failed after %d retries: %w", S3MaxRetries, lastErr)
	}
	if putResp != nil && putResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("S3 upload failed with status: %s", putResp.Status)
	}

	// 3. Create file entry in Drime with vault metadata
	s3Filename := filepath.Base(presignRes.Key)
	entryReq := VaultCreateS3EntryRequest{
		Filename:        s3Filename,
		Size:            size,
		ClientMime:      mimeType,
		ClientName:      name,
		ClientExtension: ext,
		ParentID:        parentID,
		WorkspaceID:     nil, // null for vault
		IsEncrypted:     1,
		VaultIvs:        vaultIvs,
		VaultID:         vaultID,
	}

	entryBody, _ := json.Marshal(entryReq)
	req, _ = http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/s3/entries", bytes.NewReader(entryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err = c.DoWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("create entry failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create entry failed (%s): %s", resp.Status, extractAPIError(b))
	}

	var entryRes CreateS3EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&entryRes); err != nil {
		return nil, err
	}

	return &entryRes.FileEntry, nil
}
