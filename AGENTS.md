# AGENTS.md — pty-go

## Project Overview

pty-go is a Go CLI tool that launches coding agents (opencode, Claude Code, Gemini CLI, Codex CLI) inside a pseudo-terminal (PTY). It supports auto-prompt injection and auto-exit on task completion. Multi-agent mode spawns all agents inside a tmux session. Single `package main`, no subpackages.

## Build / Run / Test Commands

```bash
# Build
go build -o pty-go .

# Run
./pty-go -opencode -auto-exit "your prompt here"

# Run multi-agent (tmux)
./pty-go -all -pane -session=mytask -auto-exit "your prompt here"
./pty-go -all -win -session=mytask -auto-exit "your prompt here"

# Run all tests (none exist yet, but use this when they do)
go test ./...

# Run a single test
go test -run TestFunctionName .

# Run a single test with verbose output
go test -v -run TestFunctionName .

# Vet (static analysis)
go vet ./...

# Format
gofmt -w .
```

There is no Makefile, no golangci-lint config, and no test files yet. Use `go vet` and `gofmt` as the primary linting/formatting checks.

## Code Style Guidelines

### Package Structure

- Everything is in `package main` — no sub-packages.
- One file per agent config: `<agent>.go` (e.g., `claudecode.go`, `codex.go`, `opencode.go`, `geminicode.go`).
- Shared types and helpers go in `agent.go`.
- Entry point and main logic in `main.go`.
- Multi-agent tmux orchestration goes in `tmux.go`.

### Imports

- Use grouped imports with a single `import (` block, sorted by stdlib then third-party, separated by a blank line:

```go
import (
    "bytes"
    "fmt"
    "os"
    "time"

    "github.com/creack/pty"
)
```

- For single-import files, use the form `import "time"` (no parentheses) — see agent config files.

### Formatting

- Use `gofmt` / `go fmt` — tabs for indentation, no trailing whitespace.
- No line length enforcement (Go convention).

### Types and Naming

- **Exported types/functions**: PascalCase (`Config`, `NewRegistry`, `SendPromptTyped`).
- **Unexported types/functions**: camelCase (`doneMarker`, `stripANSI`, `ansiRe`).
- **Constructors**: `New` prefix (`NewClaudeCode()`, `NewCodex()`, `NewOpenCode()`, `NewGeminiCode()`, `NewRegistry()`).
- **Constants**: camelCase for unexported (`doneMarker`), PascalCase for exported.
- **State enums**: Use `type state int` with `iota` const block.
- **Regex variables**: Suffix `Re` (e.g., `ansiRe`, `doneMarkerRe`, `idleRes`).
- **File naming**: lowercase, no underscores (e.g., `claudecode.go`, not `claude_code.go`).

### Error Handling

- Fatal errors in `main` use `panic(err)` — this is a CLI tool, not a library.
- Non-critical errors in goroutines are silently ignored with `_` (e.g., `pty.GetsizeFull`, `pty.Setsize`).
- Library functions that can fail should still be checked; do not swallow errors in business logic.

### Concurrency Patterns

- Use `sync.Once` for one-time cleanup actions (see `exit()` in `main.go`).
- Use `chan struct{}` for signaling between goroutines (e.g., `cmdDone`, `stdinEnabled`).
- Use `time.AfterFunc` for delayed/fallback actions — store the timer and call `Stop()` with `defer` if applicable.
- Goroutines for I/O: stdin copy, signal handling, process wait — each in its own goroutine.

### Agent Config Convention

Each agent is defined as a `New*()` function returning `*Config`:

```go
func NewAgentName() *Config {
    return &Config{
        Name:            "display-name",
        Bin:             "binary-name",
        Args:            []string{"--flags"},
        ReadyPattern:    `regex\s+pattern`,
        SendPrompt:      SendPromptTyped,      // or SendPromptSingleLine
        GracePeriod:     10 * time.Second,
        FallbackTimeout: 10 * time.Second,
        ReadyWait:       2 * time.Second,
        FormatPrompt:    DefaultFormatPrompt,   // or ClaudeFormatPrompt
        IdlePatterns:    []string{`regex\s+pattern`},
    }
}
```

- `ReadyPattern`: regex matched against ANSI-stripped output to detect the agent is ready for input.
- `IdlePatterns`: regex list — if any pattern matches 3+ times in recent output, the agent is considered idle/done.
- `SendPrompt`: use `SendPromptTyped` for agents needing Ctrl+U/W clearing before typing; use `SendPromptSingleLine` for agents that accept single-line paste.

### PTY / Terminal Handling

- Always `defer ptmx.Close()` and `defer term.Restore(...)`.
- Window resize: relay `SIGWINCH` signals to the PTY via `pty.Setsize`.
- Control bytes: `\x15` (Ctrl+U), `\x17` (Ctrl+W), `\x03` (Ctrl+C), `\x0d` (Enter/Return).

### Adding a New Agent

1. Create `<agentname>.go` with a `New<AgentName>() *Config` function.
2. Add an entry to `NewRegistry()` in `agent.go`.
3. Add a CLI flag case in `main.go`'s argument parser.
4. Follow the config convention above with appropriate patterns and timeouts.
