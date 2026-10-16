package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ntxinh/RepoLens/internal/repository"
)

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// -- service messages -------------------------------------------------

	case StatusesLoaded:
		m.loading = false
		m.rebuild()
		return m, nil

	case StatusMsg:
		st := msg.Status
		m.statuses[st.Path] = &st
		if m.svc != nil {
			m.loading = len(m.statuses) < len(m.svc.Reg.Entries())
		}
		m.rebuild()
		return m, nil

	case ScanFinished:
		m.loading = true
		return m, m.refreshAllCmd()

	case DetailMsg:
		m.detail = msg
		m.view = viewDetail
		m.viewport.SetContent(m.detailContent())
		m.viewport.GotoTop()
		return m, nil

	case LaunchResult:
		if msg.Err != nil {
			m.notice = "launch: " + msg.Err.Error()
		} else {
			m.notice = "launched"
		}
		return m, nil

	case TickMsg:
		// Auto-refresh only when idle; a running refresh owns the queue.
		if !m.loading && m.view == viewList && !m.searching {
			m.loading = true
			return m, tea.Batch(m.refreshAllCmd(), m.tickCmd())
		}
		return m, m.tickCmd()

	case error:
		m.loading = false
		m.notice = msg.Error()
		return m, nil

	// -- window ------------------------------------------------------------

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetHeight(max(4, m.height-6))
		m.table.SetWidth(m.width)
		m.viewport.Width = m.width
		m.viewport.Height = max(4, m.height-4)
		m.rebuild()
		return m, nil

	// -- keys --------------------------------------------------------------

	case tea.KeyMsg:
		if m.searching {
			switch msg.String() {
			case "enter", "esc":
				m.searching = false
				m.search.Blur()
				if msg.String() == "esc" {
					m.search.SetValue("")
				}
				m.rebuild()
			default:
				var cmd tea.Cmd
				m.search, cmd = m.search.Update(msg)
				m.rebuild()
				return m, cmd
			}
			return m, nil
		}

		// Detail view: viewport scrolls; esc/q/enter returns to the list.
		if m.view == viewDetail {
			switch msg.String() {
			case "esc", "q", "enter":
				m.view = viewList
				return m, nil
			case "ctrl+c":
				m.persistUIState()
				return m, tea.Quit
			case "o", "t", "f", "g", "w", "c":
				return m, m.dispatchKey(msg.String())
			}
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.persistUIState()
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "esc":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
			if m.filter != filterAll || m.search.Value() != "" {
				m.filter = filterAll
				m.search.SetValue("")
				m.rebuild()
			}
			return m, nil
		case "/":
			m.searching = true
			m.search.Focus()
			return m, nil
		case "a":
			m.filter = filterAttention
			m.rebuild()
			return m, nil
		case "d":
			if m.filter == filterDirty {
				m.filter = filterAll
			} else {
				m.filter = filterDirty
			}
			m.rebuild()
			return m, nil
		case "s":
			m.sort = sortOrder[(indexOf(m.sort)+1)%len(sortOrder)]
			m.rebuild()
			return m, nil
		case "enter":
			if p := m.selectedPath(); p != "" && m.svc != nil {
				return m, m.detailCmd(p)
			}
			return m, nil
		case "o", "t", "f", "g", "w", "c":
			return m, m.dispatchKey(msg.String())
		case "r":
			return m, m.refreshSelectedCmd()
		case "R":
			m.loading = true
			return m, m.refreshAllCmd()
		case "S":
			m.loading = true
			return m, m.scanCmd()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// dispatchKey maps a key to its launch/copy action.
func (m Model) dispatchKey(key string) tea.Cmd {
	switch key {
	case "c":
		return m.copyPathCmd()
	default:
		kind := map[string]string{
			"o": "editor", "t": "terminal", "f": "file_manager",
			"g": "git_tui", "w": "worktree_manager",
		}[key]
		return m.launchCmd(kind)
	}
}

// launchCmd dispatches to detached or interactive launcher by command kind.
func (m Model) launchCmd(kind string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	cfg := m.svc.Cfg.Commands
	var cmd []string
	interactive := false
	switch kind {
	case "editor":
		cmd = cfg.Editor
	case "terminal":
		cmd = cfg.Terminal
	case "file_manager":
		cmd = cfg.FileManager
	case "git_tui":
		cmd, interactive = cfg.GitTUI, true
	case "worktree_manager":
		cmd, interactive = cfg.WorktreeManager, true
	}
	if len(cmd) == 0 {
		return func() tea.Msg {
			return LaunchResult{Err: fmt.Errorf("commands.%s not configured", kind)}
		}
	}
	if interactive {
		return m.launchInteractive(cmd)
	}
	return m.launchDetached(cmd)
}

func (m Model) copyPathCmd() tea.Cmd {
	path := m.selectedPath()
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		return LaunchResult{Err: clipboard.WriteAll(path)}
	}
}

func indexOf(s sortMode) int {
	for i, v := range sortOrder {
		if v == s {
			return i
		}
	}
	return 0
}

// rebuild sorts, filters, and re-renders the table from m.statuses,
// preserving the selected repository when it still exists.
func (m *Model) rebuild() {
	prevPath := m.selectedPath()

	paths := make([]string, 0, len(m.statuses))
	for p := range m.statuses {
		paths = append(paths, p)
	}
	m.sortPaths(paths)

	query := strings.ToLower(m.search.Value())
	var rows []table.Row
	m.order = m.order[:0]
	for _, p := range paths {
		st := m.statuses[p]
		if !m.passes(st, query) {
			continue
		}
		m.order = append(m.order, p)
		rows = append(rows, m.rowFor(st))
	}
	m.table.SetColumns(m.columns())
	m.table.SetRows(rows)

	// Restore selection: the table clamps cursor to -1 on empty row sets
	// and never recovers, so we always re-derive it.
	target := 0
	for i, p := range m.order {
		if p == prevPath {
			target = i
			break
		}
	}
	if len(m.order) > 0 {
		m.table.SetCursor(target)
	}
}

func (m *Model) passes(st *repository.Status, query string) bool {
	switch m.filter {
	case filterAttention:
		if !st.NeedsAttention() {
			return false
		}
	case filterDirty:
		if !st.Changes.Dirty() {
			return false
		}
	}
	if query != "" {
		name := strings.ToLower(repoName(st))
		if !strings.Contains(name, query) &&
			!strings.Contains(strings.ToLower(st.Branch), query) &&
			!strings.Contains(strings.ToLower(st.Path), query) {
			return false
		}
	}
	return true
}

func (m *Model) sortPaths(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		a, b := m.statuses[paths[i]], m.statuses[paths[j]]
		switch m.sort {
		case sortDirty:
			if a.Changes.Dirty() != b.Changes.Dirty() {
				return a.Changes.Dirty()
			}
			return repoName(a) < repoName(b)
		case sortName:
			return repoName(a) < repoName(b)
		case sortBranch:
			return a.Branch < b.Branch
		case sortAheadBehind:
			return (a.Ahead + a.Behind) > (b.Ahead + b.Behind)
		case sortLastCommit:
			return lastCommitTime(a).After(lastCommitTime(b))
		case sortPath:
			return a.Path < b.Path
		default: // attention: needs-attention first, then name
			if a.NeedsAttention() != b.NeedsAttention() {
				return a.NeedsAttention()
			}
			return repoName(a) < repoName(b)
		}
	})
}

func lastCommitTime(s *repository.Status) time.Time {
	if s.LastCommit != nil {
		return s.LastCommit.Timestamp
	}
	return time.Time{}
}

func repoName(s *repository.Status) string {
	if i := strings.LastIndexByte(s.Path, '/'); i >= 0 {
		return s.Path[i+1:]
	}
	return s.Path
}
