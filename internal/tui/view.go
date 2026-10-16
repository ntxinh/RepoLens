package tui

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/table"

	"github.com/ntxinh/RepoLens/internal/repository"
)

// View implements tea.Model.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")

	switch {
	case m.view == viewDetail:
		b.WriteString(m.detailView())
	case len(m.statuses) == 0 && !m.loading:
		b.WriteString(emptyView())
	case m.showHelp:
		b.WriteString(helpView())
	default:
		if m.searching {
			b.WriteString(m.search.View())
			b.WriteString("\n")
		}
		b.WriteString(m.table.View())
		b.WriteString("\n")
	}

	b.WriteString("\n" + m.footerView())
	return b.String()
}

func (m Model) headerView() string {
	total, attention, dirty, behind := 0, 0, 0, 0
	for _, st := range m.statuses {
		total++
		if st.NeedsAttention() {
			attention++
		}
		if st.Changes.Dirty() {
			dirty++
		}
		if st.HasCounts && st.Behind > 0 {
			behind++
		}
	}
	last := "—"
	for _, st := range m.statuses {
		if st.CheckedAt.After(mustTime(last)) {
			last = st.CheckedAt.Format("15:04:05")
		}
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render("RepoLens"))
	s.WriteString(mutedStyle.Render("  " + m.version))
	fmt.Fprintf(&s, "  %d repos", total)
	if attention > 0 {
		s.WriteString("  " + warnStyle.Render(fmt.Sprintf("%d attention", attention)))
	}
	if dirty > 0 {
		fmt.Fprintf(&s, "  %d dirty", dirty)
	}
	if behind > 0 {
		fmt.Fprintf(&s, "  %d behind", behind)
	}
	if m.loading {
		s.WriteString("  " + mutedStyle.Render("refreshing…"))
	}
	s.WriteString(mutedStyle.Render("  refresh " + last))

	var right []string
	right = append(right, "sort:"+sortNames[m.sort])
	if m.filter != filterAll {
		right = append(right, "filter:"+filterNames[m.filter])
	}
	if q := m.search.Value(); q != "" {
		right = append(right, "search:"+clean(q))
	}
	if len(right) > 0 {
		s.WriteString(mutedStyle.Render("  ·  " + strings.Join(right, " ")))
	}
	return s.String()
}

func (m Model) footerView() string {
	if m.notice != "" {
		return warnStyle.Render(m.notice)
	}
	hint := "j/k move · enter details · / search · a attention · d dirty · r refresh · R all · s sort · ? help · q quit"
	if m.view == viewDetail {
		hint = "j/k scroll · o editor · t terminal · g git tui · c copy path · esc back"
	}
	if m.searching {
		hint = "type to filter · enter/esc close search"
	}
	return mutedStyle.Render(hint)
}

func emptyView() string {
	return "\n  No repositories registered yet.\n\n" +
		"  Add discovery roots to " + keyStyle.Render("repolens config init") + " or run:\n" +
		"    " + keyStyle.Render("repolens scan <root>") + "\n" +
		"  then relaunch — or press " + keyStyle.Render("S") + " to scan configured roots now.\n"
}

// columns adapts to terminal width.
func (m Model) columns() []table.Column {
	wide := m.width >= 110
	med := m.width >= 80

	cols := []table.Column{{Title: "", Width: 2}} // status dot column
	if wide {
		return append(cols,
			table.Column{Title: "Repository", Width: 24},
			table.Column{Title: "Branch", Width: 18},
			table.Column{Title: "Upstream", Width: 18},
			table.Column{Title: "+/-", Width: 7},
			table.Column{Title: "Changes", Width: 16},
			table.Column{Title: "Last commit", Width: 10},
			table.Column{Title: "Path", Width: 30},
		)
	}
	if med {
		return append(cols,
			table.Column{Title: "Repository", Width: 24},
			table.Column{Title: "Branch", Width: 16},
			table.Column{Title: "+/-", Width: 7},
			table.Column{Title: "Changes", Width: 16},
			table.Column{Title: "Path", Width: 22},
		)
	}
	return append(cols,
		table.Column{Title: "Repository", Width: 20},
		table.Column{Title: "Branch", Width: 12},
		table.Column{Title: "+/-", Width: 7},
		table.Column{Title: "Chg", Width: 10},
	)
}

func (m Model) rowFor(st *repository.Status) table.Row {
	dot := okStyle.Render("●")
	if st.Err != nil {
		dot = errorStyle.Render("●")
	} else if st.NeedsAttention() {
		dot = warnStyle.Render("●")
	}

	upstream := "—"
	if st.HasUpstream {
		upstream = clean(st.Upstream)
	}
	ab := "—"
	if st.HasCounts {
		ab = fmt.Sprintf("+%d -%d", st.Ahead, st.Behind)
	}
	changes := changesLabel(st)
	branch := clean(st.Branch)
	if st.Detached {
		branch = "(detached)"
	}
	if st.Err != nil {
		branch = "!"
		upstream, ab, changes = "—", "—", "—"
	}
	last := "—"
	if st.LastCommit != nil {
		last = shortAgo(st.LastCommit.Timestamp)
	}

	name := clean(repoName(st))
	path := clean(shortPath(st.Path))

	wide := m.width >= 110
	med := m.width >= 80
	switch {
	case wide:
		return table.Row{dot, name, branch, upstream, ab, changes, last, path}
	case med:
		return table.Row{dot, name, branch, ab, changes, path}
	default:
		return table.Row{dot, name, branch, ab, changes}
	}
}

func changesLabel(st *repository.Status) string {
	c := st.Changes
	if !c.Dirty() {
		if st.Err == nil && st.Branch != "" {
			return "clean"
		}
		return "—"
	}
	var parts []string
	if c.Staged > 0 {
		parts = append(parts, fmt.Sprintf("S%d", c.Staged))
	}
	if c.Unstaged > 0 {
		parts = append(parts, fmt.Sprintf("M%d", c.Unstaged))
	}
	if c.Untracked > 0 {
		parts = append(parts, fmt.Sprintf("?%d", c.Untracked))
	}
	if c.Conflict > 0 {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("C%d", c.Conflict)))
	}
	return strings.Join(parts, " ")
}

func shortAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
	}
}

func shortPath(p string) string {
	if strings.HasPrefix(p, homePrefix()) {
		return "~" + p[len(homePrefix()):]
	}
	return p
}

var homeCached string

func homePrefix() string {
	if homeCached == "" {
		if h := strings.TrimRight(os.Getenv("HOME"), "/"); h != "" {
			homeCached = h
		} else {
			homeCached = "\x00" // sentinel: don't repeat lookup
		}
	}
	return homeCached
}

// clean strips control characters from repo-derived strings before they
// reach the terminal — branch names and paths are attacker-controlled.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

func mustTime(string) time.Time { return time.Time{} }

func helpView() string {
	rows := [][2]string{
		{"j / k", "move selection"},
		{"enter", "repository details"},
		{"/", "search / filter"},
		{"a", "repositories needing attention"},
		{"d", "toggle dirty-only filter"},
		{"r / R", "refresh selected / refresh all"},
		{"s", "cycle sort mode"},
		{"S", "scan configured discovery roots"},
		{"o", "open in configured editor"},
		{"t", "open terminal at path"},
		{"g", "open configured Git TUI"},
		{"c", "copy repository path"},
		{"?", "toggle this help"},
		{"esc", "close panel / clear filters"},
		{"q", "quit"},
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s  %s\n", keyStyle.Render(fmt.Sprintf("%-8s", r[0])), r[1])
	}
	return b.String()
}

// detailView renders the per-repository detail screen.
func (m Model) detailView() string {
	var b strings.Builder
	d := m.detail
	b.WriteString(titleStyle.Render("  " + clean(repoName(&d.Status))))
	b.WriteString(mutedStyle.Render("  " + clean(d.Path)))
	b.WriteString("\n\n")
	b.WriteString(m.viewport.View())
	return b.String()
}

// detailContent builds the scrollable body: status, commits, diff.
func (m Model) detailContent() string {
	d := m.detail
	st := d.Status
	var b strings.Builder

	if d.Err != nil {
		fmt.Fprintf(&b, "  %s\n\n", errorStyle.Render("error: "+d.Err.Error()))
	}
	if st.Err != nil {
		fmt.Fprintf(&b, "  %s\n\n", errorStyle.Render("status error: "+st.Err.Error()))
	}

	// -- state -------------------------------------------------------------
	branch := "(detached)"
	if !st.Detached && st.Branch != "" {
		branch = clean(st.Branch)
	}
	fmt.Fprintf(&b, "  Branch:    %s\n", branch)
	up := "no upstream"
	if st.HasUpstream {
		up = clean(st.Upstream)
	}
	fmt.Fprintf(&b, "  Upstream:  %s\n", up)
	ab := "unknown"
	if st.HasCounts {
		ab = fmt.Sprintf("+%d ahead  -%d behind", st.Ahead, st.Behind)
	}
	fmt.Fprintf(&b, "  Tracking:  %s\n", ab)
	fmt.Fprintf(&b, "  Changes:   %s\n", changesLabel(&st))
	if conds := st.Conditions(); len(conds) > 0 {
		var cs []string
		for _, c := range conds {
			cs = append(cs, string(c))
		}
		fmt.Fprintf(&b, "  Flags:     %s\n", warnStyle.Render(strings.Join(cs, ", ")))
	}
	fmt.Fprintf(&b, "  Checked:   %s\n", st.CheckedAt.Format("15:04:05"))

	// -- recent commits ----------------------------------------------------
	b.WriteString("\n  " + titleStyle.Render("Recent commits") + "\n")
	if len(d.Commits) == 0 {
		b.WriteString("    (none)\n")
	}
	for _, c := range d.Commits {
		fmt.Fprintf(&b, "    %s  %s  %s  %s\n",
			mutedStyle.Render(clean(c.Hash)),
			clean(c.Subject),
			mutedStyle.Render(clean(c.Author)),
			mutedStyle.Render(shortAgo(c.Timestamp)))
	}

	// -- diff --------------------------------------------------------------
	if d.Diff != nil {
		b.WriteString("\n  " + titleStyle.Render("Diff") + "\n")
		writeDiffSection(&b, "staged", d.Diff.Staged)
		writeDiffSection(&b, "unstaged", d.Diff.Unstaged)
		if len(d.Diff.Staged) == 0 && len(d.Diff.Unstaged) == 0 {
			b.WriteString("    working tree clean\n")
		}
		if d.Diff.Truncated {
			b.WriteString("\n  " + warnStyle.Render("⚠ diff truncated at 256 KiB") + "\n")
		}
	}
	return b.String()
}

func writeDiffSection(b *strings.Builder, label string, diff []byte) {
	if len(diff) == 0 {
		return
	}
	fmt.Fprintf(b, "\n    %s\n", mutedStyle.Render("── "+label+" ──"))
	for line := range strings.Lines(clean(string(diff))) {
		trimmed := strings.TrimRight(line, "\n")
		var styled string
		switch {
		case strings.HasPrefix(trimmed, "+") && !strings.HasPrefix(trimmed, "+++"):
			styled = okStyle.Render(trimmed)
		case strings.HasPrefix(trimmed, "-") && !strings.HasPrefix(trimmed, "---"):
			styled = errorStyle.Render(trimmed)
		case strings.HasPrefix(trimmed, "@@"):
			styled = warnStyle.Render(trimmed)
		case strings.HasPrefix(trimmed, "diff ") || strings.HasPrefix(trimmed, "index "):
			styled = mutedStyle.Render(trimmed)
		default:
			styled = trimmed
		}
		b.WriteString("    " + styled + "\n")
	}
}
