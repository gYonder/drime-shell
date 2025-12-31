package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type driveFileEntriesPage struct {
	Data        []FileEntry `json:"data"`
	CurrentPage int         `json:"current_page"`
	LastPage    int         `json:"last_page"`
}

func effectiveListEntriesOptions(opts *ListEntriesOptions) ListEntriesOptions {
	effective := ListEntriesOptions{
		WorkspaceID: 0,
		PerPage:     MaxPerPage,
		Page:        0,
		OrderBy:     "name",
		OrderDir:    "asc",
	}

	if opts == nil {
		return effective
	}

	effective.WorkspaceID = opts.WorkspaceID
	effective.DeletedOnly = opts.DeletedOnly
	effective.StarredOnly = opts.StarredOnly
	effective.TrackedOnly = opts.TrackedOnly
	effective.Query = opts.Query
	effective.Filters = opts.Filters
	effective.Backup = opts.Backup
	effective.Page = opts.Page

	if opts.PerPage > 0 {
		effective.PerPage = opts.PerPage
	}
	if opts.OrderBy != "" {
		effective.OrderBy = opts.OrderBy
	}
	if opts.OrderDir != "" {
		effective.OrderDir = opts.OrderDir
	}

	return effective
}

func buildDriveFileEntriesQuery(parentID *int64, opts ListEntriesOptions, pageNum int) url.Values {
	vals := url.Values{}
	vals.Set("workspaceId", fmt.Sprintf("%d", opts.WorkspaceID))
	vals.Set("orderBy", opts.OrderBy)
	vals.Set("orderDir", opts.OrderDir)
	vals.Set("perPage", fmt.Sprintf("%d", opts.PerPage))

	if pageNum > 0 {
		vals.Set("page", fmt.Sprintf("%d", pageNum))
	}
	if opts.Backup != nil {
		vals.Set("backup", fmt.Sprintf("%d", *opts.Backup))
	}
	if opts.DeletedOnly {
		vals.Set("deletedOnly", "true")
	}
	if opts.StarredOnly {
		vals.Set("starredOnly", "true")
	}
	if opts.Query != "" {
		vals.Set("query", opts.Query)
	}
	if opts.Filters != "" {
		vals.Set("filters", opts.Filters)
	}
	if parentID != nil {
		vals.Set("parentIds", fmt.Sprintf("%d", *parentID))
	}

	return vals
}

func (c *HTTPClient) fetchDriveFileEntriesPage(ctx context.Context, parentID *int64, opts ListEntriesOptions, pageNum int) (*driveFileEntriesPage, error) {
	vals := buildDriveFileEntriesQuery(parentID, opts, pageNum)
	reqURL := fmt.Sprintf("%s/drive/file-entries?%s", c.BaseURL, vals.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.DoWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		msg := extractAPIError(body)
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("API error: %s", msg)
	}

	var page driveFileEntriesPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (c *HTTPClient) listDriveFileEntries(ctx context.Context, parentID *int64, opts ListEntriesOptions) ([]FileEntry, error) {
	// If a specific page was requested, do a single fetch.
	if opts.Page > 0 {
		page, err := c.fetchDriveFileEntriesPage(ctx, parentID, opts, opts.Page)
		if err != nil {
			return nil, err
		}
		return page.Data, nil
	}

	// Auto-page until last_page.
	initialCap := opts.PerPage
	if initialCap > 1000 {
		initialCap = 1000
	}
	all := make([]FileEntry, 0, initialCap)

	pageNum := 1
	for {
		page, err := c.fetchDriveFileEntriesPage(ctx, parentID, opts, pageNum)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Data...)

		if page.LastPage <= 0 {
			// Defensive: if server doesn't return paging fields, stop after first page.
			break
		}
		if page.CurrentPage >= page.LastPage {
			break
		}
		pageNum = page.CurrentPage + 1
	}

	return all, nil
}

// ListByParentIDWithOptions lists entries with filtering options (starred, trash, etc.)
func (c *HTTPClient) ListByParentIDWithOptions(ctx context.Context, parentID *int64, opts *ListEntriesOptions) ([]FileEntry, error) {
	effective := effectiveListEntriesOptions(opts)

	// Special case: tracked files are provided by a dedicated endpoint.
	if effective.TrackedOnly {
		tracked, err := c.GetTrackedFiles(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]FileEntry, len(tracked))
		for i, t := range tracked {
			entries[i] = FileEntry{
				ID:      t.ID,
				Name:    t.Name,
				Type:    t.Type,
				Size:    t.FileSize,
				Tracked: 1,
			}
		}
		return entries, nil
	}

	return c.listDriveFileEntries(ctx, parentID, effective)
}

// SearchWithOptions searches with filtering options.
func (c *HTTPClient) SearchWithOptions(ctx context.Context, query string, opts *ListEntriesOptions) ([]FileEntry, error) {
	local := &ListEntriesOptions{}
	if opts != nil {
		*local = *opts
	}
	local.Query = query
	return c.ListByParentIDWithOptions(ctx, nil, local)
}
