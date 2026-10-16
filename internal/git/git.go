// Package git is the only package allowed to spawn Git subprocesses.
// Every command runs with an argument array (never a shell), a per-command
// timeout, and a shared concurrency limit. Upstream packages consume the
// typed results below — they never see raw command output.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ErrNotRepository is returned when a path is not inside a Git working tree.
var ErrNotRepository = errors.New("not a git repository")

// CommandError reports a failed git invocation.
type CommandError struct {
	Args   []string
	Err    error
	Stderr string
}

func (e *CommandError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), msg)
}

func (e *CommandError) Unwrap() error { return e.Err }

// Runner executes git commands with a timeout and bounded concurrency.
type Runner struct {
	Timeout time.Duration
	sem     chan struct{}
}

// NewRunner returns a Runner. timeout is applied per command; maxConcurrent
// bounds the number of simultaneous git subprocesses.
func NewRunner(timeout time.Duration, maxConcurrent int) *Runner {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Runner{Timeout: timeout, sem: make(chan struct{}, maxConcurrent)}
}

// Run executes `git <args...>` and returns stdout. The caller passes
// repository paths via "-C", never interpolated into a string.
func (r *Runner) Run(ctx context.Context, args ...string) ([]byte, error) {
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, &CommandError{Args: args, Err: ctx.Err(), Stderr: stderr.String()}
	}
	if err != nil {
		return nil, &CommandError{Args: args, Err: err, Stderr: stderr.String()}
	}
	return stdout.Bytes(), nil
}

// FileEntry describes one path from porcelain status output.
type FileEntry struct {
	Path     string // NUL-safe: may contain spaces, tabs, newlines, non-ASCII
	X, Y     byte   // staged / unstaged status codes ('.' = unmodified)
	Kind     byte   // '1' ordinary, '2' renamed/copied, 'u' unmerged, '?' untracked
	OrigPath string // set for renames: original path
}

// Commit is the latest local commit's metadata.
type Commit struct {
	Hash      string
	Subject   string
	Author    string
	Timestamp time.Time
}

// Snapshot is the typed result of a read-only status collection.
type Snapshot struct {
	Path        string
	TopLevel    string // resolved working-tree root
	GitDir      string
	Branch      string // "" when detached
	Detached    bool
	Upstream    string // "" when no upstream configured
	HasUpstream bool
	Ahead       int
	Behind      int
	HasCounts   bool // false when tracking counts are unavailable
	Files       []FileEntry
	LastCommit  *Commit // nil when the repository has no commits
}

// Detect reports whether path is inside a Git working tree and resolves its
// top-level and git directories. Uses git itself (not `.git` presence) so
// linked worktrees and submodules are handled.
func (r *Runner) Detect(ctx context.Context, path string) (topLevel, gitDir string, err error) {
	out, err := r.Run(ctx, "-C", path, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) && strings.Contains(ce.Stderr, "not a git repository") {
			return "", "", ErrNotRepository
		}
		return "", "", fmt.Errorf("detect %s: %w", path, err)
	}
	if strings.TrimSpace(string(out)) != "true" {
		return "", "", fmt.Errorf("detect %s: unexpected rev-parse output", path)
	}

	top, err := r.Run(ctx, "-C", path, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", "", fmt.Errorf("detect %s: %w", path, err)
	}
	gd, err := r.Run(ctx, "-C", path, "rev-parse", "--git-dir")
	if err != nil {
		return "", "", fmt.Errorf("detect %s: %w", path, err)
	}
	return strings.TrimSpace(string(top)), strings.TrimSpace(string(gd)), nil
}

// Status collects a full read-only snapshot for the repository at path.
// Individual optional pieces (upstream, last commit) degrade to absence
// rather than failing the whole snapshot.
func (r *Runner) Status(ctx context.Context, path string) (*Snapshot, error) {
	snap := &Snapshot{Path: path}

	topLevel, gitDir, err := r.Detect(ctx, path)
	if err != nil {
		return nil, err
	}
	snap.TopLevel, snap.GitDir = topLevel, gitDir

	// One porcelain v2 invocation yields file entries plus branch, upstream,
	// and ahead/behind in its # branch headers.
	out, err := r.Run(ctx, "-C", topLevel, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return nil, fmt.Errorf("status %s: %w", path, err)
	}
	parsePorcelainV2(out, snap)

	// Latest local commit; nil (not an error) for a repository with no commits.
	log, err := r.Run(ctx, "-C", topLevel, "log", "-1", "--format=%h%x00%s%x00%an%x00%ct")
	if err == nil {
		snap.LastCommit = parseLogLine(log)
	} else {
		var ce *CommandError
		if !errors.As(err, &ce) || !isNoCommitsError(ce) {
			return nil, fmt.Errorf("log %s: %w", path, err)
		}
	}

	return snap, nil
}

func isNoCommitsError(e *CommandError) bool {
	return strings.Contains(e.Stderr, "does not have any commits yet") ||
		strings.Contains(e.Stderr, "ambiguous argument 'HEAD'")
}
