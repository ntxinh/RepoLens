package git

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecentCommits(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "log"))
	writeFile(t, dir, "second.txt", "two\n")
	gitAt(t, dir, "add", "second.txt")
	gitAt(t, dir, "commit", "-qm", "second commit")

	commits, err := runner(t).RecentCommits(context.Background(), dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Subject != "second commit" || commits[1].Subject != "initial" {
		t.Fatalf("order wrong: %+v", commits)
	}
	if commits[0].Author != "RepoLens Test" {
		t.Fatalf("author: %q", commits[0].Author)
	}
}
func TestRecentCommitsEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "init", "-q")
	commits, err := runner(t).RecentCommits(context.Background(), dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected no commits, got %v", commits)
	}
}

func TestDiff(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "diff"))

	writeFile(t, dir, "unstaged.txt", "u\n")       // untracked — no diff entry
	writeFile(t, dir, "tracked.txt", "unstaged\n") // unstaged change

	d, err := runner(t).Diff(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Unstaged) == 0 || !strings.Contains(string(d.Unstaged), "+unstaged") {
		t.Fatalf("expected unstaged diff:\n%s", d.Unstaged)
	}
	if len(d.Staged) != 0 {
		t.Fatalf("expected empty staged diff:\n%s", d.Staged)
	}

	gitAt(t, dir, "add", "tracked.txt")
	d, err = runner(t).Diff(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Staged) == 0 || !strings.Contains(string(d.Staged), "+unstaged") {
		t.Fatalf("expected staged diff:\n%s", d.Staged)
	}
}
