package commands

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mikael.mansson2/drime-shell/internal/api"
	"github.com/mikael.mansson2/drime-shell/internal/session"
	"github.com/mikael.mansson2/drime-shell/internal/ui"
)

func init() {
	Register(&Command{
		Name:        "ws",
		Description: "List or manage workspaces",
		Usage: `ws [command] [args]

List all workspaces:
  ws                   Show available workspaces

Switch to a workspace:
  ws <name>            Switch to workspace by name
  ws <id>              Switch to workspace by ID
  ws 0                 Switch to default workspace
  ws default           Switch to default workspace

Create/manage workspaces:
  ws new <name>        Create a new workspace
  ws rename <name>     Rename the current workspace
  ws rm [name|id]      Delete a workspace (defaults to current, requires confirmation)

Member Management:
  ws members           List members and pending invites
  ws roles             List available roles
  ws invite <email> [role] Invite a user (default role: Member)
  ws kick <email>      Remove a member or cancel an invite
  ws role <email> <role> Change a member's role
  ws leave             Leave the current workspace`,
		Run: wsCmd,
	})
}

func wsCmd(ctx context.Context, s *session.Session, env *ExecutionEnv, args []string) error {
	if len(args) == 0 {
		return listWorkspaces(ctx, s, env)
	}

	// Check for subcommands
	switch strings.ToLower(args[0]) {
	case "new", "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: ws new <name>")
		}
		return createWorkspace(ctx, s, env, strings.Join(args[1:], " "))
	case "rename":
		if len(args) < 2 {
			return fmt.Errorf("usage: ws rename <name>")
		}
		return renameWorkspace(ctx, s, env, strings.Join(args[1:], " "))
	case "rm", "delete":
		var targetID int64 = s.WorkspaceID
		if len(args) > 1 {
			wsID, _, err := ResolveWorkspace(ctx, s, args[1])
			if err != nil {
				return fmt.Errorf("ws rm: %v", err)
			}
			targetID = wsID
		}
		return deleteWorkspace(ctx, s, env, targetID)
	case "members":
		return listWorkspaceMembers(ctx, s, env)
	case "roles":
		return listWorkspaceRoles(ctx, s, env)
	case "invite":
		if len(args) < 2 {
			return fmt.Errorf("usage: ws invite <email> [role]")
		}
		role := "Member"
		if len(args) > 2 {
			role = args[2]
		}
		return inviteMember(ctx, s, env, args[1], role)
	case "kick", "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: ws kick <email>")
		}
		return kickMember(ctx, s, env, args[1])
	case "role":
		if len(args) < 3 {
			return fmt.Errorf("usage: ws role <email> <role>")
		}
		return changeMemberRole(ctx, s, env, args[1], args[2])
	case "leave":
		return leaveWorkspace(ctx, s, env)
	default:
		return switchWorkspace(ctx, s, env, args[0])
	}
}

// ResolveWorkspace resolves a workspace identifier (name or ID) to workspace ID and name.
// Accepts: numeric ID, workspace name (case-insensitive), "default" or "0" for personal workspace.
// Returns: (workspaceID, workspaceName, error)
func ResolveWorkspace(ctx context.Context, s *session.Session, target string) (int64, string, error) {
	// Handle default workspace
	if strings.EqualFold(target, "default") || target == "0" {
		return 0, "default", nil
	}

	// Try to parse as numeric ID first
	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		// Find name in cache
		for _, ws := range s.Workspaces {
			if ws.ID == id {
				return id, ws.Name, nil
			}
		}
		// Not in cache, try to refresh
		workspaces, err := s.Client.GetWorkspaces(ctx)
		if err == nil {
			s.Workspaces = workspaces
			for _, ws := range workspaces {
				if ws.ID == id {
					return id, ws.Name, nil
				}
			}
		}
		// Still not found - use ID anyway (API will validate)
		return id, "", nil
	}

	// Try to find by name (case-insensitive)
	targetLower := strings.ToLower(target)
	for _, ws := range s.Workspaces {
		if strings.ToLower(ws.Name) == targetLower {
			return ws.ID, ws.Name, nil
		}
	}

	// Not in cache, try to refresh
	workspaces, err := s.Client.GetWorkspaces(ctx)
	if err == nil {
		s.Workspaces = workspaces
		for _, ws := range workspaces {
			if strings.ToLower(ws.Name) == targetLower {
				return ws.ID, ws.Name, nil
			}
		}
	}

	return 0, "", fmt.Errorf("workspace '%s' not found", target)
}

func listWorkspaces(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	// Fetch workspaces from API (with caching)
	workspaces, err := ui.WithSpinner(env.Stdout, "", false, func() ([]api.Workspace, error) {
		return s.Client.GetWorkspaces(ctx)
	})
	if err != nil {
		return fmt.Errorf("failed to fetch workspaces: %w", err)
	}

	// Cache the workspaces
	s.Workspaces = workspaces

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("ID"),
		ui.HeaderStyle.Render("NAME"),
		ui.HeaderStyle.Render("STATUS"),
	)

	// Default workspace (always ID 0)
	defaultMarker := ""
	if s.WorkspaceID == 0 {
		defaultMarker = ui.StarStyle.Render("← active")
	}
	t.AddRow(ui.MutedStyle.Render("0"), ui.DirStyle.Render("default"), defaultMarker)

	// Other workspaces
	for _, ws := range workspaces {
		marker := ""
		if ws.ID == s.WorkspaceID {
			marker = ui.StarStyle.Render("← active")
		}
		t.AddRow(ui.MutedStyle.Render(fmt.Sprintf("%d", ws.ID)), ui.DirStyle.Render(ws.Name), marker)
	}
	t.Render()

	return nil
}

func switchWorkspace(ctx context.Context, s *session.Session, env *ExecutionEnv, target string) error {
	wasInVault := s.InVault

	// Resolve workspace by name or ID
	targetWsID, targetWsName, err := ResolveWorkspace(ctx, s, target)
	if err != nil {
		return err
	}

	// Skip if already on this workspace (and not switching from vault)
	if targetWsID == s.WorkspaceID && !wasInVault {
		if targetWsID == 0 {
			fmt.Fprintln(env.Stdout, "Already on default workspace")
		} else {
			fmt.Fprintf(env.Stdout, "Already on workspace '%s'\n", targetWsName)
		}
		return nil
	}

	// Switch workspace: clear cache and reload folder tree
	err = ui.WithSpinnerErr(env.Stderr, "", false, func() error {
		newCache := api.NewFileCache()
		if err := newCache.LoadFolderTree(ctx, s.Client, s.UserID, s.Username, targetWsID); err != nil {
			return fmt.Errorf("failed to load folder tree: %w", err)
		}

		// Prefetch root directory
		entries, err := s.Client.ListByParentIDWithOptions(ctx, nil, api.ListOptions(targetWsID))
		if err == nil {
			newCache.AddChildren("/", entries)
		}

		// Swap in new cache + state only after successful load
		s.Cache = newCache
		s.WorkspaceID = targetWsID
		s.WorkspaceName = targetWsName
		s.CWD = "/"
		s.PreviousDir = ""
		s.InVault = false // Ensure we're out of vault mode

		return nil
	})
	if err != nil {
		return err
	}

	// Display switch message with stats
	if targetWsID == 0 {
		fmt.Fprintln(env.Stdout, "Switched to default workspace")
	} else {
		fmt.Fprintf(env.Stdout, "Switched to workspace '%s'\n", ui.WorkspaceStyle.Render(targetWsName))
		// Fetch and display workspace stats with spinner (skip for default workspace - too slow)
		stats, err := ui.WithSpinner(env.Stderr, "", false, func() (*api.WorkspaceStats, error) {
			return s.Client.GetWorkspaceStats(ctx, targetWsID)
		})
		if err == nil && stats != nil {
			fmt.Fprintf(env.Stdout, "%s\n",
				ui.MutedStyle.Render(fmt.Sprintf("  %d files, %s", stats.Files, formatSize(stats.Size))))
		}
	}

	return nil
}

func createWorkspace(ctx context.Context, s *session.Session, env *ExecutionEnv, name string) error {
	if name == "" {
		return fmt.Errorf("workspace name is required")
	}

	ws, err := ui.WithSpinner(env.Stdout, "", false, func() (*api.Workspace, error) {
		return s.Client.CreateWorkspace(ctx, name)
	})
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Update cached workspaces list
	s.Workspaces = append(s.Workspaces, *ws)

	fmt.Fprintf(env.Stdout, "%s Created workspace '%s' (ID: %d)\n",
		ui.SuccessStyle.Render("✓"),
		ui.WorkspaceStyle.Render(ws.Name),
		ws.ID)
	fmt.Fprintf(env.Stdout, "%s\n", ui.MutedStyle.Render("Use 'ws "+ws.Name+"' to switch to it"))

	return nil
}

func renameWorkspace(ctx context.Context, s *session.Session, env *ExecutionEnv, newName string) error {
	if s.WorkspaceID == 0 {
		return fmt.Errorf("cannot rename the default workspace")
	}

	if newName == "" {
		return fmt.Errorf("new name is required")
	}

	ws, err := ui.WithSpinner(env.Stdout, "", false, func() (*api.Workspace, error) {
		return s.Client.UpdateWorkspace(ctx, s.WorkspaceID, newName)
	})
	if err != nil {
		return fmt.Errorf("failed to rename workspace: %w", err)
	}

	oldName := s.WorkspaceName
	s.WorkspaceName = ws.Name

	// Update cached workspaces list
	for i := range s.Workspaces {
		if s.Workspaces[i].ID == s.WorkspaceID {
			s.Workspaces[i].Name = ws.Name
			break
		}
	}

	fmt.Fprintf(env.Stdout, "%s Renamed workspace '%s' → '%s'\n",
		ui.SuccessStyle.Render("✓"),
		oldName,
		ui.WorkspaceStyle.Render(ws.Name))

	return nil
}

func deleteWorkspace(ctx context.Context, s *session.Session, env *ExecutionEnv, workspaceID int64) error {
	if workspaceID == 0 {
		return fmt.Errorf("cannot delete the default workspace")
	}

	// Find workspace name for confirmation
	var wsName string
	for _, ws := range s.Workspaces {
		if ws.ID == workspaceID {
			wsName = ws.Name
			break
		}
	}

	if wsName == "" {
		// Try to fetch from API
		workspaces, err := s.Client.GetWorkspaces(ctx)
		if err == nil {
			s.Workspaces = workspaces
			for _, ws := range workspaces {
				if ws.ID == workspaceID {
					wsName = ws.Name
					break
				}
			}
		}
		if wsName == "" {
			return fmt.Errorf("workspace with ID %d not found", workspaceID)
		}
	}

	// Require confirmation
	fmt.Fprintf(env.Stdout, "%s This will permanently delete workspace '%s' (ID: %d) and all its contents.\n",
		ui.WarningStyle.Render("⚠"),
		wsName,
		workspaceID)
	fmt.Fprintf(env.Stdout, "Type '%s' to confirm: ", ui.ErrorStyle.Render(wsName))

	reader := bufio.NewReader(env.Stdin)
	confirmation, _ := reader.ReadString('\n')
	confirmation = strings.TrimSpace(confirmation)

	if confirmation != wsName {
		fmt.Fprintln(env.Stdout, "Deletion cancelled.")
		return nil
	}

	err := ui.WithSpinnerErr(env.Stdout, "", false, func() error {
		if err := s.Client.DeleteWorkspace(ctx, workspaceID); err != nil {
			return err
		}

		// Remove from cached list
		newWorkspaces := make([]api.Workspace, 0, len(s.Workspaces))
		for _, ws := range s.Workspaces {
			if ws.ID != workspaceID {
				newWorkspaces = append(newWorkspaces, ws)
			}
		}
		s.Workspaces = newWorkspaces

		// If we deleted the current workspace, switch to default
		if s.WorkspaceID == workspaceID {
			s.WorkspaceID = 0
			s.WorkspaceName = ""
			s.CWD = "/"
			s.PreviousDir = ""

			// Reload cache for default workspace
			s.Cache = api.NewFileCache()
			_ = s.Cache.LoadFolderTree(ctx, s.Client, s.UserID, s.Username, 0)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	fmt.Fprintf(env.Stdout, "%s Deleted workspace '%s'\n",
		ui.SuccessStyle.Render("✓"),
		wsName)

	return nil
}

func listWorkspaceMembers(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	if s.WorkspaceID == 0 {
		return fmt.Errorf("cannot list members of personal workspace")
	}

	ws, err := s.Client.GetWorkspace(ctx, s.WorkspaceID)
	if err != nil {
		return err
	}

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("NAME"),
		ui.HeaderStyle.Render("EMAIL"),
		ui.HeaderStyle.Render("ROLE"),
		ui.HeaderStyle.Render("STATUS"),
	)

	// List members
	for _, m := range ws.Members {
		name := m.DisplayName
		if name == "" {
			name = m.FirstName + " " + m.LastName
		}
		if strings.TrimSpace(name) == "" {
			name = m.Email
		}
		role := m.RoleName
		if m.IsOwner {
			role += " (Owner)"
		}
		t.AddRow(name, m.Email, role, "Active")
	}

	// List invites
	for _, i := range ws.Invites {
		t.AddRow("-", i.Email, i.RoleName, "Pending")
	}

	t.Render()
	return nil
}

func listWorkspaceRoles(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	roles, err := s.Client.GetWorkspaceRoles(ctx)
	if err != nil {
		return err
	}

	t := ui.NewTable(env.Stdout)
	t.SetHeaders(
		ui.HeaderStyle.Render("ID"),
		ui.HeaderStyle.Render("NAME"),
		ui.HeaderStyle.Render("DESCRIPTION"),
	)
	for _, r := range roles {
		t.AddRow(fmt.Sprintf("%d", r.ID), r.Name, r.Description)
	}
	t.Render()
	return nil
}

func inviteMember(ctx context.Context, s *session.Session, env *ExecutionEnv, email, roleName string) error {
	if s.WorkspaceID == 0 {
		return fmt.Errorf("cannot invite to personal workspace")
	}

	roleID, err := resolveRoleID(ctx, s, roleName)
	if err != nil {
		return err
	}

	if err := s.Client.InviteMember(ctx, s.WorkspaceID, []string{email}, roleID); err != nil {
		return err
	}

	fmt.Fprintf(env.Stdout, "Invited %s as %s\n", email, roleName)
	return nil
}

func kickMember(ctx context.Context, s *session.Session, env *ExecutionEnv, target string) error {
	if s.WorkspaceID == 0 {
		return fmt.Errorf("cannot kick from personal workspace")
	}

	ws, err := s.Client.GetWorkspace(ctx, s.WorkspaceID)
	if err != nil {
		return err
	}

	// Check invites first
	for _, i := range ws.Invites {
		if strings.EqualFold(i.Email, target) {
			if err := s.Client.CancelInvite(ctx, i.ID); err != nil {
				return err
			}
			fmt.Fprintf(env.Stdout, "Cancelled invite for %s\n", target)
			return nil
		}
	}

	// Check members
	for _, m := range ws.Members {
		if strings.EqualFold(m.Email, target) || fmt.Sprintf("%d", m.MemberID) == target {
			if m.IsOwner {
				return fmt.Errorf("cannot kick the owner")
			}
			if m.MemberID == s.UserID {
				return fmt.Errorf("cannot kick yourself (use 'ws leave')")
			}

			// Confirm
			fmt.Fprintf(env.Stdout, "Are you sure you want to remove %s from the workspace? [y/N] ", m.Email)
			reader := bufio.NewReader(env.Stdin)
			response, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(response)) != "y" {
				fmt.Fprintln(env.Stdout, "Cancelled")
				return nil
			}

			if err := s.Client.RemoveMember(ctx, s.WorkspaceID, m.MemberID); err != nil {
				return err
			}
			fmt.Fprintf(env.Stdout, "Removed %s\n", m.Email)
			return nil
		}
	}

	return fmt.Errorf("member or invite not found: %s", target)
}

func changeMemberRole(ctx context.Context, s *session.Session, env *ExecutionEnv, target, roleName string) error {
	if s.WorkspaceID == 0 {
		return fmt.Errorf("cannot change roles in personal workspace")
	}

	roleID, err := resolveRoleID(ctx, s, roleName)
	if err != nil {
		return err
	}

	ws, err := s.Client.GetWorkspace(ctx, s.WorkspaceID)
	if err != nil {
		return err
	}

	// Check invites
	for _, i := range ws.Invites {
		if strings.EqualFold(i.Email, target) {
			if err := s.Client.ChangeMemberRole(ctx, s.WorkspaceID, i.ID, roleID, true); err != nil {
				return err
			}
			fmt.Fprintf(env.Stdout, "Changed role for invite %s to %s\n", target, roleName)
			return nil
		}
	}

	// Check members
	for _, m := range ws.Members {
		if strings.EqualFold(m.Email, target) || fmt.Sprintf("%d", m.MemberID) == target {
			if err := s.Client.ChangeMemberRole(ctx, s.WorkspaceID, m.MemberID, roleID, false); err != nil {
				return err
			}
			fmt.Fprintf(env.Stdout, "Changed role for %s to %s\n", m.Email, roleName)
			return nil
		}
	}

	return fmt.Errorf("member or invite not found: %s", target)
}

func leaveWorkspace(ctx context.Context, s *session.Session, env *ExecutionEnv) error {
	if s.WorkspaceID == 0 {
		return fmt.Errorf("cannot leave personal workspace")
	}

	ws, err := s.Client.GetWorkspace(ctx, s.WorkspaceID)
	if err != nil {
		return err
	}

	// Find current user member ID
	var memberID int64
	for _, m := range ws.Members {
		if m.MemberID == s.UserID {
			memberID = m.MemberID
			if m.IsOwner {
				return fmt.Errorf("owner cannot leave workspace (delete it or transfer ownership via web app)")
			}
			break
		}
	}

	if memberID == 0 {
		return fmt.Errorf("you are not a member of this workspace")
	}

	fmt.Fprintf(env.Stdout, "Are you sure you want to leave workspace '%s'? [y/N] ", ws.Name)
	reader := bufio.NewReader(env.Stdin)
	response, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Fprintln(env.Stdout, "Cancelled")
		return nil
	}

	if err := s.Client.RemoveMember(ctx, s.WorkspaceID, memberID); err != nil {
		return err
	}

	fmt.Fprintln(env.Stdout, "Left workspace")
	return switchWorkspace(ctx, s, env, "0")
}

func resolveRoleID(ctx context.Context, s *session.Session, roleName string) (int, error) {
	roles, err := s.Client.GetWorkspaceRoles(ctx)
	if err != nil {
		return 0, err
	}

	for _, r := range roles {
		if strings.EqualFold(r.Name, roleName) || strings.EqualFold(r.Name, "Workspace "+roleName) {
			return r.ID, nil
		}
		if fmt.Sprintf("%d", r.ID) == roleName {
			return r.ID, nil
		}
	}

	return 0, fmt.Errorf("role not found: %s (use 'ws roles' to list)", roleName)
}
