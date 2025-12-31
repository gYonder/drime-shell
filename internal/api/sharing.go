package api

import (
	"context"
	"fmt"
	"net/http"
)

// CreateShareableLink creates a new public link for a file entry
func (c *HTTPClient) CreateShareableLink(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error) {
	var result struct {
		Link ShareableLink `json:"link"`
	}
	path := fmt.Sprintf("/file-entries/%d/shareable-link", entryID)
	err := c.doJSON(ctx, http.MethodPost, path, nil, req, &result, true)
	if err != nil {
		return nil, err
	}
	return &result.Link, nil
}

// CreateFileRequest creates a new file request for a folder
func (c *HTTPClient) CreateFileRequest(ctx context.Context, entryID int64, title, description string) (*ShareableLink, error) {
	req := ShareableLinkRequest{
		Request: &FileRequestPayload{
			Title:       title,
			Description: description,
		},
		AllowDownload: false,
		AllowEdit:     false,
	}
	return c.CreateShareableLink(ctx, entryID, req)
}

// UpdateShareableLink updates an existing public link
// Uses Laravel method spoofing: POST with ?_method=PUT and X-HTTP-Method-Override header
func (c *HTTPClient) UpdateShareableLink(ctx context.Context, entryID int64, req ShareableLinkRequest) (*ShareableLink, error) {
	var result struct {
		Link ShareableLink `json:"link"`
	}
	path := fmt.Sprintf("/file-entries/%d/shareable-link?_method=PUT", entryID)
	headers := map[string]string{
		"X-HTTP-Method-Override": "PUT",
	}
	err := c.doJSONWithHeaders(ctx, http.MethodPost, path, nil, headers, req, &result, true)
	if err != nil {
		return nil, err
	}
	return &result.Link, nil
}

// DeleteShareableLink removes a public link
func (c *HTTPClient) DeleteShareableLink(ctx context.Context, entryID int64) error {
	path := fmt.Sprintf("/file-entries/%d/shareable-link", entryID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil, true)
}

// GetShareableLink retrieves the public link for a file entry (if one exists)
func (c *HTTPClient) GetShareableLink(ctx context.Context, entryID int64) (*ShareableLink, error) {
	var result struct {
		Link ShareableLink `json:"link"`
	}
	path := fmt.Sprintf("/file-entries/%d/shareable-link", entryID)
	err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &result, true)
	if err != nil {
		return nil, err
	}
	return &result.Link, nil
}

// ShareEntry shares a file entry with specified emails and permissions
func (c *HTTPClient) ShareEntry(ctx context.Context, entryID int64, emails []string, permissions []string) error {
	path := fmt.Sprintf("/file-entries/%d/share", entryID)
	req := struct {
		Emails      []string `json:"emails"`
		Permissions []string `json:"permissions"`
	}{
		Emails:      emails,
		Permissions: permissions,
	}
	return c.doJSON(ctx, http.MethodPost, path, nil, req, nil, true)
}
