# Configuration

RepoLens reads a single TOML file. Invalid keys and values are reported at startup with the expected format.

## Paths

| Path | Purpose |
|---|---|
| `${XDG_CONFIG_HOME:-~/.config}/repolens/config.toml` | Configuration |
| `${XDG_DATA_HOME:-~/.local/share}/repolens/` | Registry (registered repos, tags, prefs) |
| `${XDG_CACHE_HOME:-~/.cache}/repolens/` | Status cache |
| `${XDG_STATE_HOME:-~/.local/state}/repolens/` | Logs |

```sh
repolens config init   # write a documented default config
repolens config path   # print the resolved config path
repolens --config PATH # use an alternate file
```

## Reference

```toml
[general]
refresh_interval_seconds = 60       # auto-refresh interval; 0 = manual only
discovery_max_depth = 4             # max depth scanning beneath a root
follow_symlinks = false             # follow symlinked directories during discovery
show_hidden_directories = true      # include dot-directories in discovery
default_sort = "attention"          # attention | dirty | name | branch | ahead_behind | last_commit | path
theme = "auto"                      # auto | dark | light (respects NO_COLOR)

[discovery]
roots = [                           # directories scanned for repositories
  "~/Projects",
]
exclude = [                         # glob patterns matched against path segments
  "**/node_modules/**",
  "**/vendor/**",
  "**/target/**",
  "**/dist/**",
  "**/.cache/**",
]

[display]
show_path = true                    # path column
show_upstream = true                # upstream column
show_ahead_behind = true            # ahead/behind column
show_last_commit = true             # last-activity column

[commands]
editor = ["code", "{path}"]               # open repository in editor
terminal = ["ghostty", "--working-directory", "{path}"]
file_manager = ["xdg-open", "{path}"]
git_tui = ["lazygit"]                     # optional Git TUI
worktree_manager = ["wtm", "{path}"]      # the user's existing worktree manager
```

## Command rules

- Commands are `[executable, arg, arg, ...]` arrays. They are launched directly — never through a shell.
- `{path}` is an explicit placeholder substituted with the absolute repository path. Any string without it is used verbatim.
- Missing executables produce an actionable error in the UI, not a launch attempt.
- Repository-provided scripts are never executed automatically.

## Expansion rules

- `~` expands to `$HOME` only at the start of a path value.
- `$VAR`/`${VAR}` environment variables expand inside path values; unknown variables are a validation error.
- All paths (roots, command executables) are normalized to absolute form.

## Notes

- No credentials or tokens belong in this file — nothing here may authenticate to a remote.
- An example file ships at `configs/config.example.toml`.
- Sort and filter selections made in the TUI persist to the registry, not this file.
