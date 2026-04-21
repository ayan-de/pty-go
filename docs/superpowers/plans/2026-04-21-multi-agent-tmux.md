# Multi-Agent tmux Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `-all` flag that spawns opencode, claudecode, and codex inside a tmux session with prompt injection and auto-exit.

**Architecture:** Recursive self-spawning. When `-all` is set, pty-go creates a tmux session and spawns itself (single-agent mode) inside each pane/window. The outer process attaches to the session and exits. Inner processes handle prompt injection and auto-exit independently.

**Tech Stack:** Go, tmux CLI (via os/exec), github.com/creack/pty

---

### Task 1: Create tmux.go with runMultiAgent()

**Files:**
- Create: `tmux.go`

- [ ] **Step 1: Create tmux.go with runMultiAgent function**

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
)

type multiAgentConfig struct {
	SessionName string
	Layout      string
	Agents      []string
	Prompt      string
	Chdir       string
	AutoExit    bool
}

func tmuxCmd(args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func tmuxOutput(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func buildAgentArgs(multiCfg *multiAgentConfig, agentName string) []string {
	args := []string{"-" + agentName}
	if multiCfg.AutoExit {
		args = append(args, "-auto-exit")
	}
	if multiCfg.Chdir != "" {
		args = append(args, "-chdir", multiCfg.Chdir)
	}
	args = append(args, multiCfg.Prompt)
	return args
}

func runMultiAgent(cfg *multiAgentConfig) error {
	if cfg.SessionName == "" {
		return fmt.Errorf("-session is required with -all")
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux is required for -all mode: %w", err)
	}

	existing, _ := tmuxOutput("has-session", "-t", cfg.SessionName)
	if existing == "" {
		return fmt.Errorf("tmux session %q already exists", cfg.SessionName)
	}

	if err := tmuxCmd("new-session", "-d", "-s", cfg.SessionName, "-x", "220", "-y", "50"); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	firstAgent := cfg.Agents[0]
	firstArgs := buildAgentArgs(cfg, firstAgent)
	if err := tmuxCmd("send-keys", "-t", cfg.SessionName, self, fmt.Sprintf("%q", firstArgs)); err != nil {
		return fmt.Errorf("failed to send command for %s: %w", firstAgent, err)
	}
	if err := tmuxCmd("send-keys", "-t", cfg.SessionName, "Enter"); err != nil {
		return fmt.Errorf("failed to press enter for %s: %w", firstAgent, err)
	}

	for i, agentName := range cfg.Agents[1:] {
		switch cfg.Layout {
		case "pane":
			if err := tmuxCmd("split-window", "-t", cfg.SessionName, "-h"); err != nil {
				return fmt.Errorf("failed to split window: %w", err)
			}
		case "win":
			if err := tmuxCmd("new-window", "-t", cfg.SessionName, "-n", agentName); err != nil {
				return fmt.Errorf("failed to create window: %w", err)
			}
		}

		target := fmt.Sprintf("%s:%d", cfg.SessionName, i+1)
		agentArgs := buildAgentArgs(cfg, agentName)
		if err := tmuxCmd("send-keys", "-t", target, self, fmt.Sprintf("%q", agentArgs)); err != nil {
			return fmt.Errorf("failed to send command for %s: %w", agentName, err)
		}
		if err := tmuxCmd("send-keys", "-t", target, "Enter"); err != nil {
			return fmt.Errorf("failed to press enter for %s: %w", agentName, err)
		}
	}

	if cfg.Layout == "pane" {
		tmuxCmd("select-layout", "-t", cfg.SessionName, "even-horizontal")
	}

	tmuxCmd("select-pane", "-t", cfg.SessionName+":0")

	attachCmd := exec.Command("tmux", "attach", "-t", cfg.SessionName)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr
	return attachCmd.Run()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build -o pty-go .`
Expected: success

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: Commit**

```bash
git add tmux.go
git commit -m "feat: add tmux.go with runMultiAgent for multi-agent mode"
```

---

### Task 2: Update main.go with new flags and dispatch

**Files:**
- Modify: `main.go:29-53` (flag parsing)
- Modify: `main.go:28` (add multiAgentConfig variable)

- [ ] **Step 1: Add new flag variables and parsing in main()**

Replace the flag parsing block in `main.go` (lines 29-53) with:

```go
	var chdir string
	var autoExit bool
	var agentName string
	var allMode bool
	var paneMode bool
	var winMode bool
	var sessionName string
	var args []string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-chdir":
			i++
			if i < len(os.Args) {
				chdir = os.Args[i]
			}
		case "-auto-exit":
			autoExit = true
		case "-all":
			allMode = true
		case "-pane":
			paneMode = true
		case "-win":
			winMode = true
		case "-session":
			i++
			if i < len(os.Args) {
				sessionName = os.Args[i]
			}
		case "-codex":
			agentName = "codex"
		case "-opencode":
			agentName = "opencode"
		case "-claudecode", "-claude":
			agentName = "claudecode"
		case "-gemini":
			agentName = "gemini"
		default:
			args = append(args, os.Args[i])
		}
	}

	if allMode {
		if paneMode && winMode {
			os.Stderr.WriteString("error: -pane and -win are mutually exclusive\n")
			os.Exit(1)
		}
		if !paneMode && !winMode {
			os.Stderr.WriteString("error: specify -pane or -win with -all\n")
			os.Exit(1)
		}
		layout := "pane"
		if winMode {
			layout = "win"
		}
		prompt := JoinArgs(args)
		multiCfg := &multiAgentConfig{
			SessionName: sessionName,
			Layout:      layout,
			Agents:      []string{"opencode", "claudecode", "codex"},
			Prompt:      prompt,
			Chdir:       chdir,
			AutoExit:    autoExit,
		}
		if err := runMultiAgent(multiCfg); err != nil {
			os.Stderr.WriteString("error: " + err.Error() + "\n")
			os.Exit(1)
		}
		return
	}
```

This is inserted before the existing `if agentName == ""` block. The rest of `main()` stays unchanged.

- [ ] **Step 2: Verify it compiles**

Run: `go build -o pty-go .`
Expected: success

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: Run gofmt**

Run: `gofmt -w .`
Expected: no output (all formatted)

- [ ] **Step 5: Test the help/error cases**

Run: `./pty-go -all "test"`
Expected: error message about -pane or -win required

Run: `./pty-go -all -pane -win "test"`
Expected: error about mutual exclusivity

Run: `./pty-go -all -pane "test"`
Expected: error about -session required

- [ ] **Step 6: Commit**

```bash
git add main.go
git commit -m "feat: add -all, -pane, -win, -session flags for multi-agent mode"
```

---

### Task 3: Fix send-keys command construction in tmux.go

**Files:**
- Modify: `tmux.go`

The `send-keys` + `Enter` approach in Task 1 may not correctly pass arguments with spaces. We need to use a single `send-keys` call with the full command string followed by `C-m` (Enter).

- [ ] **Step 1: Replace send-keys logic with proper command construction**

In `tmux.go`, replace the `buildAgentArgs` helper and the send-keys calls. The full `tmux.go` should be:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type multiAgentConfig struct {
	SessionName string
	Layout      string
	Agents      []string
	Prompt      string
	Chdir       string
	AutoExit    bool
}

func tmuxCmd(args ...string) error {
	cmd := exec.Command("tmux", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func tmuxHasSession(name string) bool {
	err := exec.Command("tmux", "has-session", "-t", name).Run()
	return err == nil
}

func buildCommand(self string, cfg *multiAgentConfig, agentName string) string {
	parts := []string{self, "-" + agentName}
	if cfg.AutoExit {
		parts = append(parts, "-auto-exit")
	}
	if cfg.Chdir != "" {
		parts = append(parts, "-chdir", cfg.Chdir)
	}
	parts = append(parts, cfg.Prompt)
	for i, p := range parts {
		if strings.Contains(p, " ") || strings.Contains(p, "\"") {
			parts[i] = "'" + strings.ReplaceAll(p, "'", "'\\''") + "'"
		}
	}
	return strings.Join(parts, " ")
}

func runMultiAgent(cfg *multiAgentConfig) error {
	if cfg.SessionName == "" {
		return fmt.Errorf("-session is required with -all")
	}

	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux is required for -all mode: %w", err)
	}

	if tmuxHasSession(cfg.SessionName) {
		return fmt.Errorf("tmux session %q already exists", cfg.SessionName)
	}

	if err := tmuxCmd("new-session", "-d", "-s", cfg.SessionName, "-x", "220", "-y", "50"); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	firstCmd := buildCommand(self, cfg, cfg.Agents[0])
	if err := tmuxCmd("send-keys", "-t", cfg.SessionName, firstCmd, "C-m"); err != nil {
		return fmt.Errorf("failed to start %s: %w", cfg.Agents[0], err)
	}

	for i, agentName := range cfg.Agents[1:] {
		switch cfg.Layout {
		case "pane":
			if err := tmuxCmd("split-window", "-t", cfg.SessionName, "-h"); err != nil {
				return fmt.Errorf("failed to split window: %w", err)
			}
		case "win":
			if err := tmuxCmd("new-window", "-t", cfg.SessionName, "-n", agentName); err != nil {
				return fmt.Errorf("failed to create window: %w", err)
			}
		}

		target := fmt.Sprintf("%s:%d", cfg.SessionName, i+1)
		cmd := buildCommand(self, cfg, agentName)
		if err := tmuxCmd("send-keys", "-t", target, cmd, "C-m"); err != nil {
			return fmt.Errorf("failed to start %s: %w", agentName, err)
		}
	}

	if cfg.Layout == "pane" {
		tmuxCmd("select-layout", "-t", cfg.SessionName, "even-horizontal")
	}

	tmuxCmd("select-pane", "-t", cfg.SessionName+":0")

	attachCmd := exec.Command("tmux", "attach", "-t", cfg.SessionName)
	attachCmd.Stdin = os.Stdin
	attachCmd.Stdout = os.Stdout
	attachCmd.Stderr = os.Stderr
	return attachCmd.Run()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build -o pty-go .`
Expected: success

- [ ] **Step 3: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 4: Run gofmt**

Run: `gofmt -w .`

- [ ] **Step 5: Commit**

```bash
git add tmux.go
git commit -m "fix: use proper shell-escaped send-keys for tmux command injection"
```

---

### Task 4: Update AGENTS.md

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: Add tmux.go to file structure docs and new flags to run docs**

Add `tmux.go` to the package structure section and document the new flags in the Run section.

- [ ] **Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs: document multi-agent tmux mode in AGENTS.md"
```
