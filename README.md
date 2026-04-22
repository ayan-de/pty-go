# pty-go

A Go CLI tool that launches coding agents inside a pseudo-terminal (PTY) with support for auto-prompt injection and auto-exit on task completion. Supports multi-agent mode to run opencode, Claude Code, and Codex CLI simultaneously inside a tmux session.

## Supported Agents

| Agent | Binary | Flag | Ready Pattern | Input Method |
|-------|--------|------|---------------|-------------|
| [opencode](https://github.com/anomalyco/opencode) | `opencode` | `-opencode` | `Ask anything` | Typed with Ctrl+U/W clearing |
| [Claude Code](https://docs.anthropic.com/en/docs/claude-code) | `claude` | `-claudecode` / `-claude` | `Press Ctrl-C again to exit` | Single-line paste (no control chars) |
| [Gemini CLI](https://geminicli.com) | `gemini` | `-gemini` | `Gemini CLI` | Single-line paste (no control chars) |
| [Codex CLI](https://github.com/openai/codex) | `codex` | `-codex` | `›` | Single-line paste (no control chars) |

## Prerequisites

- [Go](https://go.dev/dl/) 1.26+
- At least one installed coding agent (see table above)
- [tmux](https://github.com/tmux/tmux) (required for multi-agent mode)

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
| `-gemini` | Use Gemini CLI as the agent |
| `-codex` | Use OpenAI Codex CLI as the agent |
| `-chdir <path>` | Set the working directory for the agent |
| `-auto-exit` | Automatically exit when the agent finishes the task |
| `-all` | Multi-agent mode — spawn opencode, Claude Code, and Codex CLI together |
| `-pane` | Split into horizontal panes (use with `-all`) |
| `-win` | Open each agent in a separate tmux window (use with `-all`) |
| `-session <name>` | Set the tmux session name (required with `-all`) |
| `-tmux [name]` | Spawn agent inside a tmux session (optional custom name) |

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

**Use Gemini CLI:**

```bash
pty-go -gemini -auto-exit "Write a unit test for the Registry function"
```

**Use Codex CLI:**

```bash
pty-go -codex -chdir ~/Projects/my-app -auto-exit "Implement the missing API handler"
```

**Multi-word prompt:**

```bash
pty-go -opencode -auto-exit Refactor the database layer to use connection pooling
```

**Spawn in tmux session:**

```bash
pty-go -tmux -opencode -auto-exit "Your prompt here"
```

## Tmux Mode

Spawn a single agent inside a dedicated tmux session.

```bash
pty-go -tmux [name] [-opencode|-claudecode|-gemini|-codex] [flags] "prompt"
```

If `name` is omitted, the session is named `pty-go-<agent>`.

### Examples

**opencode in tmux:**

```bash
pty-go -tmux -opencode -auto-exit "Implement a new feature"
```

**Claude Code in tmux with custom session name:**

```bash
pty-go -tmux mysession -claudecode -auto-exit "Review and refactor the auth module"
```

## Multi-Agent Mode

Spawn multiple coding agents in a tmux session. Each agent receives the same prompt and works independently. When all agents finish, the tmux session closes.

### Requirements

- `tmux` must be installed

### Usage

```bash
pty-go -all -pane -session=<name> [-auto-exit] [-chdir <path>] "your prompt"
pty-go -all -win -session=<name> [-auto-exit] [-chdir <path>] "your prompt"
```

### Flags

| Flag | Description |
|------|-------------|
| `-all` | Enable multi-agent mode (opencode, Claude Code, Codex CLI) |
| `-pane` | Split agents into horizontal panes in one window |
| `-win` | Open each agent in a separate tmux window |
| `-session <name>` | Name for the tmux session (required) |

`-pane` and `-win` are mutually exclusive. One must be specified with `-all`.

### Examples

**Three agents in horizontal panes:**

```bash
pty-go -all -pane -session=refactor -auto-exit "Refactor the database layer to use connection pooling"
```

**Three agents in separate tmux windows:**

```bash
pty-go -all -win -session=review -auto-exit -chdir ~/Projects/my-app "Review all recent changes"
```

**With a working directory:**

```bash
pty-go -all -pane -session=fix -auto-exit -chdir ~/Projects/api "Fix the lint errors in the handlers package"
```

## How It Works

1. Launches the agent binary inside a PTY
2. Waits for the agent's ready indicator (e.g., `Ask anything` for opencode)
3. Types the prompt into the PTY
4. With `-auto-exit`, monitors output for a completion marker or idle patterns, then shuts down the agent

### Multi-Agent Mode

1. Creates a detached tmux session
2. Spawns pty-go (in single-agent mode) inside each pane/window — one per agent
3. Each inner process independently handles ready detection, prompt injection, and auto-exit
4. Attaches to the tmux session so you can watch all agents work in real-time
5. When agents finish, their panes/windows close; when all close, the session ends

## License

MIT
