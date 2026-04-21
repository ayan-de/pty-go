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
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
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

	for _, agentName := range cfg.Agents[1:] {
		var target string
		switch cfg.Layout {
		case "pane":
			if err := tmuxCmd("split-window", "-t", cfg.SessionName, "-h"); err != nil {
				return fmt.Errorf("failed to split window: %w", err)
			}
			tmuxCmd("select-layout", "-t", cfg.SessionName, "even-horizontal")
			target = cfg.SessionName
		case "win":
			if err := tmuxCmd("new-window", "-t", cfg.SessionName, "-n", agentName); err != nil {
				return fmt.Errorf("failed to create window: %w", err)
			}
			target = cfg.SessionName
		}

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
