// Package tui implements the Bubble Tea dashboard: model, update, view.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	keyStyle   = lipgloss.NewStyle().Bold(true)
)

// Model is the root Bubble Tea model for the dashboard.
type Model struct {
	version  string
	showHelp bool
	width    int
	height   int
}

// New returns a dashboard model for the given build version.
func New(version string) Model {
	return Model{version: version}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
		case "esc":
			m.showHelp = false
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("RepoLens"))
	b.WriteString(mutedStyle.Render("  " + m.version + "  ·  0 repositories  ·  0 need attention"))
	b.WriteString("\n\n")

	if m.showHelp {
		b.WriteString(helpView())
	} else {
		b.WriteString("  No repositories registered yet.\n\n")
		b.WriteString("  RepoLens discovers Git working trees beneath configured root\n")
		b.WriteString("  directories. Registration and scanning arrive in Phase 2.\n")
		b.WriteString("  See " + keyStyle.Render("docs/repolens-spec.md") + " for the roadmap.\n")
	}

	b.WriteString("\n" + mutedStyle.Render("  ? help · q quit"))
	return b.String()
}

func helpView() string {
	rows := [][2]string{
		{"j / k", "move selection (when a list exists)"},
		{"enter", "repository details"},
		{"/", "search / filter"},
		{"a", "repositories needing attention"},
		{"d", "toggle dirty-only filter"},
		{"r", "refresh"},
		{"s", "cycle sort mode"},
		{"o", "open in configured editor"},
		{"t", "open terminal at path"},
		{"g", "open configured Git TUI"},
		{"c", "copy repository path"},
		{"?", "toggle this help"},
		{"esc", "close panel"},
		{"q", "quit"},
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s  %s\n", keyStyle.Render(fmt.Sprintf("%-8s", r[0])), r[1])
	}
	return b.String()
}
