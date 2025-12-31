package api

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestEncodeFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters []Filter
		want    string
	}{
		{
			name:    "empty filters",
			filters: []Filter{},
			want:    "",
		},
		{
			name:    "nil filters",
			filters: nil,
			want:    "",
		},
		{
			name: "single filter",
			filters: []Filter{
				{Key: FilterKeyType, Value: "image", Operator: FilterOpEquals},
			},
		},
		{
			name: "shareableLink filter",
			filters: []Filter{
				{Key: FilterKeyShareableLink, Value: "*", Operator: FilterOpHas},
			},
		},
		{
			name: "multiple filters",
			filters: []Filter{
				{Key: FilterKeyType, Value: "pdf", Operator: FilterOpEquals},
				{Key: FilterKeySharedByMe, Value: true, Operator: FilterOpEquals},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeFilters(tt.filters)

			if len(tt.filters) == 0 {
				if got != "" {
					t.Errorf("EncodeFilters() = %q, want empty string", got)
				}
				return
			}

			// Decode and verify it's valid
			decoded, err := base64.StdEncoding.DecodeString(got)
			if err != nil {
				t.Errorf("EncodeFilters() returned invalid base64: %v", err)
				return
			}

			var filters []Filter
			if err := json.Unmarshal(decoded, &filters); err != nil {
				t.Errorf("EncodeFilters() returned invalid JSON: %v", err)
				return
			}

			if len(filters) != len(tt.filters) {
				t.Errorf("EncodeFilters() decoded to %d filters, want %d", len(filters), len(tt.filters))
			}
		})
	}
}

func TestListOptions(t *testing.T) {
	opts := ListOptions(123)

	if opts.WorkspaceID != 123 {
		t.Errorf("ListOptions().WorkspaceID = %d, want 123", opts.WorkspaceID)
	}
	if opts.OrderBy != "name" {
		t.Errorf("ListOptions().OrderBy = %q, want \"name\"", opts.OrderBy)
	}
	if opts.OrderDir != "asc" {
		t.Errorf("ListOptions().OrderDir = %q, want \"asc\"", opts.OrderDir)
	}
}

func TestSearchOptions(t *testing.T) {
	opts := SearchOptions(456, "test query")

	if opts.WorkspaceID != 456 {
		t.Errorf("SearchOptions().WorkspaceID = %d, want 456", opts.WorkspaceID)
	}
	if opts.Query != "test query" {
		t.Errorf("SearchOptions().Query = %q, want \"test query\"", opts.Query)
	}
	if opts.OrderBy != "updated_at" {
		t.Errorf("SearchOptions().OrderBy = %q, want \"updated_at\"", opts.OrderBy)
	}
	if opts.OrderDir != "desc" {
		t.Errorf("SearchOptions().OrderDir = %q, want \"desc\"", opts.OrderDir)
	}
}

func TestListOptionsChaining(t *testing.T) {
	opts := ListOptions(789).
		WithDeletedOnly().
		WithStarredOnly().
		WithOrder("file_size", "desc")

	if opts.WorkspaceID != 789 {
		t.Errorf("opts.WorkspaceID = %d, want 789", opts.WorkspaceID)
	}
	if !opts.DeletedOnly {
		t.Error("opts.DeletedOnly = false, want true")
	}
	if !opts.StarredOnly {
		t.Error("opts.StarredOnly = false, want true")
	}
	if opts.OrderBy != "file_size" {
		t.Errorf("opts.OrderBy = %q, want \"file_size\"", opts.OrderBy)
	}
	if opts.OrderDir != "desc" {
		t.Errorf("opts.OrderDir = %q, want \"desc\"", opts.OrderDir)
	}
}

func TestListOptionsWithFilters(t *testing.T) {
	filters := []Filter{
		{Key: FilterKeyType, Value: "image", Operator: FilterOpEquals},
		{Key: FilterKeyPublic, Value: true, Operator: FilterOpEquals},
	}

	opts := SearchOptions(100, "photos").WithFilters(filters)

	if opts.Filters == "" {
		t.Error("opts.Filters should not be empty")
	}

	// Decode and verify
	decoded, _ := base64.StdEncoding.DecodeString(opts.Filters)
	var decodedFilters []Filter
	json.Unmarshal(decoded, &decodedFilters)

	if len(decodedFilters) != 2 {
		t.Errorf("decoded %d filters, want 2", len(decodedFilters))
	}
}

func TestFileTypeConstants(t *testing.T) {
	// Verify file type constants match expected API values
	types := map[string]string{
		"folder":      FileTypeFolder,
		"text":        FileTypeText,
		"audio":       FileTypeAudio,
		"video":       FileTypeVideo,
		"image":       FileTypeImage,
		"pdf":         FileTypePDF,
		"spreadsheet": FileTypeSpreadsheet,
		"word":        FileTypeWord,
		"photoshop":   FileTypePhotoshop,
		"archive":     FileTypeArchive,
		"powerPoint":  FileTypePowerPoint,
	}

	for expected, got := range types {
		if got != expected {
			t.Errorf("FileType constant %q = %q, want %q", got, got, expected)
		}
	}
}
