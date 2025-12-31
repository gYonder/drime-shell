package api

import (
	"context"
	"fmt"
	"net/http"
)

// ListFileRequests returns a list of all file requests
func (c *HTTPClient) ListFileRequests(ctx context.Context) ([]FileRequest, error) {
	var result struct {
		Pagination struct {
			Data []FileRequest `json:"data"`
		} `json:"pagination"`
	}

	// Use MaxPerPage to get all requests in one go
	path := fmt.Sprintf("/file-requests/all?perPage=%d", MaxPerPage)
	err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &result, true)
	if err != nil {
		return nil, err
	}

	return result.Pagination.Data, nil
}

// DeleteFileRequest deletes a file request by ID
func (c *HTTPClient) DeleteFileRequest(ctx context.Context, requestID int64) error {
	path := fmt.Sprintf("/file-requests/%d", requestID)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil, true)
}
