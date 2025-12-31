package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// EditorKeyMap defines the keybindings for the editor
type EditorKeyMap struct {
	Save key.Binding
	Quit key.Binding
	Help key.Binding
}

// DefaultEditorKeyMap returns the default keybindings (nano-like)
func DefaultEditorKeyMap() EditorKeyMap {
	return EditorKeyMap{
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("^S", "save"),
		),
		Quit: key.NewBinding(
			key.WithKeys("ctrl+q", "ctrl+x"),
			key.WithHelp("^Q/^X", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("ctrl+g"),
			key.WithHelp("^G", "help"),
		),
	}
}

// ShortHelp returns keybindings to show in short help view
func (k EditorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Save, k.Quit, k.Help}
}

// FullHelp returns keybindings for the expanded help view
func (k EditorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Save, k.Quit, k.Help},
	}
}

// EditorResult contains the result of the editor session
type EditorResult struct {
	Content  string
	Filename string
	Saved    bool
}

// EditorModel is the bubbletea model for the editor
type EditorModel struct {
	textarea    textarea.Model
	help        help.Model
	statusStyle lipgloss.Style
	helpStyle   lipgloss.Style
	keymap      EditorKeyMap
	filename    string
	original    string // Original content when opened
	lastSaved   string // Content at last save (for "Modified" tracking)
	width       int
	height      int
	saved       bool // User pressed save at least once
	quitting    bool
	showHelp    bool
}

// NewEditor creates a new editor model
func NewEditor(filename, content string) EditorModel {
	ta := textarea.New()
	ta.SetValue(content)
	ta.Focus()
	ta.ShowLineNumbers = true
	ta.KeyMap.InsertNewline.SetEnabled(true)

	// Style the textarea
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))

	h := help.New()
	h.ShowAll = false

	return EditorModel{
		textarea:    ta,
		help:        h,
		keymap:      DefaultEditorKeyMap(),
		filename:    filename,
		original:    content,
		lastSaved:   content,
		saved:       false,
		statusStyle: lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Padding(0, 1),
		helpStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("241")),
	}
}

// Init implements tea.Model
func (m EditorModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model
func (m EditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve space for status bar (1 line) and help bar (1 line)
		m.textarea.SetWidth(msg.Width)
		m.textarea.SetHeight(msg.Height - 3)
		m.help.Width = msg.Width

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keymap.Save):
			// Mark as saved and update lastSaved content
			m.saved = true
			m.lastSaved = m.textarea.Value()
			return m, nil

		case key.Matches(msg, m.keymap.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, m.keymap.Help):
			m.showHelp = !m.showHelp
			m.help.ShowAll = m.showHelp
			return m, nil
		}
	}

	// Update textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m EditorModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Main editor area
	b.WriteString(m.textarea.View())
	b.WriteString("\n")

	// Status bar
	status := m.filename
	currentContent := m.textarea.Value()
	modifiedSinceLastSave := currentContent != m.lastSaved

	if modifiedSinceLastSave {
		status += " [Modified]"
	} else if m.saved {
		status += " [Saved]"
	}

	// Position info
	line := m.textarea.Line() + 1
	col := m.textarea.LineInfo().ColumnOffset + 1
	pos := fmt.Sprintf(" Ln %d, Col %d ", line, col)

	// Build status bar with file on left, position on right
	statusWidth := m.width - lipgloss.Width(pos) - 2
	if statusWidth < 0 {
		statusWidth = 0
	}
	paddedStatus := lipgloss.NewStyle().Width(statusWidth).Render(status)
	fullStatus := m.statusStyle.Render(paddedStatus + pos)
	b.WriteString(fullStatus)
	b.WriteString("\n")

	// Help bar
	helpView := m.helpStyle.Render(m.help.View(m.keymap))
	b.WriteString(helpView)

	return b.String()
}

// Result returns the editor result after quitting
func (m EditorModel) Result() EditorResult {
	return EditorResult{
		Content:  m.textarea.Value(),
		Saved:    m.saved,
		Filename: m.filename,
	}
}

// Modified returns true if content differs from original
func (m EditorModel) Modified() bool {
	return m.textarea.Value() != m.original
}

// RunEditor opens the editor in fullscreen and returns the result
func RunEditor(filename, content string) (EditorResult, error) {
	model := NewEditor(filename, content)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return EditorResult{}, err
	}

	m, ok := finalModel.(EditorModel)
	if !ok {
		return EditorResult{}, fmt.Errorf("unexpected model type")
	}

	return m.Result(), nil
}
