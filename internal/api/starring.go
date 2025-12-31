package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// StarEntries marks the given entries as starred
func (c *HTTPClient) StarEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	body := map[string][]int64{
		"entryIds": entryIDs,
	}
	bodyBytes, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/file-entries/star?workspaceId=%d", c.BaseURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("StarEntries failed: %s", resp.Status)
	}

	return nil
}

// UnstarEntries removes the starred tag from the given entries
func (c *HTTPClient) UnstarEntries(ctx context.Context, entryIDs []int64, workspaceID int64) error {
	body := map[string][]int64{
		"entryIds": entryIDs,
	}
	bodyBytes, _ := json.Marshal(body)

	url := fmt.Sprintf("%s/file-entries/unstar?workspaceId=%d", c.BaseURL, workspaceID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("UnstarEntries failed: %s", resp.Status)
	}

	return nil
}
