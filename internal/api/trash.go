package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// RestoreEntries restores entries from trash to their original location
func (c *HTTPClient) RestoreEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	body := map[string][]int64{
		"entryIds": entryIDs,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/file-entries/restore", q, body, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return fmt.Errorf("RestoreEntries failed: %s", msg)
	}
	return nil
}

// EmptyTrash permanently deletes all items in trash
func (c *HTTPClient) EmptyTrash(ctx context.Context, workspaceID int64) error {
	body := map[string]interface{}{
		"entryIds":   []int64{},
		"emptyTrash": true,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/file-entries/delete", q, body, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return fmt.Errorf("EmptyTrash failed: %s", msg)
	}
	return nil
}

// DeleteEntriesForever permanently deletes entries (bypasses trash)
func (c *HTTPClient) DeleteEntriesForever(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	body := map[string]interface{}{
		"entryIds":      entryIDs,
		"deleteForever": true,
	}

	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	status, respBody, err := c.do(ctx, http.MethodPost, "/file-entries/delete", q, body, true)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return fmt.Errorf("DeleteEntriesForever failed: %s", msg)
	}
	return nil
}
