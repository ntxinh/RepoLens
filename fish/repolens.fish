# RepoLens fish functions.
# Install: cp fish/repolens.fish ~/.config/fish/functions/ (or conf.d/)
# RepoLens never modifies your fish config — install manually.

# croot: cd into a registered repository.
#   croot          → pick interactively via repolens TUI list (prints first match)
#   croot QUERY    → cd into the unique repo matching QUERY
function croot --description "cd into a registered repository"
    if not command -q repolens
        echo "croot: repolens not on PATH" >&2
        return 1
    end

    set -l path (repolens pick $argv 2>/dev/null)
    if test -z "$path"
        echo "croot: no unique match for '$argv'" >&2
        repolens pick $argv 2>&1 >&2
        return 1
    end
    if not test -d "$path"
        echo "croot: $path does not exist" >&2
        return 1
    end
    cd "$path"
end
