package api

import (
	"context"
	"fmt"
	"net/http"
)

// GetTrackedFiles returns the list of files currently being tracked
func (c *HTTPClient) GetTrackedFiles(ctx context.Context) ([]TrackedFile, error) {
	var result struct {
		Status     string `json:"status"`
		Pagination struct {
			Data []TrackedFile `json:"data"`
		} `json:"pagination"`
	}
	// Note: The endpoint is /track/all
	err := c.doJSON(ctx, http.MethodGet, "/track/all", nil, nil, &result, true)
	if err != nil {
		return nil, err
	}
	return result.Pagination.Data, nil
}

// GetTrackingStats returns detailed tracking statistics for a file
func (c *HTTPClient) GetTrackingStats(ctx context.Context, entryID int64) (*TrackingStatsResponse, error) {
	var stats TrackingStatsResponse
	path := fmt.Sprintf("/track/infos/%d", entryID)
	err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &stats, true)
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// SetTracking enables or disables tracking for a file
func (c *HTTPClient) SetTracking(ctx context.Context, entryID int64, tracked bool) error {
	req := SetTrackingRequest{
		FileID:  entryID,
		Tracked: tracked,
	}
	return c.doJSON(ctx, http.MethodPost, "/track/setTracked", nil, req, nil, true)
}
