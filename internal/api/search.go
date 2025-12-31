package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type GetEntryResponse struct {
	Status    string    `json:"status"`
	FileEntry FileEntry `json:"fileEntry"`
}

func (c *HTTPClient) GetEntry(ctx context.Context, entryID int64, workspaceID int64) (*FileEntry, error) {
	q := url.Values{}
	q.Set("workspaceId", fmt.Sprintf("%d", workspaceID))
	path := fmt.Sprintf("/file-entries/%d", entryID)
	status, respBody, err := c.do(ctx, http.MethodGet, path, q, nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		msg := extractAPIError(respBody)
		if msg == "" {
			msg = fmt.Sprintf("%d %s", status, http.StatusText(status))
		}
		return nil, fmt.Errorf("GetEntry failed: %s", msg)
	}

	var res GetEntryResponse
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, err
	}
	return &res.FileEntry, nil
}

func (c *HTTPClient) Search(ctx context.Context, query string) ([]FileEntry, error) {
	return c.SearchWithOptions(ctx, query, nil)
}
