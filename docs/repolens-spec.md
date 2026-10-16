# RepoLens — Product & Technical Specification

> **Project type:** Local-first Git repository dashboard  
> **Primary platform:** Fedora Workstation 44, Wayland, Niri, DankMaterialShell, Fish  
> **Primary interface:** TUI  
> **Preferred implementation language:** Go  
> **Status:** Initial implementation specification  
> **Working title:** RepoLens

## 1. Product Overview

RepoLens is a fast, local-first dashboard for inspecting the state of many Git repositories from one terminal interface.

It helps developers answer:

- Which repositories have uncommitted changes?
- Which branches are ahead of or behind their upstream?
- Which repositories have untracked files, conflicts, or detached HEAD state?
- Which repositories have not been active recently?
- Which repositories need attention before work can continue?
- Where is a repository located, and how can I open it quickly in my editor or terminal?

RepoLens is an **observability and navigation tool**, not another Git worktree lifecycle manager.

### 1.1 Existing tool boundary

The user already has a Git worktree manager implemented in Go as a TUI. RepoLens must complement it rather than duplicate it.

**RepoLens owns:**
- Multi-repository discovery and registration
- Repository status overview
- Branch and upstream tracking information
- Dirty, untracked, conflicted, and detached-HEAD indicators
- Repository filtering, sorting, and attention prioritization
- Recent local activity summaries
- Read-only inspection of commits and diffs
- Opening repositories in configured tools
- Optional scripts and external integrations

**The existing worktree manager owns:**
- Creating, deleting, moving, or pruning worktrees
- Worktree-specific lifecycle workflows
- Any existing worktree management features already provided by that tool

RepoLens may display worktrees as repository entries and may launch the existing manager, but it must not implement a second worktree lifecycle system in the MVP.

## 2. Goals

1. Show the state of many local Git repositories in one screen.
2. Make repositories requiring attention easy to identify.
3. Refresh status quickly without freezing the UI.
4. Work offline without an account or remote service.
5. Respect Git configuration and repository state.
6. Integrate naturally with a keyboard-driven Linux desktop and Fish shell.
7. Provide a small, maintainable codebase with tests and clear documentation.

## 3. Non-Goals

The MVP will not:

- Replace Git, lazygit, or the existing Git worktree manager.
- Provide a complete Git staging/commit/rebase/merge interface.
- Create, delete, move, or prune worktrees.
- Require GitHub, GitLab, or any network connection.
- Run background daemons that must remain active after RepoLens exits.
- Automatically fetch, pull, push, checkout, reset, clean, or modify repositories.
- Include AI-generated summaries or cloud processing.
- Store repository contents or source code in a central database.
- Act as a general-purpose project management tool.

## 4. Target Environment

### 4.1 Supported environment

- Fedora Workstation 44
- Linux
- Wayland
- Niri window manager
- DankMaterialShell (DMS)
- Fish shell
- Git installed and available on `PATH`

### 4.2 Terminal compatibility

RepoLens should work in common terminal emulators, including Ghostty and other standards-compliant terminal emulators.

The application must not depend on X11 APIs or a specific desktop environment. It should remain usable in a plain terminal session.

### 4.3 Runtime dependencies

Required:
- Git executable available on `PATH`

Optional:
- `zoxide` or another directory-jump tool, if a future integration is added
- `gh` CLI for optional GitHub metadata in a later phase
- `fzf` must not be required; fuzzy filtering should work natively

Do not require Node.js, Python, a database server, or a background service at runtime.

## 5. Proposed Technology Stack

| Area | Choice | Rationale |
|---|---|---|
| Language | Go | Fast startup, simple distribution, fits the user's existing TUI experience |
| TUI framework | Bubble Tea | Model/update/view architecture for terminal applications |
| TUI components | Bubbles | Lists, text inputs, spinners, viewports where useful |
| Styling | Lip Gloss | Consistent terminal layout and styles |
| Git integration | Git CLI | Honors installed Git behavior and avoids reimplementing Git semantics |
| Configuration | TOML | Human-editable configuration |
| Config parsing | A maintained Go TOML library | Keep config loading isolated behind an internal package |
| Fuzzy search | In-process matching | No external command dependency |
| Logging | Standard library `log/slog` | Structured diagnostics without cluttering the TUI |
| Testing | `testing`, `testify` only if useful | Standard Go tests and assertions |
| Build/release | Go toolchain + GitHub Actions | Reproducible builds and release artifacts |

Use current stable, compatible dependency versions when implementation begins. Pin dependencies through `go.mod` and commit `go.sum`.

### 5.1 Architecture principle

Prefer the Git CLI for status and metadata instead of parsing `.git` internals directly. Use machine-readable output such as porcelain status and NUL-delimited fields where supported. Do not parse human-readable output when a stable machine-readable format exists.

All Git invocation should pass arguments as an argument array, not through a shell command string. Never interpolate repository paths into shell commands.

## 6. Core User Experience

### 6.1 Main dashboard

The main screen should display one row per discovered repository or worktree.

Suggested columns:

| Column | Description |
|---|---|
| Status | Clean, modified, untracked, conflicted, unknown, or error |
| Repository | Directory/repository name |
| Branch | Current branch or detached HEAD |
| Upstream | Upstream branch, if configured |
| Ahead/Behind | Local commits ahead and remote-tracking commits behind |
| Changes | Counts of modified, staged, untracked, and conflicted files |
| Last activity | Most recent local commit date, when available |
| Path | Repository path, optionally shortened |

The layout must adapt to terminal width:
- Wide terminal: show the full configured column set.
- Medium terminal: shorten path and omit less important columns.
- Narrow terminal: use a compact row with essential status, repository, branch, and change counts.
- Never require horizontal scrolling for basic navigation.

### 6.2 Dashboard header

Display:
- Number of repositories
- Number requiring attention
- Number with local changes
- Number behind upstream, where known
- Last refresh time
- Current filter or search query

Counts must distinguish “unknown/not checked” from zero.

### 6.3 Attention indicators

Repositories may be marked for attention when they have:
- Merge conflicts
- Uncommitted changes
- Untracked files
- Commits ahead of upstream
- Commits behind upstream
- Detached HEAD
- Missing or inaccessible repository path
- Git command failure
- Stale status data

These are informational indicators, not automatic actions. Avoid a single ambiguous red/yellow/green status that hides multiple conditions. Use readable labels or symbols and provide a legend.

### 6.4 Repository detail panel

Selecting a repository opens a detail view with:
- Absolute path
- Current branch or detached HEAD commit
- Upstream and tracking state
- Staged, unstaged, untracked, and conflicted file counts
- Ahead/behind counts, if available
- Latest local commit: abbreviated hash, subject, author, and date
- Recent local commits
- Current status summary
- Available actions

### 6.5 Changed files and diff inspection

The MVP may provide a read-only changed-file list and diff viewer:
- Separate staged and unstaged changes where practical.
- Show untracked files as entries, but do not pretend they have a Git diff.
- Support scrolling through long diffs.
- Handle binary files, large diffs, renamed files, and deleted files gracefully.
- Clearly identify when output is truncated.
- Do not write patches or apply changes.

For safety and scope, stage/unstage, commit, reset, checkout, merge, rebase, clean, stash, pull, and push are excluded from the MVP.

## 7. Repository Discovery and Registration

### 7.1 Registration methods

Support:
1. Add a single repository by path.
2. Add a directory and discover repositories beneath it.
3. Register multiple root directories in configuration.
4. Remove a repository from the RepoLens registry without deleting files.
5. Refresh or re-scan registered roots.

### 7.2 Discovery rules

- Accept absolute and relative paths; normalize to absolute paths.
- Resolve symlinks consistently and document the chosen behavior.
- Detect a Git working tree using Git itself, not only the presence of a `.git` directory. This is required for linked worktrees and submodules.
- Do not recurse into `.git` internals.
- Avoid following symlinks by default during recursive discovery.
- Support configurable maximum depth.
- Support exclude patterns for directories such as `node_modules`, `vendor`, `target`, `dist`, `.cache`, and other user-defined paths.
- Avoid registering the same physical working tree multiple times.
- Treat each linked worktree as its own working directory entry while optionally showing the common Git directory.
- Do not confuse nested repositories with their parent repository; provide a documented policy and test it.
- Discovery must be cancellable and should not block the UI.

### 7.3 Registry behavior

The registry stores paths and user preferences, not source contents.

A repository that disappears should be marked unavailable rather than silently deleted from the registry. Offer an explicit “remove from dashboard” action.

## 8. Git Status and Metadata

### 8.1 Status collection

Collect, where available:
- Current branch
- Detached HEAD state
- Working tree cleanliness
- Staged changes
- Unstaged changes
- Untracked files
- Merge conflicts
- Upstream branch
- Ahead/behind counts
- Latest local commit metadata
- Repository path validity

Use porcelain status output with a version explicitly selected and parsed. Prefer NUL-delimited output for filenames to support spaces, newlines, and unusual characters.

### 8.2 Upstream and remote state

- Ahead/behind status must be calculated against locally available refs.
- Do not silently run `git fetch`.
- Display the timestamp or freshness of the last known remote-tracking data where feasible.
- If the upstream is missing, show “no upstream” rather than zero ahead/behind.
- If tracking data is unavailable, show “unknown” rather than guessing.
- Network access is never required for basic dashboard status.

### 8.3 Last activity

The MVP should use the latest local commit timestamp as the primary activity indicator.

Do not claim this represents the last time a person opened or edited the repository. File modification times may be offered later as a separate signal, but should not be mixed with commit activity.

### 8.4 Errors

Per-repository failures must not crash the whole dashboard. Capture:
- Path missing or inaccessible
- Not a Git repository
- Git executable missing
- Git command timeout
- Repository lock or transient Git failure
- Malformed or unexpected command output

Show a concise error state in the dashboard and detailed diagnostics in a log file or debug mode.

## 9. Sorting, Filtering, and Search

### 9.1 Built-in filters

- All repositories
- Needs attention
- Dirty
- Clean
- Ahead of upstream
- Behind upstream
- Conflicted
- Detached HEAD
- Unavailable/error
- Recently active

### 9.2 Search

Search across:
- Repository name
- Branch name
- Repository path
- Optional user-defined tags

Search should update interactively and remain responsive with hundreds of registered repositories.

### 9.3 Sorting

Allow sorting by:
- Attention priority
- Dirty status
- Repository name
- Branch name
- Ahead/behind count
- Last local commit time
- Path

Preserve the user's selected sort and filter between runs.

## 10. Keyboard Navigation

The exact keymap should be configurable, but the initial defaults should be discoverable and consistent.

Suggested default bindings:

| Key | Action |
|---|---|
| `j` / `k` or arrows | Move selection |
| `Enter` | Open repository details |
| `/` | Search/filter |
| `a` | Show repositories requiring attention |
| `d` | Toggle dirty-only filter |
| `r` | Refresh selected repository or all repositories, depending on context |
| `s` | Cycle sort mode |
| `o` | Open selected repository in configured editor/file manager |
| `t` | Open terminal at repository path |
| `g` | Open configured Git TUI, if installed |
| `c` | Copy repository path |
| `?` | Show keybinding help |
| `Esc` | Close panel or clear search |
| `q` | Quit or go back |

Avoid destructive actions in the primary keymap. Confirm before launching any future operation that changes repository state.

## 11. Opening Tools

Allow configurable commands for:
- Editor
- Terminal
- File manager
- Existing Git worktree manager
- Optional Git TUI

Example intent:
- Open repository in VS Code or another editor.
- Open Ghostty at the selected repository path.
- Open the configured file manager at the repository path.
- Launch the existing worktree manager without implementing its functionality again.

Requirements:
- Do not hardcode the user's editor or terminal.
- Validate executable availability and show actionable errors.
- Launch commands without a shell by default.
- Support explicit argument arrays in configuration.
- Never execute repository-provided scripts automatically.
- Document how terminal applications are spawned under Wayland/Niri.

## 12. Configuration

Default path:
- `${XDG_CONFIG_HOME:-~/.config}/repolens/config.toml`

Data path:
- `${XDG_DATA_HOME:-~/.local/share}/repolens/`

Cache path:
- `${XDG_CACHE_HOME:-~/.cache}/repolens/`

Log path:
- `${XDG_STATE_HOME:-~/.local/state}/repolens/` where supported.

Illustrative configuration:

```toml
[general]
refresh_interval_seconds = 60
discovery_max_depth = 4
follow_symlinks = false
show_hidden_directories = true
default_sort = "attention"
theme = "auto"

[discovery]
roots = [
  "~/Projects",
  "~/Work",
  "~/Sandbox",
]
exclude = [
  "**/node_modules/**",
  "**/vendor/**",
  "**/target/**",
  "**/dist/**",
  "**/.cache/**",
]

[display]
show_path = true
show_upstream = true
show_ahead_behind = true
show_last_commit = true

[commands]
editor = ["code", "{path}"]
terminal = ["ghostty", "--working-directory", "{path}"]
file_manager = ["xdg-open", "{path}"]
git_tui = ["lazygit"]
worktree_manager = ["your-worktree-manager-command", "{path}"]
```

This is illustrative, not a guarantee that every field is supported in the first release. Validate configuration at startup and report invalid keys or values clearly.

### 12.1 Configuration rules

- Expand `~` and supported environment variables deliberately and document expansion rules.
- Use `{path}` as an explicit placeholder, not shell interpolation.
- Treat command configuration as an executable plus argument array.
- Provide `repolens config path` and `repolens config init`.
- Ship a documented example configuration.
- Do not store credentials or tokens in the configuration file.

## 13. Data Storage and Caching

### 13.1 MVP storage

Use a small local registry/config file unless testing demonstrates a strong need for a database.

Store:
- Registered repository paths
- User-defined tags
- Per-repository preferences
- Last selected repository
- UI preferences
- Last successful scan timestamp

Do not store:
- Source files
- Credentials
- Full diffs by default
- Remote repository contents

### 13.2 Cache behavior

- Cache status only as a performance optimization.
- Mark cached values as stale when their freshness cannot be guaranteed.
- Refresh on demand.
- A cached status must never be presented as freshly verified.
- Use atomic writes for persistent state.
- Recover gracefully from corrupted cache/registry files, preserving a backup when practical.

## 14. Refresh and Performance

- Use asynchronous/background tasks for repository discovery and Git status collection.
- Keep the UI responsive while scanning.
- Support cancellation when a scan is no longer needed.
- Limit concurrent Git subprocesses with a configurable or sensible default.
- Apply a timeout per Git command.
- Do not repeatedly scan the same repository concurrently.
- Debounce interactive search and resize events.
- Refresh only what is necessary for targeted refresh actions.
- Support a manual “refresh all” action.
- Do not run a permanent daemon in the MVP.

### 14.1 Performance targets

These are initial engineering targets to validate on representative hardware, not guaranteed measurements:
- UI becomes interactive quickly when opening a registry with cached entries.
- Search feels immediate for 1,000 registered repositories.
- A single repository error does not delay or fail the full scan.
- The application remains usable while scanning several hundred local repositories.
- Memory use remains reasonable and does not grow continuously during repeated refreshes.

Measure cold-start, warm-start, scan duration, cancellation latency, and memory use in a benchmark or manual performance checklist.

## 15. CLI Interface

The TUI is the primary experience, but a small CLI enables scripting and Fish integration.

Suggested commands:

```text
repolens
repolens scan [ROOT...]
repolens list
repolens status [PATH]
repolens open [PATH]
repolens config init
repolens config path
repolens version
repolens --help
```

Suggested options:
- `--json` for machine-readable output on suitable read-only commands
- `--no-color` for scripts and logs
- `--config PATH` to use an alternate configuration
- `--debug` for diagnostics

CLI output should use stable field names and documented exit codes. JSON output must not include ANSI escape sequences.

Do not add every possible command in the MVP. Keep the CLI surface small and consistent.

## 16. Fish Shell Integration

Fish integration is optional and must not be required for core functionality.

Potential functions:
- `croot`: open or change directory to a selected registered repository.
- `repolens`: launch the dashboard.
- Optional completion for CLI subcommands and flags.

Avoid modifying the user's Fish configuration automatically. Provide a separate integration file and installation/uninstallation instructions.

Changing the current shell's working directory must happen in a Fish function or generated shell integration, not in a child process.

## 17. Security and Safety

1. Default to read-only Git operations.
2. Never invoke a shell with user-controlled repository paths.
3. Treat repository names, branch names, commit messages, and file names as untrusted terminal content.
4. Sanitize or safely render control characters to prevent terminal escape injection.
5. Support unusual filenames, including spaces, tabs, newlines, and non-ASCII characters.
6. Never execute hooks, build scripts, or project-defined commands automatically.
7. Do not read or display credential files as part of repository discovery.
8. Do not collect or transmit telemetry by default.
9. Do not require root privileges.
10. Clearly identify any optional feature that invokes external commands.
11. Ensure opening an editor or terminal uses configured argument arrays and avoids shell interpolation.
12. Do not automatically fetch or access network remotes.

## 18. Error Handling and Observability

- Present actionable, concise errors in the UI.
- Provide a debug mode for detailed diagnostics.
- Include repository path and operation name in diagnostic messages.
- Avoid logging full environment variables, secrets, or credential-bearing URLs.
- One repository failing must not terminate the dashboard.
- Distinguish user cancellation from command failure and timeout.
- If configuration is invalid, identify the key and expected value format.

## 19. Testing Strategy

### 19.1 Unit tests

Test:
- Configuration parsing and validation
- Path normalization
- Search and filtering
- Sorting and attention scoring
- Status parsing
- Ahead/behind unknown states
- Filename handling
- Terminal-safe rendering
- Registry persistence and recovery

### 19.2 Integration tests

Create temporary Git repositories using the installed Git executable. Test:
- Clean repository
- Staged and unstaged modifications
- Untracked files
- Deleted and renamed files
- Merge conflicts
- Detached HEAD
- Missing upstream
- Ahead/behind tracking refs
- Linked worktrees
- Submodules where practical
- Missing or inaccessible path
- Git command timeout or failure
- Repositories with unusual filenames

Tests must not use the developer's real repositories or modify the user's Git configuration.

### 19.3 TUI tests

Test:
- Navigation and selection
- Search/filter behavior
- Refresh lifecycle
- Empty registry
- Loading state
- Error state
- Narrow terminal layout
- Help overlay
- Cancellation and quit behavior

### 19.4 Quality checks

The project should provide documented commands for:
- `gofmt`
- `go vet ./...`
- `go test ./...`
- Race tests where practical
- Build for the host platform
- Static analysis/linting if adopted

## 20. Project Structure

Suggested initial structure:

```text
repolens/
├── cmd/
│   └── repolens/
│       └── main.go
├── internal/
│   ├── app/            # Application lifecycle and orchestration
│   ├── config/         # Configuration schema, loading, validation
│   ├── discovery/      # Root scanning and repository detection
│   ├── git/            # Git command runner and parsers
│   ├── repository/     # Domain models and status semantics
│   ├── registry/       # Registered repository persistence
│   ├── cache/          # Optional status cache
│   ├── service/        # Refresh and aggregation logic
│   ├── tui/
│   │   ├── model.go
│   │   ├── update.go
│   │   ├── view.go
│   │   └── components/
│   ├── launcher/       # Editor/terminal/file-manager launching
│   └── logging/
├── configs/
│   └── config.example.toml
├── completions/
│   └── repolens.fish
├── docs/
│   ├── ARCHITECTURE.md
│   ├── CONFIGURATION.md
│   ├── DEVELOPMENT.md
│   └── SECURITY.md
├── scripts/
├── .github/
│   └── workflows/
├── .editorconfig
├── .gitignore
├── AGENTS.md
├── Makefile
├── README.md
├── go.mod
└── go.sum
```

Keep the structure proportional to the MVP. Do not create empty abstractions or packages before they are needed.

## 21. Implementation Phases

### Phase 0 — Project foundation

- Initialize Go module and repository.
- Add project documentation and contribution conventions.
- Add formatting, vetting, test, and build commands.
- Establish a minimal Bubble Tea application.
- Add CI for formatting, tests, and build.

**Exit criteria:** the app builds and launches on Fedora 44, shows a usable empty state, and CI passes.

### Phase 1 — Git status engine

- Implement a safe Git command runner.
- Implement repository detection and path normalization.
- Parse status, branch, upstream, and ahead/behind information.
- Add tests using temporary repositories.
- Handle errors and timeouts per repository.

**Exit criteria:** accurate read-only status for the core repository scenarios.

### Phase 2 — Multi-repository registry and discovery

- Add root-directory scanning.
- Add repository registration and removal from the registry.
- Add exclusions, max depth, duplicate handling, and linked-worktree detection.
- Persist registry state.

**Exit criteria:** users can register project roots and reliably rediscover repositories.

### Phase 3 — Dashboard TUI

- Add main list and responsive columns.
- Add selection, sorting, search, and built-in filters.
- Add attention indicators and status counts.
- Add asynchronous refresh and cancellation.
- Add empty, loading, stale, and error states.

**Exit criteria:** users can inspect hundreds of repositories without blocking the UI.

### Phase 4 — Details and navigation

- Add repository detail view.
- Add recent local commit metadata.
- Add read-only changed-file list and diff viewer if stable.
- Add configurable editor, terminal, file manager, and Git TUI launch actions.
- Add keybinding help.

**Exit criteria:** a user can identify an issue, inspect it, and open the repository in their preferred tool.

### Phase 5 — CLI and Fish integration

- Add minimal read-only CLI commands.
- Add JSON output where useful.
- Add Fish completion and optional helper function.
- Document installation without automatic shell configuration changes.

**Exit criteria:** common tasks can be scripted and the TUI can be launched from Fish.

### Phase 6 — Packaging and release

- Add Linux build artifacts for supported architectures.
- Add checksums for release artifacts.
- Document install/uninstall paths.
- Consider an RPM/COPR package after release workflows stabilize.
- Test in a clean Fedora environment.

**Exit criteria:** a user can install, run, update, and uninstall without leaving unexpected files or changing system configuration.

### Later / Optional

- Optional GitHub CLI integration for pull request and CI status.
- Repository tags and custom attention rules.
- Desktop notifications for configured conditions.
- File watcher-based refresh, only if polling is insufficient.
- Exportable reports.
- Optional DMS quick-launch integration.

These features must not delay the local-first MVP.

## 22. Acceptance Criteria

The MVP is accepted when:

1. It builds and runs on Fedora Workstation 44.
2. It works in a Wayland/Niri session and a standard terminal.
3. It can register multiple repository roots and discover Git working trees.
4. It displays current branch, dirty state, staged/unstaged/untracked/conflict counts, and upstream tracking state where available.
5. It distinguishes zero ahead/behind from unknown or missing tracking information.
6. It does not fetch, pull, push, stage, commit, reset, clean, or otherwise mutate repositories automatically.
7. A failure in one repository does not crash the dashboard.
8. Search, sorting, filters, and attention indicators work.
9. Users can open a repository in configured tools without shell interpolation.
10. Configuration and registry survive restarts.
11. The interface remains responsive during scans and can cancel long-running discovery.
12. Tests cover common status states, linked worktrees, error conditions, and unusual filenames.
13. `gofmt`, `go vet`, and `go test ./...` pass in CI.
14. Installation and removal instructions are documented.
15. No cloud account, telemetry, or background daemon is required.

## 23. UX and Design Guidelines

- Keyboard-first, predictable navigation.
- Compact and information-dense without making status codes cryptic.
- Use color as a supplement, never as the only indicator.
- Respect terminal color capability and `NO_COLOR`.
- Support dark and light terminal themes through adaptive styles.
- Keep status meanings consistent throughout the dashboard.
- Avoid flicker during refresh; update rows as results arrive where practical.
- Show a clear loading indicator for long scans.
- Preserve selection across refresh when the selected repository still exists.
- Do not unexpectedly jump the selection while the user is navigating.

## 24. Documentation Deliverables

The repository should include:
- `README.md`: overview, screenshots/demo, installation, quick start, keybindings, limitations.
- `AGENTS.md`: coding conventions, architecture boundaries, safe Git rules, test commands.
- `docs/ARCHITECTURE.md`: package responsibilities and data flow.
- `docs/CONFIGURATION.md`: all configuration fields with examples.
- `docs/DEVELOPMENT.md`: local setup, tests, build and release workflow.
- `docs/SECURITY.md`: subprocess safety, read-only defaults, and reporting vulnerabilities.
- `CHANGELOG.md`: user-visible changes by release.
- `LICENSE`: choose a suitable open-source license before public release.

## 25. Definition of Done for Every Feature

A feature is complete only when:
- Requirements and behavior are documented.
- Success, error, and cancellation paths are handled.
- Tests cover the important cases.
- UI labels and help text are updated.
- No destructive behavior is introduced silently.
- The feature works without network access unless explicitly documented as optional.
- It respects Fedora/Wayland/Niri constraints.
- It does not duplicate the existing Git worktree manager's responsibilities.

---

## Product Summary

RepoLens is a **read-only, multi-repository Git status and navigation dashboard**. Its main value is making local development work visible across many repositories, highlighting what needs attention, and launching the right tool quickly. Keep the MVP focused; add remote integrations and automation only after the local status engine and TUI are dependable.
