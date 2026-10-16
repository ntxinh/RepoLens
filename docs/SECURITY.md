# Security

RepoLens is a read-only local tool, but it displays and acts on attacker-controlled content: repository names, branch names, commit messages, and file paths can come from any cloned repo.

## Subprocess safety

- `git` is invoked with argument arrays (`exec.CommandContext`), never a shell string. Nothing user-controlled is interpolated into a command.
- Only read-only Git verbs are allowed: `status`, `rev-parse`, `rev-list`, `log`, `diff`, `for-each-ref`, `branch --show-current`. Mutating verbs (`fetch`, `pull`, `push`, `commit`, `checkout`, `reset`, `clean`, `stash`, `merge`, `rebase`, `gc`, `worktree`) are never issued.
- External tools (editor, terminal, file manager, worktree manager) launch from configured executable + arg arrays with an explicit `{path}` placeholder. No shell, no implicit interpolation.
- Repo-provided hooks, build scripts, and project-defined commands are never executed.

## Terminal rendering

- Control characters in repo-derived strings are stripped or escaped before display to prevent terminal escape-sequence injection.
- Filenames are parsed from NUL-delimited porcelain output, not line-split human output — spaces, tabs, newlines, and non-ASCII are handled.

## Local-only guarantees

- No network access: no fetch/pull/push, no remote API calls in the MVP. GitHub CLI integration is a documented optional later phase.
- No telemetry, no account, no daemon left running after exit.
- Config files store no credentials. Credential files in repositories are never read during discovery.

## Logging

- Diagnostics go to `slog` under `${XDG_STATE_HOME}/repolens/` with path + operation context.
- Logs never include full environment dumps, secrets, or credential-bearing URLs.

## Reporting

Report vulnerabilities via GitHub private security advisory on [ntxinh/RepoLens](https://github.com/ntxinh/RepoLens). Do not open a public issue for exploitable problems.
