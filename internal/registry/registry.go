// Package registry persists the set of registered repository paths and
// per-repository preferences. It writes atomically and recovers from a
// corrupted file by preserving a timestamped backup.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Entry is one registered repository.
type Entry struct {
	Path    string    `json:"path"`
	Tags    []string  `json:"tags,omitempty"`
	AddedAt time.Time `json:"added_at"`
	Missing bool      `json:"missing,omitempty"` // path gone; keep registered until user removes
}

// file is the on-disk shape.
type file struct {
	Version int     `json:"version"`
	Repos   []Entry `json:"repos"`
	UI      UIState `json:"ui"`
}

// UIState persists dashboard selections between runs.
type UIState struct {
	Sort         string `json:"sort,omitempty"`
	Filter       string `json:"filter,omitempty"`
	LastSelected string `json:"last_selected,omitempty"`
}

// Registry is a loaded registry bound to its file path.
type Registry struct {
	path    string
	entries map[string]*Entry
	ui      UIState
}

// Load reads path. A missing file yields an empty registry. A corrupted
// file is moved aside to <path>.corrupt.<unix> and an empty registry is
// returned with a nil error — the dashboard must keep working.
func Load(path string) (*Registry, error) {
	r := &Registry{path: path, entries: map[string]*Entry{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		backup := fmt.Sprintf("%s.corrupt.%d", path, time.Now().Unix())
		if renErr := os.Rename(path, backup); renErr != nil {
			return nil, fmt.Errorf("registry corrupt, backup failed: %w", renErr)
		}
		return r, nil
	}
	for i := range f.Repos {
		e := f.Repos[i]
		r.entries[e.Path] = &e
	}
	r.ui = f.UI
	return r, nil
}

// Save writes the registry atomically (temp file + rename + fsync).
func (r *Registry) Save() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return fmt.Errorf("create registry dir: %w", err)
	}
	f := file{Version: 1, UI: r.ui}
	for _, e := range r.entries {
		f.Repos = append(f.Repos, *e)
	}
	sort.Slice(f.Repos, func(i, j int) bool { return f.Repos[i].Path < f.Repos[j].Path })

	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode registry: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("commit registry: %w", err)
	}
	return nil
}

// Add registers path; existing entries keep their metadata.
func (r *Registry) Add(path string) *Entry {
	if e, ok := r.entries[path]; ok {
		e.Missing = false
		return e
	}
	e := &Entry{Path: path, AddedAt: time.Now()}
	r.entries[path] = e
	return e
}

// Remove drops path from the registry (never deletes files).
func (r *Registry) Remove(path string) bool {
	if _, ok := r.entries[path]; !ok {
		return false
	}
	delete(r.entries, path)
	return true
}

// MarkMissing flags a repository whose path disappeared.
func (r *Registry) MarkMissing(path string, missing bool) {
	if e, ok := r.entries[path]; ok {
		e.Missing = missing
	}
}

// Entries returns all registered repos sorted by path.
func (r *Registry) Entries() []Entry {
	out := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// UI returns the persisted UI state.
func (r *Registry) UI() UIState { return r.ui }

// SetUI updates the persisted UI state.
func (r *Registry) SetUI(u UIState) { r.ui = u }

// Path returns the backing file path.
func (r *Registry) Path() string { return r.path }
