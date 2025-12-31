package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

func init() {
	Register(&Command{
		Name:        "search",
		Description: "Search for files with advanced filtering",
		Usage: `search [query] [flags]

Flags:
  --type <type>      Filter by file type (image, video, audio, pdf, text, folder, etc.)
  --owner <id>       Filter by owner ID
  --public           Show only files with public visibility
  --shared           Show all files shared by me (email invites + public links)
  --link             Show only files with a generated public link (Send & Track)
  --trash            Show only files in trash
  --starred          Show only starred files
  --after <date>     Show files created after date (YYYY-MM-DD, "today", "yesterday")
  --before <date>    Show files created before date
  --sort <field>     Sort by: name, size, created, updated (default: updated)
  --asc              Sort ascending
  --desc             Sort descending (default)

Examples:
  search "project" --type image
  search --shared --type pdf
  search --after 2023-01-01 --sort size`,
		Run: search,
	})
}

func search(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	fs := pflag.NewFlagSet("search", pflag.ContinueOnError)

	fileType := fs.String("type", "", "Filter by file type")
	ownerID := fs.Int64("owner", 0, "Filter by owner ID")
	public := fs.Bool("public", false, "Show only public files")
	shared := fs.Bool("shared", false, "Show files shared by me")
	link := fs.Bool("link", false, "Show files with public link")
	trash := fs.Bool("trash", false, "Show files in trash")
	starred := fs.Bool("starred", false, "Show starred files")
	after := fs.String("after", "", "Created after date")
	before := fs.String("before", "", "Created before date")
	sortBy := fs.String("sort", "updated", "Sort field")
	asc := fs.Bool("asc", false, "Sort ascending")
	desc := fs.Bool("desc", false, "Sort descending")

	fs.SetOutput(env.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	query := strings.Join(fs.Args(), " ")

	// Build filters
	var filters []api.Filter

	if *fileType != "" {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyType,
			Value:    *fileType,
			Operator: api.FilterOpEquals,
		})
	}

	if *ownerID != 0 {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyOwnerID,
			Value:    *ownerID,
			Operator: api.FilterOpEquals,
		})
	}

	if *public {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyPublic,
			Value:    true,
			Operator: api.FilterOpEquals,
		})
	}

	if *shared {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeySharedByMe,
			Value:    true,
			Operator: api.FilterOpEquals,
		})
	}

	if *link {
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyShareableLink,
			Value:    "*", // Value doesn't matter for 'has' usually, but '*' is safe
			Operator: api.FilterOpHas,
		})
	}

	// Date parsing
	if *after != "" {
		t, err := parseDate(*after)
		if err != nil {
			return fmt.Errorf("invalid --after date: %w", err)
		}
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyCreatedAt,
			Value:    t.Format(time.RFC3339),
			Operator: api.FilterOpGreater,
		})
	}

	if *before != "" {
		t, err := parseDate(*before)
		if err != nil {
			return fmt.Errorf("invalid --before date: %w", err)
		}
		filters = append(filters, api.Filter{
			Key:      api.FilterKeyCreatedAt,
			Value:    t.Format(time.RFC3339),
			Operator: api.FilterOpLess,
		})
	}

	// Sorting
	orderBy := "updated_at"
	switch *sortBy {
	case "name":
		orderBy = "name"
	case "size":
		orderBy = "file_size"
	case "created":
		orderBy = "created_at"
	case "updated":
		orderBy = "updated_at"
	}

	orderDir := "desc"
	if *asc {
		orderDir = "asc"
	} else if *desc {
		orderDir = "desc"
	}

	// Use the new helper for search options
	opts := api.SearchOptions(s.WorkspaceID, query).
		WithFilters(filters).
		WithOrder(orderBy, orderDir)
	if *trash {
		opts.WithDeletedOnly()
	}
	if *starred {
		opts.WithStarredOnly()
	}

	entries, err := s.Client.SearchWithOptions(ctx, query, opts)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Fprintln(env.Stdout, "No results found.")
		return nil
	}

	// Render results
	// Size | Owner | Date | Name

	// Calculate widths
	maxSize := 4  // "Size"
	maxOwner := 5 // "Owner"
	maxDate := 12 // "MMM DD HH:MM"

	rows := make([]struct {
		size, owner, date, name string
	}, len(entries))

	for i, e := range entries {
		size := ui.FormatSize(e.Size)
		if len(size) > maxSize {
			maxSize = len(size)
		}

		owner := e.Owner()
		if owner == "" {
			owner = "-"
		}
		if len(owner) > maxOwner {
			maxOwner = len(owner)
		}

		date := e.UpdatedAt.Format("Jan 02 15:04")
		if len(date) > maxDate {
			maxDate = len(date)
		}

		rows[i] = struct{ size, owner, date, name string }{size, owner, date, e.Name}
	}

	// Print header
	fmt.Fprintf(env.Stdout, "%-*s  %-*s  %-*s  %s\n", maxSize, "Size", maxOwner, "Owner", maxDate, "Updated", "Name")

	// Print rows
	for _, r := range rows {
		fmt.Fprintf(env.Stdout, "%-*s  %-*s  %-*s  %s\n",
			maxSize, r.size,
			maxOwner, r.owner,
			maxDate, r.date,
			r.name)
	}

	return nil
}

func parseDate(input string) (time.Time, error) {
	now := time.Now()
	switch strings.ToLower(input) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local), nil
	case "yesterday":
		return time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, time.Local), nil
	}
	// Try YYYY-MM-DD
	t, err := time.Parse("2006-01-02", input)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, input)
}
