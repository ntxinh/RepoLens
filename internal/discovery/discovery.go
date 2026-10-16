// Package discovery scans registered root directories for Git working
// trees. Detection always goes through git rev-parse (so linked worktrees
// and submodules are recognized); a `.git` entry in a directory is only a
// fast-path filter deciding where to ask.
package discovery

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/ntxinh/RepoLens/internal/git"
)

// Options controls a scan.
type Options struct {
	MaxDepth    int      // directories below the root; <0 = unlimited
	Excludes    []string // doublestar patterns matched against the relative path
	Hidden      bool     // include dot-directories
	FollowLinks bool     // follow symlinked directories
}

// Repo is a discovered working tree.
type Repo struct {
	Path   string // resolved top-level
	GitDir string
}

// Scanner finds repositories with a git.Runner.
type Scanner struct{ runner *git.Runner }

// NewScanner wraps a runner.
func NewScanner(r *git.Runner) *Scanner { return &Scanner{runner: r} }

// Scan walks root and returns every distinct working tree beneath it,
// deduplicated by resolved top-level. Cancellation stops the walk; repos
// found so far are returned with the context error.
func (s *Scanner) Scan(ctx context.Context, root string, o Options) ([]Repo, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	seen := map[string]struct{}{}
	var repos []Repo

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return nil // unreadable entry: skip, don't fail the scan
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // never follow symlinks during the walk
		}
		if !d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		base := d.Name()
		if rel != "." {
			if base == ".git" {
				return fs.SkipDir // never descend into git internals
			}
			if !o.Hidden && strings.HasPrefix(base, ".") {
				return fs.SkipDir
			}
			if o.MaxDepth >= 0 && strings.Count(rel, string(filepath.Separator))+1 > o.MaxDepth {
				return fs.SkipDir
			}
			if excluded(rel, o.Excludes) {
				return fs.SkipDir
			}
		}

		// Fast-path: only dirs containing a .git entry can be worktree roots.
		if _, statErr := os.Lstat(filepath.Join(path, ".git")); statErr != nil {
			return nil
		}
		top, gitDir, detectErr := s.runner.Detect(ctx, path)
		if detectErr != nil {
			return nil // .git present but not a worktree (e.g. corrupt): skip
		}
		canon, _ := filepath.EvalSymlinks(top)
		if canon == "" {
			canon = top
		}
		if _, dup := seen[canon]; !dup {
			seen[canon] = struct{}{}
			repos = append(repos, Repo{Path: top, GitDir: gitDir})
		}
		return nil
	})

	if walkErr != nil && walkErr != context.Canceled && walkErr != context.DeadlineExceeded {
		return repos, walkErr
	}
	return repos, ctx.Err()
}

// excluded reports whether rel matches any exclude pattern. Patterns
// match both the directory itself and content beneath it, so the common
// "**/name/**" form excludes the directory "name" anywhere in the tree.
func excluded(rel string, patterns []string) bool {
	relSlash := filepath.ToSlash(rel)
	inside := relSlash + "/x" // sentinel: "dir is under pattern's subtree"
	for _, p := range patterns {
		if ok, _ := doublestar.Match(p, relSlash); ok {
			return true
		}
		if ok, _ := doublestar.Match(p, inside); ok {
			return true
		}
	}
	return false
}
