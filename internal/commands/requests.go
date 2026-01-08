package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/gYonder/drime-shell/internal/api"
	"github.com/gYonder/drime-shell/internal/session"
	"github.com/gYonder/drime-shell/internal/ui"
)

func init() {
	Registry["request"] = requestCommand
	Registry["req"] = requestCommand
}

var requestCommand = &Command{
	Name:        "request",
	Description: "Manage file requests",
	Run: func(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
		if len(args) == 0 {
			return requestListCommand.Run(ctx, s, env, args)
		}

		subcmd := args[0]
		switch subcmd {
		case "ls", "list":
			return requestListCommand.Run(ctx, s, env, args[1:])
		case "create", "add", "new":
			return requestCreateCommand.Run(ctx, s, env, args[1:])
		case "rm", "remove", "delete", "del":
			return requestRemoveCommand.Run(ctx, s, env, args[1:])
		default:
			return fmt.Errorf("unknown subcommand: %s", subcmd)
		}
	},
}

var requestListCommand = &Command{
	Name:        "ls",
	Description: "List active file requests",
	Run: func(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
		requests, err := s.Client.ListFileRequests(ctx)
		if err != nil {
			return err
		}

		if len(requests) == 0 {
			fmt.Fprintln(env.Stdout, "No active file requests found.")
			return nil
		}

		// Create table
		headers := []string{"ID", "Name", "Folder", "Link", "Uploads", "Expires"}
		rows := make([][]string, len(requests))

		for i, req := range requests {
			expires := "-"
			if req.ExpiresAt != nil {
				expires = req.ExpiresAt.Format("2006-01-02")
			}

			link := fmt.Sprintf("https://dri.me/%s", req.ShareHash)

			rows[i] = []string{
				fmt.Sprintf("%d", req.ID),
				req.Title,
				req.FileName,
				link,
				fmt.Sprintf("%d", req.UploadsCount),
				expires,
			}
		}

		t := ui.NewTable(env.Stdout)
		t.SetHeaders(headers...)
		for _, row := range rows {
			t.AddRow(row...)
		}
		t.Render()
		return nil
	},
}

var requestCreateCommand = &Command{
	Name:        "create",
	Description: "Create a new file request",
	Run: func(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: request create <folder> [flags]")
		}

		folderPath := args[0]

		// Parse flags
		var title, desc, expires, password, customLink string

		// Simple flag parsing
		for i := 1; i < len(args); i++ {
			arg := args[i]
			if strings.HasPrefix(arg, "--title=") {
				title = strings.TrimPrefix(arg, "--title=")
			} else if arg == "--title" && i+1 < len(args) {
				title = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--desc=") {
				desc = strings.TrimPrefix(arg, "--desc=")
			} else if arg == "--desc" && i+1 < len(args) {
				desc = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--expire=") {
				expires = strings.TrimPrefix(arg, "--expire=")
			} else if arg == "--expire" && i+1 < len(args) {
				expires = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--password=") {
				password = strings.TrimPrefix(arg, "--password=")
			} else if arg == "--password" && i+1 < len(args) {
				password = args[i+1]
				i++
			} else if strings.HasPrefix(arg, "--custom-link=") {
				customLink = strings.TrimPrefix(arg, "--custom-link=")
			} else if arg == "--custom-link" && i+1 < len(args) {
				customLink = args[i+1]
				i++
			}
		}

		// Resolve folder
		resolvedPath := s.ResolvePath(folderPath)
		entry, ok := s.Cache.Get(resolvedPath)
		if !ok {
			// Try to stat it remotely if not in cache
			// For now, just error if not found in cache/tree
			return fmt.Errorf("folder not found: %s", folderPath)
		}

		if entry.Type != "folder" {
			return fmt.Errorf("path is not a folder: %s", folderPath)
		}

		// Default title to folder name if not provided
		if title == "" {
			title = entry.Name
		}

		// Step 1: Create the request
		fmt.Fprintf(env.Stdout, "Creating file request for '%s'...\n", entry.Name)
		link, err := s.Client.CreateFileRequest(ctx, entry.ID, title, desc)
		if err != nil {
			return err
		}

		// Step 2: Update with options if needed
		needsUpdate := false
		updateReq := api.ShareableLinkRequest{
			AllowDownload: false,
			AllowEdit:     false,
		}

		if expires != "" {
			// Parse date
			t, err := time.Parse("2006-01-02", expires)
			if err != nil {
				return fmt.Errorf("invalid date format (use YYYY-MM-DD): %w", err)
			}
			// Set to end of day
			t = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
			iso := t.Format(time.RFC3339)
			updateReq.ExpiresAt = &iso
			needsUpdate = true
		}

		if password != "" {
			updateReq.Password = &password
			needsUpdate = true
		}

		if customLink != "" {
			updateReq.PersonalLink = true
			updateReq.PersonnalLinkValue = customLink
			needsUpdate = true
		}

		if needsUpdate {
			fmt.Fprintln(env.Stdout, "Applying security settings...")
			// Re-attach the request payload as it might be required by the backend validation
			// although typically updates only need the changed fields.
			// Based on HAR, the update call didn't include the 'request' object, just the settings.

			updatedLink, err := s.Client.UpdateShareableLink(ctx, entry.ID, updateReq)
			if err != nil {
				return fmt.Errorf("request created but failed to update settings: %w", err)
			}
			link = updatedLink
		}

		// Output result
		url := fmt.Sprintf("https://dri.me/%s", link.Hash)
		if link.PersonnalLinkValue != "" {
			url = fmt.Sprintf("https://dri.me/%s", link.PersonnalLinkValue)
		}

		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
		fmt.Fprintf(env.Stdout, "%s File request created successfully!\n", successStyle.Render("✔"))
		fmt.Fprintf(env.Stdout, "Link: %s\n", url)

		return nil
	},
}

var requestRemoveCommand = &Command{
	Name:        "rm",
	Description: "Remove a file request",
	Run: func(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("usage: request rm <id>")
		}

		idStr := args[0]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid request ID: %s", idStr)
		}

		err = s.Client.DeleteFileRequest(ctx, id)
		if err != nil {
			return err
		}

		fmt.Fprintf(env.Stdout, "File request %d deleted.\n", id)
		return nil
	},
}
