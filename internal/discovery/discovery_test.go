package discovery

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ntxinh/RepoLens/internal/git"
)

func mkRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init %s: %v\n%s", dir, err, out)
	}
}

func scanner(t *testing.T) *Scanner {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	return NewScanner(git.NewRunner(10*time.Second, 4))
}

func TestScanFindsRepos(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, filepath.Join(root, "alpha"))
	mkRepo(t, filepath.Join(root, "deep", "level2", "beta"))
	os.MkdirAll(filepath.Join(root, "plain"), 0o755) // not a repo

	repos, err := scanner(t).Scan(context.Background(), root, Options{MaxDepth: 4, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	got := paths(repos)
	want := []string{filepath.Join(root, "alpha"), filepath.Join(root, "deep", "level2", "beta")}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestScanRespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, filepath.Join(root, "a", "b", "c", "repo")) // depth 4 (find semantics)

	opts := Options{MaxDepth: 3, Hidden: true}
	repos, err := scanner(t).Scan(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Fatalf("depth 3 should find nothing, got %v", paths(repos))
	}
	opts.MaxDepth = 4
	repos, err = scanner(t).Scan(context.Background(), root, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("depth 4 should find repo, got %v", paths(repos))
	}
}

func TestScanExcludes(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, filepath.Join(root, "keep"))
	mkRepo(t, filepath.Join(root, "node_modules", "dep"))
	mkRepo(t, filepath.Join(root, "vendor", "vendored"))

	repos, err := scanner(t).Scan(context.Background(), root, Options{
		MaxDepth: 4, Hidden: true,
		Excludes: []string{"**/node_modules/**", "**/vendor/**"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := paths(repos)
	if len(got) != 1 || got[0] != filepath.Join(root, "keep") {
		t.Fatalf("got %v", got)
	}
}

func TestScanHiddenDirs(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, filepath.Join(root, ".hidden", "repo"))
	mkRepo(t, filepath.Join(root, "visible"))

	repos, _ := scanner(t).Scan(context.Background(), root, Options{MaxDepth: 4, Hidden: false})
	if len(repos) != 1 || filepath.Base(repos[0].Path) != "visible" {
		t.Fatalf("hidden=false got %v", paths(repos))
	}
	repos, _ = scanner(t).Scan(context.Background(), root, Options{MaxDepth: 4, Hidden: true})
	if len(repos) != 2 {
		t.Fatalf("hidden=true got %v", paths(repos))
	}
}

func TestScanSkipsGitInternals(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	mkRepo(t, repoDir)
	// A .git dir is never a worktree; ensure walk doesn't loop into it.
	repos, err := scanner(t).Scan(context.Background(), root, Options{MaxDepth: 5, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].Path != repoDir {
		t.Fatalf("got %v", paths(repos))
	}
}

func TestScanDeduplicates(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	mkRepo(t, repo)
	// symlink root→root2 doesn't matter; nested discovery reaching the
	// same toplevel twice (e.g. via submodule dir) must dedupe.
	repos, err := scanner(t).Scan(context.Background(), root, Options{MaxDepth: 4, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 {
		t.Fatalf("got %v", paths(repos))
	}
}

func TestScanCancellation(t *testing.T) {
	root := t.TempDir()
	for i := range 50 {
		mkRepo(t, filepath.Join(root, "repo"+string(rune('a'+i%26))+string(rune('a'+i/26))))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	repos, err := scanner(t).Scan(ctx, root, Options{MaxDepth: 2, Hidden: true})
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if len(repos) > 50 {
		t.Fatal("found more than exist")
	}
	_ = repos
}

func TestScanNestedRepos(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "outer")
	mkRepo(t, outer)
	inner := filepath.Join(outer, "nested", "inner")
	mkRepo(t, inner)

	repos, err := scanner(t).Scan(context.Background(), root, Options{MaxDepth: 4, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	got := paths(repos)
	if len(got) != 2 {
		t.Fatalf("nested repos both expected, got %v", got)
	}
}

func TestLinkedWorktreeDiscovered(t *testing.T) {
	root := t.TempDir()
	main_ := filepath.Join(root, "main")
	mkRepo(t, main_)
	wt := filepath.Join(root, "linked")
	cmd := exec.Command("git", "-C", main_, "worktree", "add", "-q", wt)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %v\n%s", err, out)
	}
	repos, err := scanner(t).Scan(context.Background(), root, Options{MaxDepth: 3, Hidden: true})
	if err != nil {
		t.Fatal(err)
	}
	got := paths(repos)
	found := map[string]bool{}
	for _, p := range got {
		found[p] = true
	}
	if !found[main_] || !found[wt] {
		t.Fatalf("expected main+linked, got %v", got)
	}
}

func paths(repos []Repo) []string {
	out := make([]string, len(repos))
	for i, r := range repos {
		out[i] = r.Path
	}
	sort.Strings(out)
	return out
}
