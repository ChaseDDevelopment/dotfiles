# Auto-install/refresh completions for the `codex` CLI
# Loads on every Fish startup via conf.d

# Guard: only if codex exists
type -q codex; or return

set -l target ~/.config/fish/completions/codex.fish

# Helper: get file mtime (GNU/BSD compat)
function __mtime
    # GNU stat
    if command -q stat
        stat --version 2>/dev/null 1>/dev/null
        if test $status -eq 0
            # GNU coreutils present
            stat -c %Y $argv
            return
        end
    end
    # BSD/macOS stat
    stat -f %m $argv
end

# Helper: write fish completions using whatever Codex supports
function __codex_write_fish_completions
    set -l outfile $argv[1]

    # Prefer known subcommands; silence errors
    if codex completions fish 2>/dev/null > $outfile
        return 0
    end
    if codex completions --shell fish 2>/dev/null > $outfile
        return 0
    end
    if codex --completions fish 2>/dev/null > $outfile
        return 0
    end
    if codex generate-completions fish 2>/dev/null > $outfile
        return 0
    end
    if codex gen-completions fish 2>/dev/null > $outfile
        return 0
    end

    return 1
end

# Determine if we should (re)generate:
# - missing file, or
# - older than 14 days, or
# - env var forces an update
set -l need_update 0
if not test -f $target
    set need_update 1
else
    set -l now (date +%s)
    set -l mtime (__mtime $target 2>/dev/null)
    if test -z "$mtime"
        set need_update 1
    else if test (math $now - $mtime) -gt 1209600
        set need_update 1
    end
end

if set -q CODEX_COMPLETIONS_FORCE
    set need_update 1
end

if test $need_update -eq 1
    mkdir -p ~/.config/fish/completions
    # Write to a temp file then move atomically
    set -l tmp (mktemp)
    if __codex_write_fish_completions $tmp
        mv $tmp $target
    else
        rm -f $tmp
        # Don’t block shell if this fails
    end
end

functions -q __update_codex_completions; or function __update_codex_completions --description "Manually refresh codex completions"
    set -x CODEX_COMPLETIONS_FORCE 1
    source (status filename)
    set -e CODEX_COMPLETIONS_FORCE
    echo "codex completions refreshed → $target"
end
