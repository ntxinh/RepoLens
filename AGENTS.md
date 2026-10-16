# AGENTS.md

Working rules for this repository. The product/technical spec lives at `docs/repolens-spec.md`; the design summary at `DESIGN.md`. Read the spec before changing behavior.

## Hard rules — Git safety

These are the product's guarantees. Breaking them is a bug, not a style choice.

- **Read-only.** Never run `git` commands that mutate a repository: no `fetch`, `pull`, `push`, `commit`, `stage`, `checkout`, `reset`, `clean`, `stash`, `merge`, `rebase`, `gc`, `worktree *`. Status, log, diff, rev-list, rev-parse, for-each-ref are allowed.
- **Argument arrays only.** Invoke `git` via `exec.CommandContext` (or equivalent) with an argument slice. Never build a shell command string; never interpolate a repo path, branch name, or any user/repo-derived value into a command.
- **Untrusted content.** Branch names, commit subjects, author names, and file paths are attacker-controlled terminal content. Sanitize control characters before rendering. Filenames can contain spaces, tabs, newlines, and non-ASCII — use NUL-delimited porcelain output, never line-split human output.
- **No automatic network access.** A behind-count reflects stale local refs; show it as such, don't fetch to freshen it.
- **One repo failure must not fail the scan.** Errors are per-repository status data, not panics.

## Package boundaries

Only `internal/git` may spawn Git subprocesses. It exposes typed results (`Status`, `BranchInfo`, ahead/behind, commit metadata); every other package consumes those types — no package upstream of it sees raw command output.

```
cmd/repolens          → entry point + CLI subcommands
internal/app          → wiring: config → registry → service → tui
internal/config       → TOML schema, loading, validation (only package that parses TOML)
internal/discovery    → root scanning, repo detection via `git rev-parse`, exclusions
internal/git          → command runner (timeouts, concurrency limits) + parsers
internal/repository   → domain types, status semantics, attention rules
internal/registry     → registered-path persistence (atomic writes, corrupt-file recovery)
internal/cache        → status cache; cached values must be marked stale, never "fresh"
internal/service      → refresh orchestration, cancellation, aggregation
internal/tui          → Bubble Tea model/update/view + components
internal/launcher     → editor/terminal/file-manager spawn (arg arrays, {path} placeholder)
internal/logging      → slog setup
```

Keep this proportional to the MVP — no empty packages, no interfaces with one implementation.

## Testing

- Unit tests: config validation, path normalization, status parsing, sort/filter/attention logic, registry persistence/recovery, terminal-safe rendering.
- Integration tests build temporary repos with the real `git` binary (`t.TempDir()`). Never touch the developer's real repos or Git config.
- TUI tests drive the Bubble Tea model directly: send `tea.Msg`, assert state.

```sh
make test         # all tests
make test-race    # race detector
make vet fmt-check
```

## Conventions

- `gofmt` clean; `go vet` clean. Wrap errors with `%w`. `log/slog` for diagnostics — never print to the TUI.
- Pin dependencies in `go.mod`; commit `go.sum`. Don't add a dependency for something stdlib already does.
- Config is validated at startup; report the invalid key and expected format.
- Shell integrations (Fish functions, completions) are separate files with manual install instructions — never modify the user's shell config automatically.

## Definition of done

A feature is done when: behavior documented, success/error/cancellation paths handled, tests cover the important cases, UI labels and help updated, nothing silently mutates, and it works without network access.
