# pty-go

A Go CLI tool that launches coding agents inside a pseudo-terminal (PTY) with support for auto-prompt injection and auto-exit on task completion.

## Supported Agents

| Agent | Binary | Flag | Ready Pattern | Input Method |
|-------|--------|------|---------------|-------------|
| [opencode](https://github.com/anomalyco/opencode) | `opencode` | `-opencode` | `Ask anything` | Typed with Ctrl+U/W clearing |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `claude` | `-claudecode` / `-claude` | `Press Ctrl-C again to exit` | Single-line paste (no control chars) |
| [Codex CLI](https://github.com/openai/codex) | `codex` | `-codex` | `›` | Single-line paste (no control chars) |

## Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- At least one installed coding agent (see table above)

## Install

```bash
git clone https://github.com/<your-username>/pty-go.git
cd pty-go
go build -o pty-go .
```

Optionally move the binary to your PATH:

```bash
sudo mv pty-go /usr/local/bin/
```

## Usage

```bash
pty-go [flags] [prompt...]
```

### Flags

| Flag | Description |
|------|-------------|
| `-opencode` | Use opencode as the agent (default) |
| `-claudecode` / `-claude` | Use Claude Code as the agent |
| `-codex` | Use OpenAI Codex CLI as the agent |
| `-chdir <path>` | Set the working directory for the agent |
| `-auto-exit` | Automatically exit when the agent finishes the task |

### Examples

**Interactive session with opencode:**

```bash
pty-go
```

**Send a prompt and auto-exit when done:**

```bash
pty-go -auto-exit "Explain the main function in main.go"
```

**Use Claude Code in a specific project directory:**

```bash
pty-go -claudecode -chdir ~/Projects/my-app -auto-exit "Fix the lint errors"
```

**Use Codex CLI:**

```bash
pty-go -codex -chdir ~/Projects/my-app -auto-exit "Implement the missing API handler"
```

**Multi-word prompt:**

```bash
pty-go -opencode -auto-exit Refactor the database layer to use connection pooling
```

## How It Works

1. Launches the agent binary inside a PTY
2. Waits for the agent's ready indicator (e.g., `Ask anything` for opencode)
3. Types the prompt into the PTY
4. With `-auto-exit`, monitors output for a completion marker or idle patterns, then shuts down the agent

## License

MIT
