# =============================================================================
# Tmux Session Management for Fish Shell
# =============================================================================
# Automatic tmux session management and cleanup
# Based on your existing tmux session management logic
# =============================================================================

# Only run in interactive mode and not in WarpTerminal
if status is-interactive
    
    # Skip tmux auto-attach in WarpTerminal (it has its own session management)
    if test "$TERM_PROGRAM" != "WarpTerminal"
        
        # Function to cleanup orphaned tmux sessions
        function cleanup_sessions
            # Get list of all tmux sessions
            for session in (tmux list-sessions -F "#S" 2>/dev/null)
                # Get clients for this session
                set -l clients (tmux list-clients -t "$session" 2>/dev/null)
                
                # If session is not "Main" and has no clients, kill it
                if test "$session" != "Main" -a (count $clients) -eq 0
                    tmux kill-session -t "$session" 2>/dev/null
                end
            end
        end
        
        # Function to get the next available session name
        function get_next_session_name
            set -l base_name "Session"
            set -l index 1
            
            # Find the next available session name
            while tmux has-session -t "$base_name$index" 2>/dev/null
                set index (math $index + 1)
            end
            
            echo "$base_name$index"
        end
        
        # Main tmux management logic
        # Only run if tmux is available and we're not already in tmux/screen
        if command -q tmux; and not string match -rq "screen" $TERM; and not string match -rq "tmux" $TERM
            
            # Clean up orphaned sessions first
            cleanup_sessions
            
            # Check if "Main" session exists
            if tmux has-session -t "Main" 2>/dev/null
                # Get clients connected to Main session
                set -l clients (tmux list-clients -t "Main" 2>/dev/null)
                
                # If Main session has clients, create a new session
                if test (count $clients) -gt 0
                    set -l session_name (get_next_session_name)
                    tmux new-session -d -s "$session_name"
                    tmux attach-session -t "$session_name"
                else
                    # Main session exists but has no clients, attach to it
                    tmux attach-session -t "Main"
                end
            else
                # Main session doesn't exist, create it
                tmux new-session -s "Main" -d
                tmux attach-session -t "Main"
            end
        end
    end
end

# =============================================================================
# Tmux Utility Functions
# =============================================================================

# Function to create a new tmux session with a specific name
function tmux_new_session
    set -l session_name $argv[1]
    
    if test -z "$session_name"
        echo "Usage: tmux_new_session <session_name>"
        return 1
    end
    
    if tmux has-session -t "$session_name" 2>/dev/null
        echo "Session '$session_name' already exists. Attaching..."
        tmux attach-session -t "$session_name"
    else
        echo "Creating new session '$session_name'..."
        tmux new-session -s "$session_name"
    end
end

# Function to kill all sessions except Main
function tmux_cleanup_all
    echo "Cleaning up all tmux sessions except 'Main'..."
    
    for session in (tmux list-sessions -F "#S" 2>/dev/null)
        if test "$session" != "Main"
            echo "Killing session: $session"
            tmux kill-session -t "$session" 2>/dev/null
        end
    end
    
    echo "Cleanup complete."
end

# Function to list all tmux sessions with client information
function tmux_status
    echo "Tmux Sessions Status:"
    echo "===================="
    
    for session in (tmux list-sessions -F "#S" 2>/dev/null)
        set -l clients (tmux list-clients -t "$session" 2>/dev/null | wc -l)
        set -l windows (tmux list-windows -t "$session" 2>/dev/null | wc -l)
        
        echo "Session: $session"
        echo "  Clients: $clients"
        echo "  Windows: $windows"
        echo
    end
end

# Function to quickly switch to Main session
function tmux_main
    if tmux has-session -t "Main" 2>/dev/null
        tmux attach-session -t "Main"
    else
        echo "Main session doesn't exist. Creating..."
        tmux new-session -s "Main"
    end
end

# Function to detach from current tmux session
function tmux_detach
    if test -n "$TMUX"
        tmux detach-client
    else
        echo "Not currently in a tmux session."
    end
end

# =============================================================================
# Tmux Aliases and Abbreviations
# =============================================================================

# Only set up if tmux is available
if command -q tmux
    # Quick session management
    alias tns "tmux_new_session"
    alias tca "tmux_cleanup_all"
    alias tst "tmux_status"
    alias tm "tmux_main"
    alias td "tmux_detach"
    
    # Quick tmux commands
    alias tls "tmux list-sessions"
    alias tlw "tmux list-windows"
    alias tsp "tmux split-window"
    alias tsh "tmux split-window -h"
    alias tsv "tmux split-window -v"
end

# =============================================================================
# Tmux Environment Detection
# =============================================================================

# Set environment variable to indicate we're using custom tmux management
if command -q tmux
    set -gx CUSTOM_TMUX_MGMT 1
end

# Function to check if we're in tmux
function in_tmux
    test -n "$TMUX"
end

# Function to get current tmux session name
function current_tmux_session
    if in_tmux
        tmux display-message -p '#S'
    else
        echo "Not in tmux session"
        return 1
    end
end