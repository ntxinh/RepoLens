package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/ntxinh/RepoLens/internal/git"
)

func snapWith(files []git.FileEntry) *git.Snapshot {
	return &git.Snapshot{Path: "/x", Branch: "main", Files: files}
}

func TestFromSnapshotCounts(t *testing.T) {
	snap := snapWith([]git.FileEntry{
		{Kind: '1', X: 'M', Y: '.'}, // staged modify
		{Kind: '1', X: '.', Y: 'M'}, // unstaged modify
		{Kind: '1', X: 'M', Y: 'M'}, // both
		{Kind: '2', X: 'R', Y: '.'}, // rename (staged)
		{Kind: 'u', X: 'U', Y: 'U'}, // conflict
		{Kind: '?'},                 // untracked
		{Kind: '?'},                 // untracked
	})
	s := FromSnapshot(snap, time.Now())
	c := s.Changes
	if c.Staged != 3 { // M. + MM + R.
		t.Errorf("staged = %d, want 3", c.Staged)
	}
	if c.Unstaged != 2 { // .M + MM
		t.Errorf("unstaged = %d, want 2", c.Unstaged)
	}
	if c.Untracked != 2 {
		t.Errorf("untracked = %d, want 2", c.Untracked)
	}
	if c.Conflict != 1 {
		t.Errorf("conflict = %d, want 1", c.Conflict)
	}
	if c.Renamed != 1 {
		t.Errorf("renamed = %d, want 1", c.Renamed)
	}
	if !c.Dirty() {
		t.Error("expected dirty")
	}
}

func TestConditionsAndAttention(t *testing.T) {
	errStatus := Status{Err: errors.New("boom")}
	if !errStatus.NeedsAttention() {
		t.Error("error status should need attention")
	}
	conds := errStatus.Conditions()
	if len(conds) != 1 || conds[0] != CondError {
		t.Fatalf("got %v", conds)
	}

	noUpstream := Status{Branch: "main"}
	conds = noUpstream.Conditions()
	if len(conds) != 1 || conds[0] != CondNoUpstream {
		t.Fatalf("no-upstream should be informational only, got %v", conds)
	}
	if noUpstream.NeedsAttention() {
		t.Error("no-upstream alone is informational, not attention")
	}

	multi := Status{
		Detached:  true,
		HasCounts: true, Ahead: 2, Behind: 3,
		Changes: Changes{Staged: 1, Untracked: 2, Conflict: 1},
	}
	conds = multi.Conditions()
	want := []Condition{CondConflicts, CondModified, CondUntracked, CondDetached, CondAhead, CondBehind}
	if len(conds) != len(want) {
		t.Fatalf("got %v want %v", conds, want)
	}
	for i := range want {
		if conds[i] != want[i] {
			t.Fatalf("cond[%d] = %q want %q (all: %v)", i, conds[i], want[i], conds)
		}
	}
	if !multi.NeedsAttention() {
		t.Error("dirty+ahead should need attention")
	}
}

func TestAheadBehindUnknownVsZero(t *testing.T) {
	// Spec §8.2: missing upstream and unavailable counts are not 0/0.
	unknown := Status{HasUpstream: true, HasCounts: false}
	for _, c := range unknown.Conditions() {
		if c == CondAhead || c == CondBehind {
			t.Fatal("unknown counts must not emit ahead/behind conditions")
		}
	}
	zero := Status{HasUpstream: true, HasCounts: true, Ahead: 0, Behind: 0}
	for _, c := range zero.Conditions() {
		if c == CondAhead || c == CondBehind {
			t.Fatal("0/0 must not emit ahead/behind conditions")
		}
	}
}
