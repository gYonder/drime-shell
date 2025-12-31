package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type CreateFolderRequest struct {
	ParentID *int64 `json:"parentId"`
	Name     string `json:"name"`
}

type CreateFolderResponse struct {
	Status string    `json:"status"`
	Folder FileEntry `json:"folder"`
}

func (c *HTTPClient) CreateFolder(ctx context.Context, name string, parentID *int64, workspaceID int64) (*FileEntry, error) {
	reqBody := CreateFolderRequest{
		Name:     name,
		ParentID: parentID,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/folders", q, reqBody, true)
	if err != nil {
		return nil, err
	}

	// Check if we got HTML (likely an error page or SPA redirect)
	if len(respBody) > 0 && respBody[0] == '<' {
		return nil, fmt.Errorf("CreateFolder failed: got HTML response (status %d) - API may be unavailable or endpoint incorrect", status)
	}

	if status != http.StatusOK && status != http.StatusCreated {
		// Try to extract error message from JSON response
		var errResp struct {
			Message string `json:"message"`
			Error   string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil {
			msg := errResp.Message
			if msg == "" {
				msg = errResp.Error
			}
			if msg != "" {
				return nil, fmt.Errorf("CreateFolder failed: %s", msg)
			}
		}
		return nil, fmt.Errorf("CreateFolder failed: %d %s", status, http.StatusText(status))
	}

	var res CreateFolderResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &res.Folder, nil
}

type DeleteEntriesRequest struct {
	EntryIDs      []int64 `json:"entryIds"`
	DeleteForever bool    `json:"deleteForever"`
}

func (c *HTTPClient) DeleteEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	reqBody := DeleteEntriesRequest{
		EntryIDs:      entryIDs,
		DeleteForever: false, // Default to trash
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/file-entries/delete", q, reqBody, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return fmt.Errorf("DeleteEntries failed: %s", msg)
	}
	return nil
}

type MoveRequest struct {
	DestinationID *int64  `json:"destinationId"`
	EntryIDs      []int64 `json:"entryIds"`
	WorkspaceID   *int64  `json:"workspaceId,omitempty"`
}

func (c *HTTPClient) MoveEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) error {
	// For cross-workspace moves, the URL and body workspaceId must be the DESTINATION workspace
	queryWsID := workspaceID
	var bodyWsID *int64
	if destinationWorkspaceID != nil {
		queryWsID = *destinationWorkspaceID
		bodyWsID = destinationWorkspaceID
	} else if workspaceID != 0 {
		bodyWsID = &workspaceID
	}

	reqBody := MoveRequest{
		EntryIDs:      entryIDs,
		DestinationID: destinationParentID,
		WorkspaceID:   bodyWsID,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", queryWsID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/file-entries/move", q, reqBody, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return fmt.Errorf("MoveEntries failed: %s", msg)
	}
	return nil
}

func (c *HTTPClient) CopyEntries(ctx context.Context, entryIDs []int64, destinationParentID *int64, workspaceID int64, destinationWorkspaceID *int64) ([]FileEntry, error) {
	// For cross-workspace copies, the URL and body workspaceId must be the DESTINATION workspace
	queryWsID := workspaceID
	var bodyWsID *int64
	if destinationWorkspaceID != nil {
		queryWsID = *destinationWorkspaceID
		bodyWsID = destinationWorkspaceID
	} else if workspaceID != 0 {
		bodyWsID = &workspaceID
	}

	reqBody := MoveRequest{ // Valid for Copy too (destinationId, entryIds)
		EntryIDs:      entryIDs,
		DestinationID: destinationParentID,
		WorkspaceID:   bodyWsID,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", queryWsID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/file-entries/duplicate", q, reqBody, true)
	if err != nil {
		return nil, err
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("CopyEntries failed: %d %s - %s", status, http.StatusText(status), string(respBody))
	}

	var result struct {
		Status  string      `json:"status"`
		Entries []FileEntry `json:"entries"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return result.Entries, nil
}

type RenameRequest struct {
	Name string `json:"name"`
}

type RenameResponse struct {
	Status    string    `json:"status"`
	FileEntry FileEntry `json:"fileEntry"`
}

func (c *HTTPClient) RenameEntry(ctx context.Context, entryID int64, newName string, workspaceID int64) (*FileEntry, error) {
	reqBody := RenameRequest{
		Name: newName,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	path := fmt.Sprintf("/file-entries/%d", entryID)
	status, respBody, err := c.do(ctx, http.MethodPut, path, q, reqBody, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return nil, fmt.Errorf("RenameEntry failed: %s", msg)
	}

	var res RenameResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, err
	}
	return &res.FileEntry, nil
}

func (c *HTTPClient) ExtractEntry(ctx context.Context, entryID int64, parentID *int64, workspaceID int64) error {
	// API requires parentId - use 0 for root folder
	pid := int64(0)
	if parentID != nil {
		pid = *parentID
	}
	body := map[string]interface{}{
		"parentId": pid,
		"password": nil,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	path := fmt.Sprintf("/file-entries/%d/extract", entryID)
	status, respBody, err := c.do(ctx, http.MethodPost, path, q, body, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("extraction failed: %s", string(respBody))
	}
	return nil
}
