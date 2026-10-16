package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func runner(t *testing.T) *Runner {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	return NewRunner(10*time.Second, 4)
}

// gitAt runs a git command inside dir; failures are fatal.
func gitAt(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1", "HOME="+dir, // isolate from developer config
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git -C %s %s: %v\n%s", dir, strings.Join(args, " "), err, out)
	}
}

// initRepo creates a repo with one commit and returns its path.
func initRepo(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "init", "-q")
	gitAt(t, dir, "config", "user.email", "test@repolens.local")
	gitAt(t, dir, "config", "user.name", "RepoLens Test")
	writeFile(t, dir, "tracked.txt", "one\n")
	gitAt(t, dir, "add", "tracked.txt")
	gitAt(t, dir, "commit", "-qm", "initial")
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func status(t *testing.T, dir string) *Snapshot {
	t.Helper()
	snap, err := runner(t).Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status(%s): %v", dir, err)
	}
	return snap
}

func entriesOf(snap *Snapshot, kind byte) []FileEntry {
	var out []FileEntry
	for _, f := range snap.Files {
		if f.Kind == kind {
			out = append(out, f)
		}
	}
	return out
}

func TestDetectNotRepository(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runner(t).Detect(context.Background(), dir)
	if !errors.Is(err, ErrNotRepository) {
		t.Fatalf("expected ErrNotRepository, got %v", err)
	}
}

func TestCleanRepository(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "clean"))
	snap := status(t, dir)
	if snap.Detached || snap.Branch == "" {
		t.Fatalf("expected branch, detached=false; got branch=%q detached=%v", snap.Branch, snap.Detached)
	}
	if len(snap.Files) != 0 {
		t.Fatalf("expected clean tree, got %d entries", len(snap.Files))
	}
	if snap.LastCommit == nil || snap.LastCommit.Subject != "initial" {
		t.Fatalf("missing or wrong last commit: %+v", snap.LastCommit)
	}
	if snap.HasUpstream {
		t.Fatal("fresh repo should have no upstream")
	}
}

func TestModifiedUnstaged(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "mod"))
	writeFile(t, dir, "tracked.txt", "changed\n")
	snap := status(t, dir)
	ord := entriesOf(snap, '1')
	if len(ord) != 1 || ord[0].Y != 'M' || ord[0].X != '.' {
		t.Fatalf("expected 1 unstaged-modified entry, got %+v", snap.Files)
	}
}

func TestStagedAndUnstaged(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "both"))
	writeFile(t, dir, "tracked.txt", "staged\n")
	gitAt(t, dir, "add", "tracked.txt")
	writeFile(t, dir, "tracked.txt", "staged then unstaged\n")
	snap := status(t, dir)
	ord := entriesOf(snap, '1')
	if len(ord) != 1 || ord[0].X != 'M' || ord[0].Y != 'M' {
		t.Fatalf("expected MM entry, got %+v", ord)
	}
}

func TestUntrackedFiles(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "untracked"))
	writeFile(t, dir, "new file.txt", "x")  // space in name
	writeFile(t, dir, "números.txt", "x")   // non-ASCII
	writeFile(t, dir, "tab\tname.txt", "x") // tab in name
	snap := status(t, dir)
	u := entriesOf(snap, '?')
	if len(u) != 3 {
		t.Fatalf("expected 3 untracked, got %+v", u)
	}
	paths := map[string]bool{}
	for _, e := range u {
		paths[e.Path] = true
	}
	for _, want := range []string{"new file.txt", "números.txt", "tab\tname.txt"} {
		if !paths[want] {
			t.Errorf("missing untracked path %q in %+v", want, paths)
		}
	}
}

func TestRenamedFile(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "renamed"))
	gitAt(t, dir, "mv", "tracked.txt", "renamed.txt")
	snap := status(t, dir)
	r := entriesOf(snap, '2')
	if len(r) != 1 || r[0].X != 'R' || r[0].OrigPath != "tracked.txt" || r[0].Path != "renamed.txt" {
		t.Fatalf("expected rename tracked→renamed, got %+v", r)
	}
}

func TestDeletedFile(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "deleted"))
	if err := os.Remove(filepath.Join(dir, "tracked.txt")); err != nil {
		t.Fatal(err)
	}
	snap := status(t, dir)
	ord := entriesOf(snap, '1')
	if len(ord) != 1 || ord[0].Y != 'D' {
		t.Fatalf("expected unstaged delete, got %+v", ord)
	}
}

func TestMergeConflict(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "conflict"))
	gitAt(t, dir, "checkout", "-qb", "side")
	writeFile(t, dir, "tracked.txt", "side\n")
	gitAt(t, dir, "add", "tracked.txt")
	gitAt(t, dir, "commit", "-qm", "side change")
	gitAt(t, dir, "checkout", "-q", "master")
	writeFile(t, dir, "tracked.txt", "main\n")
	gitAt(t, dir, "add", "tracked.txt")
	gitAt(t, dir, "commit", "-qm", "main change")

	// The merge itself is expected to fail — run it directly (not gitAt).
	cmd := exec.Command("git", "-C", dir, "merge", "side")
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "HOME="+dir)
	_ = cmd.Run()

	snap := status(t, dir)
	u := entriesOf(snap, 'u')
	if len(u) != 1 {
		t.Fatalf("expected 1 unmerged entry, got %+v", snap.Files)
	}
	if u[0].X != 'U' || u[0].Y != 'U' {
		t.Fatalf("expected UU conflict entry, got %+v", u[0])
	}
}

func TestDetachedHead(t *testing.T) {
	dir := initRepo(t, filepath.Join(t.TempDir(), "detached"))
	head := func() string {
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(out))
	}()
	gitAt(t, dir, "checkout", "-q", head)
	snap := status(t, dir)
	if !snap.Detached {
		t.Fatalf("expected detached HEAD, got branch=%q detached=%v", snap.Branch, snap.Detached)
	}
}

func TestNoCommits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitAt(t, dir, "init", "-q")
	snap := status(t, dir)
	if snap.LastCommit != nil {
		t.Fatalf("expected nil LastCommit, got %+v", snap.LastCommit)
	}
}

func TestAheadBehind(t *testing.T) {
	tmp := t.TempDir()

	// Seed a bare "origin" with one commit.
	seed := initRepo(t, filepath.Join(tmp, "seed"))
	bare := filepath.Join(tmp, "origin.git")
	gitAt(t, tmp, "clone", "-q", "--bare", seed, bare)

	// Clone it; local gets origin/master upstream.
	local := filepath.Join(tmp, "local")
	gitAt(t, tmp, "clone", "-q", bare, local)
	gitAt(t, local, "config", "user.email", "test@repolens.local")
	gitAt(t, local, "config", "user.name", "RepoLens Test")

	snap := status(t, local)
	if !snap.HasUpstream || snap.Upstream != "origin/master" {
		t.Fatalf("expected upstream origin/master, got %q (has=%v)", snap.Upstream, snap.HasUpstream)
	}
	if !snap.HasCounts || snap.Ahead != 0 || snap.Behind != 0 {
		t.Fatalf("expected 0/0 known counts, got +%d -%d (has=%v)", snap.Ahead, snap.Behind, snap.HasCounts)
	}

	// One local commit → ahead 1.
	writeFile(t, local, "ahead.txt", "x")
	gitAt(t, local, "add", "ahead.txt")
	gitAt(t, local, "commit", "-qm", "ahead")
	snap = status(t, local)
	if !snap.HasCounts || snap.Ahead != 1 || snap.Behind != 0 {
		t.Fatalf("expected +1 -0, got +%d -%d", snap.Ahead, snap.Behind)
	}

	// A diverged commit on origin (built in the seed clone, pushed to bare),
	// then a fetch updates the local tracking ref. Fetch in test SETUP is
	// fine — the no-fetch rule governs the product's runtime behavior.
	writeFile(t, seed, "behind.txt", "y")
	gitAt(t, seed, "add", "behind.txt")
	gitAt(t, seed, "commit", "-qm", "behind")
	gitAt(t, seed, "push", "-q", bare, "master")
	gitAt(t, local, "fetch", "-q", "origin")

	snap = status(t, local)
	if !snap.HasCounts || snap.Ahead != 1 || snap.Behind != 1 {
		t.Fatalf("expected +1 -1, got +%d -%d", snap.Ahead, snap.Behind)
	}
}

func TestLinkedWorktree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("worktree paths differ on windows")
	}
	main_ := initRepo(t, filepath.Join(t.TempDir(), "mainwt"))
	wtPath := filepath.Join(filepath.Dir(main_), "linkedwt")
	gitAt(t, main_, "worktree", "add", "-q", wtPath)

	snap := status(t, wtPath)
	if snap.TopLevel != wtPath {
		t.Fatalf("expected top-level %q, got %q", wtPath, snap.TopLevel)
	}
	if snap.GitDir == snap.TopLevel+"/.git" {
		t.Fatal("linked worktree should not report a normal .git dir")
	}
}

func TestTimeoutOrMissingGit(t *testing.T) {
	// Point PATH at nothing so git is missing.
	t.Setenv("PATH", t.TempDir())
	r := NewRunner(time.Second, 1)
	_, err := r.Run(context.Background(), "status")
	if err == nil {
		t.Fatal("expected error with git missing")
	}
	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *CommandError, got %T (%v)", err, err)
	}
}

func TestCommandErrorMessage(t *testing.T) {
	ce := &CommandError{Args: []string{"status"}, Err: fmt.Errorf("boom"), Stderr: "fatal: something\n"}
	got := ce.Error()
	if !strings.Contains(got, "git status") || !strings.Contains(got, "something") {
		t.Fatalf("unexpected message: %q", got)
	}
}
