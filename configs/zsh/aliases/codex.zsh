# Codex (OpenAI) – short alias with bypass when binary present
if (( $+commands[codex] )); then
    alias cy='codex --dangerously-bypass-approvals-and-sandbox'
fi
