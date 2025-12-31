package commands

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mikael.mansson2/drime-shell/internal/api"
)

// ConflictResolution represents the user's choice for handling a file conflict
type ConflictResolution int

const (
	ResolutionOverwrite ConflictResolution = iota
	ResolutionKeepBoth
	ResolutionSkip
)

// ResolveConflict prompts the user to resolve a file conflict
// Returns the new name (if renamed), whether to proceed, and error
func ResolveConflict(ctx context.Context, client api.DrimeClient, workspaceID int64, parentID *int64, filename string) (string, bool, error) {
	// If not interactive (e.g. piped input), we might want a default.
	// For now, assume interactive.

	// We use bubbletea for the prompt
	p := tea.NewProgram(newConflictModel(filename))
	m, err := p.Run()
	if err != nil {
		return "", false, err
	}

	model := m.(conflictModel)
	if model.canceled {
		return "", false, fmt.Errorf("operation canceled")
	}

	switch model.choice {
	case ResolutionOverwrite:
		return filename, true, nil
	case ResolutionSkip:
		return "", false, nil
	case ResolutionKeepBoth:
		// Call API to get available name
		req := api.GetAvailableNameRequest{
			Name:        filename,
			ParentID:    parentID,
			WorkspaceID: workspaceID,
		}
		resp, err := client.GetAvailableName(ctx, req)
		if err != nil {
			return "", false, fmt.Errorf("failed to get available name: %w", err)
		}
		return resp.Available, true, nil
	default:
		return "", false, fmt.Errorf("unknown choice")
	}
}

// Bubbletea model for conflict prompt

type item struct {
	title, desc string
	choice      ConflictResolution
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type conflictModel struct {
	list     list.Model
	choice   ConflictResolution
	canceled bool
	filename string
}

func newConflictModel(filename string) conflictModel {
	items := []list.Item{
		item{title: "Replace existing file", desc: "This will upload a new version of the file", choice: ResolutionOverwrite},
		item{title: "Keep both files", desc: "A number will be added to the filename", choice: ResolutionKeepBoth},
		item{title: "Skip file", desc: "File will not be uploaded", choice: ResolutionSkip},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = fmt.Sprintf("Duplicate File Found: %s already exists in this location.", filename)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetHeight(14) // Adjust height

	return conflictModel{
		list:     l,
		filename: filename,
	}
}

func (m conflictModel) Init() tea.Cmd {
	return nil
}

func (m conflictModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q", "esc":
			m.canceled = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.choice
				return m, tea.Quit
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m conflictModel) View() string {
	return "\n" + m.list.View()
}

// checkCollisionsAndResolve checks for duplicates and resolves them interactively.
// It returns a map of original filename -> new filename (or empty string if skipped).
// If overwrite is chosen, new filename is same as original.
func checkCollisionsAndResolve(ctx context.Context, client api.DrimeClient, workspaceID int64, parentID *int64, destPath string, sources []string) (map[string]string, error) {
	return checkCollisionsAndResolveWithPolicy(ctx, client, workspaceID, parentID, destPath, sources, "ask")
}

// DuplicatePolicy specifies how to handle duplicate files
type DuplicatePolicy string

const (
	DuplicatePolicyAsk     DuplicatePolicy = "ask"
	DuplicatePolicyReplace DuplicatePolicy = "replace"
	DuplicatePolicyRename  DuplicatePolicy = "rename"
	DuplicatePolicySkip    DuplicatePolicy = "skip"
)

func checkCollisionsAndResolveWithPolicy(ctx context.Context, client api.DrimeClient, workspaceID int64, parentID *int64, destPath string, sources []string, policy string) (map[string]string, error) {
	// 1. Validate
	var files []api.ValidateFile
	for _, src := range sources {
		name := filepath.Base(src)
		files = append(files, api.ValidateFile{
			Name:         name,
			Size:         0, // Size doesn't matter for duplicate check
			RelativePath: destPath,
		})
	}

	req := api.ValidateRequest{
		Files:       files,
		WorkspaceID: workspaceID,
	}

	resp, err := client.ValidateEntries(ctx, req)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	// Initialize all as same name
	for _, src := range sources {
		result[filepath.Base(src)] = filepath.Base(src)
	}

	// If no duplicates, return
	if len(resp.Duplicates) == 0 {
		return result, nil
	}

	// 2. Resolve duplicates based on policy
	duplicatesSet := make(map[string]bool)
	for _, d := range resp.Duplicates {
		base := filepath.Base(d)
		duplicatesSet[base] = true
	}

	for _, src := range sources {
		name := filepath.Base(src)
		if duplicatesSet[name] {
			var newName string
			var proceed bool

			switch policy {
			case "replace":
				// Replace existing - use same name
				newName = name
				proceed = true
			case "skip":
				// Skip this file
				proceed = false
			case "rename":
				// Get a new unique name from the API
				req := api.GetAvailableNameRequest{
					Name:        name,
					ParentID:    parentID,
					WorkspaceID: workspaceID,
				}
				availResp, err := client.GetAvailableName(ctx, req)
				if err != nil {
					return nil, fmt.Errorf("failed to get available name for %s: %w", name, err)
				}
				newName = availResp.Available
				proceed = true
			default: // "ask"
				var err error
				newName, proceed, err = ResolveConflict(ctx, client, workspaceID, parentID, name)
				if err != nil {
					return nil, err
				}
			}

			if !proceed {
				delete(result, name)
			} else {
				result[name] = newName
			}
		}
	}

	return result, nil
}
