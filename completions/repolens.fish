# RepoLens fish completions.
# Install: cp completions/repolens.fish ~/.config/fish/completions/

set -l subcommands scan add remove list status open pick config version help

complete -c repolens -f
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a scan -d "Discover and register repositories"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a add -d "Register one repository"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a remove -d "Unregister (keeps files)"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a list -d "List registered repositories"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a status -d "Show repository status"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a open -d "Open repo in configured tool"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a pick -d "Print matching repo path"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a config -d "Manage configuration"
complete -c repolens -n "not __fish_seen_subcommand_from $subcommands" -a version -d "Print version"

complete -c repolens -l config -d "Alternate config file" -r
complete -c repolens -l no-color -d "Disable ANSI colors"
complete -c repolens -n "__fish_seen_subcommand_from list status" -l json -d "Machine-readable output"
complete -c repolens -n "__fish_seen_subcommand_from open" -l tool -xa "editor terminal file_manager git_tui worktree_manager" -d "Tool to launch"
complete -c repolens -n "__fish_seen_subcommand_from config" -xa "init path" -d "Action"

# Path-taking subcommands complete directories + registered repos.
complete -c repolens -n "__fish_seen_subcommand_from add remove status open pick" -a "(repolens list 2>/dev/null)" -d "Registered"
