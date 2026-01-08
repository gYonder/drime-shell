package commands

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
	"github.com/spf13/pflag"
)

func init() {
	Register(&Command{
		Name:        "share",
		Description: "Manage sharing and links",
		Usage: `share [command] <file> [options]

Commands:
  share ls [options]            List shared files
  share link <file> [options]   Create/manage shareable link (default if no subcommand)
  share invite <file> <email>   Invite users via email

List Options:
  --by-me           List files shared by me (default)
  --with-me         List files shared with me
  --public          List files with public links

Link Options:
  -d, --delete      Delete the shareable link
  -p, --password    Set a password
  -e, --expire      Set expiration (e.g. 24h, 30m)
  -r, --role        Permission level: view, edit, download (default: download)
  --copy            Copy link to clipboard (default: true)

Info: Running 'share <file>' on a file that already has a link will display it.

Invite Options:
  --role            Permission level: view, edit, download (default: download)

Examples:
  share ls --with-me              List files shared with me
  share file.txt                  Create/show public link (downloadable)
  share file.txt --role view      Create view-only public link
  share invite file.txt user@example.com --role edit`,
		Run: share,
	})
}

func share(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "ls", "list":
			return shareList(ctx, s, env, args[1:])
		case "invite":
			return shareInvite(ctx, s, env, args[1:])
		case "link":
			return shareLink(ctx, s, env, args[1:])
		}
	}

	// Default to "link" subcommand behavior
	return shareLink(ctx, s, env, args)
}

func shareInvite(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	flags := pflag.NewFlagSet("share invite", pflag.ContinueOnError)
	role := flags.String("role", "view", "Permission level: view, edit, download")
	flags.SetOutput(env.Stderr)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() < 2 {
		return fmt.Errorf("usage: share invite <file> <email>... [--role view|edit|download]")
	}

	path := flags.Arg(0)
	emails := flags.Args()[1:]

	resolvedPath := s.ResolvePath(path)
	entry, ok := s.Cache.Get(resolvedPath)
	if !ok {
		return fmt.Errorf("file not found: %s", path)
	}

	// Validate role
	validRoles := map[string]bool{"view": true, "edit": true, "download": true}
	if !validRoles[*role] {
		return fmt.Errorf("invalid role: %s (must be view, edit, or download)", *role)
	}

	// Prepare permissions array (one per email, or same for all?)
	// API takes array of permissions. Usually parallel to emails or single permission applied to all?
	// The OpenAPI says:
	// emails: [string]
	// permissions: [string]
	// It's likely parallel arrays or if length 1, applied to all?
	// Let's assume we apply the same role to all emails.
	permissions := make([]string, len(emails))
	for i := range permissions {
		permissions[i] = *role
	}

	err := ui.WithSpinnerErr(env.Stderr, "Sending invitations...", false, func() error {
		return s.Client.ShareEntry(ctx, entry.ID, emails, permissions)
	})
	if err != nil {
		return fmt.Errorf("failed to share: %w", err)
	}

	fmt.Fprintf(env.Stdout, "Invited %d users to %s with %s permission\n", len(emails), entry.Name, *role)
	return nil
}

func shareLink(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	flags := pflag.NewFlagSet("share link", pflag.ContinueOnError)
	deleteLink := flags.BoolP("delete", "d", false, "Delete the shareable link")
	password := flags.StringP("password", "p", "", "Set a password")
	expire := flags.StringP("expire", "e", "", "Set expiration (e.g. 24h, 30m)")
	role := flags.StringP("role", "r", "download", "Permission level: view, edit, download")
	copyLink := flags.Bool("copy", true, "Copy link to clipboard")
	flags.SetOutput(env.Stderr)

	// Reorder args to allow flags after positional arguments (Unix-style)
	args = ReorderArgsForFlags(flags, args)

	if err := flags.Parse(args); err != nil {
		return err
	}

	if flags.NArg() == 0 {
		return fmt.Errorf("usage: share [link] [options] <file>")
	}

	path := flags.Arg(0)
	resolvedPath := s.ResolvePath(path)

	// Get file entry
	entry, ok := s.Cache.Get(resolvedPath)
	if !ok {
		return fmt.Errorf("file not found: %s", path)
	}

	if *deleteLink {
		if err := s.Client.DeleteShareableLink(ctx, entry.ID); err != nil {
			return err
		}
		fmt.Fprintf(env.Stdout, "Shareable link deleted for %s\n", entry.Name)
		return nil
	}

	// Map role to permissions
	var allowEdit, allowDownload bool
	switch *role {
	case "view":
		allowEdit = false
		allowDownload = false
	case "download":
		allowEdit = false
		allowDownload = true
	case "edit":
		allowEdit = true
		allowDownload = true
	default:
		return fmt.Errorf("invalid role: %s (must be view, edit, or download)", *role)
	}

	// Prepare request
	req := api.ShareableLinkRequest{
		AllowEdit:     allowEdit,
		AllowDownload: allowDownload,
	}

	if *password != "" {
		req.Password = password
	}

	if *expire != "" {
		duration, err := time.ParseDuration(*expire)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		expTime := time.Now().Add(duration).Format(time.RFC3339)
		req.ExpiresAt = &expTime
	}

	// Check if link already exists (to display existing settings)
	var existingLink *api.ShareableLink
	existingLink, _ = ui.WithSpinner(env.Stderr, "Checking link...", false, func() (*api.ShareableLink, error) {
		return s.Client.GetShareableLink(ctx, entry.ID)
	})

	// If no changes requested and link exists, just show it
	noChanges := *password == "" && *expire == "" && *role == "download"
	if noChanges && existingLink != nil && existingLink.Hash != "" {
		url := fmt.Sprintf("https://dri.me/%s", existingLink.Hash)
		fmt.Fprintf(env.Stdout, "Shareable link: %s\n", ui.RenderLink(url))
		printLinkDetails(env.Stdout, existingLink)
		if *copyLink {
			if err := clipboard.WriteAll(url); err == nil {
				fmt.Fprintln(env.Stdout, "(copied to clipboard)")
			}
		}
		return nil
	}

	// Create or Update link
	var link *api.ShareableLink
	var err error
	if existingLink != nil && existingLink.Hash != "" {
		// Populate extra fields required for update
		req.PersonalLink = existingLink.Perso == 1
		req.PersonnalLinkValue = existingLink.Hash

		// If expiration not specified in flags, keep existing
		if req.ExpiresAt == nil && existingLink.ExpiresAt != nil {
			expTime := existingLink.ExpiresAt.Format(time.RFC3339)
			req.ExpiresAt = &expTime
		}

		link, err = ui.WithSpinner(env.Stderr, "Updating link...", false, func() (*api.ShareableLink, error) {
			return s.Client.UpdateShareableLink(ctx, entry.ID, req)
		})
	} else {
		link, err = ui.WithSpinner(env.Stderr, "Creating link...", false, func() (*api.ShareableLink, error) {
			return s.Client.CreateShareableLink(ctx, entry.ID, req)
		})
	}
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://dri.me/%s", link.Hash)
	if existingLink != nil && existingLink.Hash != "" {
		fmt.Fprintf(env.Stdout, "Updated shareable link: %s\n", ui.RenderLink(url))
	} else {
		fmt.Fprintf(env.Stdout, "Shareable link: %s\n", ui.RenderLink(url))
	}
	printLinkDetails(env.Stdout, link)

	if *copyLink {
		if err := clipboard.WriteAll(url); err == nil {
			fmt.Fprintln(env.Stdout, "(copied to clipboard)")
		}
	}

	return nil
}

func printLinkDetails(w io.Writer, link *api.ShareableLink) {
	if link == nil {
		return
	}
	var details []string
	if link.AllowEdit {
		details = append(details, "edit")
	} else if link.AllowDownload {
		details = append(details, "download")
	} else {
		details = append(details, "view-only")
	}
	if link.Password != nil && *link.Password != "" {
		details = append(details, "password-protected")
	}
	if link.ExpiresAt != nil {
		details = append(details, fmt.Sprintf("expires %s", link.ExpiresAt.Format("Jan 02 15:04")))
	}
	if len(details) > 0 {
		fmt.Fprintf(w, "  (%s)\n", strings.Join(details, ", "))
	}
}

func shareList(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	flags := pflag.NewFlagSet("share ls", pflag.ContinueOnError)
	byMe := flags.Bool("by-me", false, "List files shared by me")
	withMe := flags.Bool("with-me", false, "List files shared with me")
	public := flags.Bool("public", false, "List files with public links")
	flags.SetOutput(env.Stderr)

	if err := flags.Parse(args); err != nil {
		return err
	}

	// Default to "by-me" if nothing specified
	if !*byMe && !*withMe && !*public {
		*byMe = true
	}

	var entries []api.FileEntry
	var err error

	if *byMe {
		// "Shared by me" means:
		// 1. Files shared with specific users (sharedByMe=true)
		// 2. Files I own that have a public link (shareableLink=* AND owner_id=me)
		// We fetch both and merge them.

		user, err := s.Client.Whoami(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}

		entries, err = ui.WithSpinner(env.Stdout, "Fetching shared files...", false, func() ([]api.FileEntry, error) {
			var wg sync.WaitGroup
			var mu sync.Mutex
			var allEntries []api.FileEntry
			var firstErr error

			runQuery := func(f []api.Filter) {
				defer wg.Done()
				opts := api.ListOptions(s.WorkspaceID).WithFilters(f)
				res, err := s.Client.ListByParentIDWithOptions(ctx, nil, opts)

				mu.Lock()
				defer mu.Unlock()
				if err != nil && firstErr == nil {
					firstErr = err
					return
				}
				if err == nil {
					allEntries = append(allEntries, res...)
				}
			}

			// Base filters from other flags
			var baseFilters []api.Filter
			if *public {
				baseFilters = append(baseFilters, api.Filter{Key: api.FilterKeyShareableLink, Value: "*", Operator: api.FilterOpHas})
			}
			if *withMe {
				baseFilters = append(baseFilters, api.Filter{Key: api.FilterKeyOwnerID, Value: user.ID, Operator: api.FilterOpNotEq})
			}

			// Query 1: Shared with users
			f1 := append([]api.Filter{}, baseFilters...)
			f1 = append(f1, api.Filter{Key: api.FilterKeySharedByMe, Value: true, Operator: api.FilterOpEquals})

			wg.Add(1)
			go runQuery(f1)

			// Query 2: My public links (only if not excluded by withMe)
			if !*withMe {
				f2 := append([]api.Filter{}, baseFilters...)
				f2 = append(f2, api.Filter{Key: api.FilterKeyShareableLink, Value: "*", Operator: api.FilterOpHas})
				f2 = append(f2, api.Filter{Key: api.FilterKeyOwnerID, Value: user.ID, Operator: api.FilterOpEquals})
				wg.Add(1)
				go runQuery(f2)
			}

			wg.Wait()
			if firstErr != nil {
				return nil, firstErr
			}

			// Dedup
			seen := make(map[int64]bool)
			var deduped []api.FileEntry
			for _, e := range allEntries {
				if !seen[e.ID] {
					deduped = append(deduped, e)
					seen[e.ID] = true
				}
			}
			return deduped, nil
		})
		if err != nil {
			return fmt.Errorf("failed to list shared files: %w", err)
		}

	} else {
		var filters []api.Filter

		if *public {
			filters = append(filters, api.Filter{
				Key:      api.FilterKeyShareableLink,
				Value:    "*",
				Operator: api.FilterOpHas,
			})
		}

		if *withMe {
			user, err := s.Client.Whoami(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current user: %w", err)
			}
			filters = append(filters, api.Filter{
				Key:      api.FilterKeyOwnerID,
				Value:    user.ID,
				Operator: api.FilterOpNotEq,
			})
		}

		opts := api.ListOptions(s.WorkspaceID).WithFilters(filters)

		entries, err = ui.WithSpinner(env.Stdout, "Fetching shared files...", false, func() ([]api.FileEntry, error) {
			return s.Client.ListByParentIDWithOptions(ctx, nil, opts)
		})
		if err != nil {
			return fmt.Errorf("failed to list shared files: %w", err)
		}
	}

	if len(entries) == 0 {
		fmt.Fprintln(env.Stdout, "No shared files found.")
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("ID"),
		ui.HeaderStyle.Render("NAME"),
		ui.HeaderStyle.Render("SIZE"),
		ui.HeaderStyle.Render("UPDATED"),
	)

	for _, entry := range entries {
		t.AddRow(
			ui.MutedStyle.Render(fmt.Sprintf("#%d", entry.ID)),
			ui.StyleName(entry.Name, entry.Type),
			ui.FormatSize(entry.Size),
			entry.UpdatedAt.Format("Jan 02 15:04"),
		)
	}
	t.Render()

	return nil
}
