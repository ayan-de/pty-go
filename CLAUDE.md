# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
go build -o pty-go .
```

Run with a specific agent:
```bash
./pty-go -opencode "your prompt"
./pty-go -claudecode -chdir ~/path/to/project -auto-exit "your prompt"
./pty-go -codex "your prompt"
```

## Architecture

The codebase is a small CLI tool (~400 LOC total) that launches coding agents (opencode, Claude Code, Codex CLI) inside a pseudo-terminal with automatic prompt injection and optional auto-exit.

### Core State Machine (`main.go`)

The main loop implements a state machine with four states:
1. `stateWaitingReady` - Waits for agent's ready pattern (regex match on PTY output)
2. `stateSendingPrompt` - Transitions immediately after ready pattern detected
3. `stateWorking` - Monitors for completion markers after a grace period
4. `stateDone` - Triggers agent shutdown

Key state transition mechanics:
- **Ready detection**: Uses agent-specific regex pattern to detect when agent is ready
- **Fallback timer**: If ready pattern never matches, sends prompt after `FallbackTimeout`
- **Grace period**: Waits `GracePeriod` after sending prompt before checking for completion
- **Completion detection**: Either `doneMarker` literal (`P0MX_DONE_SIGNAL`) or 3+ matches of idle pattern regex

### Agent Configuration (`agent.go`)

`Config` struct defines per-agent behavior:
- `ReadyPattern`: Regex indicating agent is ready for input
- `SendPrompt`: Function to write prompt to PTY (typed vs single-line)
- `FormatPrompt`: Wraps user prompt with done marker instruction
- `IdlePatterns`: Regexes that, when matched repeatedly, indicate agent is idle
- `GracePeriod`: Minimum time to wait before checking for completion
- `FallbackTimeout`: Maximum time to wait for ready pattern before forcing send
- `ReadyWait`: Delay after ready pattern before sending prompt (for UI stability)

### Adding a New Agent

1. Create new file (e.g., `newagent.go`) with `New[AgentName]() *Config` function
2. Register in `NewRegistry()` in `agent.go`
3. Add CLI flag parsing in `main.go` flag loop
4. Determine:
   - Ready pattern (agent startup message)
   - Prompt input method (single-line paste or typed with control chars)
   - Idle/completion patterns (what does agent print when done?)

### Prompt Sending Methods

- `SendPromptTyped`: Sends Ctrl+U/Ctrl+W to clear input, types char-by-char, sends Enter. For agents with full readline (opencode).
- `SendPromptSingleLine`: Collapses newlines to spaces, sends as single line. For agents with limited input handling (claude-code, gemini, codex).

### PTY Handling

- Uses `github.com/creack/pty` for pseudo-terminal
- `golang.org/x/term` for raw mode on stdin
- SIGWINCH handling for terminal resize propagation
- TeeReader copies PTY output to stdout while buffering for pattern matching

### Done Marker

The string `P0MX_DONE_SIGNAL` is injected into prompts and used as a literal completion marker. Agents must be instructed to print this exact string when done.
