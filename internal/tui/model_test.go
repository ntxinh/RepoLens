package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEmptyStateView(t *testing.T) {
	m := New("test")
	v := m.View()
	for _, want := range []string{"RepoLens", "0 repositories", "No repositories registered"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
}

func TestQuit(t *testing.T) {
	m := New("test")
	for _, key := range []string{"q", "ctrl+c"} {
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		if key == "ctrl+c" {
			_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		}
		if cmd == nil {
			t.Fatalf("key %q: expected quit command", key)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("key %q: expected tea.QuitMsg, got %T", key, cmd())
		}
	}
}

func TestHelpToggle(t *testing.T) {
	m := New("test")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	m = updated.(Model)
	if !m.showHelp || !strings.Contains(m.View(), "move selection") {
		t.Fatal("? should show help overlay")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.showHelp {
		t.Fatal("esc should close help overlay")
	}
}

func TestWindowSizeTracked(t *testing.T) {
	m := New("test")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)
	if m.width != 120 || m.height != 40 {
		t.Fatalf("got %dx%d", m.width, m.height)
	}
}
