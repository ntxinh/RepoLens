// Package service orchestrates discovery, refresh, and registry updates.
// The TUI and CLI both drive the app through this layer.
package service

import (
	"context"
	"errors"
	"time"

	"github.com/ntxinh/RepoLens/internal/config"
	"github.com/ntxinh/RepoLens/internal/discovery"
	"github.com/ntxinh/RepoLens/internal/git"
	"github.com/ntxinh/RepoLens/internal/registry"
	"github.com/ntxinh/RepoLens/internal/repository"
)

// Service wires the engine pieces together.
type Service struct {
	Runner  *git.Runner
	Scanner *discovery.Scanner
	Reg     *registry.Registry
	Cfg     config.Config
}

// New builds a Service from config and an open registry.
func New(cfg config.Config, reg *registry.Registry, timeout time.Duration, maxConcurrent int) *Service {
	r := git.NewRunner(timeout, maxConcurrent)
	return &Service{
		Runner:  r,
		Scanner: discovery.NewScanner(r),
		Reg:     reg,
		Cfg:     cfg,
	}
}

// ScanRoots discovers repositories under the configured roots plus any
// extra paths, registers them, and saves the registry. Returns the
// number of repositories found.
func (s *Service) ScanRoots(ctx context.Context, extra []string) (int, error) {
	roots, err := s.Cfg.Roots()
	if err != nil {
		return 0, err
	}
	roots = append(roots, extra...)

	opts := discovery.Options{
		MaxDepth: s.Cfg.General.DiscoveryMaxDepth,
		Excludes: s.Cfg.Discovery.Exclude,
		Hidden:   s.Cfg.General.ShowHiddenDirectories,
	}

	found := 0
	var firstErr error
	for _, root := range roots {
		// A path that is itself a repo is registered directly; scanning
		// beneath it also works, this just covers a repo passed as a root.
		repos, scanErr := s.Scanner.Scan(ctx, root, opts)
		for _, repo := range repos {
			s.Reg.Add(repo.Path)
			found++
		}
		if scanErr != nil && !errors.Is(scanErr, context.Canceled) && firstErr == nil {
			firstErr = scanErr
		}
	}
	if err := s.Reg.Save(); err != nil {
		return found, err
	}
	return found, firstErr
}

// RefreshAll collects status for every registered repository.
// Per-repository errors become Status.Err, never abort the refresh.
func (s *Service) RefreshAll(ctx context.Context) []repository.Status {
	entries := s.Reg.Entries()
	out := make([]repository.Status, 0, len(entries))
	for _, e := range entries {
		out = append(out, s.refreshOne(ctx, e.Path))
	}
	return out
}

// Refresh collects status for one path.
func (s *Service) Refresh(ctx context.Context, path string) repository.Status {
	return s.refreshOne(ctx, path)
}

func (s *Service) refreshOne(ctx context.Context, path string) repository.Status {
	now := time.Now()
	snap, err := s.Runner.Status(ctx, path)
	if err != nil {
		st := repository.Status{Path: path, CheckedAt: now, Err: err}
		if errors.Is(err, git.ErrNotRepository) {
			s.Reg.MarkMissing(path, true)
			st.Err = errors.New("repository path missing or no longer a git repo")
		}
		return st
	}
	s.Reg.MarkMissing(path, false)
	return repository.FromSnapshot(snap, now)
}
