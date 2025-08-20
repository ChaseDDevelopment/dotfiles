# =============================================================================
# Tmux Nested Session Prevention and Management
# =============================================================================
# Prevents nested tmux sessions and provides helpful session management
# =============================================================================

# Override tmux command to prevent nesting
function tmux --description 'Prevent nested tmux sessions'
    if set -q TMUX
        echo "⚠️  Already in a tmux session!"
        echo "   Use 'Ctrl+Space d' to detach from current session"
        echo "   Or run 'command tmux' to force nesting"
        return 1
    end
    command tmux $argv
end

# Smart attach function
function ta --description 'Attach to tmux session or create new one'
    if set -q TMUX
        echo "Already in tmux session"
        return 1
    end
    
    # Attach to first available session or create new one
    if command tmux list-sessions >/dev/null 2>&1
        command tmux attach
    else
        command tmux new-session -s main
    end
end

# List tmux sessions (works inside tmux)
function tls --description 'List tmux sessions'
    command tmux list-sessions 2>/dev/null || echo "No tmux sessions"
end

# Create new named session
function tn --description 'Create new tmux session with name'
    if set -q TMUX
        echo "Already in tmux session"
        return 1
    end
    
    if test (count $argv) -eq 0
        echo "Usage: tn <session-name>"
        return 1
    end
    
    command tmux new-session -s $argv[1]
end

# Kill tmux session by name
function tk --description 'Kill tmux session by name'
    if test (count $argv) -eq 0
        echo "Usage: tk <session-name>"
        echo "Available sessions:"
        command tmux list-sessions 2>/dev/null || echo "No sessions"
        return 1
    end
    
    command tmux kill-session -t $argv[1]
end