package api

import (
	"encoding/base64"
	"encoding/json"
)

// Filter represents a single search filter
type Filter struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Operator   string      `json:"operator"`
	IsInactive bool        `json:"isInactive,omitempty"`
}

// Filter Keys
const (
	FilterKeyType          = "type"
	FilterKeyPublic        = "public"
	FilterKeyOwnerID       = "owner_id"
	FilterKeySharedByMe    = "sharedByMe"
	FilterKeyShareableLink = "shareableLink"
	FilterKeyCreatedAt     = "created_at"
	FilterKeyUpdatedAt     = "updated_at"
)

// Filter Operators
const (
	FilterOpEquals  = "="
	FilterOpNotEq   = "!="
	FilterOpGreater = ">"
	FilterOpLess    = "<"
	FilterOpBetween = "between"
	FilterOpHas     = "has"
)

// File types supported by the API
const (
	FileTypeFolder      = "folder"
	FileTypeText        = "text"
	FileTypeAudio       = "audio"
	FileTypeVideo       = "video"
	FileTypeImage       = "image"
	FileTypePDF         = "pdf"
	FileTypeSpreadsheet = "spreadsheet"
	FileTypeWord        = "word"
	FileTypePhotoshop   = "photoshop"
	FileTypeArchive     = "archive"
	FileTypePowerPoint  = "powerPoint"
)

// EncodeFilters encodes a slice of filters into the format expected by the API
// (Base64 of JSON string). The caller is responsible for URL encoding if necessary,
// though typically the HTTP client handles that when constructing query params.
func EncodeFilters(filters []Filter) string {
	if len(filters) == 0 {
		return ""
	}
	jsonBytes, err := json.Marshal(filters)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(jsonBytes)
}

// ListOptions creates a ListEntriesOptions with default values for the given workspace.
// This is the recommended way to create options to ensure consistency.
func ListOptions(workspaceID int64) *ListEntriesOptions {
	return &ListEntriesOptions{
		WorkspaceID: workspaceID,
		OrderBy:     "name",
		OrderDir:    "asc",
	}
}

// SearchOptions creates a ListEntriesOptions configured for search.
func SearchOptions(workspaceID int64, query string) *ListEntriesOptions {
	return &ListEntriesOptions{
		WorkspaceID: workspaceID,
		Query:       query,
		OrderBy:     "updated_at",
		OrderDir:    "desc",
	}
}

// WithFilters adds encoded filters to the options.
func (o *ListEntriesOptions) WithFilters(filters []Filter) *ListEntriesOptions {
	o.Filters = EncodeFilters(filters)
	return o
}

// WithDeletedOnly sets the option to only return trashed items.
func (o *ListEntriesOptions) WithDeletedOnly() *ListEntriesOptions {
	o.DeletedOnly = true
	return o
}

// WithStarredOnly sets the option to only return starred items.
func (o *ListEntriesOptions) WithStarredOnly() *ListEntriesOptions {
	o.StarredOnly = true
	return o
}

// WithTrackedOnly sets the option to only return tracked items.
func (o *ListEntriesOptions) WithTrackedOnly() *ListEntriesOptions {
	o.TrackedOnly = true
	return o
}

// WithOrder sets the sort order.
func (o *ListEntriesOptions) WithOrder(orderBy, orderDir string) *ListEntriesOptions {
	o.OrderBy = orderBy
	o.OrderDir = orderDir
	return o
}
