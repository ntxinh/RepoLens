// Package tui implements the Bubble Tea dashboard: model, update, view.
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ntxinh/RepoLens/internal/git"
	"github.com/ntxinh/RepoLens/internal/launcher"
	"github.com/ntxinh/RepoLens/internal/repository"
	"github.com/ntxinh/RepoLens/internal/service"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true)
	mutedStyle  = lipgloss.NewStyle().Faint(true)
	keyStyle    = lipgloss.NewStyle().Bold(true)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	headerStyle = lipgloss.NewStyle().Faint(true)
)

// sortMode cycles with the 's' key.
type sortMode int

const (
	sortAttention sortMode = iota
	sortName
	sortBranch
	sortAheadBehind
	sortLastCommit
	sortPath
)

var sortNames = map[sortMode]string{
	sortAttention:   "attention",
	sortName:        "name",
	sortBranch:      "branch",
	sortAheadBehind: "ahead/behind",
	sortLastCommit:  "last commit",
	sortPath:        "path",
}

var sortOrder = []sortMode{sortAttention, sortName, sortBranch, sortAheadBehind, sortLastCommit, sortPath}

type filterMode int

const (
	filterAll filterMode = iota
	filterAttention
	filterDirty
)

var filterNames = map[filterMode]string{
	filterAll:       "all",
	filterAttention: "attention",
	filterDirty:     "dirty",
}

// Messages flowing from the service layer into the model.

// StatusesLoaded carries a fresh batch of per-repo statuses.
type StatusesLoaded []repository.Status

// StatusMsg delivers one repository's refreshed status.
type StatusMsg struct{ Status repository.Status }

// ScanFinished reports registry scan completion.
type ScanFinished struct{ Count int }

// DetailMsg carries the data for the detail view.
type DetailMsg struct {
	Path    string
	Status  repository.Status
	Commits []git.Commit
	Diff    *git.DiffResult
	Err     error
}

// LaunchResult reports a launch attempt outcome.
type LaunchResult struct{ Err error }

// viewID identifies the current screen.
type viewID int

const (
	viewList viewID = iota
	viewDetail
)

// Model is the root Bubble Tea model for the dashboard.
type Model struct {
	version string
	svc     *service.Service

	table  table.Model
	search textinput.Model

	statuses map[string]*repository.Status // keyed by path
	order    []string                      // paths, current display order

	sort      sortMode
	filter    filterMode
	searching bool
	showHelp  bool
	loading   bool
	notice    string // transient footer message (launch errors etc.)

	view     viewID
	detail   DetailMsg
	viewport viewport.Model

	width  int
	height int
}

// New returns a dashboard model. svc may be nil (shows empty state).
func New(version string, svc *service.Service) Model {
	t := table.New(
		table.WithFocused(true),
		table.WithHeight(10),
	)
	t.SetStyles(tableStyles())

	ti := textinput.New()
	ti.Prompt = "/"
	ti.CharLimit = 128

	return Model{
		version:  version,
		svc:      svc,
		table:    t,
		search:   ti,
		statuses: map[string]*repository.Status{},
		sort:     sortAttention,
		filter:   filterAll,
	}
}

func tableStyles() table.Styles {
	s := table.DefaultStyles()
	s.Selected = s.Selected.Bold(true).Foreground(lipgloss.Color("12"))
	return s
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	m.loading = true
	return m.refreshAllCmd()
}

// refreshAllCmd refreshes every registered repo; statuses stream back
// individually via statusCmd.
func (m Model) refreshAllCmd() tea.Cmd {
	if m.svc == nil {
		return nil
	}
	entries := m.svc.Reg.Entries()
	if len(entries) == 0 {
		// Still mark refresh finished so the UI knows there's nothing pending.
		return func() tea.Msg { return StatusesLoaded(nil) }
	}
	cmds := make([]tea.Cmd, len(entries))
	for i, e := range entries {
		path := e.Path
		cmds[i] = func() tea.Msg {
			return StatusMsg{Status: m.svc.Refresh(context.Background(), path)}
		}
	}
	return tea.Batch(cmds...)
}

// refreshSelectedCmd refreshes only the repo under the cursor.
func (m Model) refreshSelectedCmd() tea.Cmd {
	if m.svc == nil || len(m.order) == 0 {
		return nil
	}
	row := m.table.Cursor()
	if row < 0 || row >= len(m.order) {
		return nil
	}
	path := m.order[row]
	return func() tea.Msg {
		return StatusMsg{Status: m.svc.Refresh(context.Background(), path)}
	}
}

// scanCmd runs discovery + registration.
func (m Model) scanCmd() tea.Cmd {
	return func() tea.Msg {
		n, err := m.svc.ScanRoots(context.Background(), nil)
		if err != nil {
			return err
		}
		return ScanFinished{Count: n}
	}
}

// detailCmd loads commits + diff for the selected repository.
func (m Model) detailCmd(path string) tea.Cmd {
	return func() tea.Msg {
		d := DetailMsg{Path: path}
		if st, ok := m.statuses[path]; ok {
			d.Status = *st
		} else {
			d.Status = repository.Status{Path: path}
		}
		commits, err := m.svc.Runner.RecentCommits(context.Background(), path, 10)
		if err != nil {
			d.Err = err
			return d
		}
		d.Commits = commits
		diff, err := m.svc.Runner.Diff(context.Background(), path)
		if err != nil {
			d.Err = err
			return d
		}
		d.Diff = diff
		return d
	}
}

// selectedPath returns the repo path under the cursor, "" if none.
func (m Model) selectedPath() string {
	row := m.table.Cursor()
	if row < 0 || row >= len(m.order) {
		return ""
	}
	return m.order[row]
}

// launchDetached spawns a detached tool (editor, terminal, file manager).
func (m Model) launchDetached(cmd []string) tea.Cmd {
	path := m.selectedPath()
	if path == "" || len(cmd) == 0 {
		return nil
	}
	return func() tea.Msg {
		return LaunchResult{Err: launcher.Launch(cmd, path)}
	}
}

// launchInteractive builds a tea.ExecProcess for TUI tools (Git TUI,
// worktree manager) so they inherit the terminal, then resume.
func (m Model) launchInteractive(cmd []string) tea.Cmd {
	path := m.selectedPath()
	if path == "" || len(cmd) == 0 {
		return nil
	}
	c, err := launcher.Command(cmd, path)
	if err != nil {
		return func() tea.Msg { return LaunchResult{Err: err} }
	}
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return LaunchResult{Err: err}
	})
}
