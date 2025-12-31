package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// DownloadOptions configures a download operation
type DownloadOptions struct {
	// ResumeFrom specifies the byte offset to resume from (for Range requests)
	ResumeFrom int64
}

func (c *HTTPClient) Download(ctx context.Context, hash string, w io.Writer, progress func(int64, int64)) (*FileEntry, error) {
	return c.DownloadWithOptions(ctx, hash, w, progress, nil)
}

// DownloadWithOptions downloads a file with configurable options including resume support
func (c *HTTPClient) DownloadWithOptions(ctx context.Context, hash string, w io.Writer, progress func(int64, int64), opts *DownloadOptions) (*FileEntry, error) {
	// GET /file-entries/download/{hash}
	url := fmt.Sprintf("%s/file-entries/download/%s", c.BaseURL, hash)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	// Add Range header for resumable downloads
	resumeOffset := int64(0)
	if opts != nil && opts.ResumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", opts.ResumeFrom))
		resumeOffset = opts.ResumeFrom
	}

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check for successful response (200 OK or 206 Partial Content)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("Download failed: %s", resp.Status)
	}

	// Try to get metadata from headers
	var entry FileEntry

	// Content-Length (this is the length of the response, not the full file if partial)
	responseLength := resp.ContentLength

	// For partial content, try to get full size from Content-Range header
	// Format: "bytes 1000-1999/2000" where 2000 is total size
	if resp.StatusCode == http.StatusPartialContent {
		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
			var start, end, total int64
			if _, err := fmt.Sscanf(contentRange, "bytes %d-%d/%d", &start, &end, &total); err == nil {
				entry.Size = total
			}
		}
		// For progress tracking, adjust current position
		if entry.Size == 0 && responseLength > 0 {
			entry.Size = resumeOffset + responseLength
		}
	} else {
		entry.Size = responseLength
	}

	// Last-Modified
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		if t, err := http.ParseTime(lastMod); err == nil {
			entry.UpdatedAt = t
		}
	}

	// Check if server supports range requests
	// TODO: Add SupportsResume to FileEntry struct and uncomment this line.
	// entry.SupportsResume = resp.Header.Get("Accept-Ranges") == "bytes"

	// Wrap reader to track progress
	pw := &ProgressReader{
		Reader:     resp.Body,
		Total:      entry.Size,
		Current:    resumeOffset, // Start from resume offset for accurate progress
		OnProgress: progress,
	}

	_, err = io.Copy(w, pw)
	if err != nil {
		return nil, err
	}

	return &entry, nil
}

// CheckResumeSupport checks if the server supports Range requests for a given file
func (c *HTTPClient) CheckResumeSupport(ctx context.Context, hash string) (bool, int64, error) {
	url := fmt.Sprintf("%s/file-entries/download/%s", c.BaseURL, hash)

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	supportsRange := resp.Header.Get("Accept-Ranges") == "bytes"
	contentLength := resp.ContentLength

	return supportsRange, contentLength, nil
}

type ProgressReader struct {
	io.Reader
	OnProgress func(int64, int64)
	Total      int64
	Current    int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.OnProgress != nil {
		pr.OnProgress(pr.Current, pr.Total)
	}
	return n, err
}
