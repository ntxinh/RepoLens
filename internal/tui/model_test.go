package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ntxinh/RepoLens/internal/git"
	"github.com/ntxinh/RepoLens/internal/repository"
)

func newModel() Model {
	return New("test", nil)
}

func statusAt(path string) repository.Status {
	return repository.Status{Path: path, Branch: "main", CheckedAt: time.Now()}
}

func TestEmptyStateView(t *testing.T) {
	m := newModel()
	v := m.View()
	for _, want := range []string{"RepoLens", "No repositories registered", "repolens scan"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestQuit(t *testing.T) {
	m := newModel()
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
	} {
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("key %v: expected quit command", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("key %v: expected tea.QuitMsg, got %T", key, cmd())
		}
	}
}

func TestHelpToggle(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp {
		t.Fatal("? should show help")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showHelp {
		t.Fatal("esc should close help")
	}
}

func TestStatusRowsRender(t *testing.T) {
	m := newModel()
	m.width, m.height = 120, 30
	dirty := statusAt("/home/u/projects/dirty-repo")
	dirty.Changes = repository.Changes{Unstaged: 3, Untracked: 1}
	dirty.HasUpstream = true
	dirty.HasCounts = true
	dirty.Ahead = 2
	dirty.LastCommit = &git.Commit{Hash: "abc1234", Subject: "wip", Timestamp: time.Now().Add(-2 * time.Hour)}

	clean := statusAt("/home/u/projects/clean-repo")
	clean.HasUpstream = true
	clean.HasCounts = true

	m.statuses["/home/u/projects/dirty-repo"] = &dirty
	m.statuses["/home/u/projects/clean-repo"] = &clean
	m.rebuild()

	if len(m.order) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(m.order))
	}
	// attention sort: dirty first
	if m.order[0] != "/home/u/projects/dirty-repo" {
		t.Fatalf("attention sort should put dirty first, got %v", m.order)
	}
	v := m.View()
	for _, want := range []string{"dirty-repo", "clean-repo", "+2 -0", "M3 ?1"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestFilters(t *testing.T) {
	m := newModel()
	dirty := statusAt("/d")
	dirty.Changes = repository.Changes{Unstaged: 1}
	clean := statusAt("/c")
	clean.HasUpstream = true
	clean.HasCounts = true
	m.statuses["/d"], m.statuses["/c"] = &dirty, &clean

	m.filter = filterDirty
	m.rebuild()
	if len(m.order) != 1 || m.order[0] != "/d" {
		t.Fatalf("dirty filter got %v", m.order)
	}

	m.filter = filterAttention
	m.rebuild()
	if len(m.order) != 1 || m.order[0] != "/d" {
		t.Fatalf("attention filter got %v", m.order)
	}

	m.filter = filterAll
	m.rebuild()
	if len(m.order) != 2 {
		t.Fatalf("all filter got %v", m.order)
	}
}

func TestSearchFilter(t *testing.T) {
	m := newModel()
	a := statusAt("/alpha/beta-repo")
	b := statusAt("/alpha/gamma-repo")
	m.statuses["/alpha/beta-repo"], m.statuses["/alpha/gamma-repo"] = &a, &b
	m.search.SetValue("beta")
	m.rebuild()
	if len(m.order) != 1 || m.order[0] != "/alpha/beta-repo" {
		t.Fatalf("search got %v", m.order)
	}
}

func TestSearchModeConsumesKeys(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	m = updated.(Model)
	if !m.searching {
		t.Fatal("/ should enter search mode")
	}
	// 'q' while searching types, not quits
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m = updated.(Model)
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q in search mode must not quit")
		}
	}
	if m.search.Value() != "q" {
		t.Fatalf("expected typed 'q', got %q", m.search.Value())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.searching || m.search.Value() != "" {
		t.Fatal("esc should exit search and clear")
	}
}

func TestSortCycle(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if m.sort != sortName {
		t.Fatalf("expected sortName after one cycle, got %v", m.sort)
	}
}

func TestErrorRepoRendersDot(t *testing.T) {
	m := newModel()
	m.width = 120
	bad := statusAt("/bad")
	bad.Err = errors.New("path missing")
	m.statuses["/bad"] = &bad
	m.rebuild()
	v := m.View()
	if !strings.Contains(v, "bad") {
		t.Fatalf("error repo should still render:\n%s", v)
	}
}

func TestCleanStripsControlChars(t *testing.T) {
	got := clean("feat\x1b[31mhack\x07\n")
	if strings.ContainsAny(got, "\x1b\x07") {
		t.Fatalf("control chars survived: %q", got)
	}
	if got != "feat[31mhack " {
		t.Fatalf("got %q", got)
	}
}

func TestWindowSizeTracked(t *testing.T) {
	m := newModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("got %dx%d", m.width, m.height)
	}
}
