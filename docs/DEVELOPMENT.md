# Development

## Setup

```sh
git clone https://github.com/ntxinh/RepoLens.git
cd RepoLens
mise install        # installs Go 1.25 per mise.toml
```

Requires `git` on `PATH` — tests create real repositories with it.

## Common tasks

```sh
make build          # bin/repolens
make test           # go test ./...
make test-race      # race detector
make vet            # go vet ./...
make fmt            # gofmt -w .
make fmt-check      # fail if not gofmt-clean (CI uses this)
make lint           # vet + govulncheck when installed
make install        # go install with version ldflags
make help           # list targets
```

## Testing notes

- Unit tests cover config validation, path normalization, porcelain/status parsing, sort/filter/attention logic, registry persistence and corruption recovery, and terminal-safe rendering.
- Integration tests build temp repos via `t.TempDir()` and the real `git` binary — clean, staged/unstaged, untracked, renamed, deleted, conflicts, detached HEAD, missing upstream, ahead/behind, linked worktrees, unusual filenames.
- TUI tests drive the Bubble Tea model with injected `tea.Msg`s — no pty needed.
- Never test against your real repositories or mutate global Git config.

## CI

GitHub Actions runs `make fmt-check vet test` plus a build on the target platform. `go.mod`/`go.sum` are committed; add dependencies deliberately, not casually.

## Release

`make build` stamps `main.version` from `git describe`. Tagged releases produce Linux artifacts + checksums via CI; an RPM/COPR package is a later-phase item (spec §21 Phase 6).
