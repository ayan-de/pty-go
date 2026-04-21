# Multi-Agent tmux Mode Design

## Summary

Add `-all` flag to pty-go that spawns multiple coding agents (opencode, Claude Code, Codex CLI) inside a tmux session, each running in its own pane or window with the same prompt injected.

## CLI Interface

```
pty-go -all -pane -session=mytask -auto-exit "Refactor the database layer"
pty-go -all -win -session=mytask -auto-exit "Refactor the database layer"
```

### New Flags

| Flag | Description |
|------|-------------|
| `-all` | Enable multi-agent mode (opencode, claudecode, codex) |
| `-pane` | Horizontal split layout (panes in one window) |
| `-win` | Separate tmux windows (Ctrl+b c style) |
| `-session <name>` | Tmux session name (required with `-all`) |

### Flag Rules

- `-pane` and `-win` are mutually exclusive. Error if both or neither is specified with `-all`.
- `-session` is required when `-all` is used.
- Existing flags (`-auto-exit`, `-chdir`) work in multi-agent mode and are passed through to each inner process.

## Architecture: Recursive Self-Spawning

When `-all` is detected, `main()` delegates to `runMultiAgent()` in `tmux.go`:

1. **Create detached tmux session**: `tmux new-session -d -s <name> -x 220 -y 50`
2. **Spawn pty-go inside tmux** per agent:
   - `-pane` mode: `tmux split-window -h` for each additional agent, then `tmux send-keys` to launch `pty-go -<agent> -auto-exit "<prompt>"`
   - `-win` mode: `tmux new-window -n <agent>` then `tmux send-keys` to launch the same
3. **Attach**: `exec tmux attach -t <name>` — replaces the outer pty-go process

The inner pty-go processes handle prompt injection, ready detection, and auto-exit independently using the existing single-agent codepath. When each agent finishes, its pane/window closes. When all close, the tmux session ends.

## Key Decisions

- **No completion monitoring from outer process**: Each inner pty-go handles its own `-auto-exit`. Simpler and more robust than tracking child processes.
- **No tmux wrapper library**: Plain `os/exec.Command("tmux", ...)`. Fewer dependencies.
- **Recursive design**: Inner processes run the exact same binary with single-agent flags. No code duplication.

## Files

| File | Change |
|------|--------|
| `main.go` | Add `-all`, `-pane`, `-win`, `-session` flags. Add multi-agent dispatch path. |
| `tmux.go` (new) | `runMultiAgent()` — tmux session creation, layout, agent spawning, attach. |

No changes to agent configs, prompt helpers, or single-agent flow.
