package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	// "github.com/charmbracelet/lipgloss" // Unused for now
)

type progressMsg float64
type finishedMsg struct{ err error }

type ProgressModel struct {
	RunTask  func(p *tea.Program) error
	err      error
	TaskName string
	progress progress.Model
	Total    int64
	Current  int64
	done     bool
}

func NewProgressModel(taskName string, total int64, runTask func(*tea.Program) error) ProgressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)
	return ProgressModel{
		progress: p,
		TaskName: taskName,
		Total:    total,
		RunTask:  runTask,
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return nil
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.progress.Width = msg.Width - padding*2 - 4
		if m.progress.Width > maxWidth {
			m.progress.Width = maxWidth
		}
		return m, nil

	case progressMsg:
		var cmd tea.Cmd
		var model tea.Model
		model, cmd = m.progress.Update(msg)
		m.progress = model.(progress.Model) // Type assertion
		return m, cmd

	case finishedMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m ProgressModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("Error: %v\n", m.err)
		}
		return fmt.Sprintf("Done! %s\n", m.TaskName)
	}

	pad := strings.Repeat(" ", padding)
	return "\n" +
		pad + m.TaskName + "\n" +
		pad + m.progress.View() + "\n\n"
}

// Constants
const (
	padding  = 2
	maxWidth = 80
)

// Helper to run
func RunTransfer(taskName string, size int64, action func(send func(curr, total int64)) error) error {
	m := NewProgressModel(taskName, size, nil)
	p := tea.NewProgram(m)

	// Start task in goroutine
	go func() {
		err := action(func(curr, total int64) {
			// Calculate percentage 0.0 to 1.0
			var ratio float64
			if total > 0 {
				ratio = float64(curr) / float64(total)
			}
			p.Send(progressMsg(ratio))
		})
		p.Send(finishedMsg{err: err})
	}()

	_, err := p.Run()
	return err
}
