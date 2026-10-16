// Package repository defines domain types and status semantics.
// It consumes git.Snapshot values and owns the counts and labels the UI
// renders — no subprocesses live here.
package repository

import (
	"time"

	"github.com/ntxinh/RepoLens/internal/git"
)

// Condition flags a single reason a repository needs attention.
// Multiple conditions can hold at once; the UI renders them as readable
// labels rather than collapsing to one color.
type Condition string

const (
	CondConflicts   Condition = "conflicts"
	CondModified    Condition = "modified"
	CondUntracked   Condition = "untracked"
	CondAhead       Condition = "ahead"
	CondBehind      Condition = "behind"
	CondDetached    Condition = "detached HEAD"
	CondUnavailable Condition = "unavailable"
	CondError       Condition = "error"
	CondStale       Condition = "stale data"
	CondNoUpstream  Condition = "no upstream"
)

// Changes holds per-kind working-tree counts derived from a snapshot.
type Changes struct {
	Staged    int
	Unstaged  int
	Untracked int
	Conflict  int
	Renamed   int
}

// Dirty reports whether the working tree has any change at all.
func (c Changes) Dirty() bool {
	return c.Staged+c.Unstaged+c.Untracked+c.Conflict > 0
}

// Status is the per-repository aggregate the dashboard renders.
type Status struct {
	Path        string
	Name        string
	Branch      string // "(detached)"-style short label handled by callers
	Detached    bool
	Upstream    string
	Ahead       int
	Behind      int
	HasUpstream bool
	HasCounts   bool // false → render "unknown", not 0/0
	Changes     Changes
	LastCommit  *git.Commit
	CheckedAt   time.Time
	Stale       bool  // cached value shown before a fresh refresh completes
	Err         error // per-repository failure; nil on success
}

// Conditions returns every attention condition that holds, in display order.
func (s Status) Conditions() []Condition {
	var out []Condition
	if s.Err != nil {
		out = append(out, CondError)
	}
	if s.Changes.Conflict > 0 {
		out = append(out, CondConflicts)
	}
	if s.Changes.Staged+s.Changes.Unstaged > 0 {
		out = append(out, CondModified)
	}
	if s.Changes.Untracked > 0 {
		out = append(out, CondUntracked)
	}
	if s.Detached {
		out = append(out, CondDetached)
	}
	if s.HasCounts && s.Ahead > 0 {
		out = append(out, CondAhead)
	}
	if s.HasCounts && s.Behind > 0 {
		out = append(out, CondBehind)
	}
	if !s.HasUpstream && !s.Detached && s.Branch != "" {
		out = append(out, CondNoUpstream)
	}
	if s.Stale {
		out = append(out, CondStale)
	}
	return out
}

// NeedsAttention reports whether any condition besides informational
// no-upstream holds.
func (s Status) NeedsAttention() bool {
	for _, c := range s.Conditions() {
		if c != CondNoUpstream {
			return true
		}
	}
	return false
}

// FromSnapshot builds a Status from a raw git snapshot.
func FromSnapshot(snap *git.Snapshot, checkedAt time.Time) Status {
	s := Status{
		Path:        snap.Path,
		Branch:      snap.Branch,
		Detached:    snap.Detached,
		Upstream:    snap.Upstream,
		Ahead:       snap.Ahead,
		Behind:      snap.Behind,
		HasUpstream: snap.HasUpstream,
		HasCounts:   snap.HasCounts,
		LastCommit:  snap.LastCommit,
		CheckedAt:   checkedAt,
	}
	for _, f := range snap.Files {
		switch {
		case f.Kind == '?':
			s.Changes.Untracked++
		case f.Kind == 'u':
			s.Changes.Conflict++
		case f.Kind == '2':
			s.Changes.Renamed++
			applyXY(&s.Changes, f.X, f.Y)
		default:
			applyXY(&s.Changes, f.X, f.Y)
		}
	}
	return s
}

// applyXY folds porcelain staged/unstaged columns into counts.
// 'M','A','D','R','C','T','B' count; '.' and ' ' do not.
func applyXY(c *Changes, x, y byte) {
	if x != '.' && x != ' ' && x != '?' && x != '!' {
		c.Staged++
	}
	if y != '.' && y != ' ' && y != '?' && y != '!' {
		c.Unstaged++
	}
}
