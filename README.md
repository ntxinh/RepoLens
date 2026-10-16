# RepoLens

A fast, local-first dashboard for inspecting the state of many Git repositories from one terminal interface.

RepoLens is an **observability and navigation tool**. It does not create, delete, or mutate worktrees, and it never modifies your repositories.

## Features

- One row per repository or worktree: branch, upstream, ahead/behind, staged/unstaged/untracked/conflicted counts, last commit
- Attention indicators for conflicts, dirty trees, detached HEAD, stale data, and errors — no ambiguous single-color status
- Interactive search and built-in filters (dirty, ahead, behind, conflicted, detached, error, recently active)
- Read-only repository detail view with recent commits, changed files, and diffs
- Asynchronous scanning — the UI stays responsive across hundreds of repositories
- Configurable launch actions: editor, terminal, file manager, Git TUI, worktree manager
- No network, no account, no daemon, no telemetry

Full spec: [docs/repolens-spec.md](docs/repolens-spec.md). Architecture: [DESIGN.md](DESIGN.md).

## Requirements

- Linux (developed on Fedora 44 / Wayland)
- `git` on `PATH`
- Any standards-compliant terminal emulator

## Install

### Release binary

Download `repolens-linux-amd64` (or `-arm64`) and `sha256sums.txt` from the
[latest release](https://github.com/ntxinh/RepoLens/releases):

```sh
sha256sum -c sha256sums.txt --ignore-missing
install -m 0755 repolens-linux-amd64 ~/.local/bin/repolens
```

### From source

```sh
git clone https://github.com/ntxinh/RepoLens.git
cd RepoLens
mise install          # installs the pinned Go toolchain
make install          # installs to GOPATH/bin
```

Or directly: `go install github.com/ntxinh/RepoLens/cmd/repolens@latest`

### Uninstall

```sh
rm ~/.local/bin/repolens                    # the binary
rm -rf ~/.config/repolens                   # configuration
rm -rf ~/.local/share/repolens              # registry
rm -rf ~/.cache/repolens ~/.local/state/repolens
# plus the fish files you copied manually (see Fish integration)
```

## Quick start

```sh
repolens config init            # write a default config
repolens scan ~/Projects        # discover repos under a root
repolens                        # launch the TUI
```

Configuration lives at `${XDG_CONFIG_HOME:-~/.config}/repolens/config.toml`. See [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## Default keybindings

| Key | Action |
|---|---|
| `j` / `k` or arrows | Move selection |
| `Enter` | Repository details |
| `/` | Search / filter |
| `a` | Show repositories requiring attention |
| `d` | Toggle dirty-only filter |
| `r` | Refresh selected / all |
| `s` | Cycle sort mode |
| `o` | Open in configured editor |
| `t` | Open terminal at path |
| `g` | Open configured Git TUI |
| `c` | Copy repository path |
| `?` | Keybinding help |
| `Esc` / `q` | Close panel / go back / quit |

## Limitations

- **Read-only.** No staging, committing, fetching, pulling, pushing, cleaning, or stashing.
- Ahead/behind uses locally available tracking refs only — RepoLens never fetches on your behalf.
- Worktree lifecycle operations belong to your existing worktree manager; RepoLens launches it, it doesn't replace it.

## Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md). `make help` lists the common targets.

## Fish integration

Optional — install manually, RepoLens never touches your fish config:

```sh
cp fish/repolens.fish ~/.config/fish/functions/       # provides croot
cp completions/repolens.fish ~/.config/fish/completions/
```

`croot <query>` `cd`s into the unique registered repository matching the
query (basename or substring). `croot` alone lists candidates.

## License

[MIT](LICENSE)
